package topology

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestVOProjectRoundTrip proves VOs (verbatim documents) and projects
// (relational columns) survive import -> DB -> export. Requires
// TOPOLOGY_TEST_DATABASE_URL; point TOPOLOGY_TEST_REPO_ROOT at a topology repo
// checkout (containing virtual-organizations/ and projects/) to run against real
// data, otherwise the small fixtures below are used.
func TestVOProjectRoundTrip(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	voDir, projDir := fixtureVOProject(t)
	if repo := os.Getenv("TOPOLOGY_TEST_REPO_ROOT"); repo != "" {
		voDir = filepath.Join(repo, "virtual-organizations")
		projDir = filepath.Join(repo, "projects")
	}

	if err := ImportVOs(ctx, q, voDir); err != nil {
		t.Fatalf("ImportVOs: %v", err)
	}
	if err := ImportProjects(ctx, q, projDir); err != nil {
		t.Fatalf("ImportProjects: %v", err)
	}

	outVO := t.TempDir()
	outProj := t.TempDir()
	if err := ExportVOsToDir(ctx, q, outVO); err != nil {
		t.Fatalf("ExportVOsToDir: %v", err)
	}
	if err := ExportProjectsToDir(ctx, q, outProj); err != nil {
		t.Fatalf("ExportProjectsToDir: %v", err)
	}

	// VOs are stored verbatim: bytes must match.
	assertDirsByteEqual(t, voDir, outVO)
	// Projects are reconstructed: compare parsed structs (modulo formatting).
	assertProjectsEquivalent(t, projDir, outProj)
}

func fixtureVOProject(t *testing.T) (voDir, projDir string) {
	t.Helper()
	base := t.TempDir()
	voDir = filepath.Join(base, "virtual-organizations")
	projDir = filepath.Join(base, "projects")
	mustWrite(t, filepath.Join(voDir, "OSG.yaml"),
		"ID: 30\nName: OSG\nDisable: false\nLongName: Open Science Grid\n")
	mustWrite(t, filepath.Join(projDir, "ACE_NIAID.yaml"),
		"Description: A project.\nDepartment: OCICB\nFieldOfScience: Bioinformatics\n"+
			"Organization: NIAID\nPIName: Darrell Hurt\nID: '782'\n"+
			"Sponsor:\n  CampusGrid:\n    Name: OSG Connect\n"+
			"InstitutionID: 'https://osg-htc.org/iid/451cgt72wj62'\nFieldOfScienceID: '26.1103'\n")
	return voDir, projDir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertDirsByteEqual(t *testing.T, want, got string) {
	t.Helper()
	entries, _ := os.ReadDir(want)
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		w, _ := os.ReadFile(filepath.Join(want, e.Name()))
		g, err := os.ReadFile(filepath.Join(got, e.Name()))
		if err != nil {
			t.Errorf("VO %s missing after round-trip", e.Name())
			continue
		}
		if string(w) != string(g) {
			t.Errorf("VO %s changed: want %q got %q", e.Name(), w, g)
		}
	}
}

func assertProjectsEquivalent(t *testing.T, want, got string) {
	t.Helper()
	wp, err := ReadProjects(want)
	if err != nil {
		t.Fatalf("ReadProjects(want): %v", err)
	}
	gp, err := ReadProjects(got)
	if err != nil {
		t.Fatalf("ReadProjects(got): %v", err)
	}
	index := map[string]Project{}
	for _, p := range gp {
		index[p.Name] = p
	}
	for _, w := range wp {
		g, ok := index[w.Name]
		if !ok {
			t.Errorf("project %q missing after round-trip", w.Name)
			continue
		}
		if !reflect.DeepEqual(canonProject(w), canonProject(g)) {
			t.Errorf("project %q mismatch:\n want %#v\n got  %#v", w.Name, canonProject(w), canonProject(g))
		}
	}
	if len(wp) != len(gp) {
		t.Errorf("project count changed: want %d got %d", len(wp), len(gp))
	}
}

// canonProject normalizes a project to comparable generic form (numbers coerced,
// nil map entries dropped) so formatting differences don't cause false failures.
func canonProject(p Project) map[string]interface{} {
	m := map[string]interface{}{
		"ID": p.ID, "Description": p.Description, "Department": p.Department,
		"FieldOfScience": p.FieldOfScience, "FieldOfScienceID": p.FieldOfScienceID,
		"Organization": p.Organization, "PIName": p.PIName, "InstitutionID": p.InstitutionID,
		"Sponsor": normalize(p.Sponsor), "Extra": normalize(p.Extra),
	}
	return m
}
