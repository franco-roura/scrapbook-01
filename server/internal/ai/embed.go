package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
}

// Embed returns one vector per input, in input order.
func (e *Embedder) Embed(ctx context.Context, inputs ...string) ([][]float32, error) {
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
