// Package ai holds the clients for the models the classifier calls: Jev
// (TypeSafe) to pick a category, an embeddings endpoint, and Claude Haiku to
// name new categories. Each client is small and talks plain HTTP or the
// provider's official SDK.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Jev calls TypeSafe's Jev decision model (POST /v1/systemone).
// Docs: https://docs.typesafe.ai/introduction/quickstart
type Jev struct {
	BaseURL string // e.g. https://api.typesafe.ai
	APIKey  string
	Model   string // e.g. jev-latest
	HTTP    *http.Client
	// Price per million tokens, for metering. Jev reportedly bills input
	// only (about $0.042/M); check TypeSafe's live pricing.
	Price Price
}

// Choice is Jev's answer to a "choice" question.
type Choice struct {
	Label         string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type jevQuestion struct {
	Type         string             `json:"type"`
	Instructions string             `json:"instructions"`
	Criteria     map[string]*string `json:"criteria,omitempty"`
}

type jevRequest struct {
	Model     string                 `json:"model"`
	State     string                 `json:"state"`
	Questions map[string]jevQuestion `json:"questions"`
}

type jevResponse struct {
	Answers map[string]json.RawMessage `json:"answers"`
	Usage   struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

// Choose asks Jev which of the options (label → description, which may be
// empty) fits state best.
func (j *Jev) Choose(ctx context.Context, state, instructions string, options map[string]string) (Choice, error) {
	criteria := make(map[string]*string, len(options))
	for label, desc := range options {
		if desc == "" {
			criteria[label] = nil
		} else {
			d := desc
			criteria[label] = &d
		}
	}
	body, err := json.Marshal(jevRequest{
		Model: j.Model,
		State: state,
		Questions: map[string]jevQuestion{
			"answer": {Type: "choice", Instructions: instructions, Criteria: criteria},
		},
	})
	if err != nil {
		return Choice{}, err
	}

	var resp jevResponse
	if err := postJSON(ctx, j.client(), strings.TrimRight(j.BaseURL, "/")+"/v1/systemone", j.APIKey, body, &resp); err != nil {
		return Choice{}, fmt.Errorf("jev: %w", err)
	}
	in, out := resp.Usage.InputTokens, resp.Usage.OutputTokens
	if in == 0 {
		in = estimateTokens(state) + estimateTokens(instructions) + int64(len(options))*20
	}
	record(ctx, Usage{Service: "jev", Model: j.Model, InputTokens: in, OutputTokens: out, Cost: j.Price.cost(in, out)})
	raw, ok := resp.Answers["answer"]
	if !ok {
		return Choice{}, fmt.Errorf("jev: response has no answer")
	}
	var c Choice
	if err := json.Unmarshal(raw, &c); err != nil {
		return Choice{}, fmt.Errorf("jev: decode answer: %w", err)
	}
	if _, known := options[c.Label]; !known {
		return Choice{}, fmt.Errorf("jev: answered unknown option %q", c.Label)
	}
	return c, nil
}

func (j *Jev) client() *http.Client {
	if j.HTTP != nil {
		return j.HTTP
	}
	return defaultHTTP
}

var defaultHTTP = &http.Client{Timeout: 30 * time.Second}

// postJSON sends body with a bearer token and decodes a 2xx JSON response.
func postJSON(ctx context.Context, c *http.Client, url, token string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		return &HTTPError{Status: res.StatusCode, Body: string(truncate(data, 300))}
	}
	return json.Unmarshal(data, out)
}

// HTTPError is a non-2xx response. Retryable reports whether trying again
// later could help (rate limits and server errors).
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body) }

func (e *HTTPError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

func truncate(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}
