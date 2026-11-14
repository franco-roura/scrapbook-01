package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	return pgxpool.NewWithConfig(ctx, cfg)
}
