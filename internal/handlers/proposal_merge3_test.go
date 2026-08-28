package handlers

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestThreeWayMergeScalar(t *testing.T) {
	cases := []struct {
		name                 string
		base, live, proposed interface{}
		want                 interface{}
		wantConflict         bool
	}{
		{"proposal never touched it: keep live", "base", "live", "base", "live", false},
		{"nothing else touched it: apply proposed", "base", "base", "proposed", "proposed", false},
		{"both landed on the same value: fine", "base", "same", "same", "same", false},
		{"both changed it differently: conflict", "base", "live", "proposed", "live", true},
		{"no-op: all three equal", "x", "x", "x", "x", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, conflicts := threeWayMergeScalar("field", c.base, c.live, c.proposed)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
			if (len(conflicts) > 0) != c.wantConflict {
				t.Errorf("conflicts = %v, wantConflict = %v", conflicts, c.wantConflict)
			}
		})
	}
}

func strSet(vals ...string) []string { return vals }

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func TestThreeWayMergeSet(t *testing.T) {
	t.Run("two independent additions compose (the false-conflict regression case)", func(t *testing.T) {
		base := strSet("a")
		live := strSet("a", "b")     // someone else added b
		proposed := strSet("a", "c") // we added c
		got, conflicts := threeWayMergeSet("Tags", base, live, proposed)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none -- independent additions must compose, not conflict", conflicts)
		}
		want := strSet("a", "b", "c")
		if !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("we deleted, nobody else touched it: deletion applies", func(t *testing.T) {
		got, conflicts := threeWayMergeSet("Tags", strSet("a", "b"), strSet("a", "b"), strSet("a"))
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if !reflect.DeepEqual(sortedCopy(got), strSet("a")) {
			t.Errorf("got %v, want [a]", got)
		}
	})

	t.Run("someone else deleted it, we never touched it: their deletion wins", func(t *testing.T) {
		got, conflicts := threeWayMergeSet("Tags", strSet("a", "b"), strSet("a"), strSet("a", "b"))
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if !reflect.DeepEqual(sortedCopy(got), strSet("a")) {
			t.Errorf("got %v, want [a]", got)
		}
	})

	t.Run("both independently deleted the same entry: agreement, no conflict", func(t *testing.T) {
		got, conflicts := threeWayMergeSet("Tags", strSet("a", "b"), strSet("a"), strSet("a"))
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if !reflect.DeepEqual(sortedCopy(got), strSet("a")) {
			t.Errorf("got %v, want [a]", got)
		}
	})

	t.Run("never nil", func(t *testing.T) {
		got, _ := threeWayMergeSet("Tags", nil, nil, nil)
		if got == nil {
			t.Error("got nil, want a non-nil empty slice (must marshal to [] not null)")
		}
	})
}

func obj(fields map[string]interface{}) map[string]interface{} { return fields }

func TestMergeKeyedCollection_Services(t *testing.T) {
	t.Run("we added a service, nobody else has an opinion", func(t *testing.T) {
		base := map[string]interface{}{}
		live := map[string]interface{}{}
		proposed := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "Compute Element"})}
		got, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if _, ok := got["CE"]; !ok {
			t.Errorf("got %v, want CE present", got)
		}
	})

	t.Run("both independently added the same service identically: fine", func(t *testing.T) {
		svc := obj(map[string]interface{}{"Description": "Compute Element"})
		got, conflicts := mergeKeyedCollection("Services", map[string]interface{}{}, map[string]interface{}{"CE": svc}, map[string]interface{}{"CE": svc}, true)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if _, ok := got["CE"]; !ok {
			t.Errorf("got %v, want CE present", got)
		}
	})

	t.Run("both independently added the same service differently: conflict", func(t *testing.T) {
		base := map[string]interface{}{}
		live := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "Live's version"})}
		proposed := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "Proposed's version"})}
		_, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) == 0 {
			t.Fatal("got no conflicts, want one for the colliding CE addition")
		}
	})

	t.Run("we deleted a service nobody else touched: deletion applies", func(t *testing.T) {
		svc := obj(map[string]interface{}{"Description": "Compute Element"})
		base := map[string]interface{}{"CE": svc}
		live := map[string]interface{}{"CE": svc}
		proposed := map[string]interface{}{}
		got, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if _, ok := got["CE"]; ok {
			t.Errorf("got %v, want CE deleted", got)
		}
	})

	t.Run("we deleted a service someone else just modified: conflict", func(t *testing.T) {
		base := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "original"})}
		live := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "modified by someone else"})}
		proposed := map[string]interface{}{}
		_, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) == 0 {
			t.Fatal("got no conflicts, want one -- deleting something someone else just changed is a real conflict")
		}
	})

	t.Run("someone else deleted a service we never actually changed: their deletion wins silently", func(t *testing.T) {
		svc := obj(map[string]interface{}{"Description": "original"})
		base := map[string]interface{}{"CE": svc}
		live := map[string]interface{}{}
		proposed := map[string]interface{}{"CE": svc} // unchanged from base, just carried forward
		got, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if _, ok := got["CE"]; ok {
			t.Errorf("got %v, want CE gone (their deletion wins)", got)
		}
	})

	t.Run("someone else deleted a service we actively changed: conflict", func(t *testing.T) {
		base := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "original"})}
		live := map[string]interface{}{}
		proposed := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "we changed this"})}
		_, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) == 0 {
			t.Fatal("got no conflicts, want one -- changing something someone else deleted is a real conflict")
		}
	})

	t.Run("both independently deleted the same service: agreement, no conflict (the matrix gap the design review caught)", func(t *testing.T) {
		base := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "original"})}
		live := map[string]interface{}{}
		proposed := map[string]interface{}{}
		got, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		if _, ok := got["CE"]; ok {
			t.Errorf("got %v, want CE gone", got)
		}
	})

	t.Run("present on all three sides, non-conflicting sub-field changes compose", func(t *testing.T) {
		base := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "A", "Details": "d1"})}
		live := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "B", "Details": "d1"})}     // live changed Description only
		proposed := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "A", "Details": "d2"})} // we changed Details only
		got, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none -- independent sub-field changes must compose", conflicts)
		}
		ce := got["CE"].(map[string]interface{})
		if ce["Description"] != "B" || ce["Details"] != "d2" {
			t.Errorf("got %v, want Description=B (live's change) and Details=d2 (our change)", ce)
		}
	})

	t.Run("present on all three sides, same sub-field changed differently: conflict names the nested path", func(t *testing.T) {
		base := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "A"})}
		live := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "B"})}
		proposed := map[string]interface{}{"CE": obj(map[string]interface{}{"Description": "C"})}
		_, conflicts := mergeKeyedCollection("Services", base, live, proposed, true)
		if len(conflicts) != 1 {
			t.Fatalf("got %d conflicts, want 1", len(conflicts))
		}
		if conflicts[0].Path != "Services.CE.Description" {
			t.Errorf("conflict path = %q, want Services.CE.Description", conflicts[0].Path)
		}
	})
}

func TestThreeWayMergeResource(t *testing.T) {
	baseResource := map[string]interface{}{
		"name": "myresource", "resource_group": "RG1",
		"resource": map[string]interface{}{
			"FQDN": "host.example.org", "Description": "original", "DN": "",
			"Tags": []interface{}{"a"}, "AllowedVOs": []interface{}{}, "FQDNAliases": []interface{}{},
			"Services": map[string]interface{}{
				"CE": map[string]interface{}{"Description": "Compute Element"},
			},
		},
	}

	t.Run("independent field and independent tag both compose, no conflict", func(t *testing.T) {
		live := deepCopyMap(baseResource)
		liveRes := live["resource"].(map[string]interface{})
		liveRes["Tags"] = []interface{}{"a", "b"} // someone else tagged it
		liveRes["DN"] = "/DC=org/CN=host"         // someone else set DN

		proposed := deepCopyMap(baseResource)
		proposedRes := proposed["resource"].(map[string]interface{})
		proposedRes["Description"] = "updated by this proposal" // we changed Description only

		reconciled, conflicts := threeWayMergeResource(baseResource, live, proposed)
		if len(conflicts) != 0 {
			t.Fatalf("got conflicts %v, want none", conflicts)
		}
		res := reconciled["resource"].(map[string]interface{})
		if res["Description"] != "updated by this proposal" {
			t.Errorf("Description = %v, want our change to survive", res["Description"])
		}
		if res["DN"] != "/DC=org/CN=host" {
			t.Errorf("DN = %v, want the independent live change to survive", res["DN"])
		}
		tags := res["Tags"].([]string)
		if !reflect.DeepEqual(sortedCopy(tags), strSet("a", "b")) {
			t.Errorf("Tags = %v, want [a b] (the independent tag addition preserved)", tags)
		}
	})

	t.Run("both sides change the same field differently: conflict names it", func(t *testing.T) {
		live := deepCopyMap(baseResource)
		live["resource"].(map[string]interface{})["FQDN"] = "live-host.example.org"

		proposed := deepCopyMap(baseResource)
		proposed["resource"].(map[string]interface{})["FQDN"] = "proposed-host.example.org"

		_, conflicts := threeWayMergeResource(baseResource, live, proposed)
		if len(conflicts) != 1 || conflicts[0].Path != "resource.FQDN" {
			t.Fatalf("got %v, want exactly one conflict on resource.FQDN", conflicts)
		}
	})

	t.Run("nested service-field conflict is named with its full path", func(t *testing.T) {
		live := deepCopyMap(baseResource)
		live["resource"].(map[string]interface{})["Services"].(map[string]interface{})["CE"].(map[string]interface{})["Description"] = "live's edit"

		proposed := deepCopyMap(baseResource)
		proposed["resource"].(map[string]interface{})["Services"].(map[string]interface{})["CE"].(map[string]interface{})["Description"] = "proposed's edit"

		_, conflicts := threeWayMergeResource(baseResource, live, proposed)
		if len(conflicts) != 1 || conflicts[0].Path != "resource.Services.CE.Description" {
			t.Fatalf("got %v, want exactly one conflict on resource.Services.CE.Description", conflicts)
		}
	})
}

// deepCopyMap makes an independent copy via a JSON round trip -- simplest
// way to get a genuinely independent nested map/slice tree for test fixtures.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}
