package xmlapi

import (
	"encoding/json"
	"strconv"
)

// parseJSONMap unmarshals a JSONB blob into a string-keyed map, or nil.
func parseJSONMap(b []byte) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// toInt coerces a JSON-decoded numeric value to int.
func toInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	default:
		return 0
	}
}

// getBool reads a boolean from a JSON-decoded map (default false).
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getStr reads a string from a JSON-decoded map. Numbers are stringified so a
// value stored as a number still serializes as the XSD's string type.
func getStr(m map[string]interface{}, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// getFloat reads a float from a JSON-decoded map (default 0).
func getFloat(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}
