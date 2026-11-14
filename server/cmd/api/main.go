// Command api serves the Scrapbook/01 HTTP API locally (or on any server):
// API, MCP and River workers in one long-lived process.
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

	"github.com/aaugoustis/waitlist/server/internal/app"
	"github.com/aaugoustis/waitlist/server/internal/config"
	"github.com/aaugoustis/waitlist/server/internal/jobs"
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
		if err := app.Migrate(ctx, pool); err != nil {
			return err
		}
	}

	core := app.NewCore(cfg, pool, log)
	queue, err := jobs.New(jobs.Options{
		Pool:       pool,
		Queries:    core.Queries,
		Classifier: core.Classifier,
		Logger:     log,
		RunWorkers: cfg.RunWorkers,
	})
	if err != nil {
		return fmt.Errorf("job queue: %w", err)
	}
	if err := queue.Start(ctx); err != nil {
		return fmt.Errorf("start workers: %w", err)
	}

	handler, err := core.Handler(ctx, queue)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
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
