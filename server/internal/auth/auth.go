// Package auth checks access tokens (WorkOS AuthKit by default) and puts the
// signed-in user's id on the request context.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrUnauthorized = errors.New("unauthorized")

// DevIssuer is the "iss" of tokens signed with DEV_JWT_SECRET (local only).
const DevIssuer = "scrapbook01-dev"

// Issuer is one token issuer the API trusts.
type Issuer struct {
	// Issuer is the exact "iss" claim. Empty makes this the fallback for
	// tokens whose "iss" matches no other entry: they're accepted on the
	// signature alone, as WorkOS's SDKs do for session tokens (the keys are
	// per client id, so only our environment's tokens pass).
	Issuer string
	// Keyfunc returns the key that checks a token's signature.
	Keyfunc jwt.Keyfunc
	// Methods are the accepted signing algorithms.
	Methods []string
	// Audiences, when set, require "aud" to contain at least one of them.
	Audiences []string
}

// Verifier validates access tokens from any of its trusted issuers.
type Verifier struct {
	issuers map[string]*jwt.Parser
	keys    map[string]jwt.Keyfunc
}

// New builds a Verifier that trusts the given issuers.
func New(issuers ...Issuer) *Verifier {
	v := &Verifier{issuers: map[string]*jwt.Parser{}, keys: map[string]jwt.Keyfunc{}}
	for _, is := range issuers {
		opts := []jwt.ParserOption{
			jwt.WithValidMethods(is.Methods),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(30 * time.Second),
		}
		if is.Issuer != "" {
			opts = append(opts, jwt.WithIssuer(is.Issuer))
		}
		if len(is.Audiences) > 0 {
			opts = append(opts, jwt.WithAudience(is.Audiences...))
		}
		v.issuers[is.Issuer] = jwt.NewParser(opts...)
		v.keys[is.Issuer] = is.Keyfunc
	}
	return v
}

// JWKS returns a key lookup backed by a JWKS endpoint (fetched now, then
// refreshed in the background).
func JWKS(ctx context.Context, url string) (jwt.Keyfunc, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{url})
	if err != nil {
		return nil, fmt.Errorf("auth: load JWKS %s: %w", url, err)
	}
	return k.Keyfunc, nil
}

// Asymmetric are the algorithms accepted for JWKS-backed issuers.
var Asymmetric = []string{"RS256", "ES256", "EdDSA"}

// NewHS256 trusts one issuer signing with a shared secret. Tests use it.
func NewHS256(secret []byte, issuer string, audiences ...string) *Verifier {
	return New(Issuer{
		Issuer:    issuer,
		Keyfunc:   func(*jwt.Token) (any, error) { return secret, nil },
		Methods:   []string{"HS256"},
		Audiences: audiences,
	})
}

// UserID validates a raw token and returns the user it belongs to.
func (v *Verifier) UserID(raw string) (uuid.UUID, error) {
	id, _, err := v.Verify(raw)
	return id, err
}

type tokenClaims struct {
	jwt.RegisteredClaims
	// SubProfile is "ai_agent" on WorkOS agent tokens, which act for no user.
	SubProfile string `json:"sub_profile,omitempty"`
}

// Verify validates a raw token and returns its user id and expiry.
func (v *Verifier) Verify(raw string) (uuid.UUID, time.Time, error) {
	fail := func(format string, a ...any) (uuid.UUID, time.Time, error) {
		return uuid.Nil, time.Time{}, fmt.Errorf("%w: %s", ErrUnauthorized, fmt.Sprintf(format, a...))
	}
	// Read "iss" first to pick the issuer's checks; the signature and every
	// claim are then verified by that issuer's parser.
	var peek jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(raw, &peek); err != nil {
		return fail("%v", err)
	}
	iss := peek.Issuer
	parser, ok := v.issuers[iss]
	if !ok {
		if parser, ok = v.issuers[""]; !ok {
			return fail("untrusted issuer %q", iss)
		}
		iss = ""
	}
	var c tokenClaims
	if _, err := parser.ParseWithClaims(raw, &c, v.keys[iss]); err != nil {
		return fail("iss %q: %v", peek.Issuer, err)
	}
	if c.SubProfile == "ai_agent" {
		return fail("agent tokens are not accepted")
	}
	id := UserIDFor(c.Subject)
	if id == uuid.Nil {
		return fail("token has no user")
	}
	return id, c.ExpiresAt.Time, nil
}

// namespace makes user ids from identity-provider subjects.
var namespace = uuid.MustParse("6f1c2b8e-3d7a-4f0e-9b65-2a4c8d1e7f30")

// UserIDFor maps an identity provider's user id ("sub") to the uuid the
// database uses. A uuid subject is used as is; anything else (WorkOS's
// "user_01H...") becomes a stable name-based uuid, so no lookup table is
// needed and the app and MCP tokens for one person map to the same user.
func UserIDFor(sub string) uuid.UUID {
	if sub == "" {
		return uuid.Nil
	}
	if id, err := uuid.Parse(sub); err == nil {
		return id
	}
	return uuid.NewSHA1(namespace, []byte(sub))
}

type ctxKey struct{}

// Middleware rejects requests without a valid "Authorization: Bearer" token
// and stores the user id for handlers to read with UserFrom.
func (v *Verifier) Middleware(onError func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearer(r.Header.Get("Authorization"))
			if !ok {
				onError(w, r, fmt.Errorf("%w: missing bearer token", ErrUnauthorized))
				return
			}
			id, err := v.UserID(raw)
			if err != nil {
				onError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), id)))
		})
	}
}

// WithUser returns a context carrying the user id. Tests use it to skip tokens.
func WithUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// UserFrom returns the user id stored by Middleware.
func UserFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

func bearer(h string) (string, bool) {
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
