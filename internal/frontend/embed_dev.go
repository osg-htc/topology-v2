//go:build !embed_frontend

package frontend

import (
	"io/fs"
	"net/http"
)

// Dist returns an empty FS in dev builds; the Next.js dev server serves the UI
// and proxies /api to this backend. Production builds use the embed_frontend
// tag (embed.go) to compile the exported SPA into the binary.
func Dist() (fs.FS, bool) {
	return nil, false
}

// devPlaceholder is served at the root in dev builds so hitting the backend
// directly returns something friendly instead of a 404.
const devPlaceholder = `<!doctype html><html><head><title>Topology (dev)</title></head>
<body style="font-family:system-ui;margin:3rem">
<h1>Topology backend (dev mode)</h1>
<p>The API is served under <code>/api/v1</code>. Run the Next.js dev server for the UI.</p>
</body></html>`

func devHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(devPlaceholder))
	})
}
