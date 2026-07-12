package proposalschema

import "testing"

// TestNoUpgraderGaps guards against silent drift: if CurrentVersion is bumped
// for a kind, an upgrader must exist for every intermediate step, otherwise an
// in-flight proposal written against an older version can never be applied.
// This test fails loudly the moment a schema version is added without its
// upgrader (see AGENTS.md, "Keeping goose and proposal JSON Schemas in sync").
func TestNoUpgraderGaps(t *testing.T) {
	for kind, cur := range current {
		for v := 1; v < cur; v++ {
			if _, ok := upgraders[kind][v]; !ok {
				t.Errorf("kind %q: missing upgrader for v%d->v%d (schema was bumped to v%d without an upgrader)",
					kind, v, v+1, cur)
			}
		}
		// Every version must also have a compiled schema (init() panics if a
		// file is missing, but assert here for a clear message).
		for v := 1; v <= cur; v++ {
			if _, ok := compiled[schemaKey(kind, v)]; !ok {
				t.Errorf("kind %q: missing schema for v%d", kind, v)
			}
		}
	}
}

// TestResourceValidation exercises the resource schema on good and bad payloads.
func TestResourceValidation(t *testing.T) {
	good := []byte(`{"name":"X_Res","resource_group":"X_RG","resource":{"FQDN":"x.example.org","Active":true}}`)
	if err := Validate("resource", 1, good); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}

	bads := map[string][]byte{
		"missing FQDN":           []byte(`{"name":"X","resource_group":"RG","resource":{"Active":true}}`),
		"missing name":           []byte(`{"resource_group":"RG","resource":{"FQDN":"x"}}`),
		"unknown top-level field": []byte(`{"name":"X","resource_group":"RG","resource":{"FQDN":"x"},"bogus":1}`),
	}
	for label, payload := range bads {
		if err := Validate("resource", 1, payload); err == nil {
			t.Errorf("expected %s to fail validation", label)
		}
	}
}

// TestUpgradeNoOp confirms upgrading a current-version payload is a no-op.
func TestUpgradeNoOp(t *testing.T) {
	payload := []byte(`{"name":"X","resource_group":"RG","resource":{"FQDN":"x"}}`)
	out, ver, err := Upgrade("resource", CurrentVersion("resource"), payload)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if ver != CurrentVersion("resource") {
		t.Fatalf("expected current version, got %d", ver)
	}
	if string(out) != string(payload) {
		t.Fatalf("no-op upgrade changed payload")
	}
}
