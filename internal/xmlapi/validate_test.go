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

	summary, err := xmlapi.BuildResourceSummary(ctx, q, xmlapi.Filters{}, false)
	if err != nil {
		t.Fatalf("BuildResourceSummary: %v", err)
	}
	validate(t, q, "rgsummary.xsd", summary)

	dts, err := xmlapi.BuildDowntimes(ctx, q, xmlapi.Filters{}, time.Now())
	if err != nil {
		t.Fatalf("BuildDowntimes: %v", err)
	}
	validate(t, q, "rgdowntime.xsd", dts)
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
