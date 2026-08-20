package paritytest

import (
	"strings"
	"testing"
)

func TestDecodeXML_AttributesTextChildrenAreDistinctSlots(t *testing.T) {
	doc := `<Root id="7"><Name>foo</Name></Root>`
	got, err := decodeXML(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("decodeXML: %v", err)
	}
	if got["@id"] != "7" {
		t.Fatalf("expected @id=7, got %v", got["@id"])
	}
	names, _ := got["Name"].([]interface{})
	if len(names) != 1 {
		t.Fatalf("expected exactly one Name child, got %v", got["Name"])
	}
	nameNode, _ := names[0].(map[string]interface{})
	if nameNode["#text"] != "foo" {
		t.Fatalf("expected Name text 'foo', got %v", nameNode["#text"])
	}
}

func TestDecodeXML_WhitespaceIndentationIgnored(t *testing.T) {
	// Indentation between sibling elements must not surface as text content
	// on the parent -- only genuine non-whitespace text counts.
	doc := "<Root>\n  <A>x</A>\n  <B>y</B>\n</Root>"
	got, err := decodeXML(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("decodeXML: %v", err)
	}
	if _, has := got["#text"]; has {
		t.Fatalf("expected no #text on Root from indentation whitespace, got %v", got["#text"])
	}
}

func TestDecodeXML_SelfClosingAndEmptyElement_Identical(t *testing.T) {
	selfClosing, err := decodeXML(strings.NewReader(`<Root><Tags/></Root>`))
	if err != nil {
		t.Fatalf("decodeXML (self-closing): %v", err)
	}
	empty, err := decodeXML(strings.NewReader(`<Root><Tags></Tags></Root>`))
	if err != nil {
		t.Fatalf("decodeXML (empty): %v", err)
	}
	if diffs := diffTrees("root", selfClosing, empty, nil); len(diffs) != 0 {
		t.Fatalf("expected self-closing and empty-element forms to decode identically, got %v", diffs)
	}
}

func TestDecodeXML_RepeatedTag_AlwaysASlice_EvenForOne(t *testing.T) {
	doc := `<Root><Resource>a</Resource></Root>`
	got, err := decodeXML(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("decodeXML: %v", err)
	}
	if _, ok := got["Resource"].([]interface{}); !ok {
		t.Fatalf("expected Resource to decode as a slice even with one occurrence, got %T", got["Resource"])
	}
}

func TestDecodeXML_MissingElement_IsAbsentNotEmptyString(t *testing.T) {
	got, err := decodeXML(strings.NewReader(`<Root><A>x</A></Root>`))
	if err != nil {
		t.Fatalf("decodeXML: %v", err)
	}
	if _, has := got["B"]; has {
		t.Fatalf("expected key B to be entirely absent, got %v", got["B"])
	}
}
