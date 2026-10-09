package topology

import (
	"context"
	"os"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestSupportCenters_ContactsSurviveImportAndExport guards a real, silent loss:
// support-centers.yaml entries carry a Contacts block (40 of 52 real centers)
// but SupportCenterYAML had no field for it, so the importer dropped it -- and
// with it from every backup/restore cycle, since export writes what's in the DB.
func TestSupportCenters_ContactsSurviveImportAndExport(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	const src = `
Advanced LIGO:
  Community: Advanced LIGO
  Contacts:
    Security Contact:
    - ID: 547c65a6ed5e9e755c023418a47b8b92e88f0523
      Name: Peter Couvares
  Description: Advanced LIGO
  ID: 92
Plain:
  Community: Other
  Description: no contacts here
  ID: 7
`
	var scs map[string]SupportCenterYAML
	if err := yaml.Unmarshal([]byte(src), &scs); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := ImportSupportCenters(ctx, q, scs); err != nil {
		t.Fatalf("ImportSupportCenters: %v", err)
	}

	dir := t.TempDir()
	if err := ExportFullToDir(ctx, q, dir); err != nil {
		t.Fatalf("ExportFullToDir: %v", err)
	}
	got, err := ReadSupportCenters(dir)
	if err != nil {
		t.Fatalf("ReadSupportCenters: %v", err)
	}
	if !reflect.DeepEqual(got["Advanced LIGO"].Extra, scs["Advanced LIGO"].Extra) {
		t.Errorf("Contacts lost across import+export:\n got  %#v\n want %#v", got["Advanced LIGO"].Extra, scs["Advanced LIGO"].Extra)
	}
	if len(got["Plain"].Extra) != 0 {
		t.Errorf("a center with no extra keys gained some: %#v", got["Plain"].Extra)
	}
	if got["Advanced LIGO"].ID == nil || *got["Advanced LIGO"].ID != 92 {
		t.Errorf("ID not preserved: %v", got["Advanced LIGO"].ID)
	}
}

// TestSupportCenters_RealFileRoundTrip runs the same check over the real
// support-centers.yaml (52 centers, 40 with Contacts) when
// TOPOLOGY_TEST_REAL_TREE points at a v1 topology/ directory.
func TestSupportCenters_RealFileRoundTrip(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	root := os.Getenv("TOPOLOGY_TEST_REAL_TREE")
	if dbURL == "" || root == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL and TOPOLOGY_TEST_REAL_TREE to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	src, err := ReadSupportCenters(root)
	if err != nil || len(src) == 0 {
		t.Skipf("no support-centers.yaml under %s (%v)", root, err)
	}
	if err := ImportSupportCenters(ctx, q, src); err != nil {
		t.Fatalf("ImportSupportCenters: %v", err)
	}
	dir := t.TempDir()
	if err := ExportFullToDir(ctx, q, dir); err != nil {
		t.Fatalf("ExportFullToDir: %v", err)
	}
	got, err := ReadSupportCenters(dir)
	if err != nil {
		t.Fatalf("ReadSupportCenters(export): %v", err)
	}
	if len(got) != len(src) {
		t.Fatalf("center count %d -> %d", len(src), len(got))
	}
	withContacts := 0
	for name, want := range src {
		g := got[name]
		if g.LongName != want.LongName || g.Community != want.Community || g.Description != want.Description {
			t.Errorf("%s: scalar fields differ: %+v vs %+v", name, g, want)
		}
		if want.ID != nil && (g.ID == nil || *g.ID != *want.ID) {
			t.Errorf("%s: ID %v -> %v", name, want.ID, g.ID)
		}
		if !reflect.DeepEqual(g.Extra, want.Extra) {
			t.Errorf("%s: extra (Contacts) differ:\n got  %#v\n want %#v", name, g.Extra, want.Extra)
		}
		if _, ok := want.Extra["Contacts"]; ok {
			withContacts++
		}
	}
	t.Logf("%d centers round-tripped, %d with Contacts", len(src), withContacts)
}
