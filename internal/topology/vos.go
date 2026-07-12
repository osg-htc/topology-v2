package topology

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bbockelm/topology-v2/internal/db"
)

// VODoc is a virtual organization stored as a lossless document: the raw YAML is
// preserved verbatim for round-trip, plus a couple of parsed fields for lookups.
type VODoc struct {
	Name    string
	VOID    int64
	Disable bool
	Raw     []byte
}

// voHead parses only the indexed fields from a VO file.
type voHead struct {
	ID      *int64 `yaml:"ID"`
	Disable bool   `yaml:"Disable"`
}

// ReadVOs loads every <VO>.yaml from a virtual-organizations directory.
func ReadVOs(dir string) ([]VODoc, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []VODoc
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		var head voHead
		_ = yaml.Unmarshal(raw, &head)
		id := GenID(name)
		if head.ID != nil {
			id = *head.ID
		}
		out = append(out, VODoc{Name: name, VOID: id, Disable: head.Disable, Raw: raw})
	}
	return out, nil
}

// Project is a project modeled relationally. Common fields are typed; Sponsor
// (a small polymorphic block) stays a map, and any unmodeled keys (e.g.
// ResourceAllocations) land in the inline Extra catch-all for lossless
// round-trip. The file name is the project name.
type Project struct {
	ID               string                 `yaml:"ID,omitempty"`
	Description      string                 `yaml:"Description,omitempty"`
	Department       string                 `yaml:"Department,omitempty"`
	FieldOfScience   string                 `yaml:"FieldOfScience,omitempty"`
	FieldOfScienceID string                 `yaml:"FieldOfScienceID,omitempty"`
	Organization     string                 `yaml:"Organization,omitempty"`
	PIName           string                 `yaml:"PIName,omitempty"`
	InstitutionID    string                 `yaml:"InstitutionID,omitempty"`
	Sponsor          map[string]interface{} `yaml:"Sponsor,omitempty"`
	Extra            map[string]interface{} `yaml:",inline"`

	Name string `yaml:"-"`
}

// ReadProjects loads every <Project>.yaml from a projects directory (skipping
// the _CAMPUS_GRIDS index file).
func ReadProjects(dir string) ([]Project, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Project
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var p Project
		if err := decodeYAMLLenient(raw, &p); err != nil {
			return nil, fmt.Errorf("parsing project %s: %w", e.Name(), err)
		}
		p.Name = strings.TrimSuffix(e.Name(), ".yaml")
		out = append(out, p)
	}
	return out, nil
}

// SponsorTypeName extracts the sponsor kind and name for the indexed columns.
func (p Project) SponsorTypeName() (string, string) {
	for k, v := range p.Sponsor {
		if m, ok := v.(map[string]interface{}); ok {
			if name, ok := m["Name"].(string); ok {
				return k, name
			}
		}
		return k, ""
	}
	return "", ""
}

// ImportVOs writes VO documents into the vos table.
func ImportVOs(ctx context.Context, q *db.Queries, dir string) error {
	vos, err := ReadVOs(dir)
	if err != nil {
		return err
	}
	for _, v := range vos {
		if err := q.UpsertVO(ctx, v.Name, v.VOID, v.Disable, v.Raw); err != nil {
			return fmt.Errorf("import vo %q: %w", v.Name, err)
		}
	}
	return nil
}

// ImportProjects parses projects into their relational columns.
func ImportProjects(ctx context.Context, q *db.Queries, dir string) error {
	projects, err := ReadProjects(dir)
	if err != nil {
		return err
	}
	for _, p := range projects {
		sType, sName := p.SponsorTypeName()
		if err := q.UpsertProject(ctx, db.ProjectRow{
			Name: p.Name, ProjectID: p.ID, Description: p.Description,
			Department: p.Department, FieldOfScience: p.FieldOfScience,
			FieldOfScienceID: p.FieldOfScienceID, Organization: p.Organization,
			PIName: p.PIName, InstitutionID: p.InstitutionID,
			Sponsor: mustJSONAny(p.Sponsor), SponsorType: sType, SponsorName: sName,
			Extra: mustJSON(p.Extra),
		}); err != nil {
			return fmt.Errorf("import project %q: %w", p.Name, err)
		}
	}
	return nil
}

// ExportVOsToDir writes the vos table back out as <VO>.yaml files (verbatim).
func ExportVOsToDir(ctx context.Context, q *db.Queries, dir string) error {
	vos, err := q.ListVOs(ctx)
	if err != nil {
		return err
	}
	if len(vos) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, v := range vos {
		if err := os.WriteFile(filepath.Join(dir, v.Name+".yaml"), v.Raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ExportProjectsToDir reconstructs each project from its relational columns and
// writes it back as <Project>.yaml.
func ExportProjectsToDir(ctx context.Context, q *db.Queries, dir string) error {
	projects, err := q.ListProjects(ctx)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, r := range projects {
		p := Project{
			ID: r.ProjectID, Description: r.Description, Department: r.Department,
			FieldOfScience: r.FieldOfScience, FieldOfScienceID: r.FieldOfScienceID,
			Organization: r.Organization, PIName: r.PIName, InstitutionID: r.InstitutionID,
			Sponsor: fromJSONMap(r.Sponsor), Extra: fromJSONMap(r.Extra),
		}
		if err := writeYAMLFile(filepath.Join(dir, r.Name+".yaml"), &p); err != nil {
			return err
		}
	}
	return nil
}

// fromJSONMap unmarshals a JSON object into a map, or nil.
func fromJSONMap(b []byte) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}
