package githistory

import (
	"encoding/json"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// These mirror the unexported resourceProposal/rgProposal/siteProposal/
// facilityProposal/projectProposal/bundleOp/bundleProposal JSON shapes in
// internal/handlers/proposals.go exactly, field-for-field -- so a historical
// row is byte-for-byte the same shape a live proposal for that kind already
// produces, and needs no special-casing anywhere it's read (StructuredView,
// ListProposalsByEntity, schema validation on a future revise). They're a
// separate copy, not a shared/exported type, so this batch-CLI package
// doesn't need to import the HTTP handler layer for four small structs.
//
// v1's git history never carries resource_group/site/facility contacts --
// those come from a separate contacts repo merged at request time, never
// stored in this repo -- so Contacts is always an empty (not nil) slice
// here, matching how every edit form in this app unconditionally includes
// modeled fields rather than omitting empty ones.

type resourcePayload struct {
	ResourceGroup string            `json:"resource_group"`
	Name          string            `json:"name"`
	Resource      topology.Resource `json:"resource"`
}

type resourceGroupPayload struct {
	Name             string             `json:"name"`
	Site             string             `json:"site"`
	Production       *bool              `json:"production"`
	SupportCenter    string             `json:"support_center"`
	GroupDescription string             `json:"group_description"`
	Contacts         []db.EntityContact `json:"contacts"`
}

type sitePayload struct {
	Name         string             `json:"name"`
	Facility     string             `json:"facility"`
	LongName     string             `json:"long_name"`
	Description  string             `json:"description"`
	AddressLine1 string             `json:"address_line1"`
	AddressLine2 string             `json:"address_line2"`
	City         string             `json:"city"`
	State        string             `json:"state"`
	Country      string             `json:"country"`
	Zipcode      string             `json:"zipcode"`
	Latitude     *float64           `json:"latitude"`
	Longitude    *float64           `json:"longitude"`
	Contacts     []db.EntityContact `json:"contacts"`
}

type facilityPayload struct {
	Name          string             `json:"name"`
	InstitutionID string             `json:"institution_id"`
	Contacts      []db.EntityContact `json:"contacts"`
}

type projectPayload struct {
	Name             string                 `json:"name"`
	ID               string                 `json:"id"`
	Description      string                 `json:"description"`
	Department       string                 `json:"department"`
	FieldOfScience   string                 `json:"field_of_science"`
	FieldOfScienceID string                 `json:"field_of_science_id"`
	Organization     string                 `json:"organization"`
	PIName           string                 `json:"pi_name"`
	InstitutionID    string                 `json:"institution_id"`
	Sponsor          map[string]interface{} `json:"sponsor"`
}

type bundleOpPayload struct {
	EntityKind    string          `json:"entity_kind"`
	Operation     string          `json:"operation"`
	TargetName    string          `json:"target_name"`
	ProposedState json.RawMessage `json:"proposed_state"`
	SchemaVersion int             `json:"schema_version"`
}

type bundlePayload struct {
	Operations []bundleOpPayload `json:"operations"`
}

func noContacts() []db.EntityContact { return []db.EntityContact{} }

// normalizeResource returns a copy of r with nil slice/map fields the
// resource proposal schema requires to be an array/object (never null)
// replaced by empty ones -- Go's encoding/json marshals a nil slice or map
// as JSON null, not [] or {}, and a resource parsed straight from an old
// YAML file that simply never mentioned e.g. FQDNAliases has exactly that
// nil zero value. Same fix as emptySlice in
// internal/handlers/proposal_snapshots.go, applied here for the same
// reason: a nil collection here isn't "cleared", it's "empty".
func normalizeResource(r *topology.Resource) topology.Resource {
	out := *r
	if out.FQDNAliases == nil {
		out.FQDNAliases = []string{}
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if out.AllowedVOs == nil {
		out.AllowedVOs = []string{}
	}
	if out.Services == nil {
		out.Services = map[string]*topology.Service{}
	}
	if out.ContactLists == nil {
		out.ContactLists = map[string]map[string]topology.Contact{}
	}
	return out
}
