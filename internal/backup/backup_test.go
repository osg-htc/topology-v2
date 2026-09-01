package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// tarGz builds a gzip-compressed tar of files (path -> content), the same
// shape ExtractToDir expects (and what a GitHub tarball looks like).
func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	out := &bytes.Buffer{}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// TestExtractToDir_RoundTrip proves a tar.gz's files land byte-for-byte at
// their relative paths.
func TestExtractToDir_RoundTrip(t *testing.T) {
	files := map[string]string{
		"University of Chicago/FACILITY.yaml":            "ID: 10023\n",
		"University of Chicago/UChicago/SITE.yaml":       "ID: 10181\nCity: Chicago\n",
		"University of Chicago/UChicago/UChicago_X.yaml": "Production: true\n",
		"services.yaml": "CE: 1\n",
	}
	dst := t.TempDir()
	if err := ExtractToDir(tarGz(t, files), dst); err != nil {
		t.Fatalf("ExtractToDir: %v", err)
	}
	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("missing %s after extract: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s content mismatch: got %q want %q", rel, got, want)
		}
	}
}

// TestExtractToDir_ContainsPathTraversal confirms an archive entry with a
// ../-laden name can never land outside dst. ExtractToDir neutralizes it by
// resolving the name against dst as if it were root (so "../../etc/evil"
// lands at dst/etc/evil, not /etc/evil) rather than rejecting it outright --
// both are legitimate guards; this only checks the one it actually uses.
func TestExtractToDir_ContainsPathTraversal(t *testing.T) {
	dst := t.TempDir()
	malicious := tarGz(t, map[string]string{"../../etc/evil": "pwned"})
	if err := ExtractToDir(malicious, dst); err != nil {
		t.Fatalf("ExtractToDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "etc", "evil")); err != nil {
		t.Fatalf("expected the traversal entry neutralized at dst/etc/evil, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "etc", "evil")); err == nil {
		t.Error("traversal entry escaped dst entirely")
	}
}
