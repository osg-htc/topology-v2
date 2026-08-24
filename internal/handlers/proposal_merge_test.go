package handlers

import (
	"encoding/json"
	"testing"

	"github.com/bbockelm/topology-v2/internal/models"
)

func rawJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling test fixture: %v", err)
	}
	return b
}

func decodeMap(t *testing.T, raw json.RawMessage) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshaling result: %v (raw: %s)", err, raw)
	}
	return m
}

func TestMergeShallow(t *testing.T) {
	current := map[string]interface{}{"a": "old-a", "b": "old-b", "c": "old-c"}
	incoming := map[string]interface{}{"b": "new-b", "d": "new-d"}
	merged := mergeShallow(current, incoming)

	if merged["a"] != "old-a" {
		t.Errorf("key absent from incoming must be preserved: a = %v, want old-a", merged["a"])
	}
	if merged["b"] != "new-b" {
		t.Errorf("key present in incoming must override: b = %v, want new-b", merged["b"])
	}
	if merged["c"] != "old-c" {
		t.Errorf("c = %v, want old-c (untouched)", merged["c"])
	}
	if merged["d"] != "new-d" {
		t.Errorf("a brand-new key in incoming must appear: d = %v, want new-d", merged["d"])
	}
}

func TestMergeShallow_ExplicitNullClears(t *testing.T) {
	// Presence, not value, is what wins -- an explicit null/empty in incoming
	// is a deliberate clear, not "leave untouched".
	current := map[string]interface{}{"tags": []interface{}{"a", "b"}}
	incoming := map[string]interface{}{"tags": nil}
	merged := mergeShallow(current, incoming)
	if merged["tags"] != nil {
		t.Errorf("explicit null in incoming should clear the field: got %v", merged["tags"])
	}
}

func TestMergeShallow_NestedValueReplacedWhole(t *testing.T) {
	// Deliberately shallow, not recursive: a nested object in incoming
	// replaces current's nested object wholesale rather than being merged
	// key-by-key. This is what lets deleting an entry from a nested
	// collection (e.g. a resource's Services map) actually delete it.
	current := map[string]interface{}{
		"nested": map[string]interface{}{"x": "old-x", "y": "old-y"},
	}
	incoming := map[string]interface{}{
		"nested": map[string]interface{}{"x": "new-x"},
	}
	merged := mergeShallow(current, incoming)
	nested := merged["nested"].(map[string]interface{})
	if _, stillThere := nested["y"]; stillThere {
		t.Errorf("nested key y survived a shallow merge -- mergeShallow is supposed to replace nested values wholesale, not deep-merge them")
	}
	if nested["x"] != "new-x" {
		t.Errorf("nested.x = %v, want new-x", nested["x"])
	}
}

func TestMergeProposedState_NonResourceKind_PreservesUnmodeledFields(t *testing.T) {
	base := rawJSON(t, map[string]interface{}{
		"name":          "Site1",
		"facility":      "Fac1",
		"description":   "the original description",
		"address_line1": "123 Main St",
		"contacts":      []interface{}{map[string]interface{}{"contact_type": "Administrative", "name": "Alice"}},
	})
	// An editor that only knows about "name" and "facility" -- everything
	// else, including contacts, is simply never mentioned.
	incoming := rawJSON(t, map[string]interface{}{
		"name":     "Site1-renamed",
		"facility": "Fac1",
	})

	merged := mergeProposedState(models.KindSite, base, incoming)
	m := decodeMap(t, merged)

	if m["name"] != "Site1-renamed" {
		t.Errorf("name = %v, want Site1-renamed", m["name"])
	}
	if m["description"] != "the original description" {
		t.Errorf("description was dropped by an editor that never mentioned it: got %v", m["description"])
	}
	if m["address_line1"] != "123 Main St" {
		t.Errorf("address_line1 was dropped: got %v", m["address_line1"])
	}
	contacts, ok := m["contacts"].([]interface{})
	if !ok || len(contacts) != 1 {
		t.Errorf("contacts were dropped by an editor that never mentioned them: got %v", m["contacts"])
	}
}

// baseResourcePayload builds a resource proposal payload with fields no
// simple edit form models (DN, VOOwnership) alongside ones every edit form
// does (FQDN, Tags) -- the exact shape that caused real data loss before
// the write-time-merge fix.
func baseResourcePayload() map[string]interface{} {
	return map[string]interface{}{
		"name":           "myresource",
		"resource_group": "RG1",
		"resource": map[string]interface{}{
			"FQDN":        "host.example.org",
			"DN":          "/DC=org/CN=host.example.org",
			"Tags":        []interface{}{"CE", "OSG"},
			"VOOwnership": map[string]interface{}{"LIGO": float64(100)},
			"Services": map[string]interface{}{
				"CE": map[string]interface{}{
					"Description": "Compute Element",
					"Details":     map[string]interface{}{"hidden": false},
				},
				"GridFtp": map[string]interface{}{
					"Description": "GridFtp Storage Element",
				},
			},
		},
	}
}

func TestMergeProposedState_Resource_PreservesUnmodeledFields(t *testing.T) {
	base := rawJSON(t, baseResourcePayload())
	// A simple edit form that only knows Description -- doesn't mention DN,
	// VOOwnership, Tags, or Services at all.
	incoming := rawJSON(t, map[string]interface{}{
		"name":           "myresource",
		"resource_group": "RG1",
		"resource": map[string]interface{}{
			"FQDN":        "host.example.org",
			"Description": "now with a description",
		},
	})

	merged := mergeProposedState(models.KindResource, base, incoming)
	m := decodeMap(t, merged)
	res := m["resource"].(map[string]interface{})

	if res["Description"] != "now with a description" {
		t.Errorf("Description = %v, want the new value", res["Description"])
	}
	if res["DN"] != "/DC=org/CN=host.example.org" {
		t.Errorf("DN was dropped by an editor that never modeled it: got %v", res["DN"])
	}
	vo, ok := res["VOOwnership"].(map[string]interface{})
	if !ok || vo["LIGO"] != float64(100) {
		t.Errorf("VOOwnership was dropped: got %v", res["VOOwnership"])
	}
	tags, ok := res["Tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("Tags was dropped: got %v", res["Tags"])
	}
	svcs, ok := res["Services"].(map[string]interface{})
	if !ok || len(svcs) != 2 {
		t.Errorf("Services was dropped by an editor that never mentioned it: got %v", res["Services"])
	}
}

func TestMergeProposedState_Resource_ExplicitClearWorks(t *testing.T) {
	base := rawJSON(t, baseResourcePayload())
	// This editor DOES model Tags, and the user cleared them -- an explicit
	// empty array must actually clear, not be treated as "not mentioned".
	incoming := rawJSON(t, map[string]interface{}{
		"name":           "myresource",
		"resource_group": "RG1",
		"resource": map[string]interface{}{
			"FQDN": "host.example.org",
			"Tags": []interface{}{},
		},
	})

	merged := mergeProposedState(models.KindResource, base, incoming)
	res := decodeMap(t, merged)["resource"].(map[string]interface{})
	tags, ok := res["Tags"].([]interface{})
	if !ok || len(tags) != 0 {
		t.Errorf("explicit empty Tags should clear the field: got %v", res["Tags"])
	}
	// Unrelated unmodeled fields must still survive alongside the clear.
	if res["DN"] != "/DC=org/CN=host.example.org" {
		t.Errorf("DN was dropped by an unrelated clear: got %v", res["DN"])
	}
}

// TestMergeProposedState_Resource_ServicesOmitted_StaysUntouched is a direct
// regression test for a real bug found live this session: the first version
// of this merge unconditionally overwrote Services with
// mergeServiceEntries(...), which returned nil whenever the incoming
// payload didn't mention Services at all -- silently wiping every service a
// resource had the moment it was edited through any tool that doesn't model
// services. The fix only touches the key when incoming actually mentions it.
func TestMergeProposedState_Resource_ServicesOmitted_StaysUntouched(t *testing.T) {
	base := rawJSON(t, baseResourcePayload())
	incoming := rawJSON(t, map[string]interface{}{
		"name":           "myresource",
		"resource_group": "RG1",
		"resource": map[string]interface{}{
			"FQDN":        "host.example.org",
			"Description": "edited without touching services",
		},
	})

	merged := mergeProposedState(models.KindResource, base, incoming)
	res := decodeMap(t, merged)["resource"].(map[string]interface{})
	svcs, ok := res["Services"].(map[string]interface{})
	if !ok {
		t.Fatalf("Services became %v (non-object) when incoming never mentioned it -- the exact bug this test guards against", res["Services"])
	}
	if len(svcs) != 2 {
		t.Errorf("Services lost entries when incoming never mentioned it: got %v", svcs)
	}
}

func TestMergeProposedState_Resource_ServicesExplicitlyCleared(t *testing.T) {
	base := rawJSON(t, baseResourcePayload())
	incoming := rawJSON(t, map[string]interface{}{
		"name":           "myresource",
		"resource_group": "RG1",
		"resource": map[string]interface{}{
			"FQDN":     "host.example.org",
			"Services": map[string]interface{}{},
		},
	})

	merged := mergeProposedState(models.KindResource, base, incoming)
	res := decodeMap(t, merged)["resource"].(map[string]interface{})
	svcs, ok := res["Services"].(map[string]interface{})
	if !ok || len(svcs) != 0 {
		t.Errorf("explicit empty Services should clear every service: got %v", res["Services"])
	}
}

func TestMergeProposedState_Resource_ServiceFieldsMergedPerKey(t *testing.T) {
	base := rawJSON(t, baseResourcePayload())
	// Only CE is mentioned (with just Description changed, Details omitted);
	// GridFtp is omitted entirely from the incoming Services map (deleting
	// it); a brand-new service is added.
	incoming := rawJSON(t, map[string]interface{}{
		"name":           "myresource",
		"resource_group": "RG1",
		"resource": map[string]interface{}{
			"FQDN": "host.example.org",
			"Services": map[string]interface{}{
				"CE": map[string]interface{}{
					"Description": "Compute Element (updated)",
				},
				"NewSvc": map[string]interface{}{
					"Description": "brand new",
				},
			},
		},
	})

	merged := mergeProposedState(models.KindResource, base, incoming)
	res := decodeMap(t, merged)["resource"].(map[string]interface{})
	svcs := res["Services"].(map[string]interface{})

	if len(svcs) != 2 {
		t.Fatalf("got %d services, want 2 (CE, NewSvc): %v", len(svcs), svcs)
	}
	ce, ok := svcs["CE"].(map[string]interface{})
	if !ok {
		t.Fatalf("CE missing or not an object: %v", svcs["CE"])
	}
	if ce["Description"] != "Compute Element (updated)" {
		t.Errorf("CE.Description = %v, want the updated value", ce["Description"])
	}
	details, ok := ce["Details"].(map[string]interface{})
	if !ok || details["hidden"] != false {
		t.Errorf("CE.Details was dropped even though incoming didn't mention it, only Description: got %v", ce["Details"])
	}
	if _, stillThere := svcs["GridFtp"]; stillThere {
		t.Errorf("GridFtp should have been deleted (omitted from incoming's Services map): got %v", svcs["GridFtp"])
	}
	newSvc, ok := svcs["NewSvc"].(map[string]interface{})
	if !ok || newSvc["Description"] != "brand new" {
		t.Errorf("NewSvc wasn't added correctly: got %v", svcs["NewSvc"])
	}
}

func TestMergeProposedState_FallsBackWhenNotDecodable(t *testing.T) {
	incoming := json.RawMessage(`[1,2,3]`) // an array, not an object
	base := rawJSON(t, map[string]interface{}{"name": "x"})

	got := mergeProposedState(models.KindSite, base, incoming)
	if string(got) != string(incoming) {
		t.Errorf("non-object incoming: got %s, want incoming unchanged (%s)", got, incoming)
	}

	// A non-object base (e.g. a legacy/corrupt stored value) must not block
	// a perfectly good incoming submission -- fall back to incoming as-is
	// rather than erroring or silently dropping the submission.
	badBase := json.RawMessage(`"just a string"`)
	goodIncoming := rawJSON(t, map[string]interface{}{"name": "y"})
	got = mergeProposedState(models.KindSite, badBase, goodIncoming)
	if string(got) != string(goodIncoming) {
		t.Errorf("non-object base: got %s, want incoming unchanged (%s)", got, goodIncoming)
	}
}
