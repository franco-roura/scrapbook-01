// Package config reads the API's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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

	// RunWorkers processes background jobs (classification) in this process.
	RunWorkers bool

	// Jev (TypeSafe) picks among the user's existing categories.
	TypeSafeAPIKey  string
	TypeSafeBaseURL string
	JevModel        string
	// JevThreshold is the confidence needed to file into an existing category.
	JevThreshold float64

	// Embeddings: any OpenAI-compatible /embeddings endpoint returning 1536
	// dimensions (OpenRouter by default).
	EmbeddingsAPIKey  string
	EmbeddingsBaseURL string
	EmbeddingsModel   string

	// Claude names new categories; without a key no new categories are made.
	AnthropicAPIKey string
	NamerModel      string
	NamerEnabled    bool

	// ExpoAccessToken is optional (needed only with Expo push security on).
	ExpoAccessToken string

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

		RunWorkers:        env("RUN_WORKERS", "true") == "true",
		TypeSafeAPIKey:    os.Getenv("TYPESAFE_API_KEY"),
		TypeSafeBaseURL:   env("TYPESAFE_BASE_URL", "https://api.typesafe.ai"),
		JevModel:          env("JEV_MODEL", "jev-latest"),
		EmbeddingsAPIKey:  os.Getenv("EMBEDDINGS_API_KEY"),
		EmbeddingsBaseURL: env("EMBEDDINGS_BASE_URL", "https://openrouter.ai/api/v1"),
		EmbeddingsModel:   env("EMBEDDINGS_MODEL", "openai/text-embedding-3-small"),
		AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		NamerModel:        env("CATEGORY_NAMER_MODEL", "claude-haiku-5-5"),
		NamerEnabled:      env("CATEGORY_NAMER_ENABLED", "true") == "true",
		ExpoAccessToken:   os.Getenv("EXPO_ACCESS_TOKEN"),
	}
	var errs []error
	if t, err := strconv.ParseFloat(env("JEV_THRESHOLD", "0.6"), 64); err != nil || t < 0 || t > 1 {
		errs = append(errs, errors.New("JEV_THRESHOLD must be a number from 0 to 1"))
	} else {
		c.JevThreshold = t
	}
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}

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
