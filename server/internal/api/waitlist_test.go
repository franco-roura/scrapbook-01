package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/franco-roura/scrapbook-01/server/internal/auth"
	"github.com/franco-roura/scrapbook-01/server/internal/waitlist"
)

type fakeWaitlist struct {
	joined []string
	err    error
}

func (f *fakeWaitlist) Join(_ context.Context, email, name string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.joined = append(f.joined, email+"|"+name)
	return "pending", nil
}

func waitlistServer(t *testing.T, wl Waitlist) *httptest.Server {
	t.Helper()
	opts := Options{
		Queries:     newFake(),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier:    auth.NewHS256(testSecret, testIssuer, "https://api.example.test/mcp"),
		CORSOrigins: []string{"https://landing.example.test"},
	}
	if wl != nil {
		opts.Waitlist = wl
	}
	srv := httptest.NewServer(New(opts).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func postWaitlist(t *testing.T, srv *httptest.Server, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/waitlist", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://landing.example.test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.Header.Get("Access-Control-Allow-Origin") != "https://landing.example.test" {
		t.Errorf("missing CORS header for the landing origin")
	}
	return res.StatusCode, string(b)
}

func TestJoinWaitlist(t *testing.T) {
	wl := &fakeWaitlist{}
	srv := waitlistServer(t, wl)

	// No sign-in needed.
	if code, body := postWaitlist(t, srv, `{"name":" Ana ","email":" ana@example.com "}`); code != http.StatusOK || !strings.Contains(body, `"pending"`) {
		t.Fatalf("join: %d %s", code, body)
	}
	if len(wl.joined) != 1 || wl.joined[0] != "ana@example.com|Ana" {
		t.Fatalf("joined %v, want trimmed email and name", wl.joined)
	}

	for _, body := range []string{
		`{"name":"","email":"ana@example.com"}`,
		`{"name":"Ana","email":"not-an-email"}`,
		`{"name":"Ana","email":"ana@localhost"}`,
		`{"name":"Ana","email":"Ana <ana@example.com>"}`,
		`{"name":"Ana","email":"ana@example.com","extra":1}`,
	} {
		if code, _ := postWaitlist(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", body, code)
		}
	}
	if len(wl.joined) != 1 {
		t.Errorf("invalid requests reached WorkOS: %v", wl.joined)
	}
}

func TestJoinWaitlistErrors(t *testing.T) {
	ok := `{"name":"Ana","email":"ana@example.com"}`
	if code, _ := postWaitlist(t, waitlistServer(t, nil), ok); code != http.StatusServiceUnavailable {
		t.Errorf("without a key: got %d, want 503", code)
	}
	if code, body := postWaitlist(t, waitlistServer(t, &fakeWaitlist{err: waitlist.ErrAlreadyUser}), ok); code != http.StatusConflict || !strings.Contains(body, "already_user") {
		t.Errorf("existing user: got %d %s, want 409", code, body)
	}
	if code, _ := postWaitlist(t, waitlistServer(t, &fakeWaitlist{err: errors.New("boom")}), ok); code != http.StatusBadGateway {
		t.Errorf("WorkOS down: got %d, want 502", code)
	}
}
