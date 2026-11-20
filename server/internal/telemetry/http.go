package telemetry

import (
	"context"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
)

type routeKey struct{}

// route collects the transaction name while the request goes through nested
// muxes: the innermost match wins.
type route struct{ name string }

// Middleware starts one transaction per request, named after the matched
// route pattern ("GET /v1/items/{id}"), and reports panics. Wrap the
// outermost handler with it, and each mux with Routes. CORS preflights
// aren't traced: they'd only use up the Sentry quota.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !Enabled() || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		h := sentry.CurrentHub().Clone()
		rt := &route{}
		ctx := context.WithValue(sentry.SetHubOnContext(r.Context(), h), routeKey{}, rt)
		tx := sentry.StartTransaction(ctx, r.Method+" unmatched",
			sentry.ContinueTrace(h, r.Header.Get(sentry.SentryTraceHeader), r.Header.Get(sentry.SentryBaggageHeader)),
			sentry.WithOpName("http.server"),
			sentry.WithTransactionSource(sentry.SourceRoute),
		)
		tx.SetData("http.request.method", r.Method)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		r = r.WithContext(tx.Context())

		defer func() {
			v := recover()
			if v != nil {
				if v != http.ErrAbortHandler {
					h.RecoverWithContext(tx.Context(), v)
				}
				sw.status = http.StatusInternalServerError
			}
			if rt.name == "" && r.Pattern != "" {
				rt.name = patternName(r.Method, r.Pattern)
			}
			if rt.name != "" {
				tx.Name = rt.name
			}
			// No URL: paths can carry push tokens. The route pattern is enough.
			h.Scope().SetTag("http.route", tx.Name)
			tx.Status = sentry.HTTPtoSpanStatus(sw.status)
			tx.SetData("http.response.status_code", sw.status)
			tx.Finish()
			if v != nil {
				panic(v)
			}
		}()
		next.ServeHTTP(sw, r)
	})
}

// Routes names the request's transaction after the pattern mux matched, if no
// mux nested inside it already did.
func Routes(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if rt, ok := r.Context().Value(routeKey{}).(*route); ok && rt.name == "" && r.Pattern != "" {
			rt.name = patternName(r.Method, r.Pattern)
		}
	})
}

// NameTransaction overrides the request's transaction name, e.g. with the MCP
// tool being called, so each tool gets its own latency numbers.
func NameTransaction(ctx context.Context, name string) {
	if rt, ok := ctx.Value(routeKey{}).(*route); ok {
		rt.name = name
	}
}

// patternName adds the method to patterns registered without one ("/mcp").
func patternName(method, pattern string) string {
	if strings.Contains(pattern, " ") {
		return pattern
	}
	return method + " " + pattern
}

type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.written {
		w.status, w.written = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach Flush and friends.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Transport traces outbound calls as "http.client" spans named "METHOD
// host/path". It adds no headers and records no bodies. With no span in the
// request's context (telemetry off, or a call outside a request) it just
// calls base. A nil base means http.DefaultTransport.
func Transport(base http.RoundTripper) http.RoundTripper {
	return &roundTripper{base: base}
}

// HostOnlyTransport is Transport for user-provided URLs (link previews):
// spans name only the host, never the path.
func HostOnlyTransport(base http.RoundTripper) http.RoundTripper {
	return &roundTripper{base: base, hostOnly: true}
}

type roundTripper struct {
	base     http.RoundTripper
	hostOnly bool
}

func (t *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	parent := sentry.SpanFromContext(req.Context())
	if parent == nil {
		return base.RoundTrip(req)
	}
	desc := req.Method + " " + req.URL.Host
	if !t.hostOnly {
		desc += req.URL.Path
	}
	span := parent.StartChild("http.client", sentry.WithDescription(desc))
	span.SetData("http.request.method", req.Method)
	span.SetData("server.address", req.URL.Hostname())
	defer span.Finish()
	res, err := base.RoundTrip(req)
	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		return res, err
	}
	span.Status = sentry.HTTPtoSpanStatus(res.StatusCode)
	span.SetData("http.response.status_code", res.StatusCode)
	return res, nil
}

// Client returns c (or a new client) with its transport traced.
func Client(c *http.Client) *http.Client {
	if c == nil {
		c = &http.Client{}
	}
	cp := *c
	cp.Transport = Transport(c.Transport)
	return &cp
}
