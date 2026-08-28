package topology

import "testing"

func TestProject_SponsorTypeName(t *testing.T) {
	cases := []struct {
		name     string
		sponsor  map[string]interface{}
		wantType string
		wantName string
	}{
		{
			name:     "well-formed sponsor block",
			sponsor:  map[string]interface{}{"CampusGrid": map[string]interface{}{"ID": float64(14), "Name": "OSG Connect"}},
			wantType: "CampusGrid", wantName: "OSG Connect",
		},
		{
			name:     "sponsor object with no Name key",
			sponsor:  map[string]interface{}{"VirtualOrganization": map[string]interface{}{"ID": float64(5)}},
			wantType: "VirtualOrganization", wantName: "",
		},
		{
			name:     "Name present but not a string",
			sponsor:  map[string]interface{}{"VirtualOrganization": map[string]interface{}{"Name": 123}},
			wantType: "VirtualOrganization", wantName: "",
		},
		{
			name:     "sponsor value isn't an object at all",
			sponsor:  map[string]interface{}{"VirtualOrganization": "not-a-map"},
			wantType: "VirtualOrganization", wantName: "",
		},
		{
			name:     "nil sponsor",
			sponsor:  nil,
			wantType: "", wantName: "",
		},
		{
			name:     "empty sponsor",
			sponsor:  map[string]interface{}{},
			wantType: "", wantName: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Project{Sponsor: c.sponsor}
			gotType, gotName := p.SponsorTypeName()
			if gotType != c.wantType || gotName != c.wantName {
				t.Errorf("SponsorTypeName() = (%q, %q), want (%q, %q)", gotType, gotName, c.wantType, c.wantName)
			}
		})
	}
}
