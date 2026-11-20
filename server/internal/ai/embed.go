package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/franco-roura/scrapbook-01/server/internal/telemetry"
)

// Dimensions is the embedding size the database columns are declared with.
const Dimensions = 1536

// Embedder calls an OpenAI-compatible /embeddings endpoint (OpenRouter by
// default, or OpenAI directly).
type Embedder struct {
	BaseURL string // e.g. https://openrouter.ai/api/v1
	APIKey  string
	Model   string // e.g. openai/text-embedding-3-small
	HTTP    *http.Client
	// Price per million input tokens, for metering ($0.02/M for
	// text-embedding-3-small).
	Price Price
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int64 `json:"prompt_tokens"`
	} `json:"usage"`
}

// Embed returns one vector per input, in input order.
func (e *Embedder) Embed(ctx context.Context, inputs ...string) (_ [][]float32, err error) {
	ctx, span := telemetry.StartSpan(ctx, "gen_ai.embeddings", "embeddings "+e.Model)
	defer func() { span.End(err) }()
	body, err := json.Marshal(embedRequest{Model: e.Model, Input: inputs})
	if err != nil {
		return nil, err
	}
	c := e.HTTP
	if c == nil {
		c = defaultHTTP
	}
	var resp embedResponse
	if err := postJSON(ctx, c, strings.TrimRight(e.BaseURL, "/")+"/embeddings", e.APIKey, body, &resp); err != nil {
		return nil, fmt.Errorf("embeddings: %w", err)
	}
	tokens := resp.Usage.PromptTokens
	if tokens == 0 {
		for _, in := range inputs {
			tokens += estimateTokens(in)
		}
	}
	record(ctx, Usage{Service: "embeddings", Model: e.Model, InputTokens: tokens, Cost: e.Price.cost(tokens, 0)})
	if len(resp.Data) != len(inputs) {
		return nil, fmt.Errorf("embeddings: got %d vectors for %d inputs", len(resp.Data), len(inputs))
	}
	sort.Slice(resp.Data, func(i, j int) bool { return resp.Data[i].Index < resp.Data[j].Index })
	out := make([][]float32, len(inputs))
	for i, d := range resp.Data {
		if len(d.Embedding) != Dimensions {
			return nil, fmt.Errorf("embeddings: model %q returned %d dimensions, the database expects %d", e.Model, len(d.Embedding), Dimensions)
		}
		out[i] = d.Embedding
	}
	return out, nil
}

// estimateTokens is a rough count (~4 characters per token) for providers
// that don't report usage.
func estimateTokens(s string) int64 { return int64(len(s)/4 + 1) }

// VectorLiteral formats v the way pgvector parses text: "[0.1,0.2,...]".
func VectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 10)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
