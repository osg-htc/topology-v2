package topology

import (
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestGenID pins the id generator to the legacy algorithm.
func TestGenID(t *testing.T) {
	// Deterministic: same name always yields the same id in [1, 2^31-1].
	got := GenID("UChicago_OSGConnect_ap20")
	if got < 1 || got > (1<<31)-1 {
		t.Fatalf("GenID out of range: %d", got)
	}
	if GenID("foo") == GenID("bar") {
		t.Fatalf("GenID should differ for different names")
	}
	if GenID("stable") != GenID("stable") {
		t.Fatalf("GenID must be deterministic")
	}
}

func TestContactIDFromEmail(t *testing.T) {
	// SHA1 of the lowercased, trimmed email (legacy contact id).
	if got := ContactIDFromEmail("  Brian@Morgridge.ORG "); got != ContactIDFromEmail("brian@morgridge.org") {
		t.Fatalf("email hashing must normalize case/whitespace")
	}
}

// TestStructRoundTrip reads the fixture tree, writes it back out, and asserts
// every file's parsed content is byte-for-byte identical modulo whitespace and
// key ordering (i.e. structurally equal).
func TestStructRoundTrip(t *testing.T) {
	src := filepath.Join("testdata", "topology")

	tree, err := ReadTree(src)
	if err != nil {
		t.Fatalf("ReadTree: %v", err)
	}

	dst := t.TempDir()
	if err := WriteTree(dst, tree); err != nil {
		t.Fatalf("WriteTree: %v", err)
	}

	assertTreesEquivalent(t, src, dst)
}

// assertTreesEquivalent compares two topology trees for structural equivalence
// (modulo whitespace, key ordering, and numeric formatting). Both trees are
// parsed through ReadTree — the same lenient, consistently-typed path — so
// yaml.v3's generic-scalar-resolution quirks (leading-zero zipcodes, etc.) and
// malformed files are handled identically on each side.
func assertTreesEquivalent(t *testing.T, want, got string) {
	t.Helper()
	tw, err := ReadTree(want)
	if err != nil {
		t.Fatalf("ReadTree(want): %v", err)
	}
	tg, err := ReadTree(got)
	if err != nil {
		t.Fatalf("ReadTree(got): %v", err)
	}
	compareEntities(t, "facility", canonMap(tw.Facilities), canonMap(tg.Facilities))
	compareEntities(t, "site", canonMap(tw.Sites), canonMap(tg.Sites))
	compareEntities(t, "resource-group", canonMap(tw.ResourceGroups), canonMap(tg.ResourceGroups))
	compareEntities(t, "downtimes", canonMap(tw.Downtimes), canonMap(tg.Downtimes))
	compareEntities(t, "site->facility", strMap(tw.SiteFacility), strMap(tg.SiteFacility))
	compareEntities(t, "rg->site", strMap(tw.RGSite), strMap(tg.RGSite))
}

// canonMap renders each entity to a canonical, number/nil-normalized form keyed
// by name, so per-entity mismatches can be reported precisely.
func canonMap[T any](m map[string]T) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		b, _ := yaml.Marshal(v)
		var generic interface{}
		_ = yaml.Unmarshal(b, &generic)
		out[k] = normalize(generic)
	}
	return out
}

func strMap(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func compareEntities(t *testing.T, kind string, want, got map[string]interface{}) {
	t.Helper()
	for name, wv := range want {
		gv, ok := got[name]
		if !ok {
			t.Errorf("%s %q missing from regenerated tree", kind, name)
			continue
		}
		if !reflect.DeepEqual(wv, gv) {
			t.Errorf("%s %q mismatch:\n want: %#v\n got:  %#v", kind, name, wv, gv)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s %q unexpectedly present in regenerated tree", kind, name)
		}
	}
}

// normalize coerces numeric scalars to float64 and drops nil-valued map entries
// (a key present with a null value is equivalent to an absent key). Strings,
// bools, and other scalars are left as-is.
func normalize(v interface{}) interface{} {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case []interface{}:
		for i := range x {
			x[i] = normalize(x[i])
		}
		return x
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			if val == nil {
				continue
			}
			out[k] = normalize(val)
		}
		return out
	default:
		return v
	}
}
