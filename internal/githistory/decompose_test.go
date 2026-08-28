package githistory

import (
	"encoding/json"
	"testing"

	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/topology"
)

func decodeFacility(t *testing.T, raw json.RawMessage) facilityPayload {
	t.Helper()
	var p facilityPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal facilityPayload: %v", err)
	}
	return p
}

func TestDecomposeFacility(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		changes, err := decomposeFacility("", "ANL", nil, []byte("InstitutionID: I123\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 1 {
			t.Fatalf("got %d changes, want 1", len(changes))
		}
		c := changes[0]
		if c.Operation != models.OpCreate || c.Before != nil {
			t.Fatalf("got Operation=%q Before=%s, want create with no before", c.Operation, c.Before)
		}
		p := decodeFacility(t, c.After)
		if p.Name != "ANL" || p.InstitutionID != "I123" {
			t.Fatalf("after = %+v, want Name=ANL InstitutionID=I123", p)
		}
	})

	t.Run("delete", func(t *testing.T) {
		changes, err := decomposeFacility("ANL", "", []byte("InstitutionID: I123\n"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 1 {
			t.Fatalf("got %d changes, want 1", len(changes))
		}
		c := changes[0]
		if c.Operation != models.OpDelete || c.After != nil {
			t.Fatalf("got Operation=%q After=%s, want delete with no after", c.Operation, c.After)
		}
	})

	t.Run("no-op when nothing modeled changed", func(t *testing.T) {
		changes, err := decomposeFacility("ANL", "ANL", []byte("InstitutionID: I123\n"), []byte("InstitutionID: I123\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 0 {
			t.Fatalf("got %d changes, want 0 (nothing changed)", len(changes))
		}
	})

	t.Run("field change is an update", func(t *testing.T) {
		changes, err := decomposeFacility("ANL", "ANL", []byte("InstitutionID: I123\n"), []byte("InstitutionID: I456\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 1 || changes[0].Operation != models.OpUpdate {
			t.Fatalf("got %+v, want one update", changes)
		}
	})

	t.Run("pure rename is an update, not delete+create", func(t *testing.T) {
		changes, err := decomposeFacility("ANL", "Argonne National Laboratory", []byte("InstitutionID: I123\n"), []byte("InstitutionID: I123\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 1 || changes[0].Operation != models.OpUpdate {
			t.Fatalf("got %+v, want exactly one update", changes)
		}
		before := decodeFacility(t, changes[0].Before)
		after := decodeFacility(t, changes[0].After)
		if before.Name != "ANL" || after.Name != "Argonne National Laboratory" {
			t.Fatalf("before/after names = %q/%q, want ANL/Argonne National Laboratory", before.Name, after.Name)
		}
	})
}

func TestDecomposeSite_Reparenting(t *testing.T) {
	// The real scenario this guards: ANL -> Argonne National Laboratory
	// renamed the facility, which cascades a rename of every child site's
	// path even though the site's own name never changes. That must read
	// as one update (new Facility, same Name), not a false delete+create.
	blob := []byte("LongName: ANL ASC\n")
	changes, err := decomposeSite("ANL ASC", "ANL ASC", "ANL", "Argonne National Laboratory", blob, blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Operation != models.OpUpdate {
		t.Fatalf("got %+v, want exactly one update", changes)
	}
	var before, after sitePayload
	if err := json.Unmarshal(changes[0].Before, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(changes[0].After, &after); err != nil {
		t.Fatal(err)
	}
	if before.Facility != "ANL" || after.Facility != "Argonne National Laboratory" {
		t.Fatalf("before/after facility = %q/%q", before.Facility, after.Facility)
	}
	if before.Name != "ANL ASC" || after.Name != "ANL ASC" {
		t.Fatalf("site's own name must not change: before=%q after=%q", before.Name, after.Name)
	}
}

func TestDecomposeSite_NoOp(t *testing.T) {
	blob := []byte("LongName: ANL ASC\n")
	changes, err := decomposeSite("ANL ASC", "ANL ASC", "ANL", "ANL", blob, blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("got %d changes, want 0", len(changes))
	}
}

func TestDecomposeProject_CreateUpdateDelete(t *testing.T) {
	changes, err := decomposeProject("", "AMFORA", nil, []byte("PIName: Ian Foster\n"))
	if err != nil || len(changes) != 1 || changes[0].Operation != models.OpCreate {
		t.Fatalf("create: changes=%+v err=%v", changes, err)
	}

	changes, err = decomposeProject("AMFORA", "AMFORA", []byte("PIName: Ian Foster\n"), []byte("PIName: Someone Else\n"))
	if err != nil || len(changes) != 1 || changes[0].Operation != models.OpUpdate {
		t.Fatalf("update: changes=%+v err=%v", changes, err)
	}

	changes, err = decomposeProject("AMFORA", "", []byte("PIName: Ian Foster\n"), nil)
	if err != nil || len(changes) != 1 || changes[0].Operation != models.OpDelete {
		t.Fatalf("delete: changes=%+v err=%v", changes, err)
	}
}

const rgOld = `Production: true
SupportCenter: OSG
GroupDescription: test group
Resources:
  Res1:
    FQDN: res1.example.org
    Active: true
  Res2:
    FQDN: res2.example.org
    Active: true
`

const rgNew = `Production: true
SupportCenter: OSG
GroupDescription: test group
Resources:
  Res1:
    FQDN: res1-new.example.org
    Active: true
  Res3:
    FQDN: res3.example.org
    Active: true
`

func findResourceChange(changes []entityChange, name string) *entityChange {
	for i, c := range changes {
		if c.Kind != models.KindResource {
			continue
		}
		if c.OldName == name || c.NewName == name {
			return &changes[i]
		}
	}
	return nil
}

func TestDecomposeResourceGroup_NestedResourceKeyedDiff(t *testing.T) {
	changes, err := decomposeResourceGroup("RG1", "RG1", "Site1", "Site1", []byte(rgOld), []byte(rgNew))
	if err != nil {
		t.Fatal(err)
	}
	// RG's own fields (Production/SupportCenter/GroupDescription) and
	// name/site are all unchanged -- only the three resource-level changes
	// should appear, no RG-level change.
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3 (Res1 update, Res2 delete, Res3 create): %+v", len(changes), changes)
	}
	for _, c := range changes {
		if c.Kind != models.KindResource {
			t.Fatalf("unexpected RG-level change when only resources differed: %+v", c)
		}
	}

	res1 := findResourceChange(changes, "Res1")
	if res1 == nil || res1.Operation != models.OpUpdate {
		t.Fatalf("Res1: got %+v, want an update", res1)
	}
	res2 := findResourceChange(changes, "Res2")
	if res2 == nil || res2.Operation != models.OpDelete {
		t.Fatalf("Res2: got %+v, want a delete", res2)
	}
	res3 := findResourceChange(changes, "Res3")
	if res3 == nil || res3.Operation != models.OpCreate {
		t.Fatalf("Res3: got %+v, want a create", res3)
	}
}

func TestDecomposeResourceGroup_OwnFieldChangeOnly(t *testing.T) {
	newBlob := []byte(`Production: true
SupportCenter: OSG
GroupDescription: a new description
Resources:
  Res1:
    FQDN: res1.example.org
    Active: true
  Res2:
    FQDN: res2.example.org
    Active: true
`)
	changes, err := decomposeResourceGroup("RG1", "RG1", "Site1", "Site1", []byte(rgOld), newBlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Kind != models.KindResourceGroup || changes[0].Operation != models.OpUpdate {
		t.Fatalf("got %+v, want exactly one resource_group update and no resource-level changes", changes)
	}
}

func TestDecomposeResourceGroup_NoOp(t *testing.T) {
	changes, err := decomposeResourceGroup("RG1", "RG1", "Site1", "Site1", []byte(rgOld), []byte(rgOld))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("got %d changes, want 0", len(changes))
	}
}

// TestDecomposeResources_EmptyCollectionsAreNeverNull is a direct regression
// test for a real bug hit during a live import run: a resource whose YAML
// simply never mentions FQDNAliases/Tags/AllowedVOs/Services/ContactLists
// parses with those fields nil, and Go's encoding/json marshals a nil slice
// or map as JSON null -- which the resource proposal schema rejects (it
// requires an array/object, never null). normalizeResource must convert
// each of these to an empty (non-nil) collection before marshaling.
func TestDecomposeResources_EmptyCollectionsAreNeverNull(t *testing.T) {
	newBlob := []byte(`Resources:
  Bare:
    FQDN: bare.example.org
`)
	changes, err := decomposeResourceGroup("RG1", "RG1", "Site1", "Site1", nil, newBlob)
	if err != nil {
		t.Fatal(err)
	}
	res := findResourceChange(changes, "Bare")
	if res == nil || res.Operation != models.OpCreate {
		t.Fatalf("got %+v, want a create for Bare", res)
	}
	var payload resourcePayload
	if err := json.Unmarshal(res.After, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Resource.FQDNAliases == nil {
		t.Error("FQDNAliases is nil (would marshal as JSON null) -- want an empty slice")
	}
	if payload.Resource.Tags == nil {
		t.Error("Tags is nil (would marshal as JSON null) -- want an empty slice")
	}
	if payload.Resource.AllowedVOs == nil {
		t.Error("AllowedVOs is nil (would marshal as JSON null) -- want an empty slice")
	}
	if payload.Resource.Services == nil {
		t.Error("Services is nil (would marshal as JSON null) -- want an empty map")
	}
	if payload.Resource.ContactLists == nil {
		t.Error("ContactLists is nil (would marshal as JSON null) -- want an empty map")
	}
}

func TestResolveResourceID(t *testing.T) {
	explicit := int64(42)
	if got := resolveResourceID(&explicit, "whatever"); got != 42 {
		t.Errorf("explicit ID: got %d, want 42", got)
	}
	if got, want := resolveResourceID(nil, "some-resource"), topology.GenID("some-resource"); got != want {
		t.Errorf("nil ID: got %d, want GenID fallback %d", got, want)
	}
}
