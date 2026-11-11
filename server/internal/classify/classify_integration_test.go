//go:build integration

// Run against the docker-compose database:
//
//	TEST_DATABASE_URL=postgres://scrapbook:scrapbook@localhost:54329/scrapbook?sslmode=disable \
//	  go test -tags integration ./internal/classify/
package classify_test

import (
	"context"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/aaugoustis/waitlist/server/db/migrations"
	"github.com/aaugoustis/waitlist/server/internal/ai"
	"github.com/aaugoustis/waitlist/server/internal/classify"
	"github.com/aaugoustis/waitlist/server/internal/credits"
	"github.com/aaugoustis/waitlist/server/internal/db"
	"github.com/aaugoustis/waitlist/server/internal/jobs"
)

func setup(t *testing.T) (*db.Queries, *pgxpool.Pool) {
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
	return db.New(pool), pool
}

// bagOfWords embeds text by hashing its words: texts sharing words are close.
type bagOfWords struct{ calls int }

func (b *bagOfWords) Embed(_ context.Context, inputs ...string) ([][]float32, error) {
	b.calls++
	out := make([][]float32, len(inputs))
	for i, in := range inputs {
		v := make([]float32, ai.Dimensions)
		for _, w := range strings.Fields(strings.ToLower(in)) {
			h := fnv.New32a()
			h.Write([]byte(w))
			v[h.Sum32()%ai.Dimensions]++
		}
		var norm float64
		for _, x := range v {
			norm += float64(x * x)
		}
		for j := range v {
			v[j] /= float32(math.Sqrt(norm))
		}
		out[i] = v
	}
	return out, nil
}

// wordJev picks the option whose name or examples share the most words with
// the state; with no overlap it answers "None of these".
type wordJev struct {
	mu    sync.Mutex
	calls int
	force string // when set, always answer this
}

func (j *wordJev) Choose(_ context.Context, state, _ string, options map[string]string) (ai.Choice, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls++
	if j.force != "" {
		return ai.Choice{Label: j.force, Confidence: 0.9}, nil
	}
	words := strings.Fields(strings.ToLower(state))
	best, bestScore := classify.NoneOfThese, 0
	for label, desc := range options {
		if label == classify.NoneOfThese {
			continue
		}
		hay := strings.ToLower(label + " " + desc)
		score := 0
		for _, w := range words {
			if len(w) > 3 && strings.Contains(hay, w) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = label, score
		}
	}
	return ai.Choice{Label: best, Confidence: 0.9}, nil
}

type fixedNamer struct {
	names []string
	calls int
}

func (n *fixedNamer) Name(context.Context, string, []string) (string, error) {
	name := n.names[n.calls%len(n.names)]
	n.calls++
	return name, nil
}

type recordingNotifier struct{ got []string }

func (r *recordingNotifier) CategoryAssigned(_ context.Context, _, _ uuid.UUID, category string) error {
	r.got = append(r.got, category)
	return nil
}

func save(t *testing.T, q *db.Queries, user uuid.UUID, url, desc string) uuid.UUID {
	t.Helper()
	row, err := q.UpsertItem(context.Background(), db.UpsertItemParams{UserID: user, Url: url, Platform: "web", Description: &desc})
	if err != nil {
		t.Fatal(err)
	}
	if !row.NeedsClassifying {
		t.Fatalf("new described item %s doesn't need classifying", url)
	}
	return row.ID
}

func TestClassifierEndToEnd(t *testing.T) {
	q, pool := setup(t)
	ctx := context.Background()
	user := uuid.New()

	emb, jev, notify := &bagOfWords{}, &wordJev{}, &recordingNotifier{}
	namer := &fixedNamer{names: []string{"Parenting", "Recipes", "recipes"}}
	c := &classify.Classifier{
		Q: q, Jev: jev, Embed: emb, Namer: namer, Notify: notify,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Threshold: 0.6, SameName: 0.85, Neighbors: 5, MinSimilarity: 0.45,
	}

	// 1. First item ever: no categories, so Haiku names one. No Jev call.
	a := save(t, q, user, "https://e.com/a", "fun things to do with my daughter")
	res, err := c.Run(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if res.Category != "Parenting" || !res.NewCategory || !res.Changed || jev.calls != 0 || namer.calls != 1 {
		t.Fatalf("first item: %+v (jev %d, namer %d)", res, jev.calls, namer.calls)
	}

	// 2. A similar item: Jev files it into Parenting; no new name needed.
	b := save(t, q, user, "https://e.com/b", "fun weekend things to do with my daughter")
	if res, err = c.Run(ctx, b); err != nil {
		t.Fatal(err)
	}
	if res.Category != "Parenting" || res.NewCategory || jev.calls != 1 || namer.calls != 1 {
		t.Fatalf("second item: %+v (jev %d, namer %d)", res, jev.calls, namer.calls)
	}
	edges, err := q.ListEdgesForUser(ctx, user)
	if err != nil || len(edges) != 1 || edges[0].Weight < 0.45 {
		t.Fatalf("edges after two similar items: %+v %v", edges, err)
	}

	// 3. Unrelated item: Jev says none fit, Haiku names "Recipes".
	r1 := save(t, q, user, "https://e.com/r1", "lemon pasta recipe")
	if res, err = c.Run(ctx, r1); err != nil {
		t.Fatal(err)
	}
	if res.Category != "Recipes" || !res.NewCategory {
		t.Fatalf("recipe: %+v", res)
	}
	if edges, _ = q.ListEdgesForUser(ctx, user); len(edges) != 1 {
		t.Fatalf("unrelated item got linked: %+v", edges)
	}

	// 4. Haiku proposes "recipes" again: the name embedding matches the
	// existing category, so no duplicate is created.
	jev.force = classify.NoneOfThese
	r2 := save(t, q, user, "https://e.com/r2", "cake for a birthday")
	if res, err = c.Run(ctx, r2); err != nil {
		t.Fatal(err)
	}
	jev.force = ""
	cats, _ := q.ListCategories(ctx, user)
	if res.Category != "Recipes" || res.NewCategory || len(cats) != 2 {
		t.Fatalf("duplicate name: %+v, categories %+v", res, cats)
	}

	// 5. Running again without changes costs nothing.
	calls := jev.calls + namer.calls + emb.calls
	if res, err = c.Run(ctx, a); err != nil || res.Skipped != "already classified" || jev.calls+namer.calls+emb.calls != calls {
		t.Fatalf("rerun: %+v %v", res, err)
	}

	// 6. The user moves an item; a later description edit re-embeds it but
	// keeps their category.
	var recipesID uuid.UUID
	for _, cat := range cats {
		if cat.Name == "Recipes" {
			recipesID = cat.ID
		}
	}
	if _, err := q.SetItemCategoryByUser(ctx, db.SetItemCategoryByUserParams{ID: a, UserID: user, CategoryID: &recipesID}); err != nil {
		t.Fatal(err)
	}
	newDesc := "crafts to do with my daughter"
	if needs, err := q.SetItemDescription(ctx, db.SetItemDescriptionParams{ID: a, UserID: user, Description: &newDesc}); err != nil || !needs {
		t.Fatalf("describe: needs=%v err=%v", needs, err)
	}
	if res, err = c.Run(ctx, a); err != nil || res.Category != "" || !res.Embedded {
		t.Fatalf("locked item: %+v %v", res, err)
	}
	item, _ := q.GetItem(ctx, db.GetItemParams{ID: a, UserID: user})
	if item.CategoryID == nil || *item.CategoryID != recipesID || item.Classifying {
		t.Fatalf("locked item after run: %+v", item)
	}

	// 7. Saving the same text again doesn't queue more work.
	same := "crafts to do with my daughter"
	row, _ := q.UpsertItem(ctx, db.UpsertItemParams{UserID: user, Url: "https://e.com/a", Platform: "web", Description: &same})
	if row.NeedsClassifying {
		t.Fatal("re-saving the same description asked for classification")
	}

	if strings.Join(notify.got, ",") != "Parenting,Parenting,Recipes,Recipes" {
		t.Fatalf("notifications: %v", notify.got)
	}
	_ = pool
}

func TestQueueRunsClassification(t *testing.T) {
	q, pool := setup(t)
	ctx := context.Background()
	if err := jobs.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM river_job"); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := &classify.Classifier{
		Q: q, Jev: &wordJev{}, Embed: &bagOfWords{}, Namer: &fixedNamer{names: []string{"Travel"}}, Log: log,
		Threshold: 0.6, SameName: 0.85, Neighbors: 5, MinSimilarity: 0.45,
	}
	queue, err := jobs.New(jobs.Options{Pool: pool, Queries: q, Classifier: c, Logger: log, RunWorkers: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = queue.Stop(stopCtx)
	})

	user := uuid.New()
	id := save(t, q, user, "https://e.com/trip", "three days in Montevideo")
	if err := queue.EnqueueClassify(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := queue.EnqueueClassify(ctx, id); err != nil { // unique: collapses into one
		t.Fatal(err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		item, err := q.GetItem(ctx, db.GetItemParams{ID: id, UserID: user})
		if err != nil {
			t.Fatal(err)
		}
		if !item.Classifying {
			if item.CategoryName == nil || *item.CategoryName != "Travel" {
				t.Fatalf("classified without the expected category: %+v", item)
			}
			var jobsRun int
			_ = pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind = 'classify_item'").Scan(&jobsRun)
			if jobsRun != 1 {
				t.Fatalf("%d classify jobs, want 1", jobsRun)
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("item wasn't classified within 15s")
}

// pricedEmbedder charges 40 µ$ per call, so runs have a visible cost.
type pricedEmbedder struct{ bagOfWords }

func (p *pricedEmbedder) Embed(ctx context.Context, inputs ...string) ([][]float32, error) {
	ai.RecordUsage(ctx, ai.Usage{Service: "embeddings", Cost: 40})
	return p.bagOfWords.Embed(ctx, inputs...)
}

func TestCreditsGateAndCharge(t *testing.T) {
	q, _ := setup(t)
	ctx := context.Background()
	user := uuid.New()
	ledger := &credits.Ledger{Q: q, Starter: 50} // enough for one run
	c := &classify.Classifier{
		Q: q, Jev: &wordJev{}, Embed: &pricedEmbedder{}, Namer: &fixedNamer{names: []string{"Cooking"}},
		Credits: ledger, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Threshold: 0.6, SameName: 0.85, Neighbors: 5, MinSimilarity: 0.45,
	}

	first := save(t, q, user, "https://e.com/1", "lemon pasta recipe")
	res, err := c.Run(ctx, first)
	if err != nil || res.Category != "Cooking" {
		t.Fatalf("first run: %+v %v", res, err)
	}
	// Two embeddings (description + new category name) at 40 each.
	if b, _ := ledger.Balance(ctx, user); b != 50-80 {
		t.Fatalf("balance after first run = %d", b)
	}

	second := save(t, q, user, "https://e.com/2", "lemon cake recipe")
	res, err = c.Run(ctx, second)
	if err != nil || res.Skipped != "no credits" {
		t.Fatalf("second run: %+v %v", res, err)
	}
	if ids, _ := q.ItemsNeedingClassification(ctx, 10); len(ids) != 0 {
		t.Fatalf("sweep picked up an out-of-credits user's item: %v", ids)
	}

	// A top-up makes the item eligible again.
	if _, _, err := ledger.Grant(ctx, user, 1000, "test", nil); err != nil {
		t.Fatal(err)
	}
	if ids, _ := q.ItemsNeedingClassification(ctx, 10); len(ids) != 1 || ids[0] != second {
		t.Fatalf("after top-up the sweep found %v", ids)
	}
	if res, err = c.Run(ctx, second); err != nil || res.Category != "Cooking" {
		t.Fatalf("after top-up: %+v %v", res, err)
	}
}
