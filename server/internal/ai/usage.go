package ai

import (
	"context"
	"math"
	"sync"
)

// Usage is one model call's token counts and cost. Cost is in micro-dollars
// (1 = $0.000001). Prices quoted in dollars per million tokens are exactly
// micro-dollars per token, so cost = tokens × price.
type Usage struct {
	Service      string `json:"service"` // "jev", "embeddings", "claude"
	Model        string `json:"model"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	Cost         int64  `json:"cost"`
}

// Price is a model's list price in dollars per million tokens.
type Price struct {
	Input  float64
	Output float64
}

func (p Price) cost(in, out int64) int64 {
	return int64(math.Ceil(float64(in)*p.Input + float64(out)*p.Output))
}

// Meter collects the usage of every model call made with its context.
type Meter struct {
	mu      sync.Mutex
	records []Usage
}

type meterKey struct{}

// WithMeter returns a context whose model calls are recorded on the meter.
func WithMeter(ctx context.Context) (context.Context, *Meter) {
	m := &Meter{}
	return context.WithValue(ctx, meterKey{}, m), m
}

func record(ctx context.Context, u Usage) {
	if m, ok := ctx.Value(meterKey{}).(*Meter); ok {
		m.mu.Lock()
		m.records = append(m.records, u)
		m.mu.Unlock()
	}
}

// Records returns a copy of what was recorded.
func (m *Meter) Records() []Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Usage(nil), m.records...)
}

// Total is the summed cost in micro-dollars.
func (m *Meter) Total() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var t int64
	for _, r := range m.records {
		t += r.Cost
	}
	return t
}

// RecordUsage records a call made by a client outside this package (or a
// test double) on the context's meter, if any.
func RecordUsage(ctx context.Context, u Usage) { record(ctx, u) }
