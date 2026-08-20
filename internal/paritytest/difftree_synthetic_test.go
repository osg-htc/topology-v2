package paritytest

// TestDiffTrees validates the diff mechanism itself on synthetic cases before
// it's trusted against live v1/v2 output -- per the design review, a wrong
// default here (e.g. flagging harmless reordering, or missing a real added
// field) would be a systemic false-positive/false-negative source across
// every parity test built on top of it, not a local bug in one test.

import (
	"strings"
	"testing"
)

func TestDiffTrees_NoDiff_IdenticalTrees(t *testing.T) {
	a := map[string]interface{}{"Name": "foo", "@id": "1"}
	b := map[string]interface{}{"Name": "foo", "@id": "1"}
	if diffs := diffTrees("root", a, b, nil); len(diffs) != 0 {
		t.Fatalf("expected no diffs, got %v", diffs)
	}
}

func TestDiffTrees_AddedField(t *testing.T) {
	a := map[string]interface{}{"Name": "foo"}
	b := map[string]interface{}{"Name": "foo", "GridType": "OSG"}
	diffs := diffTrees("root", a, b, nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "root.GridType") || !strings.Contains(diffs[0], "v2 only") {
		t.Fatalf("expected exactly one 'v2 only' diff for GridType, got %v", diffs)
	}
}

func TestDiffTrees_RemovedField(t *testing.T) {
	a := map[string]interface{}{"Name": "foo", "GridType": "OSG"}
	b := map[string]interface{}{"Name": "foo"}
	diffs := diffTrees("root", a, b, nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "root.GridType") || !strings.Contains(diffs[0], "v1 only") {
		t.Fatalf("expected exactly one 'v1 only' diff for GridType, got %v", diffs)
	}
}

func TestDiffTrees_MissingKeyVsPresentEmpty_AreDifferentFindings(t *testing.T) {
	// Present-with-empty-string vs absent-entirely must not be silently
	// equated -- both are real, distinguishable states an XML/JSON document
	// can be in, and collapsing them would hide a real class of bug.
	a := map[string]interface{}{"Description": ""}
	b := map[string]interface{}{}
	diffs := diffTrees("root", a, b, nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "v1 only") {
		t.Fatalf("expected present-empty-vs-absent to be reported, got %v", diffs)
	}

	// But present-empty on both sides is genuinely no difference.
	c := map[string]interface{}{"Description": ""}
	if diffs := diffTrees("root", a, c, nil); len(diffs) != 0 {
		t.Fatalf("expected no diff when both sides are present-empty, got %v", diffs)
	}
}

func TestDiffTrees_ChangedLeafValue(t *testing.T) {
	a := map[string]interface{}{"Name": "old"}
	b := map[string]interface{}{"Name": "new"}
	diffs := diffTrees("root", a, b, nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "value differs") {
		t.Fatalf("expected a value-differs diff, got %v", diffs)
	}
}

func TestDiffTrees_ReorderedSiblings_NoDiff(t *testing.T) {
	// Same three elements, different order on each side -- must not be
	// flagged, since neither v1 nor v2 guarantees sibling ordering.
	a := []interface{}{
		map[string]interface{}{"Name": "one"},
		map[string]interface{}{"Name": "two"},
		map[string]interface{}{"Name": "three"},
	}
	b := []interface{}{
		map[string]interface{}{"Name": "three"},
		map[string]interface{}{"Name": "one"},
		map[string]interface{}{"Name": "two"},
	}
	if diffs := diffTrees("root", map[string]interface{}{"Item": a}, map[string]interface{}{"Item": b}, nil); len(diffs) != 0 {
		t.Fatalf("expected reordered-but-equal siblings to produce no diff, got %v", diffs)
	}
}

func TestDiffTrees_DifferentSiblingCount_IsFlagged(t *testing.T) {
	a := []interface{}{
		map[string]interface{}{"Name": "one"},
		map[string]interface{}{"Name": "two"},
	}
	b := []interface{}{
		map[string]interface{}{"Name": "one"},
	}
	diffs := diffTrees("root", map[string]interface{}{"Item": a}, map[string]interface{}{"Item": b}, nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "v1 only") {
		t.Fatalf("expected a 'present in v1 only' diff for the extra element, got %v", diffs)
	}
}

func TestDiffTrees_NestedReorderedSiblings_NoDiff(t *testing.T) {
	// A repeated element that itself contains a reordered nested repeated
	// list must still compare equal -- canonicalize must recurse into slices,
	// not just sort the top level.
	mk := func(tags ...string) map[string]interface{} {
		tagNodes := make([]interface{}, len(tags))
		for i, tg := range tags {
			tagNodes[i] = map[string]interface{}{"#text": tg}
		}
		return map[string]interface{}{"Name": "group", "Tag": tagNodes}
	}
	a := []interface{}{mk("alpha", "beta")}
	b := []interface{}{mk("beta", "alpha")}
	if diffs := diffTrees("root", map[string]interface{}{"Group": a}, map[string]interface{}{"Group": b}, nil); len(diffs) != 0 {
		t.Fatalf("expected nested reordering to produce no diff, got %v", diffs)
	}
}

func TestDiffTrees_OneChangedSibling_GivesFieldLevelDiff(t *testing.T) {
	// When exactly one element is unmatched on each side (one thing changed,
	// not a structural add/remove), the report should point at the specific
	// field that changed, not an opaque whole-element content blob.
	a := []interface{}{
		map[string]interface{}{"ID": map[string]interface{}{"#text": "446"}, "Name": map[string]interface{}{"#text": "old"}},
	}
	b := []interface{}{
		map[string]interface{}{"ID": map[string]interface{}{"#text": "446"}, "Name": map[string]interface{}{"#text": "new"}},
	}
	diffs := diffTrees("root", map[string]interface{}{"Resource": a}, map[string]interface{}{"Resource": b}, nil)
	if len(diffs) != 1 {
		t.Fatalf("expected exactly one field-level diff, got %v", diffs)
	}
	if !strings.Contains(diffs[0], "Name") || !strings.Contains(diffs[0], "value differs") {
		t.Fatalf("expected the diff to name the Name field specifically, got %v", diffs[0])
	}
	if strings.Contains(diffs[0], "446") {
		t.Fatalf("expected the diff to stay scoped to the changed field, not dump the unchanged ID too: %v", diffs[0])
	}
}

func TestDiffTrees_IgnoreList_SuppressesKnownPath(t *testing.T) {
	a := map[string]interface{}{"LastUpdated": "2026-01-01"}
	b := map[string]interface{}{"LastUpdated": "2026-08-20"}
	ignore := map[string]struct{}{"root.LastUpdated": {}}
	if diffs := diffTrees("root", a, b, ignore); len(diffs) != 0 {
		t.Fatalf("expected ignored path to suppress the diff, got %v", diffs)
	}
	// Without the ignore entry, the same diff must still surface.
	if diffs := diffTrees("root", a, b, nil); len(diffs) != 1 {
		t.Fatalf("expected the diff to surface without an ignore entry, got %v", diffs)
	}
}
