package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const issuer = "https://test.authkit.app"

const audience = "https://api.example.test/mcp"

var secret = []byte("test-secret-at-least-32-bytes-long!!")

func claims(sub string, mod func(*jwt.RegisteredClaims)) jwt.RegisteredClaims {
	c := jwt.RegisteredClaims{
		Subject:   sub,
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	if mod != nil {
		mod(&c)
	}
	return c
}

func hs256(t *testing.T, c jwt.RegisteredClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHS256(t *testing.T) {
	v := NewHS256(secret, issuer, audience)
	user := uuid.New()

	got, err := v.UserID(hs256(t, claims(user.String(), nil)))
	if err != nil || got != user {
		t.Fatalf("valid token: got %v, %v", got, err)
	}

	bad := map[string]string{
		"expired":        hs256(t, claims(user.String(), func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) })),
		"no expiry":      hs256(t, claims(user.String(), func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil })),
		"wrong issuer":   hs256(t, claims(user.String(), func(c *jwt.RegisteredClaims) { c.Issuer = "https://evil.example/auth/v1" })),
		"wrong audience": hs256(t, claims(user.String(), func(c *jwt.RegisteredClaims) { c.Audience = jwt.ClaimStrings{"anon"} })),
		"no subject":     hs256(t, claims("", nil)),
		"garbage":        "not.a.token",
	}
	other, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims(user.String(), nil)).SignedString([]byte("another-secret-another-secret-123"))
	bad["wrong secret"] = other

	for name, tok := range bad {
		if _, err := v.UserID(tok); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("%s: err = %v, want ErrUnauthorized", name, err)
		}
	}
}

func TestAsymmetricRejectsAlgSwitch(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	v := New(Issuer{Issuer: issuer, Keyfunc: func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, Methods: []string{"ES256"}})
	user := uuid.New()

	tok, _ := jwt.NewWithClaims(jwt.SigningMethodES256, claims(user.String(), nil)).SignedString(key)
	if got, err := v.UserID(tok); err != nil || got != user {
		t.Fatalf("ES256 token: got %v, %v", got, err)
	}
	// An HS256 token must not be accepted by an ES256-only verifier.
	if _, err := v.UserID(hs256(t, claims(user.String(), nil))); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("HS256 on ES256 verifier: err = %v", err)
	}
}

func TestMiddleware(t *testing.T) {
	v := NewHS256(secret, issuer, audience)
	user := uuid.New()
	var seen uuid.UUID
	h := v.Middleware(func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusUnauthorized)
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = UserFrom(r.Context())
	}))

	for header, want := range map[string]int{
		"":          http.StatusUnauthorized,
		"Basic abc": http.StatusUnauthorized,
		"Bearer ":   http.StatusUnauthorized,
		"bearer " + hs256(t, claims(user.String(), nil)): http.StatusOK,
	} {
		seen = uuid.Nil
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("header %.20q: status %d, want %d", header, rec.Code, want)
		}
		if want == http.StatusOK && seen != user {
			t.Errorf("handler saw user %v, want %v", seen, user)
		}
	}
}

func TestWorkOSSubjectsMapToStableUUIDs(t *testing.T) {
	v := NewHS256(secret, issuer, audience)
	want := UserIDFor("user_01HBEQKA6K4QJAS93VPE39W1JT")
	if want == uuid.Nil || want != UserIDFor("user_01HBEQKA6K4QJAS93VPE39W1JT") || want == UserIDFor("user_other") {
		t.Fatal("UserIDFor must be stable and distinct")
	}
	got, err := v.UserID(hs256(t, claims("user_01HBEQKA6K4QJAS93VPE39W1JT", nil)))
	if err != nil || got != want {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
}

func TestFallbackIssuerChecksSignatureOnly(t *testing.T) {
	key := func(*jwt.Token) (any, error) { return secret, nil }
	v := New(
		Issuer{Issuer: issuer, Keyfunc: func(*jwt.Token) (any, error) { return []byte("mcp-secret-mcp-secret-mcp-secret-1"), nil }, Methods: []string{"HS256"}, Audiences: []string{audience}},
		Issuer{Keyfunc: key, Methods: []string{"HS256"}},
	)
	odd := hs256(t, claims("user_a", func(c *jwt.RegisteredClaims) {
		c.Issuer = "https://api.workos.com/user_management/client_123"
		c.Audience = nil
	}))
	if _, err := v.UserID(odd); err != nil {
		t.Fatalf("session token with another iss: %v", err)
	}
	// A known issuer still gets its own key and audience, never the fallback.
	if _, err := v.UserID(hs256(t, claims("user_a", nil))); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("MCP issuer signed with the session key: err = %v", err)
	}
}

func TestTrustsSeveralIssuers(t *testing.T) {
	key := func(*jwt.Token) (any, error) { return secret, nil }
	v := New(
		Issuer{Issuer: "https://api.workos.com", Keyfunc: key, Methods: []string{"HS256"}},
		Issuer{Issuer: issuer, Keyfunc: key, Methods: []string{"HS256"}, Audiences: []string{audience}},
	)
	app := hs256(t, claims("user_a", func(c *jwt.RegisteredClaims) { c.Issuer = "https://api.workos.com"; c.Audience = nil }))
	if _, err := v.UserID(app); err != nil {
		t.Fatalf("app token without aud: %v", err)
	}
	mcpNoAud := hs256(t, claims("user_a", func(c *jwt.RegisteredClaims) { c.Audience = nil }))
	if _, err := v.UserID(mcpNoAud); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("MCP token without aud: err = %v", err)
	}
	agent, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "agent_1", "iss": issuer, "aud": audience, "sub_profile": "ai_agent",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(secret)
	if _, err := v.UserID(agent); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("agent token: err = %v", err)
	}
}
