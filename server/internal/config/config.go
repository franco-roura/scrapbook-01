// Package config reads the API's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	// Addr is the address the HTTP server listens on, e.g. ":8080".
	Addr string
	// DatabaseURL is a Postgres connection string.
	DatabaseURL string
	// MigrateOnStart applies pending migrations when the API boots.
	MigrateOnStart bool

	// SupabaseURL is the project URL, e.g. https://abc.supabase.co. The JWKS
	// endpoint and the expected token issuer are derived from it.
	SupabaseURL string
	// SupabaseJWTSecret enables the legacy HS256 shared-secret check. When it is
	// empty, tokens are verified against the project's JWKS instead.
	SupabaseJWTSecret string

	// CORSOrigins lists browser origins allowed to call the API, from the
	// comma-separated CORS_ORIGINS. Empty means no browser access.
	CORSOrigins []string

	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	c := Config{
		Addr:              env("ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		MigrateOnStart:    env("MIGRATE_ON_START", "true") == "true",
		SupabaseURL:       strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseJWTSecret: os.Getenv("SUPABASE_JWT_SECRET"),
		ShutdownTimeout:   10 * time.Second,
	}
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}

	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.SupabaseURL == "" {
		errs = append(errs, errors.New("SUPABASE_URL is required"))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return c, nil
}

// Issuer is the "iss" claim Supabase Auth puts in its access tokens.
func (c Config) Issuer() string { return c.SupabaseURL + "/auth/v1" }

// JWKSURL is where Supabase publishes the public keys for asymmetric tokens.
func (c Config) JWKSURL() string { return c.SupabaseURL + "/auth/v1/.well-known/jwks.json" }

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
