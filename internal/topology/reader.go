package topology

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadTree loads a topology directory tree (facilities/sites/resource-groups/
// downtimes) into a Topology. Flat root files (services.yaml, support-centers.
// yaml, etc.) are ignored here and handled separately.
func ReadTree(root string) (*Topology, error) {
	t := &Topology{
		Facilities:     map[string]*Facility{},
		Sites:          map[string]*Site{},
		ResourceGroups: map[string]*ResourceGroup{},
		Downtimes:      map[string][]*Downtime{},
		SiteFacility:   map[string]string{},
		RGSite:         map[string]string{},
	}

	facEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading topology root: %w", err)
	}
	for _, fe := range facEntries {
		if !fe.IsDir() {
			continue // skip flat files at the root
		}
		facName := fe.Name()
		facDir := filepath.Join(root, facName)

		fac := &Facility{Name: facName}
		if err := readYAMLFile(filepath.Join(facDir, "FACILITY.yaml"), fac); err != nil && !os.IsNotExist(errCause(err)) {
			return nil, err
		}
		t.Facilities[facName] = fac

		siteEntries, err := os.ReadDir(facDir)
		if err != nil {
			return nil, err
		}
		for _, se := range siteEntries {
			if !se.IsDir() {
				continue
			}
			siteName := se.Name()
			siteDir := filepath.Join(facDir, siteName)

			site := &Site{Name: siteName}
			if err := readYAMLFile(filepath.Join(siteDir, "SITE.yaml"), site); err != nil && !os.IsNotExist(errCause(err)) {
				return nil, err
			}
			t.Sites[siteName] = site
			t.SiteFacility[siteName] = facName

			rgEntries, err := os.ReadDir(siteDir)
			if err != nil {
				return nil, err
			}
			for _, re := range rgEntries {
				name := re.Name()
				if re.IsDir() || !strings.HasSuffix(name, ".yaml") {
					continue
				}
				if name == "SITE.yaml" {
					continue
				}
				full := filepath.Join(siteDir, name)
				if strings.HasSuffix(name, "_downtime.yaml") {
					var dts []*Downtime
					if err := readYAMLFile(full, &dts); err != nil {
						return nil, err
					}
					rgName := strings.TrimSuffix(name, "_downtime.yaml")
					t.Downtimes[rgName] = dts
					continue
				}
				rgName := strings.TrimSuffix(name, ".yaml")
				rg := &ResourceGroup{Name: rgName}
				if err := readYAMLFile(full, rg); err != nil {
					return nil, err
				}
				t.ResourceGroups[rgName] = rg
				t.RGSite[rgName] = siteName
			}
		}
	}
	return t, nil
}

// SupportCenterYAML is one entry in support-centers.yaml.
type SupportCenterYAML struct {
	ID          int64  `yaml:"ID"`
	LongName    string `yaml:"LongName,omitempty"`
	Community   string `yaml:"Community,omitempty"`
	Description string `yaml:"Description,omitempty"`
}

// ReadServices loads topology/services.yaml (a name -> int id map).
func ReadServices(root string) (map[string]int64, error) {
	out := map[string]int64{}
	if err := readYAMLFile(filepath.Join(root, "services.yaml"), &out); err != nil {
		if os.IsNotExist(errCause(err)) {
			return out, nil
		}
		return nil, err
	}
	return out, nil
}

// ReadSupportCenters loads topology/support-centers.yaml.
func ReadSupportCenters(root string) (map[string]SupportCenterYAML, error) {
	out := map[string]SupportCenterYAML{}
	if err := readYAMLFile(filepath.Join(root, "support-centers.yaml"), &out); err != nil {
		if os.IsNotExist(errCause(err)) {
			return out, nil
		}
		return nil, err
	}
	return out, nil
}

// readYAMLFile unmarshals a YAML file into v. A missing file is reported so the
// caller can decide whether it is fatal. Parsing is lenient about duplicate
// mapping keys (last-wins), matching PyYAML — the legacy topology repo contains
// a few malformed files that PyYAML silently accepts, and backup must not choke
// on data already in the repo.
func readYAMLFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return &fileError{path: path, err: err}
	}
	if err := DecodeYAMLLenient(data, v); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}

// DecodeYAMLLenient unmarshals YAML into v, tolerating duplicate mapping keys
// (last-wins, like PyYAML) which yaml.v3 otherwise rejects.
func DecodeYAMLLenient(data []byte, v interface{}) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	dedupeMappingKeys(&root)
	return root.Decode(v)
}

// dedupeMappingKeys walks a YAML node tree and, for every mapping, keeps only
// the last value seen for each key (PyYAML's behavior). This makes duplicate
// keys — which yaml.v3 rejects outright — non-fatal.
func dedupeMappingKeys(n *yaml.Node) {
	if n == nil {
		return
	}
	if n.Kind == yaml.MappingNode {
		order := []string{}
		keyNode := map[string]*yaml.Node{}
		valNode := map[string]*yaml.Node{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if _, seen := valNode[key]; !seen {
				order = append(order, key)
			}
			keyNode[key] = n.Content[i]
			valNode[key] = n.Content[i+1]
		}
		if len(order)*2 != len(n.Content) {
			rebuilt := make([]*yaml.Node, 0, len(order)*2)
			for _, key := range order {
				rebuilt = append(rebuilt, keyNode[key], valNode[key])
			}
			n.Content = rebuilt
		}
	}
	for _, c := range n.Content {
		dedupeMappingKeys(c)
	}
}

type fileError struct {
	path string
	err  error
}

func (e *fileError) Error() string { return e.path + ": " + e.err.Error() }
func (e *fileError) Unwrap() error { return e.err }

func errCause(err error) error {
	if fe, ok := err.(*fileError); ok {
		return fe.err
	}
	return err
}
