package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

func TestJevChoose(t *testing.T) {
	var got jevRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("request %s auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		io.WriteString(w, `{"model":"jev-latest","answers":{"answer":{"type":"choice","choice":"Recipes","confidence":0.91,
			"probabilities":{"Recipes":0.91,"Travel":0.04,"None of these":0.05}}},"usage":{"input_tokens":80,"output_tokens":0}}`)
	}))
	defer srv.Close()

	j := &Jev{BaseURL: srv.URL + "/", APIKey: "key", Model: "jev-latest"}
	c, err := j.Choose(context.Background(), "lemon pasta", "Pick one", map[string]string{
		"Recipes": "Recent items: carbonara", "Travel": "", "None of these": "nothing fits",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Label != "Recipes" || c.Confidence != 0.91 || c.Probabilities["Travel"] != 0.04 {
		t.Fatalf("choice = %+v", c)
	}
	q := got.Questions["answer"]
	if got.Model != "jev-latest" || got.State != "lemon pasta" || q.Type != "choice" || q.Criteria["Travel"] != nil ||
		*q.Criteria["Recipes"] != "Recent items: carbonara" {
		t.Fatalf("request = %+v", got)
	}
}

func TestJevErrors(t *testing.T) {
	answer := `{"answers":{"answer":{"type":"choice","choice":"Made up","confidence":1}}}`
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		io.WriteString(w, answer)
	}))
	defer srv.Close()
	j := &Jev{BaseURL: srv.URL, APIKey: "k", Model: "m"}
	opts := map[string]string{"A": ""}

	if _, err := j.Choose(context.Background(), "x", "y", opts); err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("unknown option: err = %v", err)
	}
	for code, retryable := range map[int]bool{429: true, 503: true, 400: false, 401: false} {
		status, answer = code, `{"error":"nope"}`
		_, err := j.Choose(context.Background(), "x", "y", opts)
		var he *HTTPError
		if !errors.As(err, &he) || he.Retryable() != retryable {
			t.Fatalf("HTTP %d: err = %v", code, err)
		}
	}
}

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path != "/api/v1/embeddings" || req.Model != "openai/text-embedding-3-small" || len(req.Input) != 2 {
			t.Errorf("request %s %+v", r.URL.Path, req)
		}
		vec := func(v float32) []float32 {
			out := make([]float32, Dimensions)
			out[0] = v
			return out
		}
		// Out of order on purpose: the client must sort by index.
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"index": 1, "embedding": vec(2)},
			{"index": 0, "embedding": vec(1)},
		}})
	}))
	defer srv.Close()

	e := &Embedder{BaseURL: srv.URL + "/api/v1", APIKey: "k", Model: "openai/text-embedding-3-small"}
	vecs, err := e.Embed(context.Background(), "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if vecs[0][0] != 1 || vecs[1][0] != 2 {
		t.Fatalf("order: %v %v", vecs[0][0], vecs[1][0])
	}
}

func TestEmbedRejectsWrongDimensions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[{"index":0,"embedding":[0.1,0.2]}]}`)
	}))
	defer srv.Close()
	e := &Embedder{BaseURL: srv.URL, APIKey: "k", Model: "small"}
	if _, err := e.Embed(context.Background(), "a"); err == nil || !strings.Contains(err.Error(), "2 dimensions") {
		t.Fatalf("err = %v", err)
	}
}

func TestVectorLiteral(t *testing.T) {
	if got := VectorLiteral([]float32{0.5, -1, 0.125}); got != "[0.5,-1,0.125]" {
		t.Fatalf("got %s", got)
	}
}

func TestCleanCategoryName(t *testing.T) {
	for in, want := range map[string]string{
		"Parenting":                  "Parenting",
		"  \"Recetas\".\n\nBecause…": "Recetas",
		"**Viajes**":                 "Viajes",
		"Arts & crafts":              "Arts & crafts",
		"Music   videos!":            "Music videos",
		strings.Repeat("a", 50):      strings.Repeat("a", 40),
		"   ":                        "",
	} {
		if got := CleanCategoryName(in); got != want {
			t.Errorf("CleanCategoryName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNamer(t *testing.T) {
	var body map[string]any
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
			return
		}
		io.WriteString(w, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-5-5",
			"content":[{"type":"thinking","thinking":"","signature":"s"},{"type":"text","text":"Crianza\n"}],
			"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":120,"output_tokens":3}}`)
	}))
	defer srv.Close()

	n := NewNamer("test-key", "claude-haiku-5-5", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	name, err := n.Name(context.Background(), "5 cosas para hacer con tu hija", []string{"Recetas", "Viajes"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "Crianza" {
		t.Fatalf("name = %q", name)
	}
	if body["model"] != "claude-haiku-5-5" {
		t.Fatalf("model = %v", body["model"])
	}
	if oc, _ := body["output_config"].(map[string]any); oc["effort"] != "low" {
		t.Fatalf("output_config = %v", body["output_config"])
	}
	msg, _ := json.Marshal(body["messages"])
	if !strings.Contains(string(msg), "Recetas, Viajes") || !strings.Contains(string(msg), "con tu hija") {
		t.Fatalf("messages = %s", msg)
	}

	status = http.StatusUnauthorized
	if _, err := n.Name(context.Background(), "x", nil); !errors.Is(err, ErrRefused) {
		t.Fatalf("401: err = %v, want ErrRefused", err)
	}
}

func TestMetering(t *testing.T) {
	jev := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"answers":{"answer":{"choice":"A","confidence":1}},"usage":{"input_tokens":1000,"output_tokens":0}}`)
	}))
	defer jev.Close()
	emb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := make([]float32, Dimensions)
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"index": 0, "embedding": v}}, "usage": map[string]int{"prompt_tokens": 50}})
	}))
	defer emb.Close()
	claude := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"m","type":"message","role":"assistant","model":"claude-haiku-5-5","content":[{"type":"text","text":"Recipes"}],
			"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":200,"output_tokens":300}}`)
	}))
	defer claude.Close()

	ctx, meter := WithMeter(context.Background())
	j := &Jev{BaseURL: jev.URL, APIKey: "k", Model: "jev-latest", Price: Price{Input: 0.042}}
	if _, err := j.Choose(ctx, "s", "i", map[string]string{"A": ""}); err != nil {
		t.Fatal(err)
	}
	e := &Embedder{BaseURL: emb.URL, APIKey: "k", Model: "small", Price: Price{Input: 0.02}}
	if _, err := e.Embed(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	n := NewNamer("k", "claude-haiku-5-5", option.WithBaseURL(claude.URL), option.WithMaxRetries(0))
	n.Price = Price{Input: 0.10, Output: 0.50}
	if _, err := n.Name(ctx, "lemon pasta", nil); err != nil {
		t.Fatal(err)
	}

	recs := meter.Records()
	if len(recs) != 3 {
		t.Fatalf("records = %+v", recs)
	}
	// $/M tokens = µ$/token: 1000×0.042 = 42, 50×0.02 = 1, 200×0.1 + 300×0.5 = 170.
	want := map[string]int64{"jev": 42, "embeddings": 1, "claude": 170}
	for _, r := range recs {
		if r.Cost != want[r.Service] {
			t.Errorf("%s cost = %d, want %d (%+v)", r.Service, r.Cost, want[r.Service], r)
		}
	}
	if meter.Total() != 213 {
		t.Fatalf("total = %d", meter.Total())
	}

	// Calls outside a metered context are simply not recorded.
	if _, err := e.Embed(context.Background(), "x"); err != nil || len(meter.Records()) != 3 {
		t.Fatalf("unmetered call recorded: %v", err)
	}
}
