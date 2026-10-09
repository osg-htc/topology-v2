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
	"github.com/bbockelm/topology-v2/internal/xmlapi"
)

type voEnv struct {
	ctx context.Context
	q   *db.Queries
	h   *Handler
	uid string
}

func newVOEnv(t *testing.T) *voEnv {
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
	return &voEnv{ctx: ctx, q: q, h: &Handler{queries: q}, uid: uid}
}

func (e *voEnv) apply(op, target, state string) error {
	return e.h.applyVOProposal(e.ctx, e.q, &models.Proposal{
		EntityKind: models.KindVO, Operation: op, TargetName: target, ProposedState: []byte(state),
	}, e.uid)
}

func (e *voEnv) person(t *testing.T, name, legacyID string) {
	t.Helper()
	if _, err := e.q.CreateUser(e.ctx, db.CreateUserParams{DisplayName: name, Status: "active", LegacyContactID: legacyID, IsProvisioned: true}); err != nil {
		t.Fatalf("CreateUser(%s): %v", name, err)
	}
}

func (e *voEnv) doc(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	row, err := e.q.GetVO(e.ctx, name)
	if err != nil {
		t.Fatalf("GetVO(%s): %v", name, err)
	}
	d, err := topology.VODocFromRaw(row.Raw)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return d
}

// VOs had no write path at all; only an admin "Import from GitHub" resync
// could create or change one.
func TestVOProposal_CreateAndPublicFeed(t *testing.T) {
	e := newVOEnv(t)
	e.person(t, "Ada Admin", "ada123")
	if err := e.apply(models.OpCreate, "", `{"name":"RegtestVO","vo":{
		"LongName":"Regtest Virtual Organization","Community":"Testing","AppDescription":"for tests",
		"CertificateOnly":false,"PrimaryURL":"http://example.org","Active":true,
		"Contacts":{"Administrative Contact":[{"ID":"ada123","Name":"Ada Admin"}]},
		"FieldsOfScience":{"PrimaryFields":["Physics"]},"ReportingGroups":[]}}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	row, err := e.q.GetVO(e.ctx, "RegtestVO")
	if err != nil {
		t.Fatal(err)
	}
	if row.VOID != topology.GenID("RegtestVO") || row.Disable {
		t.Fatalf("row = %+v (id should be the name hash, not disabled)", row)
	}
	// The edit must show up in the legacy /vosummary feed, the thing that matters.
	sum, err := xmlapi.BuildVOSummary(e.ctx, e.q, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.VOs) != 1 || sum.VOs[0].LongName != "Regtest Virtual Organization" || !sum.VOs[0].Active {
		t.Fatalf("vosummary = %+v", sum.VOs)
	}
	// A duplicate create is refused.
	if err := e.apply(models.OpCreate, "", `{"name":"RegtestVO","vo":{"LongName":"x"}}`); err == nil {
		t.Fatalf("duplicate create must fail")
	}
	// A bad name is refused.
	if err := e.apply(models.OpCreate, "", `{"name":"bad name/../x","vo":{}}`); err == nil {
		t.Fatalf("invalid name must be refused")
	}
}

const seededVO = `# the short description
LongName: Seeded VO
Community: Seeded
AppDescription: seeded
CertificateOnly: false
Contacts:
  Administrative Contact:
  - ID: legacy-not-a-user
    Name: Legacy Person
OASIS:
  UseOASIS: true
  Managers:
  - Name: Mgr
    DNs:
    - /DC=org/CN=mgr
Credentials:
  TokenIssuers:
  - URL: https://issuer.example.org
    DefaultUnixUser: seeded
ID: 777
`

func (e *voEnv) seed(t *testing.T, name, raw string) {
	t.Helper()
	id, disable := topology.ParseVOHead([]byte(raw))
	voID := topology.GenID(name)
	if id != nil {
		voID = *id
	}
	if err := e.q.UpsertVO(e.ctx, name, voID, disable, []byte(raw)); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// The silent-wipe class from Projects, preempted: an edit form with no control
// for OASIS / Credentials must not delete them. The snapshot carries the whole
// document and the write-time merge fills in whatever the submission omits.
func TestVOProposal_EditPreservesKeysTheFormHasNoControlFor(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "SeededVO", seededVO)

	base := e.h.snapshotVOState(e.ctx, "SeededVO")
	incoming := json.RawMessage(`{"name":"SeededVO","vo":{"LongName":"Edited Long Name"}}`)
	merged := mergeProposedState(models.KindVO, base, incoming)
	if err := e.apply(models.OpUpdate, "SeededVO", string(merged)); err != nil {
		t.Fatalf("update: %v", err)
	}
	d := e.doc(t, "SeededVO")
	if d["LongName"] != "Edited Long Name" {
		t.Fatalf("LongName = %v", d["LongName"])
	}
	for _, k := range []string{"OASIS", "Credentials", "Contacts", "Community"} {
		if _, ok := d[k]; !ok {
			t.Errorf("edit wiped %s", k)
		}
	}
	row, _ := e.q.GetVO(e.ctx, "SeededVO")
	if !strings.Contains(string(row.Raw), "# the short description") {
		t.Errorf("comment lost from the stored YAML:\n%s", row.Raw)
	}
	if row.VOID != 777 {
		t.Errorf("id changed to %d", row.VOID)
	}
}

func TestVOProposal_NullRemovesKey_NameAndIDAreImmutable(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "SeededVO", seededVO)
	base := e.h.snapshotVOState(e.ctx, "SeededVO")

	// null removes; an attempt to change the ID is ignored.
	merged := mergeProposedState(models.KindVO, base, json.RawMessage(`{"name":"SeededVO","vo":{"Community":null,"ID":999}}`))
	if err := e.apply(models.OpUpdate, "SeededVO", string(merged)); err != nil {
		t.Fatalf("update: %v", err)
	}
	d := e.doc(t, "SeededVO")
	if _, ok := d["Community"]; ok {
		t.Errorf("Community should have been removed")
	}
	if id, _ := e.q.GetVO(e.ctx, "SeededVO"); id.VOID != 777 || d["ID"] != 777 {
		t.Errorf("ID must not change: row=%d doc=%v", id.VOID, d["ID"])
	}
	// A rename is refused.
	if err := e.apply(models.OpUpdate, "SeededVO", `{"name":"Other","vo":{}}`); err == nil || !strings.Contains(err.Error(), "cannot be changed") {
		t.Fatalf("rename must be refused, got %v", err)
	}
}

// Only contacts the edit introduces are verified: 102 of the 158 contact IDs in
// the real VO files match no user, so verifying the whole list would make most
// VOs uneditable.
func TestVOProposal_ContactVerificationCoversOnlyNewContacts(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "SeededVO", seededVO)
	e.person(t, "New Person", "newperson1")
	base := e.h.snapshotVOState(e.ctx, "SeededVO")

	// Unchanged legacy contact + a new unresolvable one: refused.
	bad := mergeProposedState(models.KindVO, base, json.RawMessage(`{"name":"SeededVO","vo":{"Contacts":{
		"Administrative Contact":[{"ID":"legacy-not-a-user","Name":"Legacy Person"},{"ID":"ghost","Name":"Ghost"}]}}}`))
	if err := e.apply(models.OpUpdate, "SeededVO", string(bad)); err == nil || !strings.Contains(err.Error(), "not linked to a known person") {
		t.Fatalf("an unresolvable new contact must be refused, got %v", err)
	}
	// Unchanged legacy contact + a new real person: allowed, legacy one kept.
	ok := mergeProposedState(models.KindVO, base, json.RawMessage(`{"name":"SeededVO","vo":{"Contacts":{
		"Administrative Contact":[{"ID":"legacy-not-a-user","Name":"Legacy Person"},{"ID":"newperson1","Name":"New Person"}]}}}`))
	if err := e.apply(models.OpUpdate, "SeededVO", string(ok)); err != nil {
		t.Fatalf("legacy contact kept + real new contact should pass: %v", err)
	}
	d := e.doc(t, "SeededVO")
	list := d["Contacts"].(map[string]interface{})["Administrative Contact"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("contacts = %v", list)
	}
	// A malformed Contacts shape is refused outright.
	if err := e.apply(models.OpUpdate, "SeededVO", `{"name":"SeededVO","vo":{"Contacts":{"Administrative Contact":"nope"}}}`); err == nil {
		t.Fatalf("malformed Contacts must be refused")
	}
}

func TestVOProposal_ParentVORules(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "ParentVO1", "LongName: Parent\nID: 500\n")
	e.seed(t, "ChildVO1", "LongName: Child\n")

	// Setting a real parent rewrites to {ID, Name} with the parent's real ID.
	if err := e.apply(models.OpUpdate, "ChildVO1", `{"name":"ChildVO1","vo":{"LongName":"Child","ParentVO":{"Name":"ParentVO1"}}}`); err != nil {
		t.Fatalf("set parent: %v", err)
	}
	pv := e.doc(t, "ChildVO1")["ParentVO"].(map[string]interface{})
	if pv["Name"] != "ParentVO1" || pv["ID"] != 500 {
		t.Fatalf("ParentVO = %v, want {ID:500 Name:ParentVO1}", pv)
	}
	for name, vo := range map[string]string{
		"missing parent": `{"name":"ChildVO1","vo":{"ParentVO":{"Name":"NoSuchVO"}}}`,
		"self parent":    `{"name":"ChildVO1","vo":{"ParentVO":{"Name":"ChildVO1"}}}`,
	} {
		if err := e.apply(models.OpUpdate, "ChildVO1", vo); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	// ParentVO1 -> ChildVO1 -> ParentVO1 would be a cycle.
	err := e.apply(models.OpUpdate, "ParentVO1", `{"name":"ParentVO1","vo":{"ParentVO":{"Name":"ChildVO1"}}}`)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("a parent cycle must be refused, got %v", err)
	}
}

// Same orphan hazard as deleting a Facility with sites.
func TestVOProposal_DeleteGuards(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "BusyVO", "LongName: Busy\n")
	e.seed(t, "KidVO", "LongName: Kid\nParentVO:\n  ID: 1\n  Name: BusyVO\n")
	e.seed(t, "FreeVO", "LongName: Free\n")

	fac, _ := e.q.InsertFacility(e.ctx, db.FacilityRow{TopologyID: 900002000, Name: "vo-fac", IDExplicit: true})
	site, _ := e.q.InsertSite(e.ctx, db.SiteRow{TopologyID: 900002001, FacilityID: fac, Name: "vo-site", IDExplicit: true})
	rg, _ := e.q.InsertResourceGroup(e.ctx, db.ResourceGroupRow{GroupID: 900002002, SiteID: site, Name: "vo-rg", IDExplicit: true})
	if err := e.q.InsertResource(e.ctx, db.ResourceRow{TopologyID: 900002003, ResourceGroupID: rg, Name: "vo-res", AllowedVOs: []string{"BusyVO"}}); err != nil {
		t.Fatalf("InsertResource: %v", err)
	}
	if err := e.q.UpsertProject(e.ctx, db.ProjectRow{Name: "vo-proj", SponsorType: "VirtualOrganization", SponsorName: "BusyVO"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	err := e.apply(models.OpDelete, "BusyVO", ``)
	if err == nil {
		t.Fatalf("delete of an in-use VO must be blocked")
	}
	for _, want := range []string{"1 resource(s) allow it", "1 project(s) are sponsored by it", "1 VO(s) have it as their parent"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
	if _, err := e.q.GetVO(e.ctx, "BusyVO"); err != nil {
		t.Fatalf("BusyVO was deleted despite being in use")
	}
	if err := e.apply(models.OpDelete, "FreeVO", ``); err != nil {
		t.Fatalf("an unused VO must delete: %v", err)
	}
	if _, err := e.q.GetVO(e.ctx, "FreeVO"); err == nil {
		t.Fatalf("FreeVO still present")
	}
}

func voReq(path, name string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", name)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// The public detail must not expose what the public feed withholds (contact
// IDs, OASIS manager DNs); the authenticated document the form needs does.
func TestVOBrowse_PublicViewIsRedactedButDocumentIsFull(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "SeededVO", seededVO)

	rec := httptest.NewRecorder()
	e.h.VODetailHandler(rec, voReq("/api/v1/vos/SeededVO", "SeededVO"))
	pub := rec.Body.String()
	for _, secret := range []string{"legacy-not-a-user", "/DC=org/CN=mgr"} {
		if strings.Contains(pub, secret) {
			t.Errorf("public detail leaked %q: %s", secret, pub)
		}
	}
	if !strings.Contains(pub, "Legacy Person") || !strings.Contains(pub, "issuer.example.org") {
		t.Errorf("public detail should still show names and public credentials: %s", pub)
	}

	rec = httptest.NewRecorder()
	e.h.VODocumentHandler(rec, voReq("/api/v1/vos/SeededVO/document", "SeededVO"))
	full := rec.Body.String()
	for _, want := range []string{"legacy-not-a-user", "/DC=org/CN=mgr"} {
		if !strings.Contains(full, want) {
			t.Errorf("full document missing %q: %s", want, full)
		}
	}

	rec = httptest.NewRecorder()
	e.h.ListVOsHandler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/vos", nil))
	var list []voListRow
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 || list[0].Name != "SeededVO" || !list[0].Active {
		t.Fatalf("list = %s (%v)", rec.Body.String(), err)
	}
}

// Against all 122 real VOs: a no-op edit (the form's snapshot submitted back
// unchanged) must leave every VO's document and its /vosummary rendering
// exactly as they were. This is what makes the edit path safe to ship: it
// proves the YAML patcher, the merge and the apply step don't disturb data they
// weren't asked to touch.
func TestVOProposal_NoOpEditLeavesEveryRealVOUntouched(t *testing.T) {
	root := os.Getenv("TOPOLOGY_TEST_REAL_TREE")
	if root == "" {
		t.Skip("set TOPOLOGY_TEST_REAL_TREE (and TOPOLOGY_TEST_DATABASE_URL) to run this test")
	}
	e := newVOEnv(t)
	voDir := root + "/../virtual-organizations"
	if err := topology.ImportVOs(e.ctx, e.q, voDir); err != nil {
		t.Fatalf("ImportVOs: %v", err)
	}
	if err := topology.ImportReportingGroups(e.ctx, e.q, voDir); err != nil {
		t.Fatalf("ImportReportingGroups: %v", err)
	}
	vos, err := e.q.ListVOs(e.ctx)
	if err != nil || len(vos) < 100 {
		t.Skipf("no real VOs under %s (%d, %v)", voDir, len(vos), err)
	}
	beforeXML, err := voSummaryJSON(e)
	if err != nil {
		t.Fatal(err)
	}
	beforeDocs := map[string]map[string]interface{}{}
	for _, v := range vos {
		beforeDocs[v.Name] = e.doc(t, v.Name)
	}

	for _, v := range vos {
		base := e.h.snapshotVOState(e.ctx, v.Name)
		merged := mergeProposedState(models.KindVO, base, base)
		if err := e.apply(models.OpUpdate, v.Name, string(merged)); err != nil {
			t.Errorf("%s: no-op update failed: %v", v.Name, err)
		}
	}
	for _, v := range vos {
		after := e.doc(t, v.Name)
		if a, b := canonicalJSON(t, after), canonicalJSON(t, beforeDocs[v.Name]); a != b {
			t.Errorf("%s: document changed by a no-op edit:\n before %s\n after  %s", v.Name, b, a)
		}
		row, _ := e.q.GetVO(e.ctx, v.Name)
		if row.VOID != v.VOID || row.Disable != v.Disable {
			t.Errorf("%s: id/disable changed (%d,%v) -> (%d,%v)", v.Name, v.VOID, v.Disable, row.VOID, row.Disable)
		}
	}
	afterXML, err := voSummaryJSON(e)
	if err != nil {
		t.Fatal(err)
	}
	if beforeXML != afterXML {
		t.Errorf("/vosummary/xml changed after no-op edits of all %d VOs", len(vos))
	}
	t.Logf("%d real VOs: no-op edits left every document and the vosummary feed unchanged", len(vos))
}

func canonicalJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func voSummaryJSON(e *voEnv) (string, error) {
	sum, err := xmlapi.BuildVOSummary(e.ctx, e.q, true)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(sum)
	return string(b), err
}

// Found by the real-data no-op test: 11 of 122 real VOs carry explicit nulls
// (AppDescription: null, FieldsOfScience: null). Null means "remove" only for a
// key that has a value, so an edit that doesn't touch those keys leaves them.
func TestVOProposal_ExplicitNullsInTheFileSurviveAnEdit(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "NullsVO", "LongName: Nulls\nAppDescription: null\nFieldsOfScience: null\nCommunity: has value\n")
	base := e.h.snapshotVOState(e.ctx, "NullsVO")
	merged := mergeProposedState(models.KindVO, base, json.RawMessage(`{"name":"NullsVO","vo":{"LongName":"Nulls edited","Community":null}}`))
	if err := e.apply(models.OpUpdate, "NullsVO", string(merged)); err != nil {
		t.Fatalf("update: %v", err)
	}
	d := e.doc(t, "NullsVO")
	for _, k := range []string{"AppDescription", "FieldsOfScience"} {
		v, ok := d[k]
		if !ok || v != nil {
			t.Errorf("%s: explicit null must be kept, got present=%v value=%v", k, ok, v)
		}
	}
	if _, ok := d["Community"]; ok {
		t.Errorf("Community had a value and was set to null, so it should be removed")
	}
}

// Found by the real-data no-op test: CLAS12 names a ParentVO (JLAB) that is not
// a VO in the data. An edit that leaves that parent alone must not be blocked by
// it; choosing a different missing parent still is.
func TestVOProposal_DanglingUnchangedParentDoesNotBlockEdits(t *testing.T) {
	e := newVOEnv(t)
	e.seed(t, "DanglingVO", "LongName: Dangling\nParentVO:\n  ID: 42\n  Name: GoneVO\n")
	base := e.h.snapshotVOState(e.ctx, "DanglingVO")

	merged := mergeProposedState(models.KindVO, base, json.RawMessage(`{"name":"DanglingVO","vo":{"LongName":"Dangling edited"}}`))
	if err := e.apply(models.OpUpdate, "DanglingVO", string(merged)); err != nil {
		t.Fatalf("an unrelated edit must not be blocked by a legacy dangling parent: %v", err)
	}
	pv := e.doc(t, "DanglingVO")["ParentVO"].(map[string]interface{})
	if pv["Name"] != "GoneVO" || pv["ID"] != 42 {
		t.Fatalf("legacy ParentVO was rewritten: %v", pv)
	}
	// The form submits ParentVO as just {Name}; the stored ID must not be lost.
	form := mergeProposedState(models.KindVO, base, json.RawMessage(`{"name":"DanglingVO","vo":{"ParentVO":{"Name":"GoneVO"}}}`))
	if err := e.apply(models.OpUpdate, "DanglingVO", string(form)); err != nil {
		t.Fatalf("form-shaped resubmit: %v", err)
	}
	if pv2 := e.doc(t, "DanglingVO")["ParentVO"].(map[string]interface{}); pv2["ID"] != 42 || pv2["Name"] != "GoneVO" {
		t.Fatalf("stored ParentVO ID lost when the form resubmitted {Name} only: %v", pv2)
	}
	other := mergeProposedState(models.KindVO, base, json.RawMessage(`{"name":"DanglingVO","vo":{"ParentVO":{"Name":"AlsoGoneVO"}}}`))
	if err := e.apply(models.OpUpdate, "DanglingVO", string(other)); err == nil {
		t.Fatalf("choosing a different, missing parent must still be refused")
	}
}
