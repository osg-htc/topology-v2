package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestSupportCenterNamesHandler backs the resource-group form's Support center
// picklist. v1 hard-fails (KeyError) when a resource group names a support
// center that doesn't exist, so the form offers exactly the known names.
func TestSupportCenterNamesHandler(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	// Empty registry must serialize as [] (not null) so the UI can map over it.
	rec := httptest.NewRecorder()
	h.SupportCenterNamesHandler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/support-center-names", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "[]\n" && got != "[]" {
		t.Fatalf("empty registry body = %q, want []", got)
	}

	for i, n := range []string{"Self Supported", "Community Support Center"} {
		if err := q.UpsertSupportCenter(ctx, int64(1000+i), n, "", "", ""); err != nil {
			t.Fatalf("UpsertSupportCenter(%q): %v", n, err)
		}
	}
	rec = httptest.NewRecorder()
	h.SupportCenterNamesHandler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/support-center-names", nil))
	var got []string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	want := []string{"Community Support Center", "Self Supported"} // ordered by name
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}
