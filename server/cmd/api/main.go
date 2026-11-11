// Command api serves the Scrapbook/01 HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/aaugoustis/waitlist/server/db/migrations"
	"github.com/aaugoustis/waitlist/server/internal/ai"
	"github.com/aaugoustis/waitlist/server/internal/api"
	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/classify"
	"github.com/aaugoustis/waitlist/server/internal/config"
	"github.com/aaugoustis/waitlist/server/internal/credits"
	"github.com/aaugoustis/waitlist/server/internal/db"
	"github.com/aaugoustis/waitlist/server/internal/jobs"
	"github.com/aaugoustis/waitlist/server/internal/mcpserver"
	"github.com/aaugoustis/waitlist/server/internal/push"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("exiting", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if cfg.MigrateOnStart {
		if err := migrate(ctx, pool); err != nil {
			return err
		}
	}

	var verifier *auth.Verifier
	if cfg.SupabaseJWTSecret != "" {
		verifier = auth.NewHS256([]byte(cfg.SupabaseJWTSecret), cfg.Issuer())
		log.Info("auth: using legacy HS256 JWT secret")
	} else {
		verifier, err = auth.NewJWKS(ctx, cfg.JWKSURL(), cfg.Issuer())
		if err != nil {
			return err
		}
		log.Info("auth: using JWKS", "url", cfg.JWKSURL())
	}

	queries := db.New(pool)
	if cfg.MigrateOnStart {
		if err := jobs.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	ledger := &credits.Ledger{Q: queries, Starter: cfg.StarterCredits}
	classifier := newClassifier(cfg, queries, log)
	classifier.Credits = ledger
	queue, err := jobs.New(jobs.Options{
		Pool:       pool,
		Queries:    queries,
		Classifier: classifier,
		Logger:     log,
		RunWorkers: cfg.RunWorkers,
	})
	if err != nil {
		return fmt.Errorf("job queue: %w", err)
	}
	if err := queue.Start(ctx); err != nil {
		return fmt.Errorf("start workers: %w", err)
	}

	// The MCP endpoint (and its OAuth metadata) lives next to the app's API.
	mcpOpts := mcpserver.Options{
		Queries: queries, Verifier: verifier, Enqueuer: queue, Logger: log, Credits: ledger,
		PublicURL: cfg.PublicURL, AuthServer: cfg.Issuer(),
	}
	if classifier.Embed != nil {
		mcpOpts.Embed = classifier.Embed
	}
	if classifier.Jev != nil { // only with TYPESAFE_API_KEY set
		mcpOpts.Rerank = classifier.Jev
	}
	mcpHandler := mcpserver.New(mcpOpts).Handler()
	root := http.NewServeMux()
	root.Handle(mcpserver.Path, mcpHandler)
	root.Handle("/.well-known/", mcpHandler)
	root.Handle("/", api.New(api.Options{
		Queries:     queries,
		Logger:      log,
		Verifier:    verifier,
		Ping:        pool.Ping,
		CORSOrigins: cfg.CORSOrigins,
		Enqueuer:    queue,
		Credits:     ledger,
	}).Handler())

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	if qerr := queue.Stop(shutdownCtx); qerr != nil {
		log.Warn("stopping workers", "err", qerr)
	}
	return err
}

// newClassifier wires whichever AI services are configured. Missing keys
// turn the matching step off instead of failing the boot, so the API runs
// locally without any of them.
func newClassifier(cfg config.Config, q db.Querier, log *slog.Logger) *classify.Classifier {
	c := &classify.Classifier{
		Q: q, Log: log,
		Threshold: cfg.JevThreshold, SameName: 0.85,
		Neighbors: 5, MinSimilarity: 0.45,
	}
	if cfg.TypeSafeAPIKey != "" {
		c.Jev = &ai.Jev{BaseURL: cfg.TypeSafeBaseURL, APIKey: cfg.TypeSafeAPIKey, Model: cfg.JevModel,
			Price: ai.Price{Input: cfg.JevPrice}}
	} else {
		log.Warn("classify: TYPESAFE_API_KEY not set; items won't be matched to existing categories")
	}
	if cfg.EmbeddingsAPIKey != "" {
		c.Embed = &ai.Embedder{BaseURL: cfg.EmbeddingsBaseURL, APIKey: cfg.EmbeddingsAPIKey, Model: cfg.EmbeddingsModel,
			Price: ai.Price{Input: cfg.EmbeddingsPrice}}
	} else {
		log.Warn("classify: EMBEDDINGS_API_KEY not set; no graph links or duplicate-category checks")
	}
	if cfg.NamerEnabled && cfg.AnthropicAPIKey != "" {
		namer := ai.NewNamer(cfg.AnthropicAPIKey, cfg.NamerModel)
		namer.Price = ai.Price{Input: cfg.NamerInputPrice, Output: cfg.NamerOutputPrice}
		c.Namer = namer
	} else {
		log.Warn("classify: ANTHROPIC_API_KEY not set or namer disabled; no new categories will be created")
	}
	c.Notify = &push.Expo{Q: q, AccessToken: cfg.ExpoAccessToken}
	return c
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
