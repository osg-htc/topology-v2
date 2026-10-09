package topology

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// copyTree copies a directory tree.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Large integers in the parts of a resource kept as loosely-typed JSON
// (WLCGInformation, a service's Details, and the Extra catch-all for unmodeled
// keys) must come back
// out of a backup/export as the same integers. They used to come back as
// 1.56e+07: the export decoded the stored JSON with a plain json.Unmarshal,
// which turns every integer into a float64, and YAML writes a float64 of a
// million or more in exponent form. Three real resources (UFlorida-CMS, -HPC,
// -HPG2) carry KSI2KMax: 15600000 / KSI2KMin: 4800000.
func TestExport_LargeIntegersSurviveBackupAndRestore(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	src := t.TempDir()
	copyTree(t, filepath.Join("testdata", "topology"), src)
	rgFile := filepath.Join(src, "University of Chicago", "UChicago", "UChicago_OSGConnect.yaml")
	b, err := os.ReadFile(rgFile)
	if err != nil {
		t.Fatal(err)
	}
	const anchor = "    VOOwnership:\n      OSG: 100\n"
	if !strings.Contains(string(b), anchor) {
		t.Fatalf("fixture changed; anchor not found")
	}
	const detailsAnchor = "        Details:\n          hidden: false\n"
	if !strings.Contains(string(b), detailsAnchor) {
		t.Fatalf("fixture changed; service Details anchor not found")
	}
	b = []byte(strings.Replace(string(b), detailsAnchor, detailsAnchor+"          Limit: 3000000\n", 1))
	edited := strings.Replace(string(b), anchor, anchor+
		"    WLCGInformation:\n      KSI2KMax: 15600000\n      KSI2KMin: 4800000\n      HEPSPEC: 14000\n"+
		"    UnmodeledCounter: 2000000\n", 1)
	if err := os.WriteFile(rgFile, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	tree, err := ReadTree(src)
	if err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	if err := Import(ctx, q, tree); err != nil {
		t.Fatalf("Import: %v", err)
	}
	exported, err := Export(ctx, q)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	dst := t.TempDir()
	if err := WriteTree(dst, exported); err != nil {
		t.Fatalf("WriteTree: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(dst, "University of Chicago", "UChicago", "UChicago_OSGConnect.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"KSI2KMax: 15600000", "KSI2KMin: 4800000", "HEPSPEC: 14000", "UnmodeledCounter: 2000000", "Limit: 3000000"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("exported YAML lost %q:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), "e+") {
		t.Errorf("an integer was exported in exponent form:\n%s", out)
	}
}
