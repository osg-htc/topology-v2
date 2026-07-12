package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bbockelm/topology-v2/internal/backup"
	"github.com/bbockelm/topology-v2/internal/crypto"
	"github.com/bbockelm/topology-v2/internal/topology"
)

const backupPrefix = "backups/"
const backupSuffix = ".tar.gz.enc"

// CreateBackup exports the topology tree, encrypts it, and uploads it to S3.
// Administrator only.
func (h *Handler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondError(w, http.StatusServiceUnavailable, "object storage not configured")
		return
	}
	ctx := r.Context()
	dir, err := os.MkdirTemp("", "topo-backup-")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "temp dir")
		return
	}
	defer os.RemoveAll(dir)

	if err := topology.ExportRepoToDir(ctx, h.queries, dir); err != nil {
		respondError(w, http.StatusInternalServerError, "export: "+err.Error())
		return
	}
	archive, err := backup.ArchiveDir(dir)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "archive: "+err.Error())
		return
	}
	name := "topology-" + time.Now().UTC().Format("20060102-150405")
	enc, err := crypto.BackupSeal(h.cfg.MasterKey(), name, archive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "encrypt: "+err.Error())
		return
	}
	key := backupPrefix + name + backupSuffix
	if err := h.store.Upload(ctx, key, enc, "application/octet-stream"); err != nil {
		respondError(w, http.StatusInternalServerError, "upload: "+err.Error())
		return
	}
	h.audit(ctx, h.currentUser(r).ID, "backup.create", "backup", key, "", nil)
	respondJSON(w, http.StatusCreated, map[string]any{"key": key, "size": len(enc)})
}

// ListBackups lists S3 backup objects. Administrator only.
func (h *Handler) ListBackups(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondError(w, http.StatusServiceUnavailable, "object storage not configured")
		return
	}
	keys, err := h.store.ListKeys(r.Context(), backupPrefix)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"backups": keys})
}

type restoreRequest struct {
	Key string `json:"key"`
}

// RestoreBackup downloads, decrypts, and imports an S3 backup, replacing the
// current topology domain. Administrator only.
func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		respondError(w, http.StatusServiceUnavailable, "object storage not configured")
		return
	}
	var req restoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		respondError(w, http.StatusBadRequest, "key required")
		return
	}
	ctx := r.Context()
	enc, err := h.store.Download(ctx, req.Key)
	if err != nil {
		respondError(w, http.StatusNotFound, "backup not found")
		return
	}
	name := strings.TrimSuffix(strings.TrimPrefix(req.Key, backupPrefix), backupSuffix)
	archive, err := crypto.BackupOpen(h.cfg.MasterKey(), name, enc)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "decrypt: "+err.Error())
		return
	}
	dir, err := os.MkdirTemp("", "topo-restore-")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "temp dir")
		return
	}
	defer os.RemoveAll(dir)
	if err := backup.ExtractToDir(archive, dir); err != nil {
		respondError(w, http.StatusInternalServerError, "extract: "+err.Error())
		return
	}
	if err := h.restoreFromDir(ctx, dir); err != nil {
		respondError(w, http.StatusInternalServerError, "restore: "+err.Error())
		return
	}
	h.audit(ctx, h.currentUser(r).ID, "backup.restore", "backup", req.Key, "", nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

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
	h.audit(ctx, h.currentUser(r).ID, "backup.import_github", "backup", h.cfg.GitHubRepo, "", nil)
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
