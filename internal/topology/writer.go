package topology

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WriteTree writes a Topology back out to a directory tree matching the OSG
// layout: <root>/<Facility>/FACILITY.yaml, <Facility>/<Site>/SITE.yaml, and
// <Facility>/<Site>/<RG>.yaml (+ <RG>_downtime.yaml).
func WriteTree(root string, t *Topology) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	for facName, fac := range t.Facilities {
		facDir := filepath.Join(root, facName)
		if err := os.MkdirAll(facDir, 0o755); err != nil {
			return err
		}
		if err := writeYAMLFile(filepath.Join(facDir, "FACILITY.yaml"), fac); err != nil {
			return err
		}
	}

	for siteName, site := range t.Sites {
		facName := t.SiteFacility[siteName]
		siteDir := filepath.Join(root, facName, siteName)
		if err := os.MkdirAll(siteDir, 0o755); err != nil {
			return err
		}
		if err := writeYAMLFile(filepath.Join(siteDir, "SITE.yaml"), site); err != nil {
			return err
		}
	}

	for rgName, rg := range t.ResourceGroups {
		siteName := t.RGSite[rgName]
		facName := t.SiteFacility[siteName]
		rgDir := filepath.Join(root, facName, siteName)
		if err := os.MkdirAll(rgDir, 0o755); err != nil {
			return err
		}
		if err := writeYAMLFile(filepath.Join(rgDir, rgName+".yaml"), rg); err != nil {
			return err
		}
		if dts := t.Downtimes[rgName]; len(dts) > 0 {
			if err := writeYAMLFile(filepath.Join(rgDir, rgName+"_downtime.yaml"), dts); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeYAMLFile(path string, v interface{}) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
