package topology

import (
	"strings"
	"testing"
)

const sampleVO = `# Name is the short name of the VO
LongName: Original Long Name
# who to call
Contacts:
  Administrative Contact:
  - ID: abc
    Name: Ada
PrimaryURL: http://example.org/a  # trailing note
FieldsOfScience:
  PrimaryFields:
  - Physics
ID: 1000000000
OASIS:
  UseOASIS: false
`

func mustDoc(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	d, err := VODocFromRaw([]byte(raw))
	if err != nil {
		t.Fatalf("VODocFromRaw: %v", err)
	}
	return d
}

// An edit that changes one key must leave every other key's text -- including
// comments and order -- exactly as the file had it, so a VO edited in the app
// still diffs cleanly against its file in the GitHub repo.
func TestPatchVOYAML_ChangesOnlyWhatChanged(t *testing.T) {
	doc := mustDoc(t, sampleVO)
	doc["LongName"] = "New Long Name"
	out, err := PatchVOYAML([]byte(sampleVO), doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, keep := range []string{"# Name is the short name of the VO", "# who to call", "# trailing note", "- Physics", "ID: 1000000000"} {
		if !strings.Contains(s, keep) {
			t.Errorf("lost %q:\n%s", keep, s)
		}
	}
	if !strings.Contains(s, "LongName: New Long Name") || strings.Contains(s, "Original Long Name") {
		t.Errorf("LongName not updated:\n%s", s)
	}
	if strings.Index(s, "LongName") > strings.Index(s, "Contacts") || strings.Index(s, "Contacts") > strings.Index(s, "PrimaryURL") {
		t.Errorf("key order changed:\n%s", s)
	}
	// And the result still parses to exactly the edited document.
	if !jsonEqual(mustDoc(t, s), doc) {
		t.Errorf("round trip differs:\n got %v\nwant %v", mustDoc(t, s), doc)
	}
}

func TestPatchVOYAML_UnchangedDocIsByteStable(t *testing.T) {
	out, err := PatchVOYAML([]byte(sampleVO), mustDoc(t, sampleVO))
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(mustDoc(t, string(out)), mustDoc(t, sampleVO)) {
		t.Fatalf("no-op edit changed the document:\n%s", out)
	}
	for _, keep := range []string{"# Name is the short name of the VO", "# who to call", "# trailing note"} {
		if !strings.Contains(string(out), keep) {
			t.Errorf("no-op edit lost comment %q", keep)
		}
	}
}

func TestPatchVOYAML_RemovesAndAddsKeys(t *testing.T) {
	doc := mustDoc(t, sampleVO)
	delete(doc, "PrimaryURL")
	doc["SupportURL"] = "http://example.org/support"
	doc["ParentVO"] = map[string]interface{}{"ID": int64(9), "Name": "Fermilab"}
	out, err := PatchVOYAML([]byte(sampleVO), doc)
	if err != nil {
		t.Fatal(err)
	}
	got := mustDoc(t, string(out))
	if _, ok := got["PrimaryURL"]; ok {
		t.Errorf("PrimaryURL should be removed")
	}
	if got["SupportURL"] != "http://example.org/support" {
		t.Errorf("SupportURL not added: %v", got["SupportURL"])
	}
	if !jsonEqual(got["ParentVO"], doc["ParentVO"]) {
		t.Errorf("ParentVO = %v", got["ParentVO"])
	}
}

// A large integer (a real VO ID) must stay an integer through the JSON hop that
// the proposal workflow forces on it, not become 1e+09.
func TestNormalizeVODoc_KeepsIntegers(t *testing.T) {
	doc, err := NormalizeVODoc([]byte(`{"ID": 1000000000, "Name": "x", "Ratio": 1.5}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewVOYAML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "ID: 1000000000") || strings.Contains(string(out), "e+") {
		t.Errorf("integer mangled:\n%s", out)
	}
	if !strings.Contains(string(out), "Ratio: 1.5") {
		t.Errorf("float lost:\n%s", out)
	}
}

func TestNewVOYAML_UsesTemplateKeyOrder(t *testing.T) {
	out, err := NewVOYAML(map[string]interface{}{
		"SupportURL": "u", "Community": "c", "LongName": "l", "Zzz": "last", "AppDescription": "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	order := []string{"LongName", "AppDescription", "Community", "SupportURL", "Zzz"}
	last := -1
	for _, k := range order {
		i := strings.Index(s, k+":")
		if i < 0 || i < last {
			t.Fatalf("keys out of template order (%s):\n%s", k, s)
		}
		last = i
	}
}
