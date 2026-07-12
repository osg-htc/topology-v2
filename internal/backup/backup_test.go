package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bbockelm/topology-v2/internal/crypto"
)

// TestArchiveEncryptRoundTrip proves a directory survives archive -> encrypt ->
// decrypt -> extract byte-for-byte.
func TestArchiveEncryptRoundTrip(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"University of Chicago/FACILITY.yaml":          "ID: 10023\n",
		"University of Chicago/UChicago/SITE.yaml":      "ID: 10181\nCity: Chicago\n",
		"University of Chicago/UChicago/UChicago_X.yaml": "Production: true\n",
		"services.yaml": "CE: 1\n",
	}
	for rel, content := range files {
		full := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	archive, err := ArchiveDir(src)
	if err != nil {
		t.Fatalf("ArchiveDir: %v", err)
	}

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}
	enc, err := crypto.BackupSeal(masterKey, "topology-test", archive)
	if err != nil {
		t.Fatalf("BackupSeal: %v", err)
	}
	dec, err := crypto.BackupOpen(masterKey, "topology-test", enc)
	if err != nil {
		t.Fatalf("BackupOpen: %v", err)
	}

	dst := t.TempDir()
	if err := ExtractToDir(dec, dst); err != nil {
		t.Fatalf("ExtractToDir: %v", err)
	}

	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("missing %s after round-trip: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s content mismatch: got %q want %q", rel, got, want)
		}
	}
}

// TestWrongKeyFails confirms a different master key cannot decrypt.
func TestWrongKeyFails(t *testing.T) {
	k1 := make([]byte, 32)
	k2 := make([]byte, 32)
	k2[0] = 1
	enc, err := crypto.BackupSeal(k1, "b", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := crypto.BackupOpen(k2, "b", enc); err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}
