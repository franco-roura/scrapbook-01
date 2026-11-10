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
	"github.com/aaugoustis/waitlist/server/internal/db"
)

// fakeQueries is an in-memory db.Querier that mimics the SQL in db/queries.
type fakeQueries struct {
	mu    sync.Mutex
	items map[uuid.UUID]db.GetItemRow
	owner map[uuid.UUID]uuid.UUID
	clock time.Time
}

func newFake() *fakeQueries {
	return &fakeQueries{
		items: map[uuid.UUID]db.GetItemRow{},
		owner: map[uuid.UUID]uuid.UUID{},
		clock: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeQueries) tick() time.Time { f.clock = f.clock.Add(time.Second); return f.clock }

func (f *fakeQueries) UpsertItem(_ context.Context, a db.UpsertItemParams) (db.UpsertItemRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, it := range f.items {
		if f.owner[id] == a.UserID && it.Url == a.Url {
			if a.Description != nil {
				it.Description = a.Description
			}
			it.UpdatedAt = f.tick()
			f.items[id] = it
			return db.UpsertItemRow{ID: id}, nil
		}
	}
	id := uuid.New()
	now := f.tick()
	f.items[id] = db.GetItemRow{ID: id, Url: a.Url, Platform: a.Platform, Description: a.Description, CreatedAt: now, UpdatedAt: now}
	f.owner[id] = a.UserID
	return db.UpsertItemRow{ID: id, Inserted: true}, nil
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

func (f *fakeQueries) SetItemDescription(_ context.Context, a db.SetItemDescriptionParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[a.ID]
	if !ok || f.owner[a.ID] != a.UserID {
		return 0, nil
	}
	it.Description = a.Description
	f.items[a.ID] = it
	return 1, nil
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

const testIssuer = "https://test.supabase.co/auth/v1"

var testSecret = []byte("test-secret-at-least-32-bytes-long!!")

type harness struct {
	t   *testing.T
	srv *httptest.Server
}

func newHarness(t *testing.T) *harness {
	s := New(Options{
		Queries:     newFake(),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier:    auth.NewHS256(testSecret, testIssuer),
		CORSOrigins: []string{"http://localhost:8081"},
	})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &harness{t: t, srv: srv}
}

func token(t *testing.T, user uuid.UUID) string {
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: user.String(), Issuer: testIssuer, Audience: jwt.ClaimStrings{"authenticated"},
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
