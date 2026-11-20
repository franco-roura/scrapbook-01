package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// A made-up DSN: the mock transport never sends anything.
const testDSN = "https://public@sentry.example.test/1"

func start(t *testing.T) *sentry.MockTransport {
	t.Helper()
	tr := &sentry.MockTransport{}
	if _, err := Init(Options{DSN: testDSN, Environment: "test", TracesSampleRate: 1, Component: "api", Transport: tr}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sentry.CurrentHub().BindClient(nil) })
	return tr
}

func transactions(tr *sentry.MockTransport) []*sentry.Event {
	var out []*sentry.Event
	for _, e := range tr.Events() {
		if e.Type == "transaction" {
			out = append(out, e)
		}
	}
	return out
}

func TestOffWithoutDSN(t *testing.T) {
	sentry.CurrentHub().BindClient(nil)
	flush, err := Init(Options{})
	if err != nil {
		t.Fatal(err)
	}
	flush()
	if Enabled() {
		t.Fatal("telemetry on without a DSN")
	}
	called := false
	h := Middleware(Routes(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if sentry.SpanFromContext(r.Context()) != nil {
			t.Error("span started while off")
		}
		ctx, span := StartSpan(r.Context(), "op", "desc")
		span.SetData("k", 1)
		span.End(nil)
		SetUser(ctx, "someone")
		CaptureError(ctx, errors.New("ignored"))
		w.WriteHeader(http.StatusTeapot)
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !called || rec.Code != http.StatusTeapot {
		t.Fatalf("handler not called through: called=%v code=%d", called, rec.Code)
	}
}

// The API nests muxes and the auth middleware copies the request, like here;
// the transaction must still be named after the innermost pattern.
func TestTransactionNamedAfterInnermostRoute(t *testing.T) {
	tr := start(t)
	v1 := http.NewServeMux()
	v1.HandleFunc("GET /v1/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		SetUser(r.Context(), "00000000-0000-0000-0000-000000000001")
		w.WriteHeader(http.StatusForbidden)
	})
	copyReq := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), struct{}{}, 1)))
		})
	}
	api := http.NewServeMux()
	api.Handle("/v1/", copyReq(Routes(v1)))
	root := http.NewServeMux()
	root.Handle("/", Routes(api))

	req := httptest.NewRequest("GET", "/v1/items/abc?token=secret", nil)
	req.Header.Set("Authorization", "Bearer secret")
	Middleware(Routes(root)).ServeHTTP(httptest.NewRecorder(), req)

	txs := transactions(tr)
	if len(txs) != 1 {
		t.Fatalf("got %d transactions", len(txs))
	}
	tx := txs[0]
	if tx.Transaction != "GET /v1/items/{id}" {
		t.Errorf("name = %q", tx.Transaction)
	}
	if tx.User.ID != "00000000-0000-0000-0000-000000000001" || tx.User.Email != "" {
		t.Errorf("user = %+v", tx.User)
	}
	if tx.Request != nil && (tx.Request.QueryString != "" || len(tx.Request.Headers) > 0 || tx.Request.URL != "") {
		t.Errorf("request data leaked: %+v", tx.Request)
	}
	if got := tx.Contexts["trace"]["data"].(map[string]any)["http.response.status_code"]; got != http.StatusForbidden {
		t.Errorf("status = %v", got)
	}
}

func TestPatternWithoutMethodGetsOne(t *testing.T) {
	tr := start(t)
	root := http.NewServeMux()
	root.HandleFunc("/mcp", func(http.ResponseWriter, *http.Request) {})
	Middleware(Routes(root)).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/mcp", nil))
	if txs := transactions(tr); len(txs) != 1 || txs[0].Transaction != "POST /mcp" {
		t.Fatalf("transactions = %v", txs)
	}
}

func TestNameTransactionOverrides(t *testing.T) {
	tr := start(t)
	root := http.NewServeMux()
	root.HandleFunc("/mcp", func(_ http.ResponseWriter, r *http.Request) {
		NameTransaction(r.Context(), "MCP tools/call search")
	})
	Middleware(Routes(root)).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/mcp", nil))
	if txs := transactions(tr); len(txs) != 1 || txs[0].Transaction != "MCP tools/call search" {
		t.Fatalf("transactions = %v", txs)
	}
}

func TestPanicReportedAndRepanicked(t *testing.T) {
	tr := start(t)
	h := Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic swallowed")
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	var errs int
	for _, e := range tr.Events() {
		if e.Type == "" {
			errs++
		}
	}
	if errs != 1 || len(transactions(tr)) != 1 {
		t.Fatalf("events = %d errors, %d transactions", errs, len(transactions(tr)))
	}
}

func TestQueryTracer(t *testing.T) {
	tr := start(t)
	ctx, finish := Job(context.Background(), "worker classify", "", "")
	var qt QueryTracer
	q := qt.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{
		SQL:  "-- name: GetItem :one\nSELECT id,\n  note FROM items\nWHERE id = $1 AND user_id = $2\n",
		Args: []any{"made-up-id", "made-up-user"},
	})
	qt.TraceQueryEnd(q, nil, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag("SELECT 1")})
	// River's own queries have no sqlc header: not traced.
	q = qt.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	qt.TraceQueryEnd(q, nil, pgx.TraceQueryEndData{})
	finish(nil)

	txs := transactions(tr)
	if len(txs) != 1 {
		t.Fatalf("got %d transactions", len(txs))
	}
	spans := txs[0].Spans
	if len(spans) != 1 {
		t.Fatalf("got %d spans", len(spans))
	}
	s := spans[0]
	if s.Op != "db.sql.query" || s.Description != "SELECT id, note FROM items WHERE id = $1 AND user_id = $2" {
		t.Errorf("span = %q %q", s.Op, s.Description)
	}
	if s.Data["db.operation.name"] != "GetItem" || s.Data["db.system"] != "postgresql" {
		t.Errorf("data = %v", s.Data)
	}
	for k, v := range s.Data {
		if v == "made-up-id" {
			t.Errorf("argument recorded under %s", k)
		}
	}
}

func TestQueryOutsideSpanSkipped(t *testing.T) {
	start(t)
	var qt QueryTracer
	ctx := context.Background()
	if got := qt.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "-- name: X :one\nSELECT 1"}); got != ctx {
		t.Fatal("started a span with no parent")
	}
}

func TestSQLCQuery(t *testing.T) {
	for _, tc := range []struct{ in, name, sql string }{
		{"-- name: ListItems :many\nSELECT *\nFROM items", "ListItems", "SELECT * FROM items"},
		{"-- name: DeleteItem :exec\r\nDELETE FROM items", "DeleteItem", "DELETE FROM items"},
	} {
		name, sql, ok := sqlcQuery(tc.in)
		if !ok || name != tc.name || sql != tc.sql {
			t.Errorf("sqlcQuery(%q) = %q %q %v", tc.in, name, sql, ok)
		}
	}
	if _, _, ok := sqlcQuery("SELECT 1"); ok {
		t.Error("plain SQL treated as sqlc")
	}
}

func TestTransportSpans(t *testing.T) {
	tr := start(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(sentry.SentryTraceHeader) != "" || r.Header.Get(sentry.SentryBaggageHeader) != "" {
			t.Error("trace headers sent to a third party")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	ctx, finish := Job(context.Background(), "worker preview", "", "")
	for _, rt := range []http.RoundTripper{Transport(nil), HostOnlyTransport(nil)} {
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/private/path?q=1", nil)
		res, err := (&http.Client{Transport: rt}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	finish(nil)

	spans := transactions(tr)[0].Spans
	if len(spans) != 2 {
		t.Fatalf("got %d spans", len(spans))
	}
	host := srv.Listener.Addr().String()
	if spans[0].Description != "GET "+host+"/private/path" {
		t.Errorf("full span = %q", spans[0].Description)
	}
	if spans[1].Description != "GET "+host {
		t.Errorf("host-only span = %q", spans[1].Description)
	}
}

func TestJobContinuesTraceAndReportsError(t *testing.T) {
	tr := start(t)
	reqCtx, endReq := Job(context.Background(), "request", "", "")
	trace, baggage := TraceHeaders(reqCtx)
	if trace == "" {
		t.Fatal("no trace header")
	}
	_, finish := Job(context.Background(), "worker classify", trace, baggage)
	finish(errors.New("made-up failure"))
	endReq(nil)

	txs := transactions(tr)
	if len(txs) != 2 {
		t.Fatalf("got %d transactions", len(txs))
	}
	if a, b := txs[0].Contexts["trace"]["trace_id"], txs[1].Contexts["trace"]["trace_id"]; a != b {
		t.Errorf("trace not continued: %v vs %v", a, b)
	}
	if len(tr.Events()) != 3 {
		t.Errorf("want 2 transactions + 1 error, got %d events", len(tr.Events()))
	}
}

func TestScrub(t *testing.T) {
	e := scrub(&sentry.Event{
		ServerName: "host-1",
		User:       sentry.User{ID: "u1", Email: "someone@example.test", IPAddress: "203.0.113.1"},
		Request: &sentry.Request{
			Method: "POST", URL: "https://api.example.test/v1/items",
			Data: `{"note":"made-up"}`, QueryString: "token=x", Cookies: "c=1",
			Headers: map[string]string{"Authorization": "Bearer x"},
		},
	}, nil)
	if e.User.Email != "" || e.User.IPAddress != "" || e.User.ID != "u1" {
		t.Errorf("user = %+v", e.User)
	}
	if e.Request.Data != "" || e.Request.QueryString != "" || e.Request.Cookies != "" || len(e.Request.Headers) != 0 {
		t.Errorf("request = %+v", e.Request)
	}
	if e.ServerName != "" {
		t.Error("server name kept")
	}
}
