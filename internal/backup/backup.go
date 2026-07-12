// Package backup archives and restores the topology tree: tar.gz of the on-disk
// YAML form, encrypted for S3, and import from the existing topology GitHub repo
// (for the pre-switchover round-trip). Only administrators invoke these paths.
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ArchiveDir walks dir and returns a gzip-compressed tar of its files, with
// paths relative to dir.
func ArchiveDir(dir string) ([]byte, error) {
	out := &bytes.Buffer{}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hdr := &tar.Header{Name: filepath.ToSlash(rel), Mode: 0o644, Size: int64(len(data))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ExtractToDir extracts a gzip-compressed tar into dest. It guards against path
// traversal outside dest.
func ExtractToDir(data []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.Clean("/"+hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
}

// DownloadGitHubTarball fetches a repo tarball for ref via the GitHub API and
// returns its bytes. token may be empty for public repos.
func DownloadGitHubTarball(ctx context.Context, repo, ref, token string) ([]byte, error) {
	if ref == "" {
		ref = "master"
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/tarball/%s", repo, ref)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("github tarball status %d: %s", resp.StatusCode, body)
	}
	return io.ReadAll(resp.Body)
}

// FindTopologyRoot locates the "topology" directory within an extracted tree
// (GitHub tarballs wrap everything in a top-level <owner>-<repo>-<sha>/ dir).
func FindTopologyRoot(base string) (string, error) {
	// Direct child?
	if fi, err := os.Stat(filepath.Join(base, "topology")); err == nil && fi.IsDir() {
		return filepath.Join(base, "topology"), nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			candidate := filepath.Join(base, e.Name(), "topology")
			if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("no topology/ directory found in archive")
}
