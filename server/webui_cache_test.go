package server

import "testing"

// TestWebUICacheControl: hashed build assets are cached for good; files whose
// URL stays the same across builds (pages, the logo, icons) are revalidated, so
// a changed logo shows after an upgrade instead of a cached old one.
func TestWebUICacheControl(t *testing.T) {
	cases := map[string]string{
		"_next/static/chunks/app-3f9a.js": "public, max-age=31536000, immutable",
		"_next/static/css/1a2b.css":       "public, max-age=31536000, immutable",
		"logo.png":                        "no-cache",
		"login.jpg":                       "no-cache",
		"favicon.ico":                     "no-cache",
		"index.html":                      "no-cache",
		"login":                           "no-cache",
		"_next/data/x.json":               "no-cache",
	}
	for p, want := range cases {
		if got := webuiCacheControl(p); got != want {
			t.Errorf("%s: Cache-Control %q, want %q", p, got, want)
		}
	}
}
