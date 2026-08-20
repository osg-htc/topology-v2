package paritytest

// TestParity_Downtime_AMNHARESCE1 diffs a bounded set of downtimes -- every
// <Downtime> whose ResourceName is "AMNH-ARES-CE1" -- between v1 and v2's
// /rgdowntime/xml, rather than the whole feed. That resource has two
// downtimes on both sides, untouched by any of this session's testing, and
// keeping the comparison bounded avoids the (separately investigated, not a
// code bug) FZU_STASH data-staleness difference between the two datasets
// dominating an unrelated result.
//
// Requires TOPOLOGY_TEST_V1_URL and TOPOLOGY_TEST_V2_URL; skipped otherwise.

import (
	"os"
	"testing"
)

func TestParity_Downtime_AMNHARESCE1(t *testing.T) {
	v1URL := os.Getenv("TOPOLOGY_TEST_V1_URL")
	v2URL := os.Getenv("TOPOLOGY_TEST_V2_URL")
	if v1URL == "" || v2URL == "" {
		t.Skip("set TOPOLOGY_TEST_V1_URL and TOPOLOGY_TEST_V2_URL to run the live parity test")
	}

	v1Tree, err := fetchXMLTree(v1URL + "/rgdowntime/xml")
	if err != nil {
		t.Fatalf("fetch v1: %v", err)
	}
	v2Tree, err := fetchXMLTree(v2URL + "/rgdowntime/xml")
	if err != nil {
		t.Fatalf("fetch v2: %v", err)
	}

	const resourceName = "AMNH-ARES-CE1"
	v1DTs := findDowntimesByResourceName(v1Tree, resourceName)
	v2DTs := findDowntimesByResourceName(v2Tree, resourceName)
	if len(v1DTs) == 0 {
		t.Fatalf("no v1 downtimes found for %s", resourceName)
	}
	if len(v2DTs) == 0 {
		t.Fatalf("no v2 downtimes found for %s", resourceName)
	}

	diffs := diffTrees("Downtime", map[string]interface{}{"Downtime": v1DTs}, map[string]interface{}{"Downtime": v2DTs}, nil)
	if len(diffs) != 0 {
		for _, d := range diffs {
			t.Logf("diff: %s", d)
		}
		t.Errorf("%d diff(s) between v1 and v2 for %s's downtimes", len(diffs), resourceName)
	}
}

// findDowntimesByResourceName collects every <Downtime> across the
// Past/Current/Future buckets whose ResourceName matches.
func findDowntimesByResourceName(root map[string]interface{}, name string) []interface{} {
	var out []interface{}
	for _, bucket := range []string{"PastDowntimes", "CurrentDowntimes", "FutureDowntimes"} {
		buckets, _ := root[bucket].([]interface{})
		for _, b := range buckets {
			bm, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			dts, _ := bm["Downtime"].([]interface{})
			for _, dt := range dts {
				dtm, ok := dt.(map[string]interface{})
				if !ok {
					continue
				}
				rns, _ := dtm["ResourceName"].([]interface{})
				if len(rns) != 1 {
					continue
				}
				rn, _ := rns[0].(map[string]interface{})
				if rn["#text"] == name {
					out = append(out, dt)
				}
			}
		}
	}
	return out
}
