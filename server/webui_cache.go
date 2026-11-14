package server

import "strings"

// webuiCacheControl is the caching policy for an embedded UI path. Files under
// _next/static/ have content-hashed names, so a new build never reuses a URL and
// they can be cached for good. Everything else keeps its URL across builds
// (pages, /logo.png, icons, login.jpg), so the browser must revalidate it: after
// an upgrade it then shows the new page or logo instead of a cached old one.
func webuiCacheControl(p string) string {
	if strings.HasPrefix(p, "_next/static/") {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}
