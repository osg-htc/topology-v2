package xmlapi_test

import (
	"context"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
	"github.com/bbockelm/topology-v2/internal/xmlapi"
)

// TestXMLValidatesAgainstXSD imports a topology tree, generates /rgsummary/xml
// and /rgdowntime/xml, and validates each against the legacy XSD with xmllint.
// Requires TOPOLOGY_TEST_DATABASE_URL and xmllint; skipped otherwise.
func TestXMLValidatesAgainstXSD(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run the XSD validation test")
	}
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint not available")
	}

	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	root := filepath.Join("..", "topology", "testdata", "topology")
	if real := os.Getenv("TOPOLOGY_TEST_REAL_TREE"); real != "" {
		root = real
	}
	if err := topology.ImportTree(ctx, q, root); err != nil {
		t.Fatalf("ImportTree: %v", err)
	}
	// Import VOs/projects too (from a repo checkout) so their XML is validated.
	if repo := os.Getenv("TOPOLOGY_TEST_REPO_ROOT"); repo != "" {
		if err := topology.ImportVOs(ctx, q, filepath.Join(repo, "virtual-organizations")); err != nil {
			t.Fatalf("ImportVOs: %v", err)
		}
		if err := topology.ImportReportingGroups(ctx, q, filepath.Join(repo, "virtual-organizations")); err != nil {
			t.Fatalf("ImportReportingGroups: %v", err)
		}
		if err := topology.ImportProjects(ctx, q, filepath.Join(repo, "projects")); err != nil {
			t.Fatalf("ImportProjects: %v", err)
		}
		if err := topology.ImportCampusGrids(ctx, q, filepath.Join(repo, "projects")); err != nil {
			t.Fatalf("ImportCampusGrids: %v", err)
		}
	}

	summary, err := xmlapi.BuildResourceSummary(ctx, q, nil, xmlapi.Filters{}, false)
	if err != nil {
		t.Fatalf("BuildResourceSummary: %v", err)
	}
	validate(t, q, "rgsummary.xsd", summary)

	dts, err := xmlapi.BuildDowntimes(ctx, q, xmlapi.Filters{}, time.Now())
	if err != nil {
		t.Fatalf("BuildDowntimes: %v", err)
	}
	validate(t, q, "rgdowntime.xsd", dts)

	// The VOSummary/Projects XSDs require at least one VO/Project, so only
	// validate when data was imported (i.e. a repo root was supplied).
	vos, err := xmlapi.BuildVOSummary(ctx, q, false)
	if err != nil {
		t.Fatalf("BuildVOSummary: %v", err)
	}
	if len(vos.VOs) > 0 {
		validate(t, q, "vosummary.xsd", vos)
	}

	projects, err := xmlapi.BuildProjects(ctx, q)
	if err != nil {
		t.Fatalf("BuildProjects: %v", err)
	}
	if len(projects.Projects) > 0 {
		validate(t, q, "miscproject.xsd", projects)
	}
}

func validate(t *testing.T, _ *db.Queries, xsd string, v any) {
	t.Helper()
	body, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	doc := append([]byte(xml.Header), body...)

	tmp := filepath.Join(t.TempDir(), "out.xml")
	if err := os.WriteFile(tmp, doc, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := exec.Command("xmllint", "--noout", "--schema",
		filepath.Join("schema", xsd), tmp).CombinedOutput()
	if err != nil {
		t.Fatalf("xmllint failed for %s: %v\n%s\n--- doc (first 3k) ---\n%s", xsd, err, out, truncate(doc, 3000))
	}
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
