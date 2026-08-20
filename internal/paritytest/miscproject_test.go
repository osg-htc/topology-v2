package paritytest

// TestParity_Project_UCBerkeleyAltman diffs a single project between v1 and
// v2's /miscproject/xml. Requires TOPOLOGY_TEST_V1_URL and
// TOPOLOGY_TEST_V2_URL; skipped otherwise.

import (
	"os"
	"testing"
)

func TestParity_Project_UCBerkeleyAltman(t *testing.T) {
	v1URL := os.Getenv("TOPOLOGY_TEST_V1_URL")
	v2URL := os.Getenv("TOPOLOGY_TEST_V2_URL")
	if v1URL == "" || v2URL == "" {
		t.Skip("set TOPOLOGY_TEST_V1_URL and TOPOLOGY_TEST_V2_URL to run the live parity test")
	}

	v1Tree, err := fetchXMLTree(v1URL + "/miscproject/xml")
	if err != nil {
		t.Fatalf("fetch v1: %v", err)
	}
	v2Tree, err := fetchXMLTree(v2URL + "/miscproject/xml")
	if err != nil {
		t.Fatalf("fetch v2: %v", err)
	}

	const projectName = "UCBerkeley_Altman"
	v1Proj, err := findByChildText(v1Tree, "Project", "Name", projectName)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	v2Proj, err := findByChildText(v2Tree, "Project", "Name", projectName)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}

	diffs := diffTrees("Project", v1Proj, v2Proj, nil)
	if len(diffs) != 0 {
		for _, d := range diffs {
			t.Logf("diff: %s", d)
		}
		t.Errorf("%d diff(s) between v1 and v2 for project %s", len(diffs), projectName)
	}
}
