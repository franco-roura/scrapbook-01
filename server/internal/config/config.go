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

	// WorkOSClientID is the WorkOS environment's client id (client_...). The
	// app's session tokens are checked against its JWKS.
	WorkOSClientID string
	// WorkOSAPIURL is the WorkOS API origin, or your custom auth domain.
	WorkOSAPIURL string
	// AuthKitDomain is the AuthKit origin (e.g. https://abc-123.authkit.app).
	// It issues the tokens MCP clients (Claude, ChatGPT) use.
	AuthKitDomain string
	// DevJWTSecret, for local development only, also accepts HS256 tokens
	// with issuer "scrapbook01-dev" signed with this secret. When it's set,
	// the WorkOS settings become optional.
	DevJWTSecret string

	// CORSOrigins lists browser origins allowed to call the API, from the
	// comma-separated CORS_ORIGINS. Empty means no browser access.
	CORSOrigins []string

	// PublicURL is the API's external origin (no trailing slash). MCP clients
	// see it in the OAuth protected-resource metadata.
	PublicURL string

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

	// StarterCredits is each new user's free model budget, in micro-dollars.
	StarterCredits int64
	// List prices in dollars per million tokens, for metering.
	JevPrice, EmbeddingsPrice, NamerInputPrice, NamerOutputPrice float64

	// ExpoAccessToken is optional (needed only with Expo push security on).
	ExpoAccessToken string

	ShutdownTimeout time.Duration
}

// Load reads settings from the environment, after filling in anything unset
// from ./.env (so `go run ./cmd/api` works the same in any shell).
func Load() (Config, error) {
	if err := LoadDotEnv(".env"); err != nil {
		return Config{}, fmt.Errorf("config: read .env: %w", err)
	}
	c := Config{
		Addr:            env("ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		MigrateOnStart:  env("MIGRATE_ON_START", "true") == "true",
		WorkOSClientID:  os.Getenv("WORKOS_CLIENT_ID"),
		WorkOSAPIURL:    strings.TrimRight(env("WORKOS_API_URL", "https://api.workos.com"), "/"),
		AuthKitDomain:   strings.TrimRight(os.Getenv("WORKOS_AUTHKIT_DOMAIN"), "/"),
		DevJWTSecret:    os.Getenv("DEV_JWT_SECRET"),
		ShutdownTimeout: 10 * time.Second,

		PublicURL:         strings.TrimRight(env("PUBLIC_URL", "http://localhost:8080"), "/"),
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
	floatEnv := func(key, fallback string, dst *float64) {
		v, err := strconv.ParseFloat(env(key, fallback), 64)
		if err != nil || v < 0 {
			errs = append(errs, fmt.Errorf("%s must be a non-negative number", key))
			return
		}
		*dst = v
	}
	var starterUSD float64
	floatEnv("STARTER_CREDITS_USD", "0.10", &starterUSD)
	c.StarterCredits = int64(starterUSD * 1_000_000)
	floatEnv("JEV_PRICE_PER_MTOK", "0.042", &c.JevPrice)
	floatEnv("EMBEDDINGS_PRICE_PER_MTOK", "0.02", &c.EmbeddingsPrice)
	floatEnv("NAMER_INPUT_PRICE_PER_MTOK", "0.10", &c.NamerInputPrice)
	floatEnv("NAMER_OUTPUT_PRICE_PER_MTOK", "0.50", &c.NamerOutputPrice)
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
	if c.WorkOSClientID == "" && c.DevJWTSecret == "" {
		errs = append(errs, errors.New("WORKOS_CLIENT_ID is required"))
	}
	if c.AuthKitDomain == "" && c.DevJWTSecret == "" {
		errs = append(errs, errors.New("WORKOS_AUTHKIT_DOMAIN is required"))
	} else if c.AuthKitDomain != "" && !strings.HasPrefix(c.AuthKitDomain, "https://") && !strings.HasPrefix(c.AuthKitDomain, "http://") {
		c.AuthKitDomain = "https://" + c.AuthKitDomain
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return c, nil
}

// WorkOSEnabled reports whether WorkOS tokens are accepted.
func (c Config) WorkOSEnabled() bool { return c.WorkOSClientID != "" && c.AuthKitDomain != "" }

// SessionJWKSURL publishes the keys for the app's AuthKit session tokens.
func (c Config) SessionJWKSURL() string { return c.WorkOSAPIURL + "/sso/jwks/" + c.WorkOSClientID }

// AuthKitJWKSURL publishes the keys for tokens AuthKit issues to MCP clients.
func (c Config) AuthKitJWKSURL() string { return c.AuthKitDomain + "/oauth2/jwks" }

// MCPResource is the MCP endpoint's URL: the "resource" MCP clients ask
// AuthKit for, and the "aud" of the tokens they get.
func (c Config) MCPResource() string { return c.PublicURL + "/mcp" }

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
