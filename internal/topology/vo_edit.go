package topology

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/bbockelm/topology-v2/internal/conv"
)

// A VO is stored as its raw YAML (see VODoc). Editing one through the app means
// applying a JSON document -- the VO's top-level keys, exactly as in the file --
// back onto that YAML. These helpers do it without disturbing what didn't
// change: keys the edit leaves alone keep their original node, so their
// comments, ordering and formatting survive, and a VO edited through the form
// still diffs cleanly against its file in the GitHub repo.

// voKeyOrder is the order new keys are written in (v1's
// template-virtual-organization.yaml), so a VO created in the app reads like
// one a person wrote by hand. Keys not listed sort alphabetically after these.
var voKeyOrder = []string{
	"ID", "Name", "LongName", "AppDescription", "Community", "CertificateOnly",
	"Active", "Disable", "Contacts", "Credentials", "FieldsOfScience",
	"PrimaryURL", "PurposeURL", "SupportURL", "MembershipServicesURL",
	"ParentVO", "ReportingGroups", "OASIS", "DataFederations",
}

// VODocFromRaw parses a VO's raw YAML into the JSON-shaped document the
// proposal workflow carries.
func VODocFromRaw(raw []byte) (map[string]interface{}, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("stored VO YAML is not parseable: %w", err)
	}
	if doc == nil {
		doc = map[string]interface{}{}
	}
	return doc, nil
}

// NormalizeVODoc decodes a JSON object into a VO document. Integers stay
// integers (see conv.DecodeJSONObject): a plain json.Unmarshal would turn a VO
// ID like 1000000000 into a float64, which yaml then writes as 1e+09.
func NormalizeVODoc(raw []byte) (map[string]interface{}, error) {
	return conv.DecodeJSONObject(raw)
}

// jsonEqual compares two decoded values by their canonical JSON form (map keys
// sort), so int 9 and float 9.0 are equal and key order never matters.
func jsonEqual(a, b interface{}) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ab, bb)
}

// PatchVOYAML applies doc onto raw: keys absent from doc are removed, keys
// whose value is unchanged keep their original YAML node (comments and all),
// changed keys get a fresh value node (keeping the old one's comments), and new
// keys are appended in voKeyOrder.
func PatchVOYAML(raw []byte, doc map[string]interface{}) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("stored VO YAML is not parseable: %w", err)
	}
	if root.Kind == 0 || len(root.Content) == 0 {
		return NewVOYAML(doc)
	}
	m := root.Content[0]
	if m.Kind != yaml.MappingNode {
		return nil, errors.New("stored VO YAML is not a mapping")
	}
	existing, err := VODocFromRaw(raw)
	if err != nil {
		return nil, err
	}

	have := map[string]bool{}
	kept := make([]*yaml.Node, 0, len(m.Content))
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		nv, present := doc[k.Value]
		if !present {
			continue // removed by the edit
		}
		have[k.Value] = true
		if !jsonEqual(existing[k.Value], nv) {
			var nn yaml.Node
			if err := nn.Encode(nv); err != nil {
				return nil, fmt.Errorf("encoding %s: %w", k.Value, err)
			}
			nn.HeadComment, nn.LineComment, nn.FootComment = v.HeadComment, v.LineComment, v.FootComment
			v = &nn
		}
		kept = append(kept, k, v)
	}
	for _, k := range orderedNewKeys(doc, have) {
		kn, vn, err := keyValueNodes(k, doc[k])
		if err != nil {
			return nil, err
		}
		kept = append(kept, kn, vn)
	}
	m.Content = kept
	return encodeNode(&root)
}

// NewVOYAML renders a brand-new VO document in voKeyOrder.
func NewVOYAML(doc map[string]interface{}) ([]byte, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range orderedNewKeys(doc, nil) {
		kn, vn, err := keyValueNodes(k, doc[k])
		if err != nil {
			return nil, err
		}
		m.Content = append(m.Content, kn, vn)
	}
	return encodeNode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{m}})
}

func keyValueNodes(k string, v interface{}) (*yaml.Node, *yaml.Node, error) {
	kn := &yaml.Node{}
	if err := kn.Encode(k); err != nil {
		return nil, nil, err
	}
	vn := &yaml.Node{}
	if err := vn.Encode(v); err != nil {
		return nil, nil, fmt.Errorf("encoding %s: %w", k, err)
	}
	return kn, vn, nil
}

// orderedNewKeys lists the keys of doc not already in have, voKeyOrder first.
func orderedNewKeys(doc map[string]interface{}, have map[string]bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range voKeyOrder {
		if _, ok := doc[k]; ok && !have[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	var rest []string
	for k := range doc {
		if !have[k] && !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func encodeNode(n *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseVOHead reads the two indexed fields out of a VO's raw YAML the way the
// importer does: an explicit ID (nil when absent) and Disable.
func ParseVOHead(raw []byte) (id *int64, disable bool) {
	var head voHead
	_ = yaml.Unmarshal(raw, &head)
	return head.ID, head.Disable
}
