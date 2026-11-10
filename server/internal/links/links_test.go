package links

import (
	"errors"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name, in, want, platform string
	}{
		{"instagram drops share params and trailing slash",
			"https://www.instagram.com/p/DfYWVoBDC7L/?utm_source=ig_web_copy_link&obrf=NTc4MTIwNjQ2YQ==",
			"https://instagram.com/p/DfYWVoBDC7L", Instagram},
		{"instagram reel with igsh",
			"https://instagram.com/reel/abc123/?igsh=MWQ1ZGUxMzBkMA==",
			"https://instagram.com/reel/abc123", Instagram},
		{"tiktok text share",
			"Check out this video! https://www.tiktok.com/@cook/video/7312345?_r=1&_t=8abc.",
			"https://tiktok.com/@cook/video/7312345", TikTok},
		{"twitter becomes x and drops s/t",
			"https://twitter.com/someone/status/123?s=20&t=xyz",
			"https://x.com/someone/status/123", X},
		{"youtu.be becomes watch and keeps timestamp",
			"https://youtu.be/dQw4w9WgXcQ?si=abc&t=42",
			"https://youtube.com/watch?t=42&v=dQw4w9WgXcQ", YouTube},
		{"youtube mobile drops si",
			"https://m.youtube.com/watch?v=dQw4w9WgXcQ&si=tracking",
			"https://youtube.com/watch?v=dQw4w9WgXcQ", YouTube},
		{"web keeps real params, drops utm and fragment",
			"http://Example.com/Recipes/Pasta/?page=2&utm_campaign=x&fbclid=y#comments",
			"https://example.com/Recipes/Pasta?page=2", Web},
		{"web root path kept",
			"https://example.com/",
			"https://example.com/", Web},
		{"credentials stripped",
			"https://user:pass@example.com/a",
			"https://example.com/a", Web},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, platform, err := Normalize(tt.in)
			if err != nil {
				t.Fatalf("Normalize(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("url = %q, want %q", got, tt.want)
			}
			if platform != tt.platform {
				t.Errorf("platform = %q, want %q", platform, tt.platform)
			}
		})
	}
}

func TestNormalizeErrors(t *testing.T) {
	for _, in := range []string{"", "no link here", "ftp://example.com/file"} {
		if _, _, err := Normalize(in); !errors.Is(err, ErrNoURL) {
			t.Errorf("Normalize(%q) err = %v, want ErrNoURL", in, err)
		}
	}
}

func TestSameLinkTwiceNormalizesEqual(t *testing.T) {
	a, _, _ := Normalize("https://www.instagram.com/p/DfYWVoBDC7L/?utm_source=ig_web_copy_link")
	b, _, _ := Normalize("https://instagram.com/p/DfYWVoBDC7L?igsh=abc")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
}
