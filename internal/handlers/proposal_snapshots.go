package handlers

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// snapshotEntity builds a "before" snapshot of an entity's current live
// state, in the exact JSON shape its own proposed_state already uses -- so
// the two can be rendered side by side (and, later, diffed) directly. Called
// once, at proposal creation time (the actual branch point) -- see
// CreateProposal -- never on revise, since "branched from" means the moment
// of creation, not each subsequent draft edit.
func (h *Handler) snapshotEntity(ctx context.Context, entityKind, targetName string) json.RawMessage {
	switch entityKind {
	case models.KindResource:
		return h.snapshotResourceState(ctx, targetName)
	case models.KindResourceGroup:
		return h.snapshotResourceGroupState(ctx, targetName)
	case models.KindSite:
		return h.snapshotSiteState(ctx, targetName)
	case models.KindFacility:
		return h.snapshotFacilityState(ctx, targetName)
	case models.KindProject:
		return h.snapshotProjectState(ctx, targetName)
	default:
		return nil // downtime, bundle: not yet supported -- unchanged from before
	}
}

func (h *Handler) snapshotResourceState(ctx context.Context, targetName string) json.RawMessage {
	topID, err := strconv.ParseInt(targetName, 10, 64)
	if err != nil {
		return nil
	}
	row, err := h.queries.GetResourceRow(ctx, topID)
	if err != nil {
		return nil
	}
	res := topology.Resource{
		Active: row.Active, Description: row.Description, FQDN: row.FQDN, DN: row.DN,
		// emptySlice, not nil: the proposal schema types these as "array",
		// and a nil slice marshals to JSON null, not [] -- which would fail
		// schema validation the moment this snapshot gets merged into a
		// submission (a nil slice here isn't "cleared", it's simply "empty",
		// same as what the edit form itself always sends).
		FQDNAliases: emptySlice(row.FQDNAliases), Tags: emptySlice(row.Tags), AllowedVOs: emptySlice(row.AllowedVOs),
		VOOwnership: fromJSONAny(row.VOOwnership), WLCGInformation: fromJSONAny(row.WLCGInformation),
		Extra: fromJSONMap(row.Extra),
	}
	// Deliberately no ID: a proposal's own Resource payload never carries
	// one, so leaving it out keeps the two columns comparable field-for-field.
	svcs, err := h.queries.ListResourceServices(ctx, topID)
	if err != nil {
		return nil
	}
	if len(svcs) > 0 {
		res.Services = map[string]*topology.Service{}
		for _, s := range svcs {
			res.Services[s.ServiceName] = topology.ServiceFromBlob(s.Description, s.Details)
		}
	}
	contacts, err := h.queries.ListResourceContacts(ctx, topID)
	if err != nil {
		return nil
	}
	if len(contacts) > 0 {
		res.ContactLists = map[string]map[string]topology.Contact{}
		for _, c := range contacts {
			if res.ContactLists[c.ContactType] == nil {
				res.ContactLists[c.ContactType] = map[string]topology.Contact{}
			}
			res.ContactLists[c.ContactType][c.Rank] = topology.Contact{Name: c.ContactName, ID: c.ContactID}
		}
	}
	b, err := json.Marshal(resourceProposal{ResourceGroup: row.RGName, Name: row.Name, Resource: res})
	if err != nil {
		return nil
	}
	return b
}

func (h *Handler) snapshotResourceGroupState(ctx context.Context, targetName string) json.RawMessage {
	row, err := h.queries.GetResourceGroupRow(ctx, targetName)
	if err != nil {
		return nil
	}
	contacts, err := h.queries.ListEntityContacts(ctx, models.KindResourceGroup, targetName)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(rgProposal{
		Name: row.Name, Site: row.SiteName, Production: row.Production,
		SupportCenter: row.SupportCenter, GroupDescription: row.GroupDescription, Contacts: contacts,
	})
	if err != nil {
		return nil
	}
	return b
}

func (h *Handler) snapshotSiteState(ctx context.Context, targetName string) json.RawMessage {
	row, err := h.queries.GetSiteRow(ctx, targetName)
	if err != nil {
		return nil
	}
	contacts, err := h.queries.ListEntityContacts(ctx, models.KindSite, targetName)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(siteProposal{
		Name: row.Name, Facility: row.FacilityName, LongName: row.LongName, Description: row.Description,
		AddressLine1: row.AddressLine1, AddressLine2: row.AddressLine2, City: row.City, State: row.State,
		Country: row.Country, Zipcode: row.Zipcode, Latitude: row.Latitude, Longitude: row.Longitude,
		Contacts: contacts,
	})
	if err != nil {
		return nil
	}
	return b
}

func (h *Handler) snapshotFacilityState(ctx context.Context, targetName string) json.RawMessage {
	row, err := h.queries.GetFacilityRow(ctx, targetName)
	if err != nil {
		return nil
	}
	contacts, err := h.queries.ListEntityContacts(ctx, models.KindFacility, targetName)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(facilityProposal{Name: row.Name, InstitutionID: row.InstitutionID, Contacts: contacts})
	if err != nil {
		return nil
	}
	return b
}

func (h *Handler) snapshotProjectState(ctx context.Context, targetName string) json.RawMessage {
	row, err := h.queries.GetProjectByName(ctx, targetName)
	if err != nil {
		return nil
	}
	var sponsor map[string]interface{}
	if len(row.Sponsor) > 0 {
		_ = json.Unmarshal(row.Sponsor, &sponsor)
	}
	b, err := json.Marshal(projectProposal{
		Name: row.Name, ID: row.ProjectID, Description: row.Description, Department: row.Department,
		FieldOfScience: row.FieldOfScience, FieldOfScienceID: row.FieldOfScienceID,
		Organization: row.Organization, PIName: row.PIName, InstitutionID: row.InstitutionID, Sponsor: sponsor,
	})
	if err != nil {
		return nil
	}
	return b
}

func fromJSONMap(b []byte) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	var v map[string]interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}

func fromJSONAny(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}

// mergeShallow merges incoming onto current: every key incoming actually
// contains overwrites current's value for that key (whatever depth/shape it
// is); every key incoming omits keeps current's value untouched. This is
// what makes a proposal's stored state always complete regardless of how
// much of the entity's schema the tool that produced it actually models --
// an editor that's never heard of a field simply never mentions its key, so
// that field is preserved by construction, not by convention. See
// CreateProposal and ReviseProposal for the two places this runs.
//
// Deliberately shallow on its own, not recursive: a nested collection like a
// resource's Services map is, today, always resubmitted whole by the one
// editor that manages it, so treating an incoming key's value as atomic is
// what lets deleting an entry from that collection actually delete it.
// mergeProposedState calls this at more than one level where a field can be
// nested one level deeper than this function alone would protect (see
// mergeServiceEntries) -- this function itself stays a single generic,
// depth-agnostic building block; the extra levels are named explicitly at
// each call site, not built into a general recursive-everything rule.
func mergeShallow(current, incoming map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(current)+len(incoming))
	for k, v := range current {
		merged[k] = v
	}
	for k, v := range incoming {
		merged[k] = v
	}
	return merged
}

// mergeProposedState merges a proposal's incoming payload onto a base
// (either a fresh snapshotEntity read, for CreateProposal, or the proposal's
// own prior proposed_state, for ReviseProposal), returning the result as
// JSON. Falls back to the raw incoming value if either side fails to decode
// as a JSON object, rather than blocking the request.
//
// resourceProposal is the one shape that wraps an entity's own fields one
// level down, under a "resource" key (rgProposal/siteProposal/
// facilityProposal/projectProposal all have their fields at the top level).
// Every resource edit always includes that "resource" key, so a merge that
// stopped at the outer envelope would replace it wholesale and never reach
// the fields it's meant to protect -- merge one level deeper for this kind.
func mergeProposedState(entityKind string, base, incoming json.RawMessage) json.RawMessage {
	baseMap := fromJSONMap(base)
	incomingMap := fromJSONMap(incoming)
	if baseMap == nil || incomingMap == nil {
		return incoming
	}
	merged := mergeShallow(baseMap, incomingMap)
	if entityKind == models.KindResource {
		if baseInner, ok := baseMap["resource"].(map[string]interface{}); ok {
			if incomingInner, ok := incomingMap["resource"].(map[string]interface{}); ok {
				mergedInner := mergeShallow(baseInner, incomingInner)
				// One more level: the *set* of service names in the
				// incoming payload stays authoritative (so removing a
				// service still removes it), but for a name present on
				// both sides, merge that one service's own fields too --
				// otherwise a service's Details/Extra (which no editor has
				// UI for) gets wiped the same way DN/VOOwnership did,
				// just one level deeper. Only touch the key at all if
				// incoming actually mentions Services -- an editor that
				// omits it entirely means "leave as-is," same as any other
				// field, and must not be overwritten with a computed nil.
				if incomingServices, present := incomingInner["Services"]; present {
					mergedInner["Services"] = mergeServiceEntries(baseInner["Services"], incomingServices)
				}
				merged["resource"] = mergedInner
			}
		}
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return incoming
	}
	return b
}

// emptySlice normalizes a nil string slice to an empty (non-nil) one, so it
// marshals to JSON "[]" instead of "null".
func emptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// mergeServiceEntries merges a resource's Services map one level deeper than
// mergeShallow does on its own: the set of names present in incoming stays
// authoritative (a service incoming doesn't mention is gone, so deleting one
// still works), but for a name present in both, that service's own fields
// (Description/Details/Extra) are merged rather than incoming replacing it
// wholesale -- otherwise a field like Details, which no editor has UI for,
// gets silently dropped the same way a resource's own DN/VOOwnership did.
func mergeServiceEntries(base, incoming interface{}) interface{} {
	incomingMap, ok := incoming.(map[string]interface{})
	if !ok {
		return incoming // Services wasn't in the incoming payload at all
	}
	baseMap, ok := base.(map[string]interface{})
	if !ok {
		return incoming // nothing to merge from
	}
	out := make(map[string]interface{}, len(incomingMap))
	for name, incomingSvc := range incomingMap {
		incomingSvcMap, incomingIsObj := incomingSvc.(map[string]interface{})
		baseSvcMap, baseHasName := baseMap[name].(map[string]interface{})
		if incomingIsObj && baseHasName {
			out[name] = mergeShallow(baseSvcMap, incomingSvcMap)
		} else {
			out[name] = incomingSvc
		}
	}
	return out
}
