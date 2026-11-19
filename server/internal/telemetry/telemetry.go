// Package telemetry is the optional Sentry integration: errors, one
// transaction per HTTP request or background job, and spans for database
// queries, outbound HTTP calls and AI calls.
//
// It is off unless SENTRY_DSN is set. While off, every helper here is a
// cheap no-op, so callers never need to check.
//
// Privacy: Sentry only ever sees the internal user id (a UUID), route
// patterns, SQL text without arguments, and the hosts of outbound calls.
// Request bodies, headers, query strings and cookies are never sent.
package telemetry

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
)

// Version is the release reported to Sentry. The deploy sets it with
// -ldflags "-X github.com/franco-roura/scrapbook-01/server/internal/telemetry.Version=…".
var Version = "dev"

// Options configure Init.
type Options struct {
	// DSN is the Sentry project's DSN. Empty turns telemetry off.
	DSN string
	// Environment is the deployment stage ("prod", "development").
	Environment string
	// TracesSampleRate is the share of requests and jobs traced, 0 to 1.
	TracesSampleRate float64
	// Component tags every event ("api", "worker").
	Component string
	// Lambda sends each event before the request returns instead of from a
	// background goroutine, so nothing outlives the invocation.
	Lambda bool
	// Transport overrides how events are sent (tests).
	Transport sentry.Transport
}

const flushTimeout = 2 * time.Second

// Init starts Sentry. It returns a flush function to call before the process
// exits (or, on Lambda, before an invocation returns); it is a no-op when
// telemetry is off.
func Init(o Options) (flush func(), err error) {
	if o.DSN == "" {
		return func() {}, nil
	}
	opts := sentry.ClientOptions{
		Dsn:              o.DSN,
		Environment:      o.Environment,
		Release:          "scrapbook01-server@" + Version,
		EnableTracing:    o.TracesSampleRate > 0,
		TracesSampleRate: o.TracesSampleRate,
		SendDefaultPII:   false,
		DataCollection: &sentry.DataCollection{
			UserInfo:    sentry.Set(false),
			Cookies:     &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff},
			HTTPHeaders: &sentry.HeaderCollectionConfig{Request: &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff}, Response: &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff}},
			HTTPBodies:  []sentry.BodyType{},
			QueryParams: &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff},
		},
		// Outbound calls go to third parties (AI providers, WorkOS, Expo,
		// links' sites): never send them trace headers.
		TracePropagationTargets: []string{"^$"},
		BeforeSend:              scrub,
		BeforeSendTransaction:   scrub,
		Tags:                    map[string]string{"component": o.Component},
		Transport:               o.Transport,
	}
	if opts.Transport == nil && o.Lambda {
		opts.Transport = sentry.NewHTTPSyncTransport()
	}
	if err := sentry.Init(opts); err != nil {
		return func() {}, err
	}
	return func() { sentry.Flush(flushTimeout) }, nil
}

// Enabled reports whether Sentry is on.
func Enabled() bool { return sentry.CurrentHub().Client().IsEnabled() }

// scrub keeps events down to what this package sets on purpose.
func scrub(e *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if e.Request != nil {
		e.Request = &sentry.Request{Method: e.Request.Method, URL: e.Request.URL}
	}
	e.User = sentry.User{ID: e.User.ID}
	e.ServerName = ""
	return e
}

// CaptureError reports err with the request's or job's context.
func CaptureError(ctx context.Context, err error) {
	if err == nil || !Enabled() {
		return
	}
	hub(ctx).CaptureException(err)
}

// CapturePanic reports a recovered panic value.
func CapturePanic(ctx context.Context, v any) {
	if !Enabled() {
		return
	}
	hub(ctx).RecoverWithContext(ctx, v)
}

// SetUser attaches the signed-in user's id (never the email) to the current
// request's or job's events.
func SetUser(ctx context.Context, id string) {
	if !Enabled() {
		return
	}
	if h := sentry.GetHubFromContext(ctx); h != nil {
		h.Scope().SetUser(sentry.User{ID: id})
	}
}

func hub(ctx context.Context) *sentry.Hub {
	if h := sentry.GetHubFromContext(ctx); h != nil {
		return h
	}
	return sentry.CurrentHub()
}
