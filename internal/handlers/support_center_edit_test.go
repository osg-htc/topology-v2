package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
)

type scEnv struct {
	ctx context.Context
	q   *db.Queries
	h   *Handler
	uid string
}

func newSCEnv(t *testing.T) *scEnv {
	t.Helper()
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	uid, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-actor", Status: "active", IsProvisioned: true})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return &scEnv{ctx: ctx, q: q, h: &Handler{queries: q}, uid: uid}
}

func (e *scEnv) apply(t *testing.T, op, target, state string) error {
	t.Helper()
	return e.h.applySupportCenterProposal(e.ctx, e.q, &models.Proposal{
		EntityKind: models.KindSupportCenter, Operation: op, TargetName: target, ProposedState: []byte(state),
	}, e.uid)
}

func (e *scEnv) addRG(t *testing.T, name, supportCenter string, gid int64) {
	t.Helper()
	fac, err := e.q.InsertFacility(e.ctx, db.FacilityRow{TopologyID: gid, Name: name + "-fac", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	site, err := e.q.InsertSite(e.ctx, db.SiteRow{TopologyID: gid + 1, FacilityID: fac, Name: name + "-site", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertSite: %v", err)
	}
	if _, err := e.q.InsertResourceGroup(e.ctx, db.ResourceGroupRow{GroupID: gid + 2, SiteID: site, Name: name, SupportCenter: supportCenter, IDExplicit: true}); err != nil {
		t.Fatalf("InsertResourceGroup: %v", err)
	}
}

// Support centers had no write path at all; the only way to create or change
// one was a full "Import from GitHub" resync. This covers the new one.
func TestSupportCenterProposal_CreateAndUpdate(t *testing.T) {
	e := newSCEnv(t)
	if err := e.apply(t, models.OpCreate, "", `{"name":"regtest-sc","long_name":"Regtest SC","community":"Testing","description":"first"}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := e.q.GetSupportCenter(e.ctx, "regtest-sc")
	if err != nil {
		t.Fatalf("GetSupportCenter: %v", err)
	}
	if got.ID != topology.GenID("regtest-sc") || got.LongName != "Regtest SC" || got.Community != "Testing" || got.Description != "first" {
		t.Fatalf("created row = %+v", got)
	}

	// Update keeps the id, even though the name changes.
	if err := e.apply(t, models.OpUpdate, "regtest-sc", `{"name":"regtest-sc","long_name":"Renamed Long","community":"Testing","description":"second"}`); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := e.q.GetSupportCenter(e.ctx, "regtest-sc")
	if got2.ID != got.ID || got2.LongName != "Renamed Long" || got2.Description != "second" {
		t.Fatalf("updated row = %+v (id must not change)", got2)
	}

	// A duplicate create is rejected, not silently merged.
	if err := e.apply(t, models.OpCreate, "", `{"name":"regtest-sc"}`); err == nil {
		t.Fatalf("creating a duplicate name must fail")
	}
}

// A rename must carry the resource groups along: they reference a center by
// name, so without a repoint they'd silently orphan.
func TestSupportCenterProposal_RenameRepointsResourceGroups(t *testing.T) {
	e := newSCEnv(t)
	if err := e.apply(t, models.OpCreate, "", `{"name":"regtest-old"}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	e.addRG(t, "regtest-rg-using", "regtest-old", 900001000)

	if err := e.apply(t, models.OpUpdate, "regtest-old", `{"name":"regtest-new"}`); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := e.q.GetSupportCenter(e.ctx, "regtest-old"); err == nil {
		t.Fatalf("old name should be gone")
	}
	if n, _ := e.q.RGNamesUsingSupportCenter(e.ctx, "regtest-new"); len(n) != 1 || n[0] != "regtest-rg-using" {
		t.Fatalf("resource group not repointed to the new name: %v", n)
	}
	if n, _ := e.q.RGNamesUsingSupportCenter(e.ctx, "regtest-old"); len(n) != 0 {
		t.Fatalf("resource group still points at the old name: %v", n)
	}
}

// Same orphan hazard as deleting a Facility/Site with live children.
func TestSupportCenterProposal_DeleteBlockedByLiveResourceGroup(t *testing.T) {
	e := newSCEnv(t)
	if err := e.apply(t, models.OpCreate, "", `{"name":"regtest-busy"}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	e.addRG(t, "regtest-rg-busy", "regtest-busy", 900001100)

	err := e.apply(t, models.OpDelete, "regtest-busy", ``)
	if err == nil || !strings.Contains(err.Error(), "still use this support center") {
		t.Fatalf("delete of an in-use center must be blocked, got: %v", err)
	}
	if _, err := e.q.GetSupportCenter(e.ctx, "regtest-busy"); err != nil {
		t.Fatalf("center was deleted despite being in use: %v", err)
	}
}

// Delete is a soft delete, and the name can be reused afterwards (ids derive
// from the name, so the re-create must revive the row, not hit the primary key).
func TestSupportCenterProposal_DeleteThenRecreate(t *testing.T) {
	e := newSCEnv(t)
	if err := e.apply(t, models.OpCreate, "", `{"name":"regtest-gone","description":"v1"}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := e.apply(t, models.OpDelete, "regtest-gone", ``); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := e.q.GetSupportCenter(e.ctx, "regtest-gone"); err == nil {
		t.Fatalf("deleted center still visible")
	}
	if names, _ := e.q.ListAllSupportCenters(e.ctx); len(names) != 0 {
		t.Fatalf("deleted center still listed: %+v", names)
	}
	if err := e.apply(t, models.OpCreate, "", `{"name":"regtest-gone","description":"v2"}`); err != nil {
		t.Fatalf("re-create after delete: %v", err)
	}
	got, err := e.q.GetSupportCenter(e.ctx, "regtest-gone")
	if err != nil || got.Description != "v2" {
		t.Fatalf("revived center = %+v, %v", got, err)
	}
}

// The silent-wipe class from Projects, preempted: an edit form with no control
// for Contacts must not delete them. The snapshot carries Extra and the
// write-time merge fills it in when the submission omits it.
func TestSupportCenterProposal_EditPreservesContactsExtra(t *testing.T) {
	e := newSCEnv(t)
	if err := e.q.UpsertSupportCenterFull(e.ctx, db.SupportCenterFull{
		ID: 92, Name: "regtest-contacts", Community: "c",
		Extra: []byte(`{"Contacts":{"Security Contact":[{"ID":"abc","Name":"A Person"}]}}`),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	base := e.h.snapshotSupportCenterState(e.ctx, "regtest-contacts")
	if !strings.Contains(string(base), "A Person") {
		t.Fatalf("snapshot must carry the Contacts extra, got %s", base)
	}
	incoming := json.RawMessage(`{"name":"regtest-contacts","long_name":"","community":"c","description":"edited"}`)
	merged := mergeProposedState(models.KindSupportCenter, base, incoming)
	if err := e.apply(t, models.OpUpdate, "regtest-contacts", string(merged)); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := e.q.GetSupportCenter(e.ctx, "regtest-contacts")
	if got.Description != "edited" || !strings.Contains(string(got.Extra), "A Person") {
		t.Fatalf("edit wiped Contacts: %+v extra=%s", got, got.Extra)
	}
	if got.ID != 92 {
		t.Fatalf("id changed to %d", got.ID)
	}
}

func TestSupportCenterBrowseHandlers(t *testing.T) {
	e := newSCEnv(t)
	if err := e.apply(t, models.OpCreate, "", `{"name":"regtest-b","community":"X"}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	e.addRG(t, "regtest-rg-b", "regtest-b", 900001200)

	rec := httptest.NewRecorder()
	e.h.ListSupportCentersHandler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/support-centers", nil))
	var list []db.SupportCenterBrowseRow
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 || list[0].Name != "regtest-b" || list[0].ResourceGroups != 1 {
		t.Fatalf("list = %s (%v)", rec.Body.String(), err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/support-centers/regtest-b", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "regtest-b")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec = httptest.NewRecorder()
	e.h.SupportCenterDetailHandler(rec, req)
	var detail struct {
		Name           string   `json:"name"`
		ResourceGroups []string `json:"resource_groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil || detail.Name != "regtest-b" || len(detail.ResourceGroups) != 1 {
		t.Fatalf("detail = %s (%v)", rec.Body.String(), err)
	}
}
