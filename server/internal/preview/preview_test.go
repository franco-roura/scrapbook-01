package preview

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aaugoustis/waitlist/server/internal/db"
	"github.com/aaugoustis/waitlist/server/internal/links"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestParseOpenGraph(t *testing.T) {
	doc := `<!doctype html><html><head>
<title>Ignored when og:title exists</title>
<meta property="og:title" content="Kai~ on Instagram: &quot;Me voy a morir de hambre&quot;">
<meta property="og:description" content="42K likes,
   173 comments">
<meta property="og:site_name" content="Instagram">
<meta property="og:image" content="/media/thumb.jpg">
<meta property="og:title" content="A second og:title is ignored">
</head><body><meta property="og:image" content="https://late.example/no.jpg"></body></html>`
	p := Parse(strings.NewReader(doc), mustURL(t, "https://www.instagram.com/reel/abc/"))
	want := Preview{
		Title:       `Kai~ on Instagram: "Me voy a morir de hambre"`,
		Description: "42K likes, 173 comments",
		Site:        "Instagram",
		ImageURL:    "https://www.instagram.com/media/thumb.jpg",
	}
	if p != want {
		t.Fatalf("got %+v\nwant %+v", p, want)
	}
}

func TestParseFallbacks(t *testing.T) {
	doc := `<html><head>
<title>  Plain   title </title>
<meta name="description" content="Plain description">
<meta name="twitter:image" content="https://cdn.example/card.png">
</head></html>`
	p := Parse(strings.NewReader(doc), mustURL(t, "https://example.com/a"))
	if p.Title != "Plain title" || p.Description != "Plain description" || p.ImageURL != "https://cdn.example/card.png" {
		t.Fatalf("got %+v", p)
	}

	// Twitter tags beat the plain ones; non-http images are dropped.
	doc = `<head><meta name="twitter:title" content="T"><meta name="twitter:description" content="D">
<meta name="description" content="plain"><meta property="og:image" content="javascript:alert(1)"></head>`
	p = Parse(strings.NewReader(doc), mustURL(t, "https://example.com/"))
	if p.Title != "T" || p.Description != "D" || p.ImageURL != "" {
		t.Fatalf("got %+v", p)
	}

	if p := Parse(strings.NewReader("not html at all"), nil); !p.Empty() {
		t.Fatalf("garbage: %+v", p)
	}
}

func TestParseCapsLengths(t *testing.T) {
	long := strings.Repeat("ñ", 400)
	p := Parse(strings.NewReader(`<head><meta property="og:title" content="`+long+`"></head>`), nil)
	if r := []rune(p.Title); len(r) != maxTitle || r[len(r)-1] != '…' {
		t.Fatalf("title is %d runes", len(r))
	}
}

// testFetcher reaches httptest servers (on 127.0.0.1), which the real
// client refuses.
func testFetcher() *Fetcher { return &Fetcher{Client: &http.Client{Timeout: 5 * time.Second}} }

func TestFetchPageFollowsRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/short", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/posts/1", http.StatusFound)
	})
	mux.HandleFunc("/posts/1", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "Scrapbook01-Preview/") {
			t.Errorf("user agent %q", r.UserAgent())
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<head><meta property="og:title" content="Post"><meta property="og:image" content="img.png"></head>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, err := testFetcher().Fetch(context.Background(), srv.URL+"/short", links.Web)
	if err != nil {
		t.Fatal(err)
	}
	// Relative to where the redirect landed, not the link saved.
	if p.Title != "Post" || p.ImageURL != srv.URL+"/posts/img.png" {
		t.Fatalf("got %+v", p)
	}
}

func TestFetchRejectsNonHTMLAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pdf" {
			w.Header().Set("Content-Type", "application/pdf")
			io.WriteString(w, "%PDF")
			return
		}
		http.Error(w, "login required", http.StatusForbidden)
	}))
	defer srv.Close()
	f := testFetcher()
	if _, err := f.Fetch(context.Background(), srv.URL+"/pdf", links.Web); err == nil {
		t.Fatal("pdf: want an error")
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/wall", links.Web); err == nil {
		t.Fatal("403: want an error")
	}
	if _, err := f.Fetch(context.Background(), "file:///etc/passwd", links.Web); err == nil {
		t.Fatal("file URL: want an error")
	}
}

func TestFetchUsesOEmbedThenFallsBack(t *testing.T) {
	var oembedHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/oembed", func(w http.ResponseWriter, r *http.Request) {
		oembedHits++
		if u := r.URL.Query().Get("url"); u == "" || r.URL.Query().Has("format") {
			t.Errorf("oEmbed query %q", r.URL.RawQuery)
		}
		if strings.Contains(r.URL.Query().Get("url"), "broken") {
			http.Error(w, "nope", http.StatusNotFound)
			return
		}
		io.WriteString(w, `{"title":"Video","author_name":"Someone","provider_name":"YouTube","thumbnail_url":"https://i.ytimg.com/vi/x/hqdefault.jpg"}`)
	})
	mux.HandleFunc("/broken", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<head><meta property="og:title" content="From the page"><meta property="og:image" content="/i.jpg"></head>`)
	})
	mux.HandleFunc("/wall", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<head><title>Instagram</title><meta property="og:title" content="Instagram"></head>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	old := oembedEndpoints[links.YouTube]
	oembedEndpoints[links.YouTube] = srv.URL + "/oembed"
	t.Cleanup(func() { oembedEndpoints[links.YouTube] = old })

	f := testFetcher()
	p, err := f.Fetch(context.Background(), "https://www.youtube.com/watch?v=x", links.YouTube)
	if err != nil {
		t.Fatal(err)
	}
	want := Preview{Title: "Video", Description: "Someone", Site: "YouTube", ImageURL: "https://i.ytimg.com/vi/x/hqdefault.jpg"}
	if p != want {
		t.Fatalf("got %+v", p)
	}

	p, err = f.Fetch(context.Background(), srv.URL+"/broken", links.YouTube)
	if err != nil || p.Title != "From the page" {
		t.Fatalf("fallback: %+v, %v", p, err)
	}
	if oembedHits != 2 {
		t.Fatalf("oEmbed hit %d times", oembedHits)
	}

	// A social network's login wall (no image) isn't a preview; the same
	// page on a plain website is.
	if _, err := f.Fetch(context.Background(), srv.URL+"/wall", links.Instagram); !errors.Is(err, ErrNoPreview) {
		t.Fatalf("login wall: err = %v", err)
	}
	if p, err := f.Fetch(context.Background(), srv.URL+"/wall", links.Web); err != nil || p.Title != "Instagram" {
		t.Fatalf("web page: %+v, %v", p, err)
	}
}

func TestWithWWW(t *testing.T) {
	for in, want := range map[string]string{
		"https://tiktok.com/@a/video/1":     "https://www.tiktok.com/@a/video/1",
		"https://youtube.com/watch?v=x":     "https://www.youtube.com/watch?v=x",
		"https://www.youtube.com/watch?v=x": "https://www.youtube.com/watch?v=x",
		"https://vm.tiktok.com/abc":         "https://vm.tiktok.com/abc",
	} {
		if got := withWWW(in); got != want {
			t.Errorf("withWWW(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAllowed(t *testing.T) {
	for _, s := range []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"0.0.0.0", "100.64.0.1", "224.0.0.1", "::1", "fe80::1", "fc00::1",
		"::ffff:127.0.0.1", "::ffff:169.254.169.254", "64:ff9b::a9fe:a9fe",
	} {
		if Allowed(netip.MustParseAddr(s)) {
			t.Errorf("%s allowed", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "157.240.1.174", "2a03:2880:f12f:83:face:b00c:0:25de"} {
		if !Allowed(netip.MustParseAddr(s)) {
			t.Errorf("%s refused", s)
		}
	}
}

func TestClientRefusesPrivateAddresses(t *testing.T) {
	// A server on loopback, as the metadata endpoint or an internal service
	// would be: the guarded client must not connect, directly or by redirect.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("guarded client reached %s", r.URL)
	}))
	defer srv.Close()
	f := NewFetcher()
	_, err := f.Fetch(context.Background(), srv.URL+"/", links.Web)
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("direct: err = %v", err)
	}
	// A hostname that resolves to loopback is refused after resolution.
	_, err = f.Fetch(context.Background(), strings.Replace(srv.URL, "127.0.0.1", "localhost", 1), links.Web)
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("localhost: err = %v", err)
	}
	if _, err := f.Thumbnail(context.Background(), "http://169.254.169.254/latest/meta-data/"); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("metadata: err = %v", err)
	}
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.NRGBA{R: 255, A: 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestResize(t *testing.T) {
	out, err := Resize(pngBytes(t, 1280, 720), ThumbWidth)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 640 || cfg.Height != 360 {
		t.Fatalf("got %dx%d", cfg.Width, cfg.Height)
	}
	// Small images keep their size.
	out, _ = Resize(pngBytes(t, 300, 200), ThumbWidth)
	if cfg, _ := jpeg.DecodeConfig(bytes.NewReader(out)); cfg.Width != 300 || cfg.Height != 200 {
		t.Fatalf("small: %dx%d", cfg.Width, cfg.Height)
	}
	if _, err := Resize([]byte("<html>"), ThumbWidth); !errors.Is(err, ErrNotImage) {
		t.Fatalf("not an image: %v", err)
	}
}

func TestThumbnailRefusesNonImages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/img" {
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes(t, 800, 800))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html>")
	}))
	defer srv.Close()
	f := testFetcher()
	if _, err := f.Thumbnail(context.Background(), srv.URL+"/page"); !errors.Is(err, ErrNotImage) {
		t.Fatalf("html: %v", err)
	}
	out, err := f.Thumbnail(context.Background(), srv.URL+"/img")
	if err != nil {
		t.Fatal(err)
	}
	if cfg, _ := jpeg.DecodeConfig(bytes.NewReader(out)); cfg.Width != 640 {
		t.Fatalf("thumb width %d", cfg.Width)
	}
}

// Runner fakes.

type fakeQ struct {
	row   db.GetItemForPreviewRow
	err   error
	saved *db.SetItemPreviewParams
}

func (f *fakeQ) GetItemForPreview(context.Context, uuid.UUID) (db.GetItemForPreviewRow, error) {
	return f.row, f.err
}

func (f *fakeQ) SetItemPreview(_ context.Context, a db.SetItemPreviewParams) error {
	f.saved = &a
	return nil
}

type fakeFetch struct {
	p        Preview
	err      error
	thumbErr error
}

func (f fakeFetch) Fetch(context.Context, string, string) (Preview, error) { return f.p, f.err }
func (f fakeFetch) Thumbnail(context.Context, string) ([]byte, error) {
	return []byte("jpeg"), f.thumbErr
}

type fakeStore struct{ key string }

func (s *fakeStore) Put(_ context.Context, key string, _ []byte) (string, error) {
	s.key = key
	return "https://cdn.example/" + key, nil
}

func TestRunnerStoresCopy(t *testing.T) {
	id := uuid.New()
	q := &fakeQ{row: db.GetItemForPreviewRow{ID: id, Url: "https://x", Platform: links.Web}}
	st := &fakeStore{}
	r := &Runner{Q: q, Store: st, Log: slog.New(slog.DiscardHandler),
		Fetch: fakeFetch{p: Preview{Title: "T", Description: "D", Site: "S", ImageURL: "https://signed.cdn/x.jpg?oe=123"}}}
	if _, err := r.Run(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	want := db.SetItemPreviewParams{ID: id, Title: "T", Description: "D", Site: "S",
		Image: "https://cdn.example/previews/" + id.String() + ".jpg"}
	if q.saved == nil || *q.saved != want {
		t.Fatalf("saved %+v", q.saved)
	}
}

func TestRunnerRecordsFailuresAndSkipsDone(t *testing.T) {
	id := uuid.New()
	log := slog.New(slog.DiscardHandler)

	// The page can't be read: recorded as an empty attempt, no error.
	q := &fakeQ{row: db.GetItemForPreviewRow{ID: id, Url: "https://x"}}
	r := &Runner{Q: q, Log: log, Fetch: fakeFetch{err: errors.New("403")}}
	if _, err := r.Run(context.Background(), id); err != nil || q.saved == nil || *q.saved != (db.SetItemPreviewParams{ID: id}) {
		t.Fatalf("failed fetch: err=%v saved=%+v", err, q.saved)
	}

	// The image can't be copied: the text is kept, the image left out.
	q = &fakeQ{row: db.GetItemForPreviewRow{ID: id, Url: "https://x"}}
	r = &Runner{Q: q, Log: log, Store: &fakeStore{},
		Fetch: fakeFetch{p: Preview{Title: "T", ImageURL: "https://img"}, thumbErr: ErrNotImage}}
	if _, err := r.Run(context.Background(), id); err != nil || q.saved.Title != "T" || q.saved.Image != "" {
		t.Fatalf("failed image: err=%v saved=%+v", err, q.saved)
	}

	// Without a store (local), the original image URL is kept.
	q = &fakeQ{row: db.GetItemForPreviewRow{ID: id, Url: "https://x"}}
	r = &Runner{Q: q, Log: log, Fetch: fakeFetch{p: Preview{ImageURL: "https://img"}}}
	if _, err := r.Run(context.Background(), id); err != nil || q.saved.Image != "https://img" {
		t.Fatalf("no store: err=%v saved=%+v", err, q.saved)
	}

	// Already attempted: nothing is fetched or written.
	now := time.Now()
	q = &fakeQ{row: db.GetItemForPreviewRow{ID: id, PreviewFetchedAt: &now}}
	r = &Runner{Q: q, Log: log, Fetch: fakeFetch{err: errors.New("must not be called")}}
	if _, err := r.Run(context.Background(), id); err != nil || q.saved != nil {
		t.Fatalf("done: err=%v saved=%+v", err, q.saved)
	}

	// Database failures are returned, so the job is retried.
	q = &fakeQ{err: errors.New("connection reset")}
	if _, err := (&Runner{Q: q, Log: log, Fetch: fakeFetch{}}).Run(context.Background(), id); err == nil {
		t.Fatal("db error: want an error")
	}
}
