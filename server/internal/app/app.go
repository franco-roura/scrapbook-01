// Package app wires the API's parts together. The local server (cmd/api) and
// the AWS Lambda entry points (cmd/lambda-api, cmd/lambda-worker) share it and
// differ only in how background work is queued: River locally, an async
// Lambda invoke on AWS.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

// Enqueuer queues an item for classification.
type Enqueuer interface {
	EnqueueClassify(ctx context.Context, itemID uuid.UUID) error
}

// Core holds what both the HTTP side and the workers need.
type Core struct {
	Cfg        config.Config
	Log        *slog.Logger
	Pool       *pgxpool.Pool
	Queries    *db.Queries
	Ledger     *credits.Ledger
	Classifier *classify.Classifier
}

// NewCore builds the database queries, credit ledger and classifier.
func NewCore(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) *Core {
	q := db.New(pool)
	ledger := &credits.Ledger{Q: q, Starter: cfg.StarterCredits}
	c := newClassifier(cfg, q, log)
	c.Credits = ledger
	return &Core{Cfg: cfg, Log: log, Pool: pool, Queries: q, Ledger: ledger, Classifier: c}
}

// Handler serves the app's API, the MCP endpoint and its OAuth metadata. It
// loads the token-signing keys, so it needs network access to WorkOS.
func (c *Core) Handler(ctx context.Context, enq Enqueuer) (http.Handler, error) {
	verifier, err := newVerifier(ctx, c.Cfg, c.Log)
	if err != nil {
		return nil, err
	}
	mcpOpts := mcpserver.Options{
		Queries: c.Queries, Verifier: verifier, Enqueuer: enq, Logger: c.Log, Credits: c.Ledger,
		PublicURL: c.Cfg.PublicURL, AuthServer: c.Cfg.AuthKitDomain,
	}
	if c.Classifier.Embed != nil {
		mcpOpts.Embed = c.Classifier.Embed
	}
	if c.Classifier.Jev != nil { // only with TYPESAFE_API_KEY set
		mcpOpts.Rerank = c.Classifier.Jev
	}
	mcpHandler := mcpserver.New(mcpOpts).Handler()
	root := http.NewServeMux()
	root.Handle(mcpserver.Path, mcpHandler)
	root.Handle("/.well-known/", mcpHandler)
	root.Handle("/", api.New(api.Options{
		Queries:     c.Queries,
		Logger:      c.Log,
		Verifier:    verifier,
		Ping:        c.Pool.Ping,
		CORSOrigins: c.Cfg.CORSOrigins,
		Enqueuer:    enq,
		Credits:     c.Ledger,
	}).Handler())
	return root, nil
}

// Sweep classifies up to limit items whose classification was missed (a
// failed job, or a user who has since topped up their credits), one after
// another, until ctx ends. The AWS worker runs it on a schedule; locally,
// River's periodic sweep re-queues the same items instead.
func (c *Core) Sweep(ctx context.Context, limit int32) (done int, err error) {
	ids, err := c.Queries.ItemsNeedingClassification(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		if _, err := c.Classifier.Run(ctx, id); err != nil {
			c.Log.WarnContext(ctx, "sweep: classify failed", "item", id, "err", err)
			continue
		}
		done++
	}
	return done, nil
}

// Migrate applies the app's migrations and River's.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return jobs.Migrate(ctx, pool)
}

// newVerifier trusts two kinds of WorkOS tokens:
//   - the app's AuthKit session tokens (issuer: the WorkOS API, keys per
//     client id, no audience);
//   - tokens AuthKit issues to MCP clients over OAuth (issuer: the AuthKit
//     domain). Their audience is the MCP URL when it's set up as a Resource
//     Indicator in the dashboard, or the client id when it isn't.
//
// Both carry the same WorkOS user id, so they map to the same user.
func newVerifier(ctx context.Context, cfg config.Config, log *slog.Logger) (*auth.Verifier, error) {
	var issuers []auth.Issuer
	if cfg.DevJWTSecret != "" {
		log.Warn("auth: DEV_JWT_SECRET is set; accepting locally signed tokens. Never set it in production")
		secret := []byte(cfg.DevJWTSecret)
		issuers = append(issuers, auth.Issuer{
			Issuer: auth.DevIssuer, Keyfunc: func(*jwt.Token) (any, error) { return secret, nil }, Methods: []string{"HS256"},
		})
	}
	if !cfg.WorkOSEnabled() {
		log.Warn("auth: WorkOS not configured; only dev tokens are accepted")
		return auth.New(issuers...), nil
	}
	log.Info("auth: WorkOS AuthKit", "sessions", cfg.SessionJWKSURL(), "mcp", cfg.AuthKitJWKSURL())
	sessionKeys, err := auth.JWKS(ctx, cfg.SessionJWKSURL())
	if err != nil {
		return nil, err
	}
	mcpKeys, err := auth.JWKS(ctx, cfg.AuthKitJWKSURL())
	if err != nil {
		return nil, err
	}
	issuers = append(issuers, auth.Issuer{
		Issuer: cfg.AuthKitDomain, Keyfunc: mcpKeys, Methods: auth.Asymmetric,
		Audiences: []string{cfg.MCPResource(), cfg.WorkOSClientID},
	})
	// Session tokens: like WorkOS's SDKs, check "iss" only when WORKOS_ISSUER
	// is set; otherwise the per-client signing keys are the check.
	issuers = append(issuers, auth.Issuer{Issuer: cfg.WorkOSIssuer, Keyfunc: sessionKeys, Methods: auth.Asymmetric})
	return auth.New(issuers...), nil
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
