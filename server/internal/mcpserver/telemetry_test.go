package mcpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/franco-roura/scrapbook-01/server/internal/auth"
	"github.com/franco-roura/scrapbook-01/server/internal/mcpserver"
	"github.com/franco-roura/scrapbook-01/server/internal/telemetry"
)

type bearerRT struct {
	token string
	base  http.RoundTripper
}

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

// Each MCP call is its own Sentry transaction, named after the method, with
// the signed-in user's id. No database needed: tools/list doesn't query.
func TestMCPTransactionsNamedPerMethod(t *testing.T) {
	tr := &sentry.MockTransport{}
	if _, err := telemetry.Init(telemetry.Options{DSN: "https://public@sentry.example.test/1", TracesSampleRate: 1, Transport: tr}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sentry.CurrentHub().BindClient(nil) })

	const iss = "https://test.authkit.example.test"
	key := []byte("made-up-secret-at-least-32-bytes-long")
	user := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.String(), "iss": iss, "aud": "https://api.example.test/mcp",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	s := mcpserver.New(mcpserver.Options{
		Verifier: auth.NewHS256(key, iss, "https://api.example.test/mcp"), PublicURL: "https://api.example.test", AuthServer: iss,
	})
	srv := httptest.NewServer(telemetry.Middleware(telemetry.Routes(s.Handler())))
	defer srv.Close()

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: srv.URL + mcpserver.Path, HTTPClient: &http.Client{Transport: bearerRT{tok, http.DefaultTransport}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cs.ListTools(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	cs.Close()

	var found bool
	for _, e := range tr.Events() {
		if e.Type == "transaction" && e.Transaction == "MCP tools/list" {
			found = true
			if e.User.ID != user.String() {
				t.Errorf("user = %+v", e.User)
			}
		}
	}
	if !found {
		var names []string
		for _, e := range tr.Events() {
			names = append(names, e.Transaction)
		}
		t.Fatalf("no MCP tools/list transaction; got %v", names)
	}
}
