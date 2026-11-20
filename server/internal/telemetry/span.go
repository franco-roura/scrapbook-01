package telemetry

import (
	"context"
	"errors"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
)

// Span is a running child span. A nil *Span (telemetry off, or no request or
// job around it) is valid and does nothing.
type Span struct{ s *sentry.Span }

// StartSpan starts a child of the context's current span. It never starts a
// transaction of its own, so calls outside a request or job aren't traced.
func StartSpan(ctx context.Context, op, description string) (context.Context, *Span) {
	parent := sentry.SpanFromContext(ctx)
	if parent == nil {
		return ctx, nil
	}
	s := parent.StartChild(op, sentry.WithDescription(description))
	return s.Context(), &Span{s: s}
}

// SetData adds an attribute.
func (s *Span) SetData(key string, value any) {
	if s != nil {
		s.s.SetData(key, value)
	}
}

// End finishes the span, marked failed when err is set.
func (s *Span) End(err error) {
	if s == nil {
		return
	}
	if err != nil {
		s.s.Status = sentry.SpanStatusInternalError
	} else {
		s.s.Status = sentry.SpanStatusOK
	}
	s.s.Finish()
}

// AnnotateAI adds a model call's usage to the context's current span (the
// span the AI client started around the call).
func AnnotateAI(ctx context.Context, service, model string, inputTokens, outputTokens, costMicros int64) {
	s := sentry.SpanFromContext(ctx)
	if s == nil {
		return
	}
	s.SetData("gen_ai.system", service)
	s.SetData("gen_ai.request.model", model)
	s.SetData("gen_ai.usage.input_tokens", inputTokens)
	s.SetData("gen_ai.usage.output_tokens", outputTokens)
	s.SetData("scrapbook.cost_micro_usd", costMicros)
}

// Job starts a transaction for one background job ("worker classify"). The
// trace arguments continue the trace of the request that queued it; empty
// strings start a new trace. finish reports err and ends the transaction.
func Job(ctx context.Context, name, sentryTrace, baggage string) (context.Context, func(err error)) {
	if !Enabled() {
		return ctx, func(error) {}
	}
	h := sentry.CurrentHub().Clone()
	ctx = sentry.SetHubOnContext(ctx, h)
	h.Scope().SetTag("job", name)
	tx := sentry.StartTransaction(ctx, name,
		sentry.ContinueTrace(h, sentryTrace, baggage),
		sentry.WithOpName("queue.process"),
		sentry.WithTransactionSource(sentry.SourceTask),
	)
	return tx.Context(), func(err error) {
		if err != nil {
			h.CaptureException(err)
			tx.Status = sentry.SpanStatusInternalError
		} else {
			tx.Status = sentry.SpanStatusOK
		}
		tx.Finish()
	}
}

// TraceHeaders returns the sentry-trace and baggage values for handing the
// current trace to a background job. Both are empty when there's no span.
func TraceHeaders(ctx context.Context) (sentryTrace, baggage string) {
	s := sentry.SpanFromContext(ctx)
	if s == nil {
		return "", ""
	}
	return s.ToSentryTrace(), s.ToBaggage()
}

type querySpanKey struct{}

// QueryTracer is a pgx tracer that records each sqlc query as a "db" span
// inside the current request or job. Queries without a sqlc name header
// (River's queue, migrations) and queries outside any span are skipped.
// Arguments are never recorded.
type QueryTracer struct{}

func (QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	parent := sentry.SpanFromContext(ctx)
	if parent == nil {
		return ctx
	}
	name, sql, ok := sqlcQuery(d.SQL)
	if !ok {
		return ctx
	}
	s := parent.StartChild("db.sql.query", sentry.WithDescription(sql))
	s.SetData("db.system", "postgresql")
	s.SetData("db.operation.name", name)
	return context.WithValue(ctx, querySpanKey{}, s)
}

func (QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	s, ok := ctx.Value(querySpanKey{}).(*sentry.Span)
	if !ok {
		return
	}
	if d.Err != nil && !errors.Is(d.Err, pgx.ErrNoRows) {
		s.Status = sentry.SpanStatusInternalError
	} else {
		s.Status = sentry.SpanStatusOK
	}
	s.SetData("db.rows_affected", d.CommandTag.RowsAffected())
	s.Finish()
}

// sqlcQuery splits sqlc's "-- name: GetItem :one" header from the statement
// and collapses its whitespace, so Sentry groups the same query together.
func sqlcQuery(text string) (name, sql string, ok bool) {
	rest, found := strings.CutPrefix(text, "-- name: ")
	if !found {
		return "", "", false
	}
	header, body, _ := strings.Cut(rest, "\n")
	name, _, _ = strings.Cut(strings.TrimSpace(header), " ")
	if name == "" {
		return "", "", false
	}
	return name, strings.Join(strings.Fields(body), " "), true
}
