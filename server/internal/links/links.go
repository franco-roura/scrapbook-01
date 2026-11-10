// Package links cleans up shared URLs so the same post saved twice is stored
// once, and tells which platform a link points to.
package links

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// Platform names stored in items.platform.
const (
	Instagram = "instagram"
	TikTok    = "tiktok"
	YouTube   = "youtube"
	X         = "x"
	Facebook  = "facebook"
	Threads   = "threads"
	Reddit    = "reddit"
	LinkedIn  = "linkedin"
	Pinterest = "pinterest"
	Spotify   = "spotify"
	Telegram  = "telegram"
	Web       = "web"
)

var (
	ErrNoURL      = errors.New("no http(s) URL found")
	ErrInvalidURL = errors.New("invalid URL")
)

// urlInText finds the first http(s) URL in shared text. Apps often share
// something like "Check out this video! https://vm.tiktok.com/abc/".
var urlInText = regexp.MustCompile(`https?://[^\s<>"']+`)

// Extract returns the first http(s) URL inside text, without trailing
// punctuation that is usually part of the sentence rather than the link.
func Extract(text string) (string, error) {
	m := urlInText.FindString(text)
	if m == "" {
		return "", ErrNoURL
	}
	return strings.TrimRight(m, ".,;:!?)]}"), nil
}

// hostPlatforms maps a host (without "www." or "m.") to its platform.
var hostPlatforms = map[string]string{
	"instagram.com":     Instagram,
	"instagr.am":        Instagram,
	"tiktok.com":        TikTok,
	"vm.tiktok.com":     TikTok,
	"vt.tiktok.com":     TikTok,
	"youtube.com":       YouTube,
	"youtu.be":          YouTube,
	"music.youtube.com": YouTube,
	"x.com":             X,
	"twitter.com":       X,
	"facebook.com":      Facebook,
	"fb.watch":          Facebook,
	"fb.com":            Facebook,
	"threads.net":       Threads,
	"threads.com":       Threads,
	"reddit.com":        Reddit,
	"redd.it":           Reddit,
	"linkedin.com":      LinkedIn,
	"lnkd.in":           LinkedIn,
	"pinterest.com":     Pinterest,
	"pin.it":            Pinterest,
	"open.spotify.com":  Spotify,
	"t.me":              Telegram,
}

// dropAllQuery lists platforms whose post URLs never need query parameters;
// anything after "?" is share tracking (igsh, utm_source, _t, ...).
var dropAllQuery = map[string]bool{
	Instagram: true,
	TikTok:    true,
	Threads:   true,
	Pinterest: true,
	Spotify:   true,
	X:         true,
}

// trackingParams are removed from every other URL.
var trackingParams = map[string]bool{
	"fbclid": true, "gclid": true, "dclid": true, "msclkid": true, "mc_cid": true, "mc_eid": true,
	"igsh": true, "igshid": true, "si": true, "feature": true, "pp": true,
	"ref_src": true, "ref_url": true, "share_id": true, "rdt": true, "mibextid": true,
}

// Normalize parses raw (a bare URL or text containing one) and returns a
// canonical URL plus the platform it belongs to.
func Normalize(raw string) (normalized, platform string, err error) {
	s, err := Extract(strings.TrimSpace(raw))
	if err != nil {
		return "", "", err
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", "", ErrInvalidURL
	}

	u.Scheme = "https"
	u.User = nil
	u.Fragment = ""
	u.RawFragment = ""
	host := strings.ToLower(u.Hostname())
	if port := u.Port(); port != "" && port != "443" && port != "80" {
		host += ":" + port
	}

	bare := strings.TrimPrefix(strings.TrimPrefix(host, "www."), "m.")
	platform, known := hostPlatforms[bare]
	if !known {
		platform = Web
	}
	if known {
		host = bare
	}

	switch platform {
	case X:
		host = "x.com"
	case YouTube:
		// youtu.be/ID → youtube.com/watch?v=ID, keeping the timestamp.
		if bare == "youtu.be" {
			id := strings.Trim(u.Path, "/")
			q := u.Query()
			q.Set("v", id)
			u.RawQuery = q.Encode()
			u.Path = "/watch"
			host = "youtube.com"
		}
	}
	u.Host = host

	if dropAllQuery[platform] {
		u.RawQuery = ""
	} else if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			if trackingParams[strings.ToLower(k)] || strings.HasPrefix(strings.ToLower(k), "utm_") {
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode() // Encode sorts keys, so param order doesn't matter.
	}
	u.ForceQuery = false

	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
		u.RawPath = ""
	}
	return u.String(), platform, nil
}
