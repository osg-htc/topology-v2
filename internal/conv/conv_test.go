package conv

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// The reason integers decode as int64: the same value, decoded with a plain
// json.Unmarshal and written back out as YAML, is mangled.
func TestMapFromJSON_IntegersSurviveAYAMLRoundTrip(t *testing.T) {
	m := MapFromJSON([]byte(`{"SUs": 1000000, "Ratio": 1.5, "Name": "x", "Nested": {"N": 2000000}, "List": [3000000]}`))
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "List:\n    - 3000000\nName: x\nNested:\n    \"N\": 2000000\nRatio: 1.5\nSUs: 1000000\n"
	if string(out) != want {
		t.Fatalf("YAML after a JSON decode:\n%s\nwant:\n%s", out, want)
	}
}

// The claim above, demonstrated: a plain json.Unmarshal into a map turns
// integers into float64, which YAML then writes in exponent form.
func TestPlainJSONUnmarshalWouldMangleLargeIntegers(t *testing.T) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(`{"SUs": 1000000}`), &m); err != nil {
		t.Fatal(err)
	}
	out, _ := yaml.Marshal(m)
	if string(out) != "SUs: 1e+06\n" {
		t.Fatalf("expected the mangled form this package exists to avoid, got %q", out)
	}
}

func TestMapFromJSON_InvalidEmptyAndNonObjectAreNil(t *testing.T) {
	for name, in := range map[string][]byte{"nil": nil, "empty": {}, "garbage": []byte("{nope"), "null": []byte("null"), "array": []byte("[1]"), "scalar": []byte("3")} {
		if got := MapFromJSON(in); got != nil {
			t.Errorf("%s: got %v, want nil", name, got)
		}
	}
	if got := MapFromJSON([]byte(`{}`)); got == nil || len(got) != 0 {
		t.Errorf("an empty object should decode to an empty, non-nil map, got %v", got)
	}
}

func TestDecodeJSONObject_RejectsWhatIsNotAnObject(t *testing.T) {
	for _, in := range []string{"null", "[1]", "3", "{bad"} {
		if _, err := DecodeJSONObject([]byte(in)); err == nil {
			t.Errorf("%q should be an error", in)
		}
	}
	m, err := DecodeJSONObject([]byte(`{"a": 1, "b": 1.5, "c": 9223372036854775807}`))
	if err != nil {
		t.Fatal(err)
	}
	if m["a"] != int64(1) || m["b"] != 1.5 || m["c"] != int64(9223372036854775807) {
		t.Errorf("numbers = %#v", m)
	}
}

func TestAnyFromJSON(t *testing.T) {
	if AnyFromJSON(nil) != nil || AnyFromJSON([]byte("{bad")) != nil {
		t.Errorf("empty/invalid must be nil")
	}
	if got := AnyFromJSON([]byte(`[1, "x", {"a": 2}]`)); !reflect.DeepEqual(got, []interface{}{int64(1), "x", map[string]interface{}{"a": int64(2)}}) {
		t.Errorf("array = %#v", got)
	}
	if got := AnyFromJSON([]byte(`"s"`)); got != "s" {
		t.Errorf("scalar = %#v", got)
	}
}

// "No value" is stored as SQL NULL: nil or empty maps give nil -- never "null"
// or "{}".
func TestJSONOrNil(t *testing.T) {
	var nilMap map[string]interface{}
	for name, in := range map[string]map[string]interface{}{"nil": nilMap, "empty": {}} {
		if got := JSONOrNil(in); got != nil {
			t.Errorf("%s: got %q, want nil", name, got)
		}
	}
	if got := string(JSONOrNil(map[string]interface{}{"a": 1})); got != `{"a":1}` {
		t.Errorf("got %q", got)
	}
}

func TestJSONAnyOrNil(t *testing.T) {
	if JSONAnyOrNil(nil) != nil {
		t.Errorf("an untyped nil must give nil")
	}
	if got := string(JSONAnyOrNil([]string{})); got != "[]" {
		t.Errorf("an empty slice is a value, not 'no value': %q", got)
	}
	if got := JSONAnyOrNil(make(chan int)); got != nil {
		t.Errorf("an unmarshalable value must give nil, got %q", got)
	}
}

func TestBoolOrAndMapBool(t *testing.T) {
	tr, fa := true, false
	if !BoolOr(nil, true) || BoolOr(nil, false) || !BoolOr(&tr, false) || BoolOr(&fa, true) {
		t.Errorf("BoolOr wrong")
	}
	m := map[string]interface{}{"a": true, "b": false, "c": "yes"}
	if !MapBool(m, "a", false) || MapBool(m, "b", true) {
		t.Errorf("MapBool must read present bools")
	}
	if !MapBool(m, "missing", true) || MapBool(m, "missing", false) || !MapBool(m, "c", true) {
		t.Errorf("MapBool must fall back to the default for absent or non-bool values")
	}
}
