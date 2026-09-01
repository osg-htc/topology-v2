package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/bbockelm/topology-v2/internal/backup"
	"github.com/bbockelm/topology-v2/internal/topology"
)

type githubImportRequest struct {
	Ref string `json:"ref"`
}

// ImportFromGitHub pulls the existing topology GitHub repo and imports it,
// replacing the current topology domain. Administrator only.
func (h *Handler) ImportFromGitHub(w http.ResponseWriter, r *http.Request) {
	var req githubImportRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	ctx := r.Context()

	data, err := backup.DownloadGitHubTarball(ctx, h.cfg.GitHubRepo, req.Ref, h.cfg.GitHubToken)
	if err != nil {
		respondError(w, http.StatusBadGateway, "github: "+err.Error())
		return
	}
	dir, err := os.MkdirTemp("", "topo-gh-")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "temp dir")
		return
	}
	defer os.RemoveAll(dir)
	if err := backup.ExtractToDir(data, dir); err != nil {
		respondError(w, http.StatusInternalServerError, "extract: "+err.Error())
		return
	}
	topoRoot, err := backup.FindTopologyRoot(dir)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Import the whole repo (tree + VOs + projects); repoRoot is topoRoot's parent.
	if err := h.restoreFromDir(ctx, topoRoot); err != nil {
		respondError(w, http.StatusInternalServerError, "import: "+err.Error())
		return
	}
	h.audit(ctx, h.currentUser(r).ID, "restore.import_github", "restore", h.cfg.GitHubRepo, "", nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "imported", "repo": h.cfg.GitHubRepo})
}

// restoreFromDir replaces the topology domain with the contents of a topology
// tree directory and its sibling VO/project dirs (truncate + import).
func (h *Handler) restoreFromDir(ctx context.Context, topoDir string) error {
	if err := h.queries.TruncateTopology(ctx); err != nil {
		return err
	}
	return topology.ImportRepo(ctx, h.queries, topoDir)
}
