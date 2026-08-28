package githistory

import (
	"encoding/json"
	"reflect"

	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// entityChange is one entity-level create/update/delete detected while
// decomposing a single changed path (or, for a resource_group file, one of
// possibly several -- the RG's own fields plus each nested resource).
// Name/OldName are the entity's name(s) AT THIS POINT in history, before
// rename-normalization -- importer.go resolves the real target_name via
// PathIdentity (or, for a resource, via ResourceTopologyID directly).
type entityChange struct {
	Kind      string
	OldName   string // "" for a create
	NewName   string // "" for a delete
	Operation string // models.OpCreate | models.OpUpdate | models.OpDelete
	Before    json.RawMessage
	After     json.RawMessage
	// ResourceTopologyID is set only for Kind==models.KindResource -- a
	// resource has an immutable id independent of any rename bookkeeping,
	// resolved right here from whichever side of the change actually has the
	// parsed struct (its own explicit ID, or GenID(name) to match v1's own
	// fallback convention exactly).
	ResourceTopologyID int64
	// TargetName is filled in by importer.go, after decompose returns: a
	// resource's immutable topology_id (from ResourceTopologyID); for the
	// other four kinds, the entity's PathIdentity-resolved final name, so a
	// pre-rename historical row still shows up under the name the entity
	// has today. Empty here by construction.
	TargetName string
}

// decomposeFacility diffs one FACILITY.yaml change. old/newBlob are nil on
// the side that doesn't exist (add/delete); old/newName are "" likewise.
func decomposeFacility(oldName, newName string, oldBlob, newBlob []byte) ([]entityChange, error) {
	var oldFac, newFac *topology.Facility
	if oldBlob != nil {
		oldFac = &topology.Facility{Name: oldName}
		if err := topology.DecodeYAMLLenient(oldBlob, oldFac); err != nil {
			return nil, err
		}
	}
	if newBlob != nil {
		newFac = &topology.Facility{Name: newName}
		if err := topology.DecodeYAMLLenient(newBlob, newFac); err != nil {
			return nil, err
		}
	}
	switch {
	case oldFac == nil && newFac == nil:
		return nil, nil
	case oldFac == nil:
		after, err := json.Marshal(facilityPayload{Name: newFac.Name, InstitutionID: newFac.InstitutionID, Contacts: noContacts()})
		return oneChange(models.KindFacility, "", newName, models.OpCreate, nil, after, err)
	case newFac == nil:
		before, err := json.Marshal(facilityPayload{Name: oldFac.Name, InstitutionID: oldFac.InstitutionID, Contacts: noContacts()})
		return oneChange(models.KindFacility, oldName, "", models.OpDelete, before, nil, err)
	default:
		if oldName == newName && oldFac.InstitutionID == newFac.InstitutionID && reflect.DeepEqual(oldFac.Extra, newFac.Extra) {
			return nil, nil // nothing this importer models actually changed
		}
		before, err := json.Marshal(facilityPayload{Name: oldFac.Name, InstitutionID: oldFac.InstitutionID, Contacts: noContacts()})
		if err != nil {
			return nil, err
		}
		after, err := json.Marshal(facilityPayload{Name: newFac.Name, InstitutionID: newFac.InstitutionID, Contacts: noContacts()})
		return oneChange(models.KindFacility, oldName, newName, models.OpUpdate, before, after, err)
	}
}

// decomposeSite diffs one SITE.yaml change. oldFacility/newFacility are the
// containing facility's name on each side (from the path), independent of
// the site's own name -- a site can be reparented to a different facility
// without its own name changing at all.
func decomposeSite(oldName, newName, oldFacility, newFacility string, oldBlob, newBlob []byte) ([]entityChange, error) {
	var oldSite, newSite *topology.Site
	if oldBlob != nil {
		oldSite = &topology.Site{Name: oldName}
		if err := topology.DecodeYAMLLenient(oldBlob, oldSite); err != nil {
			return nil, err
		}
	}
	if newBlob != nil {
		newSite = &topology.Site{Name: newName}
		if err := topology.DecodeYAMLLenient(newBlob, newSite); err != nil {
			return nil, err
		}
	}
	toPayload := func(s *topology.Site, facility string) sitePayload {
		return sitePayload{
			Name: s.Name, Facility: facility, LongName: s.LongName, Description: s.Description,
			AddressLine1: s.AddressLine1, AddressLine2: s.AddressLine2, City: s.City, State: s.State,
			Country: s.Country, Zipcode: s.Zipcode, Latitude: s.Latitude, Longitude: s.Longitude,
			Contacts: noContacts(),
		}
	}
	switch {
	case oldSite == nil && newSite == nil:
		return nil, nil
	case oldSite == nil:
		after, err := json.Marshal(toPayload(newSite, newFacility))
		return oneChange(models.KindSite, "", newName, models.OpCreate, nil, after, err)
	case newSite == nil:
		before, err := json.Marshal(toPayload(oldSite, oldFacility))
		return oneChange(models.KindSite, oldName, "", models.OpDelete, before, nil, err)
	default:
		if oldName == newName && oldFacility == newFacility && reflect.DeepEqual(*oldSite, *newSite) {
			return nil, nil
		}
		before, err := json.Marshal(toPayload(oldSite, oldFacility))
		if err != nil {
			return nil, err
		}
		after, err := json.Marshal(toPayload(newSite, newFacility))
		return oneChange(models.KindSite, oldName, newName, models.OpUpdate, before, after, err)
	}
}

// decomposeProject diffs one projects/<Project>.yaml change.
func decomposeProject(oldName, newName string, oldBlob, newBlob []byte) ([]entityChange, error) {
	var oldProj, newProj *topology.Project
	if oldBlob != nil {
		p := topology.Project{Name: oldName}
		if err := topology.DecodeYAMLLenient(oldBlob, &p); err != nil {
			return nil, err
		}
		oldProj = &p
	}
	if newBlob != nil {
		p := topology.Project{Name: newName}
		if err := topology.DecodeYAMLLenient(newBlob, &p); err != nil {
			return nil, err
		}
		newProj = &p
	}
	toPayload := func(p *topology.Project) projectPayload {
		return projectPayload{
			Name: p.Name, ID: p.ID, Description: p.Description, Department: p.Department,
			FieldOfScience: p.FieldOfScience, FieldOfScienceID: p.FieldOfScienceID,
			Organization: p.Organization, PIName: p.PIName, InstitutionID: p.InstitutionID,
			Sponsor: p.Sponsor,
		}
	}
	switch {
	case oldProj == nil && newProj == nil:
		return nil, nil
	case oldProj == nil:
		after, err := json.Marshal(toPayload(newProj))
		return oneChange(models.KindProject, "", newName, models.OpCreate, nil, after, err)
	case newProj == nil:
		before, err := json.Marshal(toPayload(oldProj))
		return oneChange(models.KindProject, oldName, "", models.OpDelete, before, nil, err)
	default:
		if oldName == newName && reflect.DeepEqual(*oldProj, *newProj) {
			return nil, nil
		}
		before, err := json.Marshal(toPayload(oldProj))
		if err != nil {
			return nil, err
		}
		after, err := json.Marshal(toPayload(newProj))
		return oneChange(models.KindProject, oldName, newName, models.OpUpdate, before, after, err)
	}
}

// decomposeResourceGroup diffs one resource-group YAML change: the RG's own
// fields (as one entityChange, if they actually changed) plus a keyed diff
// of its nested Resources map (one entityChange per affected resource).
//
// A resource is matched between old and new by its YAML map key (it has no
// Name field of its own -- see internal/topology/model.go). A resource
// whose key changes with no explicit YAML ID to anchor it is
// indistinguishable from delete+create; that heuristic (explicit ID
// present and unchanged => rename, else delete+create) is applied here and
// is an accepted limitation of the source data, not a bug -- see the plan.
func decomposeResourceGroup(oldName, newName, oldSite, newSite string, oldBlob, newBlob []byte) ([]entityChange, error) {
	var oldRG, newRG *topology.ResourceGroup
	if oldBlob != nil {
		oldRG = &topology.ResourceGroup{Name: oldName}
		if err := topology.DecodeYAMLLenient(oldBlob, oldRG); err != nil {
			return nil, err
		}
	}
	if newBlob != nil {
		newRG = &topology.ResourceGroup{Name: newName}
		if err := topology.DecodeYAMLLenient(newBlob, newRG); err != nil {
			return nil, err
		}
	}

	var changes []entityChange

	rgToPayload := func(rg *topology.ResourceGroup, site string) resourceGroupPayload {
		return resourceGroupPayload{
			Name: rg.Name, Site: site, Production: rg.Production,
			SupportCenter: rg.SupportCenter, GroupDescription: rg.GroupDescription, Contacts: noContacts(),
		}
	}
	rgCoreEqual := func(a, b *topology.ResourceGroup) bool {
		type core struct {
			Production       *bool
			GroupID          *int64
			SupportCenter    string
			GroupDescription string
			Extra            map[string]interface{}
		}
		return reflect.DeepEqual(
			core{a.Production, a.GroupID, a.SupportCenter, a.GroupDescription, a.Extra},
			core{b.Production, b.GroupID, b.SupportCenter, b.GroupDescription, b.Extra},
		)
	}

	switch {
	case oldRG == nil && newRG == nil:
		return nil, nil
	case oldRG == nil:
		after, err := json.Marshal(rgToPayload(newRG, newSite))
		if c, err2 := oneChange(models.KindResourceGroup, "", newName, models.OpCreate, nil, after, err); err2 != nil {
			return nil, err2
		} else {
			changes = append(changes, c...)
		}
	case newRG == nil:
		before, err := json.Marshal(rgToPayload(oldRG, oldSite))
		if c, err2 := oneChange(models.KindResourceGroup, oldName, "", models.OpDelete, before, nil, err); err2 != nil {
			return nil, err2
		} else {
			changes = append(changes, c...)
		}
	default:
		if !(oldName == newName && oldSite == newSite && rgCoreEqual(oldRG, newRG)) {
			before, err := json.Marshal(rgToPayload(oldRG, oldSite))
			if err != nil {
				return nil, err
			}
			after, err := json.Marshal(rgToPayload(newRG, newSite))
			c, err2 := oneChange(models.KindResourceGroup, oldName, newName, models.OpUpdate, before, after, err)
			if err2 != nil {
				return nil, err2
			}
			changes = append(changes, c...)
		}
	}

	resChanges, err := decomposeResources(oldName, newName, oldRG, newRG)
	if err != nil {
		return nil, err
	}
	return append(changes, resChanges...), nil
}

// decomposeResources does the keyed diff of a resource_group's nested
// Resources map. rgName is used as-is for both sides in the common case
// (the RG file itself wasn't renamed in this same commit); when it was,
// each resource's payload still correctly names whichever RG it actually
// belonged to on that side.
func decomposeResources(oldRGName, newRGName string, oldRG, newRG *topology.ResourceGroup) ([]entityChange, error) {
	var old, new map[string]*topology.Resource
	if oldRG != nil {
		old = oldRG.Resources
	}
	if newRG != nil {
		new = newRG.Resources
	}
	seen := map[string]bool{}
	var changes []entityChange
	for name, oldRes := range old {
		seen[name] = true
		newRes := new[name]
		if newRes == nil {
			before, err := json.Marshal(resourcePayload{ResourceGroup: oldRGName, Name: name, Resource: normalizeResource(oldRes)})
			if err != nil {
				return nil, err
			}
			changes = append(changes, entityChange{
				Kind: models.KindResource, OldName: name, Operation: models.OpDelete,
				Before: before, ResourceTopologyID: resolveResourceID(oldRes.ID, name),
			})
			continue
		}
		if reflect.DeepEqual(*oldRes, *newRes) && oldRGName == newRGName {
			continue
		}
		before, err := json.Marshal(resourcePayload{ResourceGroup: oldRGName, Name: name, Resource: normalizeResource(oldRes)})
		if err != nil {
			return nil, err
		}
		after, err := json.Marshal(resourcePayload{ResourceGroup: newRGName, Name: name, Resource: normalizeResource(newRes)})
		if err != nil {
			return nil, err
		}
		changes = append(changes, entityChange{
			Kind: models.KindResource, OldName: name, NewName: name, Operation: models.OpUpdate,
			Before: before, After: after, ResourceTopologyID: resolveResourceID(newRes.ID, name),
		})
	}
	for name, newRes := range new {
		if seen[name] {
			continue
		}
		after, err := json.Marshal(resourcePayload{ResourceGroup: newRGName, Name: name, Resource: normalizeResource(newRes)})
		if err != nil {
			return nil, err
		}
		changes = append(changes, entityChange{
			Kind: models.KindResource, NewName: name, Operation: models.OpCreate,
			After: after, ResourceTopologyID: resolveResourceID(newRes.ID, name),
		})
	}
	return changes, nil
}

// resolveResourceID mirrors topology.UpsertResource's own nil-ID fallback
// (internal/topology/persist.go) exactly, so a historical resource change
// always lands on the same topology_id the live snapshot importer assigned
// that resource: an explicit YAML ID wins; otherwise it's GenID(name), v1's
// own deterministic name-hash convention.
func resolveResourceID(id *int64, name string) int64 {
	if id != nil {
		return *id
	}
	return topology.GenID(name)
}

// oneChange is a small helper so each decompose function's marshal-then-
// wrap boilerplate is one line instead of three.
func oneChange(kind, oldName, newName, op string, before, after json.RawMessage, marshalErr error) ([]entityChange, error) {
	if marshalErr != nil {
		return nil, marshalErr
	}
	return []entityChange{{Kind: kind, OldName: oldName, NewName: newName, Operation: op, Before: before, After: after}}, nil
}
