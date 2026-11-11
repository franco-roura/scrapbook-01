//go:build integration

// Run against the docker-compose database:
//
//	TEST_DATABASE_URL=postgres://scrapbook:scrapbook@localhost:54329/scrapbook?sslmode=disable \
//	  go test -tags integration ./internal/mcpserver/
package mcpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pressly/goose/v3"

	"github.com/aaugoustis/waitlist/server/db/migrations"
	"github.com/aaugoustis/waitlist/server/internal/ai"
	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/db"
	"github.com/aaugoustis/waitlist/server/internal/mcpserver"
)

const issuer = "https://test.supabase.co/auth/v1"

var secret = []byte("test-secret-at-least-32-bytes-long!!")

func token(t *testing.T, user uuid.UUID) string {
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.String(), "iss": issuer, "aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(), "client_id": "claude-connector",
	}).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type bagOfWords struct{}

func (bagOfWords) Embed(_ context.Context, inputs ...string) ([][]float32, error) {
	out := make([][]float32, len(inputs))
	for i, in := range inputs {
		v := make([]float32, ai.Dimensions)
		for _, w := range strings.Fields(strings.ToLower(in)) {
			h := fnv.New32a()
			h.Write([]byte(w))
			v[h.Sum32()%ai.Dimensions]++
		}
		var n float64
		for _, x := range v {
			n += float64(x * x)
		}
		for j := range v {
			v[j] /= float32(math.Sqrt(n))
		}
		out[i] = v
	}
	return out, nil
}

type bearer struct {
	token string
	base  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

type fixture struct {
	q      *db.Queries
	srv    *httptest.Server
	user   uuid.UUID
	ids    map[string]uuid.UUID
	logger *slog.Logger
}

func setup(t *testing.T, embed mcpserver.Embedder, tweaks ...func(*mcpserver.Options)) *fixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	p, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(pool), migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	f := &fixture{q: q, user: uuid.New(), ids: map[string]uuid.UUID{}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	recipes, err := q.InsertCategory(ctx, db.InsertCategoryParams{UserID: f.user, Name: "Recipes"})
	if err != nil {
		t.Fatal(err)
	}
	for key, desc := range map[string]string{
		"pasta": "easy lemon pasta recipe",
		"cake":  "lemon cake recipe for birthdays",
		"trip":  "weekend trip to colonia del sacramento",
	} {
		d := desc
		row, err := q.UpsertItem(ctx, db.UpsertItemParams{UserID: f.user, Url: "https://example.com/" + key, Platform: "web", Description: &d})
		if err != nil {
			t.Fatal(err)
		}
		f.ids[key] = row.ID
		vec, _ := bagOfWords{}.Embed(ctx, desc)
		if err := q.SetItemEmbedding(ctx, db.SetItemEmbeddingParams{ID: row.ID, Embedding: ai.VectorLiteral(vec[0])}); err != nil {
			t.Fatal(err)
		}
		if key != "trip" {
			if _, err := pool.Exec(ctx, "UPDATE items SET category_id = $1 WHERE id = $2", recipes.ID, row.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, id := range f.ids {
		if err := q.InsertSimilarEdges(ctx, db.InsertSimilarEdgesParams{ItemID: id, UserID: f.user, K: 5, MinSimilarity: 0.3}); err != nil {
			t.Fatal(err)
		}
	}
	// Another user's item that must never show up.
	other := "lemon pasta recipe from someone else"
	if _, err := q.UpsertItem(ctx, db.UpsertItemParams{UserID: uuid.New(), Url: "https://example.com/other", Platform: "web", Description: &other}); err != nil {
		t.Fatal(err)
	}

	opts := mcpserver.Options{
		Queries: q, Verifier: auth.NewHS256(secret, issuer), Embed: embed, Logger: f.logger,
		PublicURL: "https://api.example.test", AuthServer: issuer,
	}
	for _, tw := range tweaks {
		tw(&opts)
	}
	s := mcpserver.New(opts)
	f.srv = httptest.NewServer(s.Handler())
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fixture) connect(t *testing.T, user uuid.UUID) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   f.srv.URL + mcpserver.Path,
		HTTPClient: &http.Client{Transport: bearer{token: token(t, user), base: http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call[T any](t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (T, bool) {
	t.Helper()
	var out T
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		return out, false
	}
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%s: decode %s: %v", name, b, err)
	}
	return out, true
}

func TestAuthAndMetadata(t *testing.T) {
	f := setup(t, nil)

	res, err := http.Post(f.srv.URL+mcpserver.Path, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized ||
		!strings.Contains(res.Header.Get("WWW-Authenticate"), `resource_metadata="https://api.example.test/.well-known/oauth-protected-resource/mcp"`) {
		t.Fatalf("unauthenticated: %d %q", res.StatusCode, res.Header.Get("WWW-Authenticate"))
	}

	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		res, err := http.Get(f.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var meta struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
		}
		_ = json.NewDecoder(res.Body).Decode(&meta)
		res.Body.Close()
		if meta.Resource != "https://api.example.test/mcp" || len(meta.AuthorizationServers) != 1 || meta.AuthorizationServers[0] != issuer {
			t.Fatalf("%s: %+v", path, meta)
		}
	}
}

func TestTools(t *testing.T) {
	f := setup(t, bagOfWords{})
	cs := f.connect(t, f.user)

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
	}
	if strings.Join(names, ",") != "get_item,get_neighbors,save_item,search_library" {
		t.Fatalf("tools = %v", names)
	}

	type result struct {
		ID, Title, Category string
		Score               float64
	}
	search, _ := call[struct {
		Results []result
		Mode    string
	}](t, cs, "search_library", map[string]any{"query": "lemon pasta recipe", "limit": 2})
	if search.Mode != "semantic" || len(search.Results) != 2 || search.Results[0].ID != f.ids["pasta"].String() || search.Results[0].Category != "Recipes" {
		t.Fatalf("search: %+v", search)
	}
	for _, r := range search.Results {
		if strings.Contains(r.Title, "someone else") {
			t.Fatal("search leaked another user's item")
		}
	}

	item, _ := call[struct {
		Title   string
		Related []result
	}](t, cs, "get_item", map[string]any{"id": f.ids["pasta"].String()})
	if item.Title != "easy lemon pasta recipe" || len(item.Related) == 0 || item.Related[0].ID != f.ids["cake"].String() {
		t.Fatalf("get_item: %+v", item)
	}

	nb, _ := call[struct {
		Items      []result
		Categories []string
	}](t, cs, "get_neighbors", map[string]any{"id": f.ids["cake"].String()})
	if len(nb.Items) == 0 || nb.Items[0].ID != f.ids["pasta"].String() || len(nb.Categories) != 1 {
		t.Fatalf("get_neighbors: %+v", nb)
	}

	saved, _ := call[struct {
		ID      string
		URL     string
		Created bool
	}](t, cs, "save_item", map[string]any{"url": "Look https://www.youtube.com/watch?v=abc&si=x", "note": "knife skills video"})
	if !saved.Created || saved.URL != "https://youtube.com/watch?v=abc" {
		t.Fatalf("save_item: %+v", saved)
	}
	again, _ := call[struct{ Created bool }](t, cs, "save_item", map[string]any{"url": "https://youtube.com/watch?v=abc"})
	if again.Created {
		t.Fatal("saving the same link twice created two items")
	}

	// Another user can't read these items by id.
	stranger := f.connect(t, uuid.New())
	if _, ok := call[struct{}](t, stranger, "get_item", map[string]any{"id": f.ids["pasta"].String()}); ok {
		t.Fatal("stranger read the item")
	}
}

func TestKeywordFallback(t *testing.T) {
	f := setup(t, nil) // no embeddings configured
	cs := f.connect(t, f.user)
	search, _ := call[struct {
		Results []struct{ ID string }
		Mode    string
	}](t, cs, "search_library", map[string]any{"query": "things to do in Colonia"})
	if search.Mode != "keyword" || len(search.Results) != 1 || search.Results[0].ID != f.ids["trip"].String() {
		t.Fatalf("keyword search: %+v", search)
	}
	none, _ := call[struct{ Results []struct{ ID string } }](t, cs, "search_library", map[string]any{"query": "100%_"})
	if len(none.Results) != 0 {
		t.Fatalf("wildcards weren't escaped: %+v", none)
	}
}

// preferring is a fake Jev that puts all probability on the option whose
// text contains word.
type preferring struct {
	word string
	fail bool
}

func (p preferring) Choose(_ context.Context, _, _ string, options map[string]string) (ai.Choice, error) {
	if p.fail {
		return ai.Choice{}, errors.New("jev down")
	}
	c := ai.Choice{Probabilities: map[string]float64{}}
	for label, text := range options {
		if strings.Contains(text, p.word) {
			c.Label, c.Confidence, c.Probabilities[label] = label, 0.9, 0.9
		} else {
			c.Probabilities[label] = 0.1 / float64(len(options))
		}
	}
	return c, nil
}

func TestRerank(t *testing.T) {
	type out struct {
		Results []struct{ ID string }
		Mode    string
	}
	args := map[string]any{"query": "lemon pasta recipe", "limit": 2}

	f := setup(t, bagOfWords{}, func(o *mcpserver.Options) { o.Rerank = preferring{word: "cake"} })
	got, _ := call[out](t, f.connect(t, f.user), "search_library", args)
	if got.Mode != "semantic+rerank" || len(got.Results) != 2 || got.Results[0].ID != f.ids["cake"].String() || got.Results[1].ID != f.ids["pasta"].String() {
		t.Fatalf("reranked: %+v (cake %s, pasta %s)", got, f.ids["cake"], f.ids["pasta"])
	}

	f = setup(t, bagOfWords{}, func(o *mcpserver.Options) { o.Rerank = preferring{fail: true} })
	got, _ = call[out](t, f.connect(t, f.user), "search_library", args)
	if got.Mode != "semantic" || len(got.Results) != 2 || got.Results[0].ID != f.ids["pasta"].String() {
		t.Fatalf("rerank failure should keep vector order: %+v", got)
	}
}
