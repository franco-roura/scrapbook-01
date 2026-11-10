// Command fakeai stands in for TypeSafe (Jev), the embeddings API and the
// Claude Messages API during local development, so the whole save → sort →
// link flow runs without keys or cost. Answers are crude but deterministic:
// texts that share words are similar, and new categories are named after the
// description's longest word.
//
//	go run ./cmd/fakeai   # listens on :8787
//	TYPESAFE_API_KEY=dev TYPESAFE_BASE_URL=http://localhost:8787 \
//	EMBEDDINGS_API_KEY=dev EMBEDDINGS_BASE_URL=http://localhost:8787 \
//	ANTHROPIC_API_KEY=dev ANTHROPIC_BASE_URL=http://localhost:8787 go run ./cmd/api
//
// Never point a deployed server at this.
package main

import (
	"encoding/json"
	"hash/fnv"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"unicode"
)

const dims = 1536

func main() {
	addr := os.Getenv("FAKEAI_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/systemone", jev)
	mux.HandleFunc("POST /embeddings", embeddings)
	mux.HandleFunc("POST /v1/messages", messages)
	log.Printf("fakeai listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}

func jev(w http.ResponseWriter, r *http.Request) {
	var req struct {
		State     string `json:"state"`
		Questions map[string]struct {
			Criteria map[string]*string `json:"criteria"`
		} `json:"questions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	answers := map[string]any{}
	for id, q := range req.Questions {
		best, bestScore, fallback := "", 0, ""
		for label, desc := range q.Criteria {
			if strings.HasPrefix(label, "None") {
				fallback = label
				continue
			}
			hay := strings.ToLower(label)
			if desc != nil {
				hay += " " + strings.ToLower(*desc)
			}
			score := 0
			for _, wd := range words(req.State) {
				if len(wd) > 3 && strings.Contains(hay, wd) {
					score++
				}
			}
			if score > bestScore {
				best, bestScore = label, score
			}
		}
		if best == "" {
			best = fallback
		}
		answers[id] = map[string]any{"type": "choice", "choice": best, "confidence": 0.9, "probabilities": map[string]float64{best: 0.9}}
	}
	writeJSON(w, map[string]any{"model": "fake-jev", "answers": answers, "usage": map[string]int{"input_tokens": 0, "output_tokens": 0}})
}

func embeddings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input []string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data := make([]map[string]any, len(req.Input))
	for i, in := range req.Input {
		v := make([]float32, dims)
		for _, wd := range words(in) {
			h := fnv.New32a()
			h.Write([]byte(wd))
			v[h.Sum32()%dims]++
		}
		var norm float64
		for _, x := range v {
			norm += float64(x * x)
		}
		if norm > 0 {
			for j := range v {
				v[j] /= float32(math.Sqrt(norm))
			}
		}
		data[i] = map[string]any{"index": i, "embedding": v}
	}
	writeJSON(w, map[string]any{"data": data})
}

func messages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	text := ""
	if len(req.Messages) > 0 && len(req.Messages[0].Content) > 0 {
		text = req.Messages[0].Content[0].Text
	}
	// The namer prompt is "Item description:\n<desc>\n\nExisting categories: ..."
	desc, _, _ := strings.Cut(strings.TrimPrefix(text, "Item description:\n"), "\n\nExisting categories:")
	name := "Misc"
	for _, wd := range words(desc) {
		if len([]rune(wd)) > len([]rune(name)) || name == "Misc" && len(wd) > 3 {
			name = wd
		}
	}
	name = strings.ToUpper(name[:1]) + name[1:]
	writeJSON(w, map[string]any{
		"id": "msg_fake", "type": "message", "role": "assistant", "model": "fake",
		"content":     []map[string]string{{"type": "text", "text": name}},
		"stop_reason": "end_turn", "stop_sequence": nil,
		"usage": map[string]int{"input_tokens": 0, "output_tokens": 0},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
