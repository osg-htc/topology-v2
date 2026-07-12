package xmlapi

import (
	"context"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// Projects is the /miscproject/xml root. Field order matches miscproject.xsd.
type Projects struct {
	XMLName        struct{}     `xml:"Projects"`
	XsiNS          string       `xml:"xmlns:xsi,attr"`
	SchemaLocation string       `xml:"xsi:schemaLocation,attr"`
	Projects       []ProjectXML `xml:"Project"`
}

type ProjectXML struct {
	ID                  string        `xml:"ID,omitempty"`
	Name                string        `xml:"Name"`
	Description         string        `xml:"Description"`
	PIName              string        `xml:"PIName"`
	Organization        string        `xml:"Organization"`
	Department          string        `xml:"Department,omitempty"`
	FieldOfScience      string        `xml:"FieldOfScience"`
	Sponsor             SponsorXML    `xml:"Sponsor"`
	ResourceAllocations RAListXML     `xml:"ResourceAllocations"`
	InstitutionID       string        `xml:"InstitutionID"`
	FieldOfScienceID    string        `xml:"FieldOfScienceID"`
}

type SponsorXML struct {
	VirtualOrganization *SponsorRefXML `xml:"VirtualOrganization,omitempty"`
	CampusGrid          *SponsorRefXML `xml:"CampusGrid,omitempty"`
}
type SponsorRefXML struct {
	ID   int64  `xml:"ID"`
	Name string `xml:"Name"`
}
type RAListXML struct {
	Allocations []RAXML `xml:"ResourceAllocation"`
}
type RAXML struct {
	Type                  string          `xml:"Type"`
	SubmitResources       SubmitResXML    `xml:"SubmitResources"`
	ExecuteResourceGroups ExecuteRGsXML   `xml:"ExecuteResourceGroups"`
}
type SubmitResXML struct {
	SubmitResource []string `xml:"SubmitResource"`
}
type ExecuteRGsXML struct {
	Groups []ExecuteRGXML `xml:"ExecuteResourceGroup"`
}
type ExecuteRGXML struct {
	GroupName         string `xml:"GroupName"`
	LocalAllocationID string `xml:"LocalAllocationID"`
}

// BuildProjects assembles /miscproject/xml from the relational project rows,
// resolving VO sponsor ids from the vos table (gen_id fallback for campus grids).
func BuildProjects(ctx context.Context, q *db.Queries) (*Projects, error) {
	projects, err := q.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	vos, err := q.ListVOs(ctx)
	if err != nil {
		return nil, err
	}
	voID := map[string]int64{}
	for _, v := range vos {
		voID[v.Name] = v.VOID
	}

	out := &Projects{
		XsiNS:          "http://www.w3.org/2001/XMLSchema-instance",
		SchemaLocation: "https://topology.opensciencegrid.org/schema/miscproject.xsd",
	}
	for _, p := range projects {
		px := ProjectXML{
			ID: p.ProjectID, Name: p.Name, Description: p.Description,
			PIName: p.PIName, Organization: p.Organization, Department: p.Department,
			FieldOfScience: p.FieldOfScience, InstitutionID: p.InstitutionID,
			FieldOfScienceID: p.FieldOfScienceID,
			Sponsor:          sponsorXML(p.SponsorType, p.SponsorName, voID),
			ResourceAllocations: resourceAllocationsXML(p.Extra),
		}
		out.Projects = append(out.Projects, px)
	}
	return out, nil
}

func sponsorXML(sType, sName string, voID map[string]int64) SponsorXML {
	switch sType {
	case "VirtualOrganization":
		id := voID[sName]
		if id == 0 {
			id = topology.GenID(sName)
		}
		return SponsorXML{VirtualOrganization: &SponsorRefXML{ID: id, Name: sName}}
	case "CampusGrid":
		return SponsorXML{CampusGrid: &SponsorRefXML{ID: topology.GenID(sName), Name: sName}}
	default:
		return SponsorXML{}
	}
}

// resourceAllocationsXML builds ResourceAllocations from the project's extra map
// (present only for the few projects that declare allocations).
func resourceAllocationsXML(extra []byte) RAListXML {
	m := parseJSONMap(extra)
	out := RAListXML{}
	raw, ok := m["ResourceAllocations"].([]interface{})
	if !ok {
		return out
	}
	for _, item := range raw {
		ra, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		x := RAXML{Type: getStr(ra, "Type")}
		for _, sr := range toStringSlice(ra["SubmitResources"]) {
			x.SubmitResources.SubmitResource = append(x.SubmitResources.SubmitResource, sr)
		}
		if ergs, ok := ra["ExecuteResourceGroups"].([]interface{}); ok {
			for _, e := range ergs {
				if em, ok := e.(map[string]interface{}); ok {
					x.ExecuteResourceGroups.Groups = append(x.ExecuteResourceGroups.Groups,
						ExecuteRGXML{GroupName: getStr(em, "GroupName"), LocalAllocationID: getStr(em, "LocalAllocationID")})
				}
			}
		}
		out.Allocations = append(out.Allocations, x)
	}
	return out
}

func toStringSlice(v interface{}) []string {
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
