package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bbockelm/topology-v2/internal/conv"
	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// supportCenterProposal is a support center's proposed_state. The id is
// deliberately absent: it's assigned once at creation and never edited, because
// resource groups and the public feeds identify a center by it.
type supportCenterProposal struct {
	Name        string `json:"name"`
	LongName    string `json:"long_name"`
	Community   string `json:"community"`
	Description string `json:"description"`
	// Extra is everything support-centers.yaml carries without a modeled field
	// (chiefly Contacts, on 40 of 52 real centers). Round-tripped only, so an
	// edit form with no UI for it can't silently delete it -- see
	// snapshotSupportCenterState and mergeProposedState.
	Extra map[string]interface{} `json:"extra,omitempty"`
}

func (h *Handler) applySupportCenterProposal(ctx context.Context, q *db.Queries, p *models.Proposal, actorID string) error {
	if p.Operation == models.OpDelete {
		// Same guard as Facility/Site/ResourceGroup delete: a center still in
		// use by live resource groups would leave them pointing at a name that
		// resolves to nothing in /rgsummary/xml.
		names, err := q.RGNamesUsingSupportCenter(ctx, p.TargetName)
		if err != nil {
			return err
		}
		if len(names) > 0 {
			return fmt.Errorf("cannot delete: %d resource group(s) still use this support center", len(names))
		}
		return q.SoftDeleteSupportCenterByName(ctx, p.TargetName, actorID)
	}
	var sp supportCenterProposal
	if err := json.Unmarshal(p.ProposedState, &sp); err != nil {
		return err
	}
	if sp.Name == "" {
		return errors.New("a support center requires a name")
	}
	row := db.SupportCenterFull{
		Name: sp.Name, LongName: sp.LongName, Community: sp.Community,
		Description: sp.Description, Extra: conv.JSONOrNil(sp.Extra),
	}
	if p.Operation == models.OpUpdate {
		if err := q.UpdateSupportCenterFields(ctx, p.TargetName, row); err != nil {
			return err
		}
		// Resource groups reference a center by name, so a rename has to carry
		// them along or they'd be orphaned.
		if sp.Name != p.TargetName {
			return q.RepointResourceGroupSupportCenter(ctx, p.TargetName, sp.Name)
		}
		return nil
	}
	// Create: the id is the same stable name hash every other app-created
	// entity gets, and must not collide with a live center's id.
	row.ID = topology.GenID(sp.Name)
	if other, err := q.SupportCenterNameByID(ctx, row.ID); err != nil {
		return err
	} else if other != "" && other != sp.Name {
		return fmt.Errorf("support center id %d is already used by %q", row.ID, other)
	}
	return q.InsertSupportCenter(ctx, row)
}

func (h *Handler) snapshotSupportCenterState(ctx context.Context, targetName string) json.RawMessage {
	row, err := h.queries.GetSupportCenter(ctx, targetName)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(supportCenterProposal{
		Name: row.Name, LongName: row.LongName, Community: row.Community,
		Description: row.Description, Extra: conv.MapFromJSON(row.Extra),
	})
	if err != nil {
		return nil
	}
	return b
}

// ListSupportCentersHandler lists live support centers with how many resource
// groups use each.
func (h *Handler) ListSupportCentersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListBrowseSupportCenters(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// SupportCenterDetailHandler returns one support center, including its
// Contacts/extra block and the resource groups that use it.
func (h *Handler) SupportCenterDetailHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	sc, err := h.queries.GetSupportCenter(r.Context(), name)
	if err != nil {
		respondError(w, http.StatusNotFound, "support center not found")
		return
	}
	rgs, err := h.queries.RGNamesUsingSupportCenter(r.Context(), name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rgs == nil {
		rgs = []string{}
	}
	out := map[string]any{
		"name": sc.Name, "id": sc.ID, "long_name": sc.LongName,
		"community": sc.Community, "description": sc.Description,
		"resource_groups": rgs,
	}
	if extra := conv.MapFromJSON(sc.Extra); extra != nil {
		out["extra"] = extra
	}
	respondJSON(w, http.StatusOK, out)
}
