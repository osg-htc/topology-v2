// Package conv holds the small conversions between the loosely-typed shapes
// that YAML and JSON decode into (map[string]interface{}, *bool, ...) and the
// typed values the rest of the app uses.
//
// These used to exist as several near-identical private copies -- one per
// package, sometimes two in the same one -- which then drifted apart (one
// decoded integers as float64, another as int; one treated a nil map as "no
// value", another stored JSON null). Add new conversions of this kind HERE, and
// look here before writing one: see "Look for an existing helper first" in
// AGENTS.md.
package conv

import (
	"bytes"
	"encoding/json"
)

// DecodeJSONObject decodes a JSON object. Integers decode as int64 rather than
// float64: a value that later gets written as YAML (an Extra catch-all, a VO
// document) must not turn 1000000 into 1e+06, which is what float64 does.
// Non-integral numbers stay float64. An error is returned for invalid JSON or
// anything that is not an object (including null).
func DecodeJSONObject(b []byte) (map[string]interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]interface{}
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errNotObject
	}
	return normalizeNumbers(m).(map[string]interface{}), nil
}

type convError string

func (e convError) Error() string { return string(e) }

const errNotObject = convError("JSON value is not an object")

// MapFromJSON decodes a stored JSON object, or returns nil for empty, invalid
// or non-object input. See DecodeJSONObject for how numbers decode.
func MapFromJSON(b []byte) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	m, err := DecodeJSONObject(b)
	if err != nil {
		return nil
	}
	return m
}

// AnyFromJSON decodes stored JSON of any shape (object, array, scalar), or
// returns nil for empty or invalid input. Numbers decode as in DecodeJSONObject.
func AnyFromJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	return normalizeNumbers(v)
}

// JSONOrNil marshals a map for storage, or returns nil when it is nil or empty
// -- so "no value" is stored as SQL NULL, never as the JSON text "null" or "{}".
func JSONOrNil(m map[string]interface{}) []byte {
	if len(m) == 0 {
		return nil
	}
	return JSONAnyOrNil(m)
}

// JSONAnyOrNil marshals any value for storage, or returns nil for an untyped
// nil (or a value that cannot be marshalled). Unlike JSONOrNil it does not
// treat an empty map or slice as "no value".
func JSONAnyOrNil(v interface{}) []byte {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// BoolOr returns *p, or def when p is nil. It is how an optional (*bool) field
// takes its v1 default: absent Active means true, absent Disable means false.
func BoolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// MapBool reads a boolean key from a decoded YAML/JSON map, returning def when
// the key is absent or not a bool.
func MapBool(m map[string]interface{}, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

// normalizeNumbers turns json.Number into int64 when it is integral, else
// float64, throughout a decoded value.
func normalizeNumbers(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, x := range t {
			t[k] = normalizeNumbers(x)
		}
		return t
	case []interface{}:
		for i, x := range t {
			t[i] = normalizeNumbers(x)
		}
		return t
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	default:
		return v
	}
}
