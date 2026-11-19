// Package preview reads a saved link's public preview (its Open Graph or
// oEmbed title, description, site and image) so the app can show a card.
// Previews are for display only: sorting never sees them.
package preview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"github.com/franco-roura/scrapbook-01/server/internal/links"
)

// Preview is what a link says about itself. Empty fields weren't found.
type Preview struct {
	Title       string
	Description string
	Site        string
	ImageURL    string
}

// Empty reports whether nothing useful was found.
func (p Preview) Empty() bool {
	return p.Title == "" && p.Description == "" && p.ImageURL == ""
}

const (
	maxPageBytes = 1 << 20 // HTML read before giving up on <head>
	maxTitle     = 300
	maxDesc      = 1000
	maxSite      = 100
	userAgent    = "Scrapbook01-Preview/1.0 (+https://github.com/franco-roura/scrapbook-01)"
	acceptHTML   = "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5"
)

// oEmbed endpoints by platform: public, keyless and steadier than the HTML of
// these sites. A var so tests can point them at a local server.
var oembedEndpoints = map[string]string{
	links.YouTube: "https://www.youtube.com/oembed",
	links.TikTok:  "https://www.tiktok.com/oembed",
}

// Fetcher reads previews. Its client must refuse private addresses (see
// NewClient); tests swap it for one that reaches httptest servers.
type Fetcher struct {
	Client *http.Client
}

// NewFetcher returns a Fetcher on the guarded client.
func NewFetcher() *Fetcher { return &Fetcher{Client: NewClient()} }

// ErrNoPreview is returned when a page answered but had nothing worth showing,
// like a social network's login wall.
var ErrNoPreview = errors.New("preview: nothing usable on the page")

// Fetch reads the preview of pageURL. platform is links.Normalize's platform;
// for YouTube and TikTok it tries oEmbed first, then falls back to the page.
func (f *Fetcher) Fetch(ctx context.Context, pageURL, platform string) (Preview, error) {
	if endpoint, ok := oembedEndpoints[platform]; ok {
		if p, err := f.oembed(ctx, endpoint, withWWW(pageURL)); err == nil && p.Title != "" {
			return p, nil
		}
	}
	p, err := f.page(ctx, pageURL)
	if err != nil {
		return Preview{}, err
	}
	// Social networks show anonymous visitors a login wall titled with
	// their own name ("Instagram", "TikTok - Make Your Day"). A real post's
	// page always has an image, so without one there's nothing to show.
	if platform != links.Web && p.ImageURL == "" {
		return Preview{}, ErrNoPreview
	}
	return p, nil
}

// withWWW puts back the "www." links.Normalize strips: TikTok's oEmbed
// refuses links without it.
func withWWW(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil || strings.Count(u.Host, ".") != 1 {
		return pageURL
	}
	u.Host = "www." + u.Host
	return u.String()
}

func (f *Fetcher) get(ctx context.Context, rawURL, accept string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("preview: not an http(s) URL: %q", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "es-UY,es;q=0.9,en;q=0.8")
	res, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		res.Body.Close()
		return nil, fmt.Errorf("preview: %s answered %d", u.Host, res.StatusCode)
	}
	return res, nil
}

type oembedResponse struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	ProviderName string `json:"provider_name"`
	ThumbnailURL string `json:"thumbnail_url"`
}

func (f *Fetcher) oembed(ctx context.Context, endpoint, pageURL string) (Preview, error) {
	// Both endpoints answer JSON by default; TikTok drops fields with format=json.
	res, err := f.get(ctx, endpoint+"?url="+url.QueryEscape(pageURL), "application/json")
	if err != nil {
		return Preview{}, err
	}
	defer res.Body.Close()
	var o oembedResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, maxPageBytes)).Decode(&o); err != nil {
		return Preview{}, fmt.Errorf("preview: oEmbed: %w", err)
	}
	return clean(Preview{
		Title: o.Title, Description: o.AuthorName, Site: o.ProviderName,
		ImageURL: absolute(res.Request.URL, o.ThumbnailURL),
	}), nil
}

func (f *Fetcher) page(ctx context.Context, pageURL string) (Preview, error) {
	res, err := f.get(ctx, pageURL, acceptHTML)
	if err != nil {
		return Preview{}, err
	}
	defer res.Body.Close()
	ct := res.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "html") {
		return Preview{}, fmt.Errorf("preview: not HTML (%s)", ct)
	}
	body, err := charset.NewReader(io.LimitReader(res.Body, maxPageBytes), ct)
	if err != nil {
		return Preview{}, err
	}
	// res.Request is the last request, so relative URLs resolve against
	// where any redirects ended up.
	return Parse(body, res.Request.URL), nil
}

// Parse reads the preview from an HTML document: Open Graph first, then
// Twitter cards, then <title> and the plain description.
func Parse(r io.Reader, base *url.URL) Preview {
	meta := map[string]string{}
	var title strings.Builder
	inTitle := false
	z := html.NewTokenizer(r)
loop:
	for {
		switch z.Next() {
		case html.ErrorToken:
			break loop
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "body":
				break loop
			case "title":
				inTitle = true
			case "meta":
				if !hasAttr {
					continue
				}
				var key, content string
				for {
					k, v, more := z.TagAttr()
					switch string(k) {
					case "property", "name":
						if key == "" {
							key = strings.ToLower(strings.TrimSpace(string(v)))
						}
					case "content":
						content = string(v)
					}
					if !more {
						break
					}
				}
				if key != "" && content != "" {
					if _, seen := meta[key]; !seen { // the first one wins
						meta[key] = content
					}
				}
			}
		case html.TextToken:
			if inTitle {
				title.Write(z.Text())
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "title":
				inTitle = false
			case "head":
				break loop
			}
		}
	}
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(meta[k]); v != "" {
				return v
			}
		}
		return ""
	}
	p := Preview{
		Title:       first("og:title", "twitter:title"),
		Description: first("og:description", "twitter:description", "description"),
		Site:        first("og:site_name", "application-name"),
		ImageURL:    absolute(base, first("og:image:secure_url", "og:image", "og:image:url", "twitter:image", "twitter:image:src")),
	}
	if p.Title == "" {
		p.Title = title.String()
	}
	return clean(p)
}

// absolute resolves ref against base and keeps it only if it's http(s).
func absolute(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}

func clean(p Preview) Preview {
	p.Title = squash(p.Title, maxTitle)
	p.Description = squash(p.Description, maxDesc)
	p.Site = squash(p.Site, maxSite)
	if len(p.ImageURL) > 2048 {
		p.ImageURL = ""
	}
	return p
}

// squash collapses whitespace, drops invalid UTF-8 and cuts to max runes.
func squash(s string, max int) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:max-1])) + "…"
}

// ErrNotImage is returned for an image URL that doesn't serve an image.
var ErrNotImage = errors.New("preview: not an image")
