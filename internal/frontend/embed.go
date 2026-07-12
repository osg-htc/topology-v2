//go:build embed_frontend

package frontend

import (
	"embed"
	"io/fs"
	"net/http"
)

// distFS holds the exported Next.js SPA, compiled in under the embed_frontend
// build tag. The dist/ directory is populated by `make build-prod` before the
// tagged Go build runs.
//
//go:embed all:dist
var distFS embed.FS

// Dist returns the embedded SPA filesystem rooted at dist/.
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, false
	}
	return sub, true
}

func devHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "frontend not available", http.StatusNotFound)
	})
}
