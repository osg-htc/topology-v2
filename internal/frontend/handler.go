// Package frontend serves the embedded Next.js single-page app with an
// index.html fallback for client-side routes. In dev builds (no embedded
// assets) it serves a placeholder and lets the Next.js dev server own the UI.
package frontend

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewSPAHandler returns an http.Handler that serves static files from the
// embedded SPA, falling back to index.html for unknown (client-routed) paths.
func NewSPAHandler() http.Handler {
	dist, ok := Dist()
	if !ok {
		return devHandler()
	}
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if reqPath == "" {
			reqPath = "index.html"
		}
		info, err := fs.Stat(dist, reqPath)
		if err != nil || info.IsDir() {
			// Not a servable file. A route like "/proposals" matches the
			// "proposals/" directory (it also has child pages); prefer the
			// exported page "proposals.html" over a directory listing. Then try
			// Next.js dynamic-route [id] exports, else fall back to index.html so
			// the client router can take over.
			switch {
			case fileExists(dist, reqPath+".html"):
				r.URL.Path = "/" + reqPath + ".html"
			default:
				if resolved, ok := resolveDynamic(dist, reqPath); ok {
					r.URL.Path = "/" + resolved
				} else {
					r.URL.Path = "/index.html"
				}
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

// fileExists reports whether name is a regular (non-directory) file in dist.
func fileExists(dist fs.FS, name string) bool {
	info, err := fs.Stat(dist, name)
	return err == nil && !info.IsDir()
}

// resolveDynamic maps a concrete path like "projects/abc" to a Next.js exported
// dynamic-route file like "projects/_.html" if one exists.
func resolveDynamic(dist fs.FS, reqPath string) (string, bool) {
	dir := path.Dir(reqPath)
	candidate := path.Join(dir, "_.html")
	if _, err := fs.Stat(dist, candidate); err == nil {
		return candidate, true
	}
	if _, err := fs.Stat(dist, reqPath+".html"); err == nil {
		return reqPath + ".html", true
	}
	return "", false
}
