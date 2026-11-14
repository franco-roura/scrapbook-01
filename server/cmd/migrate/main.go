// Command migrate applies the database migrations (the app's and River's).
// Deploys run it before updating the Lambdas, which don't migrate on start.
// On Neon, use the direct connection string, not the "-pooler" one.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aaugoustis/waitlist/server/internal/app"
	"github.com/aaugoustis/waitlist/server/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	url := os.Getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return fmt.Errorf("set MIGRATE_DATABASE_URL (or DATABASE_URL)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := app.Migrate(ctx, pool); err != nil {
		return err
	}
	fmt.Println("migrations applied")
	return nil
}
