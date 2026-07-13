// Package proposalschema validates change-proposal payloads (proposed_state)
// against versioned JSON Schemas and upgrades older payloads to the current
// version. This keeps proposal JSONB decoupled from the live database DDL:
// goose migrates the live tables, while proposals carry their own schema
// version and are brought forward here by explicit upgrader functions, so an
// in-flight proposal survives a schema change instead of silently breaking.
package proposalschema

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schema/*.json
var schemaFS embed.FS

// current maps each entity kind to the latest proposed_state schema version.
var current = map[string]int{
	"resource":       1,
	"resource_group": 1,
	"site":           1,
	"facility":       1,
	"project":        1,
	"downtime":       1,
}

// compiled caches compiled validators keyed by "kind/version".
var compiled = map[string]*jsonschema.Schema{}

// upgraders[kind][v] transforms a proposed_state from version v to v+1.
// Register one per version bump; Upgrade chains them.
var upgraders = map[string]map[int]func([]byte) ([]byte, error){
	// Register an entry per version bump, e.g.:
	//   "resource": {1: func(b []byte) ([]byte, error) { ...; return b, nil }},
	"resource":       {},
	"resource_group": {},
	"site":           {},
	"facility":       {},
	"project":        {},
	"downtime":       {},
}

func init() {
	// Compile every embedded schema at startup so validation is cheap and any
	// malformed schema fails fast.
	for kind, ver := range current {
		for v := 1; v <= ver; v++ {
			key := schemaKey(kind, v)
			data, err := schemaFS.ReadFile(fmt.Sprintf("schema/%s_v%d.json", kind, v))
			if err != nil {
				panic(fmt.Sprintf("proposalschema: missing schema %s: %v", key, err))
			}
			c := jsonschema.NewCompiler()
			if err := c.AddResource(key, bytes.NewReader(data)); err != nil {
				panic(fmt.Sprintf("proposalschema: adding %s: %v", key, err))
			}
			s, err := c.Compile(key)
			if err != nil {
				panic(fmt.Sprintf("proposalschema: compiling %s: %v", key, err))
			}
			compiled[key] = s
		}
	}
}

// CurrentVersion returns the latest schema version for a kind (0 if unknown).
func CurrentVersion(kind string) int { return current[kind] }

// Known reports whether a kind has a registered schema.
func Known(kind string) bool { _, ok := current[kind]; return ok }

// Validate checks state against the schema for (kind, version).
func Validate(kind string, version int, state []byte) error {
	s, ok := compiled[schemaKey(kind, version)]
	if !ok {
		return fmt.Errorf("no schema for %s v%d", kind, version)
	}
	var v interface{}
	if err := json.Unmarshal(state, &v); err != nil {
		return fmt.Errorf("proposed_state is not valid JSON: %w", err)
	}
	if err := s.Validate(v); err != nil {
		return fmt.Errorf("proposed_state failed schema validation: %w", err)
	}
	return nil
}

// Upgrade brings a proposed_state from fromVersion to the current version by
// chaining registered upgraders, then validates the result. It returns the
// upgraded payload and the current version.
func Upgrade(kind string, fromVersion int, state []byte) ([]byte, int, error) {
	target := current[kind]
	if target == 0 {
		return nil, 0, fmt.Errorf("unknown entity kind %q", kind)
	}
	out := state
	for v := fromVersion; v < target; v++ {
		up, ok := upgraders[kind][v]
		if !ok {
			return nil, 0, fmt.Errorf("no upgrader for %s v%d->v%d", kind, v, v+1)
		}
		next, err := up(out)
		if err != nil {
			return nil, 0, fmt.Errorf("upgrading %s v%d->v%d: %w", kind, v, v+1, err)
		}
		out = next
	}
	if err := Validate(kind, target, out); err != nil {
		return nil, 0, err
	}
	return out, target, nil
}

func schemaKey(kind string, version int) string {
	return fmt.Sprintf("%s/v%d", kind, version)
}
