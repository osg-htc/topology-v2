package paritytest

// TestParity_VO_OSG diffs the OSG VO between v1 and v2's /vosummary/xml.
// Requires TOPOLOGY_TEST_V1_URL and TOPOLOGY_TEST_V2_URL; skipped otherwise.

import (
	"fmt"
	"os"
	"testing"
)

func TestParity_VO_OSG(t *testing.T) {
	v1URL := os.Getenv("TOPOLOGY_TEST_V1_URL")
	v2URL := os.Getenv("TOPOLOGY_TEST_V2_URL")
	if v1URL == "" || v2URL == "" {
		t.Skip("set TOPOLOGY_TEST_V1_URL and TOPOLOGY_TEST_V2_URL to run the live parity test")
	}

	v1Tree, err := fetchXMLTree(v1URL + "/vosummary/xml")
	if err != nil {
		t.Fatalf("fetch v1: %v", err)
	}
	v2Tree, err := fetchXMLTree(v2URL + "/vosummary/xml")
	if err != nil {
		t.Fatalf("fetch v2: %v", err)
	}

	const voName = "OSG"
	v1VO, err := findByChildText(v1Tree, "VO", "Name", voName)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	v2VO, err := findByChildText(v2Tree, "VO", "Name", voName)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}

	diffs := diffTrees("VO", v1VO, v2VO, nil)
	if len(diffs) != 0 {
		for _, d := range diffs {
			t.Logf("diff: %s", d)
		}
		t.Errorf("%d diff(s) between v1 and v2 for VO %s", len(diffs), voName)
	}
}

// findByChildText locates the element under root[tag] whose child childTag's
// text content equals value.
func findByChildText(root map[string]interface{}, tag, childTag, value string) (map[string]interface{}, error) {
	items, _ := root[tag].([]interface{})
	for _, item := range items {
		im, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		children, _ := im[childTag].([]interface{})
		if len(children) != 1 {
			continue
		}
		cm, _ := children[0].(map[string]interface{})
		if cm["#text"] == value {
			return im, nil
		}
	}
	return nil, fmt.Errorf("no %s with %s=%s found (root had %d)", tag, childTag, value, len(items))
}
