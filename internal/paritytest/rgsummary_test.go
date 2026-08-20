package paritytest

// TestParity_ResourceGroup446 is the first live parity test: it fetches
// /rgsummary/xml from a running v1 and a running v2 instance, decodes both
// generically (see decode_test.go), and diffs (see difftree_test.go) just the
// "Advanced Technology Lab" resource group (GroupID 331, containing resource
// topology_id 446 "ATLab testing node") -- not the whole feed, so the result
// isn't noise from unrelated live-data drift elsewhere in the dataset.
//
// Requires TOPOLOGY_TEST_V1_URL and TOPOLOGY_TEST_V2_URL (e.g.
// http://localhost:9000 / http://localhost:8080); skipped otherwise, so
// `go test ./...` stays green without both instances running. No existing CI
// job starts both a v1 and a v2 instance, so this stays local/manual for now.

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const targetGroupID = "331" // "Advanced Technology Lab"

func TestParity_ResourceGroup446(t *testing.T) {
	v1URL := os.Getenv("TOPOLOGY_TEST_V1_URL")
	v2URL := os.Getenv("TOPOLOGY_TEST_V2_URL")
	if v1URL == "" || v2URL == "" {
		t.Skip("set TOPOLOGY_TEST_V1_URL and TOPOLOGY_TEST_V2_URL to run the live parity test")
	}

	v1Tree, err := fetchXMLTree(v1URL + "/rgsummary/xml")
	if err != nil {
		t.Fatalf("fetch v1: %v", err)
	}
	v2Tree, err := fetchXMLTree(v2URL + "/rgsummary/xml")
	if err != nil {
		t.Fatalf("fetch v2: %v", err)
	}

	v1Group, err := findGroupByID(v1Tree, targetGroupID)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	v2Group, err := findGroupByID(v2Tree, targetGroupID)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}

	ignore := map[string]struct{}{
		// none yet -- any diff found below must be explained, not suppressed
		// reflexively (see the plan's loop/exit-criteria).
	}
	diffs := diffTrees("ResourceGroup", v1Group, v2Group, ignore)
	if len(diffs) != 0 {
		for _, d := range diffs {
			t.Logf("diff: %s", d)
		}
		t.Errorf("%d diff(s) between v1 and v2 for resource group %s", len(diffs), targetGroupID)
	}
}

func fetchXMLTree(url string) (map[string]interface{}, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, truncate(string(body), 500))
	}
	return decodeXML(resp.Body)
}

// findGroupByID locates the <ResourceGroup> whose <GroupID> text equals id.
func findGroupByID(root map[string]interface{}, id string) (map[string]interface{}, error) {
	groups, _ := root["ResourceGroup"].([]interface{})
	for _, g := range groups {
		gm, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		gids, _ := gm["GroupID"].([]interface{})
		if len(gids) != 1 {
			continue
		}
		gidNode, _ := gids[0].(map[string]interface{})
		if gidNode["#text"] == id {
			return gm, nil
		}
	}
	return nil, fmt.Errorf("no ResourceGroup with GroupID=%s found (root had %d groups)", id, len(groups))
}
