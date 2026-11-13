package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aaugoustis/waitlist/server/internal/auth"
	"github.com/aaugoustis/waitlist/server/internal/credits"
	"github.com/aaugoustis/waitlist/server/internal/db"
)

// fakeQueries is an in-memory db.Querier that mimics the SQL in db/queries.
// The embedded interface is nil: calling a method the fake doesn't define
// panics, which flags a handler using a query these tests don't expect.
type fakeQueries struct {
	db.Querier
	mu         sync.Mutex
	items      map[uuid.UUID]db.GetItemRow
	owner      map[uuid.UUID]uuid.UUID
	categories map[uuid.UUID]db.InsertCategoryRow
	catOwner   map[uuid.UUID]uuid.UUID
	feedback   []db.InsertCategoryFeedbackParams
	devices    map[string]db.UpsertDeviceParams
	edges      map[uuid.UUID][]db.ListEdgesForUserRow
	balances   map[uuid.UUID]int64
	clock      time.Time
}

func newFake() *fakeQueries {
	return &fakeQueries{
		items:      map[uuid.UUID]db.GetItemRow{},
		owner:      map[uuid.UUID]uuid.UUID{},
		categories: map[uuid.UUID]db.InsertCategoryRow{},
		catOwner:   map[uuid.UUID]uuid.UUID{},
		devices:    map[string]db.UpsertDeviceParams{},
		clock:      time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeQueries) tick() time.Time { f.clock = f.clock.Add(time.Second); return f.clock }

func (f *fakeQueries) UpsertItem(_ context.Context, a db.UpsertItemParams) (db.UpsertItemRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, it := range f.items {
		if f.owner[id] == a.UserID && it.Url == a.Url {
			changed := a.Description != nil && (it.Description == nil || *it.Description != *a.Description)
			if a.Description != nil {
				it.Description = a.Description
			}
			it.Classifying = it.Classifying || changed
			it.UpdatedAt = f.tick()
			f.items[id] = it
			return db.UpsertItemRow{ID: id, NeedsClassifying: it.Classifying}, nil
		}
	}
	id := uuid.New()
	now := f.tick()
	f.items[id] = db.GetItemRow{
		ID: id, Url: a.Url, Platform: a.Platform, Description: a.Description, CreatedAt: now, UpdatedAt: now,
		CategorySource: "auto", Classifying: a.Description != nil,
	}
	f.owner[id] = a.UserID
	return db.UpsertItemRow{ID: id, Inserted: true, NeedsClassifying: a.Description != nil}, nil
}

func (f *fakeQueries) CategoryBelongsToUser(_ context.Context, a db.CategoryBelongsToUserParams) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.catOwner[a.ID] == a.UserID, nil
}

func (f *fakeQueries) InsertCategory(_ context.Context, a db.InsertCategoryParams) (db.InsertCategoryRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, c := range f.categories {
		if f.catOwner[id] == a.UserID && strings.EqualFold(c.Name, a.Name) {
			return c, nil
		}
	}
	row := db.InsertCategoryRow{ID: uuid.New(), Name: a.Name}
	f.categories[row.ID] = row
	f.catOwner[row.ID] = a.UserID
	return row, nil
}

func (f *fakeQueries) SetItemCategoryByUser(_ context.Context, a db.SetItemCategoryByUserParams) (db.SetItemCategoryByUserRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[a.ID]
	if !ok || f.owner[a.ID] != a.UserID {
		return db.SetItemCategoryByUserRow{}, pgx.ErrNoRows
	}
	old := db.SetItemCategoryByUserRow{FromCategoryID: it.CategoryID, Description: it.Description}
	it.CategoryID, it.CategoryName, it.CategorySource = a.CategoryID, nil, "user"
	if a.CategoryID != nil {
		name := f.categories[*a.CategoryID].Name
		it.CategoryName = &name
	}
	f.items[a.ID] = it
	return old, nil
}

func (f *fakeQueries) InsertCategoryFeedback(_ context.Context, a db.InsertCategoryFeedbackParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.feedback = append(f.feedback, a)
	return nil
}

func (f *fakeQueries) UpsertDevice(_ context.Context, a db.UpsertDeviceParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.devices[a.Token] = a
	return nil
}

func (f *fakeQueries) DeleteDevice(_ context.Context, a db.DeleteDeviceParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.devices[a.Token]; ok && d.UserID == a.UserID {
		delete(f.devices, a.Token)
	}
	return nil
}

// fakeEnqueuer records which items were queued for classification.
type fakeEnqueuer struct {
	mu  sync.Mutex
	ids []uuid.UUID
}

func (e *fakeEnqueuer) EnqueueClassify(_ context.Context, id uuid.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ids = append(e.ids, id)
	return nil
}

func (e *fakeEnqueuer) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.ids)
}

func (f *fakeQueries) GetItem(_ context.Context, a db.GetItemParams) (db.GetItemRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[a.ID]
	if !ok || f.owner[a.ID] != a.UserID {
		return db.GetItemRow{}, pgx.ErrNoRows
	}
	return it, nil
}

func (f *fakeQueries) ListItems(_ context.Context, a db.ListItemsParams) ([]db.ListItemsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []db.ListItemsRow
	for id, it := range f.items {
		if f.owner[id] != a.UserID || (a.OnlyUndescribed && it.Description != nil) {
			continue
		}
		if a.BeforeCreated != nil && !it.CreatedAt.Before(*a.BeforeCreated) {
			continue
		}
		out = append(out, db.ListItemsRow(it))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > int(a.Lim) {
		out = out[:a.Lim]
	}
	return out, nil
}

func (f *fakeQueries) SetItemDescription(_ context.Context, a db.SetItemDescriptionParams) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[a.ID]
	if !ok || f.owner[a.ID] != a.UserID {
		return false, pgx.ErrNoRows
	}
	changed := (a.Description == nil) != (it.Description == nil) ||
		(a.Description != nil && *a.Description != *it.Description)
	it.Description = a.Description
	it.Classifying = a.Description != nil && (it.Classifying || changed)
	f.items[a.ID] = it
	return it.Classifying, nil
}

func (f *fakeQueries) DeleteItem(_ context.Context, a db.DeleteItemParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.items[a.ID]; !ok || f.owner[a.ID] != a.UserID {
		return 0, nil
	}
	delete(f.items, a.ID)
	return 1, nil
}

func (f *fakeQueries) ListCategories(context.Context, uuid.UUID) ([]db.ListCategoriesRow, error) {
	return nil, nil
}

const testIssuer = "https://test.authkit.app"

var testSecret = []byte("test-secret-at-least-32-bytes-long!!")

type harness struct {
	t   *testing.T
	srv *httptest.Server
	q   *fakeQueries
	enq *fakeEnqueuer
}

func newHarness(t *testing.T) *harness {
	q, enq := newFake(), &fakeEnqueuer{}
	s := New(Options{
		Queries:     q,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier:    auth.NewHS256(testSecret, testIssuer, "https://api.example.test/mcp"),
		CORSOrigins: []string{"http://localhost:8081"},
		Enqueuer:    enq,
	})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &harness{t: t, srv: srv, q: q, enq: enq}
}

func token(t *testing.T, user uuid.UUID) string {
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: user.String(), Issuer: testIssuer, Audience: jwt.ClaimStrings{"https://api.example.test/mcp"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (h *harness) do(user uuid.UUID, method, path, body string, out any) int {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, strings.NewReader(body))
	if user != uuid.Nil {
		req.Header.Set("Authorization", "Bearer "+token(h.t, user))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil && err != io.EOF {
			h.t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return res.StatusCode
}

func TestRequiresAuth(t *testing.T) {
	h := newHarness(t)
	if code := h.do(uuid.Nil, "GET", "/v1/items", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", code)
	}
	if code := h.do(uuid.Nil, "GET", "/healthz", "", nil); code != http.StatusOK {
		t.Fatalf("healthz status %d, want 200", code)
	}
}

func TestSaveFlow(t *testing.T) {
	h := newHarness(t)
	user := uuid.New()

	// Share sheet sends text + URL, no description yet.
	var first itemJSON
	code := h.do(user, "POST", "/v1/items",
		`{"url":"Look! https://www.instagram.com/p/DfYWVoBDC7L/?utm_source=ig_web_copy_link"}`, &first)
	if code != http.StatusCreated {
		t.Fatalf("create: status %d", code)
	}
	if first.URL != "https://instagram.com/p/DfYWVoBDC7L" || first.Platform != "instagram" || first.Description != nil {
		t.Fatalf("create: got %+v", first)
	}

	// It shows up in the Undescribed inbox.
	var inbox listItemsResponse
	h.do(user, "GET", "/v1/items?filter=undescribed", "", &inbox)
	if len(inbox.Items) != 1 {
		t.Fatalf("inbox has %d items, want 1", len(inbox.Items))
	}

	// Saving the same post again with a description updates the same row.
	var again itemJSON
	code = h.do(user, "POST", "/v1/items",
		`{"url":"https://instagram.com/p/DfYWVoBDC7L?igsh=x","description":"  5 dad-daughter traditions  "}`, &again)
	if code != http.StatusOK || again.ID != first.ID || again.Description == nil || *again.Description != "5 dad-daughter traditions" {
		t.Fatalf("re-save: status %d, item %+v", code, again)
	}
	h.do(user, "GET", "/v1/items?filter=undescribed", "", &inbox)
	if len(inbox.Items) != 0 {
		t.Fatalf("inbox has %d items after describing, want 0", len(inbox.Items))
	}

	// PATCH null clears it; another user can't see or change it.
	var cleared itemJSON
	if code := h.do(user, "PATCH", "/v1/items/"+first.ID.String(), `{"description":null}`, &cleared); code != http.StatusOK || cleared.Description != nil {
		t.Fatalf("patch: status %d, item %+v", code, cleared)
	}
	stranger := uuid.New()
	if code := h.do(stranger, "GET", "/v1/items/"+first.ID.String(), "", nil); code != http.StatusNotFound {
		t.Fatalf("stranger get: status %d", code)
	}
	if code := h.do(stranger, "DELETE", "/v1/items/"+first.ID.String(), "", nil); code != http.StatusNotFound {
		t.Fatalf("stranger delete: status %d", code)
	}
	if code := h.do(user, "DELETE", "/v1/items/"+first.ID.String(), "", nil); code != http.StatusNoContent {
		t.Fatalf("delete: status %d", code)
	}
	if code := h.do(user, "GET", "/v1/items/"+first.ID.String(), "", nil); code != http.StatusNotFound {
		t.Fatalf("get after delete: status %d", code)
	}
}

func TestValidation(t *testing.T) {
	h := newHarness(t)
	user := uuid.New()
	long := strings.Repeat("a", maxDescription+1)
	cases := []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/v1/items", `{"url":"not a link"}`, http.StatusUnprocessableEntity},
		{"POST", "/v1/items", `{"url":"https://x.com/a","description":"` + long + `"}`, http.StatusUnprocessableEntity},
		{"POST", "/v1/items", `{"url":"https://x.com/a","extra":1}`, http.StatusBadRequest},
		{"POST", "/v1/items", `{`, http.StatusBadRequest},
		{"POST", "/v1/items", `{"url":"https://x.com/a","description":"` + strings.Repeat("a", maxBody) + `"}`, http.StatusRequestEntityTooLarge},
		{"PATCH", "/v1/items/" + uuid.NewString(), `{}`, http.StatusBadRequest},
		{"PATCH", "/v1/items/" + uuid.NewString(), `{"description":"x","other":1}`, http.StatusBadRequest},
		{"PATCH", "/v1/items/" + uuid.NewString(), `{"description":5}`, http.StatusBadRequest},
		{"PATCH", "/v1/items/" + uuid.NewString(), `{"description":"x"}`, http.StatusNotFound},
		{"GET", "/v1/items/not-a-uuid", ``, http.StatusNotFound},
		{"GET", "/v1/items?limit=0", ``, http.StatusBadRequest},
		{"GET", "/v1/items?filter=nope", ``, http.StatusBadRequest},
		{"GET", "/v1/items?cursor=bm90LWEtY3Vyc29y", ``, http.StatusBadRequest},
	}
	for _, c := range cases {
		if got := h.do(user, c.method, c.path, c.body, nil); got != c.want {
			t.Errorf("%s %s %.40s: status %d, want %d", c.method, c.path, c.body, got, c.want)
		}
	}
}

func TestPagination(t *testing.T) {
	h := newHarness(t)
	user := uuid.New()
	for i := range 5 {
		h.do(user, "POST", "/v1/items", `{"url":"https://example.com/`+string(rune('a'+i))+`"}`, nil)
	}
	var seen []string
	path := "/v1/items?limit=2"
	for range 10 {
		var page listItemsResponse
		if code := h.do(user, "GET", path, "", &page); code != http.StatusOK {
			t.Fatalf("status %d", code)
		}
		for _, it := range page.Items {
			seen = append(seen, it.URL)
		}
		if page.NextCursor == nil {
			break
		}
		path = "/v1/items?limit=2&cursor=" + *page.NextCursor
	}
	want := "https://example.com/e https://example.com/d https://example.com/c https://example.com/b https://example.com/a"
	if got := strings.Join(seen, " "); got != want {
		t.Fatalf("pages = %s\nwant    %s", got, want)
	}
}

func TestCORS(t *testing.T) {
	h := newHarness(t)
	preflight := func(origin string) *http.Response {
		req, _ := http.NewRequest(http.MethodOptions, h.srv.URL+"/v1/items", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	ok := preflight("http://localhost:8081")
	if ok.StatusCode != http.StatusNoContent || ok.Header.Get("Access-Control-Allow-Origin") != "http://localhost:8081" ||
		!strings.Contains(ok.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("allowed origin: status %d, headers %v", ok.StatusCode, ok.Header)
	}
	if bad := preflight("https://evil.example"); bad.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unknown origin got CORS headers: %v", bad.Header)
	}
}

func TestClassificationIsQueuedOnlyForNewDescriptions(t *testing.T) {
	h := newHarness(t)
	user := uuid.New()

	var it itemJSON
	h.do(user, "POST", "/v1/items", `{"url":"https://x.com/a/status/1"}`, &it)
	if h.enq.count() != 0 || it.Classifying {
		t.Fatalf("save without description queued %d jobs (classifying=%v)", h.enq.count(), it.Classifying)
	}
	h.do(user, "PATCH", "/v1/items/"+it.ID.String(), `{"description":"pricing thread"}`, &it)
	if h.enq.count() != 1 || !it.Classifying || it.CategorySource != "auto" {
		t.Fatalf("describe: %d jobs, item %+v", h.enq.count(), it)
	}
	// Saving the same link with the same description again costs nothing.
	h.do(user, "POST", "/v1/items", `{"url":"https://x.com/a/status/1","description":"pricing thread"}`, nil)
	if h.enq.count() != 2 {
		// The fake still reports classifying=true (no worker ran), so a second
		// enqueue is expected here; River's unique jobs collapse it.
		t.Fatalf("re-save: %d jobs", h.enq.count())
	}
}

func TestUserCategoryCorrection(t *testing.T) {
	h := newHarness(t)
	user, stranger := uuid.New(), uuid.New()

	var it itemJSON
	h.do(user, "POST", "/v1/items", `{"url":"https://example.com/r","description":"lemon pasta"}`, &it)

	var cat categoryJSON
	if code := h.do(user, "POST", "/v1/categories", `{"name":"  Recipes  "}`, &cat); code != http.StatusCreated || cat.Name != "Recipes" {
		t.Fatalf("create category: %d %+v", code, cat)
	}
	var other categoryJSON
	h.do(stranger, "POST", "/v1/categories", `{"name":"Theirs"}`, &other)

	if code := h.do(user, "PATCH", "/v1/items/"+it.ID.String(), `{"categoryId":"`+other.ID.String()+`"}`, nil); code != http.StatusUnprocessableEntity {
		t.Fatalf("someone else's category: status %d", code)
	}
	if code := h.do(user, "PATCH", "/v1/items/"+it.ID.String(), `{"categoryId":"`+cat.ID.String()+`"}`, &it); code != http.StatusOK {
		t.Fatalf("set category: status %d", code)
	}
	if it.Category == nil || it.Category.ID != cat.ID || it.CategorySource != "user" {
		t.Fatalf("item after correction: %+v", it)
	}
	if len(h.q.feedback) != 1 || h.q.feedback[0].Description != "lemon pasta" || *h.q.feedback[0].ToCategoryID != cat.ID {
		t.Fatalf("feedback: %+v", h.q.feedback)
	}
	// Clearing works too, and records another correction.
	h.do(user, "PATCH", "/v1/items/"+it.ID.String(), `{"categoryId":null}`, &it)
	if it.Category != nil || len(h.q.feedback) != 2 {
		t.Fatalf("clear: item %+v, feedback %d", it, len(h.q.feedback))
	}
	if code := h.do(user, "PATCH", "/v1/items/"+it.ID.String(), `{"categoryId":"nope"}`, nil); code != http.StatusBadRequest {
		t.Fatalf("bad uuid: status %d", code)
	}
	if code := h.do(user, "POST", "/v1/categories", `{"name":"   "}`, nil); code != http.StatusUnprocessableEntity {
		t.Fatalf("blank name: status %d", code)
	}
}

func TestDevices(t *testing.T) {
	h := newHarness(t)
	user := uuid.New()
	tok := "ExponentPushToken[abc123]"
	if code := h.do(user, "PUT", "/v1/devices/"+tok, `{"platform":"ios"}`, nil); code != http.StatusNoContent {
		t.Fatalf("register: status %d", code)
	}
	if d := h.q.devices[tok]; d.UserID != user || d.Platform != "ios" {
		t.Fatalf("stored device: %+v", d)
	}
	if code := h.do(user, "PUT", "/v1/devices/not-a-token", `{"platform":"ios"}`, nil); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad token: status %d", code)
	}
	if code := h.do(user, "PUT", "/v1/devices/"+tok, `{"platform":"windows"}`, nil); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad platform: status %d", code)
	}
	if code := h.do(uuid.New(), "DELETE", "/v1/devices/"+tok, ``, nil); code != http.StatusNoContent || len(h.q.devices) != 1 {
		t.Fatalf("stranger delete removed the device")
	}
	h.do(user, "DELETE", "/v1/devices/"+tok, ``, nil)
	if len(h.q.devices) != 0 {
		t.Fatalf("delete kept the device")
	}
}

func (f *fakeQueries) ListGraphItems(_ context.Context, a db.ListGraphItemsParams) ([]db.ListGraphItemsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []db.ListGraphItemsRow
	for id, it := range f.items {
		if f.owner[id] == a.UserID {
			out = append(out, db.ListGraphItemsRow{ID: id, Url: it.Url, Platform: it.Platform, Description: it.Description,
				CategoryID: it.CategoryID, CategoryName: it.CategoryName})
		}
	}
	return out, nil
}

func (f *fakeQueries) ListEdgesForUser(_ context.Context, user uuid.UUID) ([]db.ListEdgesForUserRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.edges[user], nil
}

func TestGraphEndpoints(t *testing.T) {
	h := newHarness(t)
	user, stranger := uuid.New(), uuid.New()
	var a, b, c itemJSON
	h.do(user, "POST", "/v1/items", `{"url":"https://e.com/a","description":"alpha"}`, &a)
	h.do(user, "POST", "/v1/items", `{"url":"https://e.com/b","description":"beta"}`, &b)
	h.do(user, "POST", "/v1/items", `{"url":"https://e.com/c"}`, &c)
	var cat categoryJSON
	h.do(user, "POST", "/v1/categories", `{"name":"Letters"}`, &cat)
	h.do(user, "PATCH", "/v1/items/"+a.ID.String(), `{"categoryId":"`+cat.ID.String()+`"}`, nil)
	h.q.mu.Lock()
	h.q.edges = map[uuid.UUID][]db.ListEdgesForUserRow{user: {{Src: a.ID, Dst: b.ID, Weight: 0.8}}}
	h.q.mu.Unlock()

	type graphJSON struct {
		Nodes []struct{ ID, Kind, Label string }
		Links []struct{ Source, Target, Kind string }
	}
	var g graphJSON
	if code := h.do(user, "GET", "/v1/graph", "", &g); code != http.StatusOK || len(g.Nodes) != 4 || len(g.Links) != 2 {
		t.Fatalf("graph: %d %+v", code, g)
	}

	var local graphJSON
	h.do(user, "GET", "/v1/items/"+b.ID.String()+"/neighbors?depth=1", "", &local)
	if len(local.Nodes) != 3 || local.Nodes[0].ID != b.ID.String() { // b, a, Letters
		t.Fatalf("neighbors: %+v", local)
	}
	if code := h.do(user, "GET", "/v1/items/"+b.ID.String()+"/neighbors?depth=3", "", nil); code != http.StatusBadRequest {
		t.Fatalf("depth 3: %d", code)
	}
	if code := h.do(stranger, "GET", "/v1/items/"+b.ID.String()+"/neighbors", "", nil); code != http.StatusNotFound {
		t.Fatalf("stranger: %d", code)
	}
	var empty graphJSON
	if code := h.do(stranger, "GET", "/v1/graph", "", &empty); code != http.StatusOK || len(empty.Nodes) != 0 {
		t.Fatalf("stranger graph: %d %+v", code, empty)
	}
}

func (f *fakeQueries) EnsureStarterCredits(_ context.Context, a db.EnsureStarterCreditsParams) (int64, error) {
	return a.Starter, nil
}

func (f *fakeQueries) AverageClassifyCost(context.Context, uuid.UUID) (float64, error) { return 0, nil }

func (f *fakeQueries) ListCreditLedger(context.Context, db.ListCreditLedgerParams) ([]db.ListCreditLedgerRow, error) {
	return []db.ListCreditLedgerRow{
		{Amount: -42, Kind: "usage", Detail: []byte(`{"step":"classify"}`), CreatedAt: time.Now()},
		{Amount: 100_000, Kind: "starter", Detail: []byte(`{}`), CreatedAt: time.Now()},
	}, nil
}

func TestCredits(t *testing.T) {
	q := newFake()
	s := New(Options{
		Queries: q, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier: auth.NewHS256(testSecret, testIssuer, "https://api.example.test/mcp"),
		Credits:  &credits.Ledger{Q: q, Starter: 90_000},
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	h := &harness{t: t, srv: srv, q: q}

	var got struct {
		BalanceUSD float64 `json:"balanceUsd"`
		SavesLeft  int     `json:"savesLeft"`
		ModelShare float64 `json:"modelShare"`
		Recent     []struct {
			AmountUSD float64 `json:"amountUsd"`
			Kind      string
			Step      string
		}
	}
	if code := h.do(uuid.New(), "GET", "/v1/credits", "", &got); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	// $0.09 at the default ~30 µ$ per save.
	if got.BalanceUSD != 0.09 || got.SavesLeft != 3000 || got.ModelShare != 0.8 || len(got.Recent) != 2 ||
		got.Recent[0].Step != "classify" || got.Recent[0].AmountUSD != -0.000042 {
		t.Fatalf("credits = %+v", got)
	}
}

func (f *fakeQueries) GetCreditBalance(_ context.Context, user uuid.UUID) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.balances[user]
	if !ok {
		return 0, pgx.ErrNoRows
	}
	return b, nil
}

func TestWaitingForCredits(t *testing.T) {
	q := newFake()
	s := New(Options{
		Queries: q, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier: auth.NewHS256(testSecret, testIssuer, "https://api.example.test/mcp"),
		Credits:  &credits.Ledger{Q: q, Starter: 100},
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	h := &harness{t: t, srv: srv, q: q}
	broke, fresh, funded := uuid.New(), uuid.New(), uuid.New()
	q.balances = map[uuid.UUID]int64{broke: -20, funded: 500}

	for user, want := range map[uuid.UUID]bool{broke: true, fresh: false, funded: false} {
		var it itemJSON
		h.do(user, "POST", "/v1/items", `{"url":"https://e.com/x","description":"something"}`, &it)
		if !it.Classifying || it.WaitingForCredits != want {
			t.Errorf("user balance %v: classifying=%v waiting=%v, want waiting=%v", q.balances[user], it.Classifying, it.WaitingForCredits, want)
		}
		var page listItemsResponse
		h.do(user, "GET", "/v1/items", "", &page)
		if len(page.Items) != 1 || page.Items[0].WaitingForCredits != want {
			t.Errorf("list for balance %v: %+v", q.balances[user], page.Items)
		}
	}
	// Undescribed items aren't waiting for anything.
	var bare itemJSON
	h.do(broke, "POST", "/v1/items", `{"url":"https://e.com/y"}`, &bare)
	if bare.WaitingForCredits {
		t.Error("undescribed item marked as waiting")
	}
}
