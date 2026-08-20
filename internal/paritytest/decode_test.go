package paritytest

// Generic decoders shared by the parity tests. Neither v1 nor v2's own
// XSD-shaped Go structs (internal/xmlapi) are used here on purpose: decoding
// into a hand-maintained struct silently drops any field the struct's author
// didn't know about, on both sides, hiding exactly the kind of drift a parity
// test exists to catch. Decoding into a fully generic tree instead means a
// missing/extra key is visible to the differ by construction.
//
// Tree shape produced by decodeXML, used uniformly by diffTrees:
//   - an XML element becomes a map[string]interface{}
//   - an attribute "foo" becomes key "@foo" -> string
//   - non-whitespace text content becomes key "#text" -> string
//   - EVERY child element tag becomes key "tagname" -> []interface{}, always a
//     slice (even for a tag that occurs exactly once) so the differ never has
//     to special-case "one occurrence" vs "many" -- it always compares
//     same-tag siblings as an order-tolerant multiset.

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// decodeXML parses r and returns the root element as a generic tree.
func decodeXML(r io.Reader) (map[string]interface{}, error) {
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("decodeXML: no root element: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return decodeXMLElement(dec, se)
		}
	}
}

func decodeXMLElement(dec *xml.Decoder, start xml.StartElement) (map[string]interface{}, error) {
	node := make(map[string]interface{})
	for _, a := range start.Attr {
		node["@"+a.Name.Local] = a.Value
	}

	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("decodeXML: unexpected end inside <%s>: %w", start.Name.Local, err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			text.Write(t)
		case xml.StartElement:
			child, err := decodeXMLElement(dec, t)
			if err != nil {
				return nil, err
			}
			key := t.Name.Local
			existing, _ := node[key].([]interface{})
			node[key] = append(existing, child)
		case xml.EndElement:
			if trimmed := strings.TrimSpace(text.String()); trimmed != "" {
				node["#text"] = trimmed
			}
			return node, nil
		}
	}
}
