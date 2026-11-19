package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/franco-roura/scrapbook-01/server/internal/config"
	"github.com/franco-roura/scrapbook-01/server/internal/telemetry"
)

// Pool opens the local server's pool, with queries traced when Sentry is on.
func Pool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("database url: %w", err)
	}
	cfg.ConnConfig.Tracer = telemetry.QueryTracer{}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// InitTelemetry starts Sentry when SENTRY_DSN is set. component tags the
// events ("api", "worker"). Call the returned flush before exiting.
func InitTelemetry(cfg config.Config, component string, lambda bool) (flush func(), err error) {
	return telemetry.Init(telemetry.Options{
		DSN: cfg.SentryDSN, Environment: cfg.SentryEnvironment, TracesSampleRate: cfg.SentryTracesSampleRate,
		Component: component, Lambda: lambda, Debug: cfg.SentryDebug,
	})
}

// LambdaPool opens a small pool suited to Lambda: each instance handles one
// request at a time, so two connections are plenty, and idle ones close
// quickly so frozen instances don't hold Neon connections open.
func LambdaPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("database url: %w", err)
	}
	cfg.MaxConns = 2
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 30 * time.Second
	cfg.ConnConfig.Tracer = telemetry.QueryTracer{}
	return pgxpool.NewWithConfig(ctx, cfg)
}
