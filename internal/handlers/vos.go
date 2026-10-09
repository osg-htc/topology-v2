package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/bbockelm/topology-v2/internal/conv"
	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// voProposal is a VO's proposed_state. VO is the whole document -- the VO's
// top-level keys exactly as they appear in its YAML file -- one level down, the
// same wrapping resources use, because VO files have their own keys (8 real
// ones even have a "Name") that must not collide with the envelope's.
//
// A key whose value is null means "remove it"; a key that is absent means
// "leave it as it is" (see mergeProposedState), which is what keeps an edit
// form with no control for OASIS, Credentials or DataFederations from deleting
// them.
type voProposal struct {
	Name string          `json:"name"`
	VO   json.RawMessage `json:"vo"`
}

// voNameRE is what a VO's file name may be: every one of the 122 real VOs fits.
var voNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (h *Handler) applyVOProposal(ctx context.Context, q *db.Queries, p *models.Proposal, actorID string) error {
	if p.Operation == models.OpDelete {
		return h.deleteVO(ctx, q, p.TargetName, actorID)
	}
	var vp voProposal
	if err := json.Unmarshal(p.ProposedState, &vp); err != nil {
		return err
	}
	if vp.Name == "" {
		return errors.New("a VO requires a name")
	}
	doc, err := topology.NormalizeVODoc(vp.VO)
	if err != nil {
		return fmt.Errorf("invalid VO document: %w", err)
	}
	// A null value means "remove this key" -- but only for a key that has a
	// real value to remove. Real files carry explicit nulls (AppDescription:
	// null, FieldsOfScience: null, on 11 of 122 VOs); leaving those exactly as
	// they are is what lets an edit that doesn't touch them be a true no-op.
	var curDoc map[string]interface{}
	var curRow *db.VORow
	if p.Operation == models.OpUpdate {
		if cur, err := q.GetVO(ctx, p.TargetName); err == nil {
			curRow = cur
			curDoc, err = topology.VODocFromRaw(cur.Raw)
			if err != nil {
				return err
			}
		}
	}
	for k, v := range doc {
		if v != nil {
			continue
		}
		if had, ok := curDoc[k]; ok && had == nil {
			continue // an explicit null the file already has: leave it
		}
		delete(doc, k)
	}

	if p.Operation == models.OpUpdate {
		if vp.Name != p.TargetName {
			return errors.New("a VO's name is its file name and cannot be changed; create a new VO instead")
		}
		if curRow == nil {
			return fmt.Errorf("VO %q not found", p.TargetName)
		}
		cur := curRow
		// A VO's ID is assigned once and never edited: resources, projects and
		// the public feeds identify it by it.
		if id, ok := curDoc["ID"]; ok {
			doc["ID"] = id
		} else {
			delete(doc, "ID")
		}
		if err := h.checkVODoc(ctx, q, vp.Name, doc, curDoc); err != nil {
			return err
		}
		raw, err := topology.PatchVOYAML(cur.Raw, doc)
		if err != nil {
			return err
		}
		id, disable := topology.ParseVOHead(raw)
		return q.UpdateVOFields(ctx, p.TargetName, topology.IDOrGen(id, vp.Name), disable, raw)
	}

	// Create.
	if !voNameRE.MatchString(vp.Name) {
		return fmt.Errorf("invalid VO name %q: use letters, digits, '_', '-' or '.', starting with a letter or digit", vp.Name)
	}
	if err := h.checkVODoc(ctx, q, vp.Name, doc, nil); err != nil {
		return err
	}
	raw, err := topology.NewVOYAML(doc)
	if err != nil {
		return err
	}
	id, disable := topology.ParseVOHead(raw)
	return q.InsertVO(ctx, vp.Name, topology.IDOrGen(id, vp.Name), disable, raw)
}

// checkVODoc validates a VO document before it is written: a ParentVO must
// name a real, different VO and not create a cycle (and is rewritten to the
// {ID, Name} shape every real file uses), and contacts must be well-formed and
// -- for any person this edit adds -- resolve to a real user, the same rule
// every other entity's contacts are held to.
//
// Only contacts the edit introduces are verified. 102 of the 158 contact IDs in
// the real VO files don't match a user (they predate the app's user table), so
// verifying the whole list would make most VOs uneditable until every legacy
// contact was replaced; an unchanged legacy contact simply stays as it is.
func (h *Handler) checkVODoc(ctx context.Context, q *db.Queries, name string, doc, prev map[string]interface{}) error {
	prevParentName := ""
	if pp, ok := prev["ParentVO"].(map[string]interface{}); ok {
		prevParentName, _ = pp["Name"].(string)
	}
	if pv, ok := doc["ParentVO"]; ok {
		pm, _ := pv.(map[string]interface{})
		pname, _ := pm["Name"].(string)
		if pname == "" {
			return errors.New("ParentVO must name a VO")
		}
		if pname == prevParentName {
			// Unchanged: restore it exactly as stored -- the form submits only
			// {Name}, and the stored block carries an ID that must survive. Real
			// data has parents that no longer exist as VOs (CLAS12 -> JLAB), and
			// re-validating a reference this edit didn't touch would block every
			// other edit of that VO.
			doc["ParentVO"] = prev["ParentVO"]
			return h.requireResolvedVOContacts(ctx, q, doc["Contacts"], prev["Contacts"])
		}
		if pname == name {
			return errors.New("a VO cannot be its own parent")
		}
		parent, err := q.GetVO(ctx, pname)
		if err != nil {
			return fmt.Errorf("parent VO %q does not exist", pname)
		}
		// Walk up: reaching this VO again means the edit would close a loop.
		seen := map[string]bool{name: true, pname: true}
		for cur := parent; ; {
			cd, err := topology.VODocFromRaw(cur.Raw)
			if err != nil {
				break
			}
			up, _ := cd["ParentVO"].(map[string]interface{})
			un, _ := up["Name"].(string)
			if un == "" {
				break
			}
			if seen[un] {
				return fmt.Errorf("parent VO %q would make a cycle", pname)
			}
			seen[un] = true
			if cur, err = q.GetVO(ctx, un); err != nil {
				break
			}
		}
		doc["ParentVO"] = map[string]interface{}{"ID": parent.VOID, "Name": parent.Name}
	}
	return h.requireResolvedVOContacts(ctx, q, doc["Contacts"], prev["Contacts"])
}

// voContactPairs flattens a VO's Contacts block into (type, ID, Name) triples.
func voContactPairs(v interface{}) ([][3]string, error) {
	var out [][3]string
	if v == nil {
		return out, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.New("Contacts must be a map of contact type to a list of people")
	}
	for ctype, l := range m {
		list, ok := l.([]interface{})
		if !ok {
			return nil, fmt.Errorf("Contacts[%q] must be a list", ctype)
		}
		for _, e := range list {
			pm, ok := e.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("Contacts[%q] has an entry that is not a person", ctype)
			}
			id, _ := pm["ID"].(string)
			nm, _ := pm["Name"].(string)
			out = append(out, [3]string{ctype, id, nm})
		}
	}
	return out, nil
}

func (h *Handler) requireResolvedVOContacts(ctx context.Context, q *db.Queries, next, prev interface{}) error {
	nextPairs, err := voContactPairs(next)
	if err != nil {
		return err
	}
	prevPairs, _ := voContactPairs(prev)
	had := map[[2]string]bool{}
	for _, p := range prevPairs {
		had[[2]string{p[0], p[1]}] = true
	}
	for _, c := range nextPairs {
		if c[1] == "" && c[2] == "" {
			continue
		}
		if had[[2]string{c[0], c[1]}] {
			continue // already on this VO: not something this edit introduced
		}
		if ok, err := q.LegacyContactIDExists(ctx, c[1]); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("contact %q is not linked to a known person — pick an existing one or invite a new one", c[2])
		}
	}
	return nil
}

// deleteVO soft-deletes a VO unless something still refers to it: a resource
// that allows it, a project it sponsors, or a VO that names it as its parent.
// Each of those would be left pointing at a VO that no longer exists in the
// feeds -- the same orphan hazard as deleting a Facility with sites.
func (h *Handler) deleteVO(ctx context.Context, q *db.Queries, name, actorID string) error {
	var problems []string
	if rs, err := q.ResourceNamesAllowingVO(ctx, name); err != nil {
		return err
	} else if len(rs) > 0 {
		problems = append(problems, fmt.Sprintf("%d resource(s) allow it", len(rs)))
	}
	if ps, err := q.ProjectNamesSponsoredByVO(ctx, name); err != nil {
		return err
	} else if len(ps) > 0 {
		problems = append(problems, fmt.Sprintf("%d project(s) are sponsored by it", len(ps)))
	}
	if kids, err := childVOs(ctx, q, name); err != nil {
		return err
	} else if len(kids) > 0 {
		problems = append(problems, fmt.Sprintf("%d VO(s) have it as their parent", len(kids)))
	}
	if len(problems) > 0 {
		return fmt.Errorf("cannot delete: still in use — %s", joinAnd(problems))
	}
	return q.SoftDeleteVOByName(ctx, name, actorID)
}

func joinAnd(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "; "
		}
		out += s
	}
	return out
}

// childVOs lists live VOs whose ParentVO is name.
func childVOs(ctx context.Context, q *db.Queries, name string) ([]string, error) {
	vos, err := q.ListVOs(ctx)
	if err != nil {
		return nil, err
	}
	var kids []string
	for _, v := range vos {
		doc, err := topology.VODocFromRaw(v.Raw)
		if err != nil {
			continue
		}
		if pm, ok := doc["ParentVO"].(map[string]interface{}); ok && pm["Name"] == name {
			kids = append(kids, v.Name)
		}
	}
	return kids, nil
}

func (h *Handler) snapshotVOState(ctx context.Context, targetName string) json.RawMessage {
	row, err := h.queries.GetVO(ctx, targetName)
	if err != nil {
		return nil
	}
	doc, err := topology.VODocFromRaw(row.Raw)
	if err != nil {
		return nil
	}
	inner, err := json.Marshal(doc)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(voProposal{Name: row.Name, VO: inner})
	if err != nil {
		return nil
	}
	return b
}

// ---- read API ----------------------------------------------------------

type voListRow struct {
	Name            string `json:"name"`
	ID              int64  `json:"id"`
	LongName        string `json:"long_name"`
	Community       string `json:"community"`
	Active          bool   `json:"active"`
	Disable         bool   `json:"disable"`
	ParentVO        string `json:"parent_vo"`
	PrimaryURL      string `json:"primary_url"`
	ReportingGroups int    `json:"reporting_group_count"`
}

// ListVOsHandler lists live VOs. Public, like the rest of the browse API, so
// it carries nothing the public /vosummary feed doesn't.
func (h *Handler) ListVOsHandler(w http.ResponseWriter, r *http.Request) {
	vos, err := h.queries.ListVOs(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]voListRow, 0, len(vos))
	for _, v := range vos {
		doc, err := topology.VODocFromRaw(v.Raw)
		if err != nil {
			doc = map[string]interface{}{}
		}
		row := voListRow{
			Name: v.Name, ID: v.VOID, Active: conv.MapBool(doc, "Active", true), Disable: v.Disable,
		}
		row.LongName, _ = doc["LongName"].(string)
		row.Community, _ = doc["Community"].(string)
		row.PrimaryURL, _ = doc["PrimaryURL"].(string)
		if pm, ok := doc["ParentVO"].(map[string]interface{}); ok {
			row.ParentVO, _ = pm["Name"].(string)
		}
		if rg, ok := doc["ReportingGroups"].([]interface{}); ok {
			row.ReportingGroups = len(rg)
		}
		out = append(out, row)
	}
	respondJSON(w, http.StatusOK, out)
}

// redactVODoc returns a copy of a VO document safe to show anonymously: contact
// IDs and OASIS manager IDs/DNs removed -- what /vosummary withholds without
// includePII. Everything else is already public in that feed.
func redactVODoc(doc map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(doc))
	for k, v := range doc {
		out[k] = v
	}
	if c, ok := doc["Contacts"].(map[string]interface{}); ok {
		nc := map[string]interface{}{}
		for ctype, l := range c {
			var people []interface{}
			if list, ok := l.([]interface{}); ok {
				for _, e := range list {
					if pm, ok := e.(map[string]interface{}); ok {
						people = append(people, map[string]interface{}{"Name": pm["Name"]})
					}
				}
			}
			nc[ctype] = people
		}
		out["Contacts"] = nc
	}
	if o, ok := doc["OASIS"].(map[string]interface{}); ok {
		no := map[string]interface{}{}
		for k, v := range o {
			if k != "Managers" {
				no[k] = v
			}
		}
		if mg, ok := o["Managers"]; ok {
			no["ManagerCount"] = managerCount(mg)
		}
		out["OASIS"] = no
	}
	return out
}

func managerCount(v interface{}) int {
	switch t := v.(type) {
	case []interface{}:
		return len(t)
	case map[string]interface{}:
		return len(t)
	}
	return 0
}

// VODetailHandler returns one VO's public, redacted view plus what uses it.
func (h *Handler) VODetailHandler(w http.ResponseWriter, r *http.Request) {
	h.writeVODetail(w, r, false)
}

// VODocumentHandler returns one VO's full document, contact IDs and all, for
// the edit form. Authenticated.
func (h *Handler) VODocumentHandler(w http.ResponseWriter, r *http.Request) {
	h.writeVODetail(w, r, true)
}

func (h *Handler) writeVODetail(w http.ResponseWriter, r *http.Request, full bool) {
	ctx := r.Context()
	name := chi.URLParam(r, "name")
	row, err := h.queries.GetVO(ctx, name)
	if err != nil {
		respondError(w, http.StatusNotFound, "VO not found")
		return
	}
	doc, err := topology.VODocFromRaw(row.Raw)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !full {
		doc = redactVODoc(doc)
	}
	resources, _ := h.queries.ResourceNamesAllowingVO(ctx, name)
	projects, _ := h.queries.ProjectNamesSponsoredByVO(ctx, name)
	kids, _ := childVOs(ctx, h.queries, name)
	for _, s := range []*[]string{&resources, &projects, &kids} {
		if *s == nil {
			*s = []string{}
		}
	}
	sort.Strings(kids)
	respondJSON(w, http.StatusOK, map[string]any{
		"name": row.Name, "id": row.VOID, "disable": row.Disable,
		"vo": doc, "resources": resources, "projects": projects, "child_vos": kids,
	})
}

// ReportingGroupNamesHandler lists the shared reporting-group names, for the
// VO form's ReportingGroups picker.
func (h *Handler) ReportingGroupNamesHandler(w http.ResponseWriter, r *http.Request) {
	names, err := h.queries.ListReportingGroupNames(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, names)
}
