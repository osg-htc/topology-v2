package githistory

import (
	"testing"

	"github.com/bbockelm/topology-v2/internal/models"
)

func TestClassifyPath(t *testing.T) {
	cases := []struct {
		path string
		want classified
		ok   bool
	}{
		{
			path: "topology/ANL/FACILITY.yaml",
			want: classified{Kind: models.KindFacility, Name: "ANL"},
			ok:   true,
		},
		{
			path: "topology/ANL/ANL ASC/SITE.yaml",
			want: classified{Kind: models.KindSite, Name: "ANL ASC", Facility: "ANL"},
			ok:   true,
		},
		{
			path: "topology/ANL/ANL ASC/ANLASC.yaml",
			want: classified{Kind: models.KindResourceGroup, Name: "ANLASC", Site: "ANL ASC"},
			ok:   true,
		},
		{
			path: "projects/AMFORA.yaml",
			want: classified{Kind: models.KindProject, Name: "AMFORA"},
			ok:   true,
		},
		// Not entity files this importer understands -- excluded, not errors.
		{path: "topology/ANL/ANL ASC/ANLASC_downtime.yaml", ok: false},
		{path: "projects/_CAMPUS_GRIDS.yaml", ok: false},
		{path: "topology/services.yaml", ok: false},
		{path: "topology/support-centers.yaml", ok: false},
		{path: "virtual-organizations/LIGO.yaml", ok: false},
		{path: "", ok: false},
	}
	for _, c := range cases {
		got, ok := classifyPath(c.path)
		if ok != c.ok {
			t.Errorf("classifyPath(%q): ok = %v, want %v", c.path, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("classifyPath(%q) = %+v, want %+v", c.path, got, c.want)
		}
	}
}
