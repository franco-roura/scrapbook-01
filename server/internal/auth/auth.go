// Package auth checks Supabase Auth access tokens and puts the signed-in
// user's id on the request context.
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

// audience is the "aud" claim Supabase sets on signed-in users' tokens.
const audience = "authenticated"

var ErrUnauthorized = errors.New("unauthorized")

// Verifier validates access tokens.
type Verifier struct {
	keyfunc jwt.Keyfunc
	parser  *jwt.Parser
}

// New builds a Verifier from a key lookup function and the signing methods it
// accepts. Most callers want NewJWKS or NewHS256.
func New(kf jwt.Keyfunc, methods []string, issuer string) *Verifier {
	return &Verifier{
		keyfunc: kf,
		parser: jwt.NewParser(
			jwt.WithValidMethods(methods),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(30*time.Second),
		),
	}
}

// NewJWKS verifies tokens signed with the project's asymmetric signing keys,
// fetched (and refreshed in the background) from jwksURL.
func NewJWKS(ctx context.Context, jwksURL, issuer string) (*Verifier, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("auth: load JWKS: %w", err)
	}
	return New(k.Keyfunc, []string{"ES256", "RS256", "EdDSA"}, issuer), nil
}

// NewHS256 verifies tokens signed with the legacy shared JWT secret.
func NewHS256(secret []byte, issuer string) *Verifier {
	return New(func(*jwt.Token) (any, error) { return secret, nil }, []string{"HS256"}, issuer)
}

// UserID validates a raw token and returns the user id in its "sub" claim.
func (v *Verifier) UserID(raw string) (uuid.UUID, error) {
	id, _, err := v.Verify(raw)
	return id, err
}

// Verify validates a raw token and returns its user id and expiry. Tokens
// Supabase issues to OAuth clients (MCP connectors) pass the same checks.
func (v *Verifier) Verify(raw string) (uuid.UUID, time.Time, error) {
	var claims jwt.RegisteredClaims
	if _, err := v.parser.ParseWithClaims(raw, &claims, v.keyfunc); err != nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("%w: token has no user", ErrUnauthorized)
	}
	return id, claims.ExpiresAt.Time, nil
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
