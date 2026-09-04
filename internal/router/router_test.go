package router

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/bbockelm/topology-v2/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	c := &config.Config{InstanceKey: hex.EncodeToString(raw)}
	if err := c.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey: %v", err)
	}
	return c
}

// TestNew_BuildsWithoutADatabase confirms router.New itself never touches
// queries at construction time (handlers.New only derives keys from the
// master key) -- wiring the route tree must not require a live database
// connection just to build.
func TestNew_BuildsWithoutADatabase(t *testing.T) {
	r, h, err := New(testConfig(t), nil, zerolog.Nop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if r == nil || h == nil {
		t.Fatal("New returned a nil router or handler alongside a nil error")
	}
}

func TestHealthz_Unauthenticated(t *testing.T) {
	r, _, err := New(testConfig(t), nil, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz = %d, want 200", rec.Code)
	}
}

// TestProtectedRoute_WithoutSessionCookie confirms RequireAuth rejects a
// request with no session cookie before ever touching the (here nil)
// database -- a route under it must never panic just because no one is
// logged in.
func TestProtectedRoute_WithoutSessionCookie(t *testing.T) {
	r, _, err := New(testConfig(t), nil, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/v1/dashboard (no cookie) = %d, want 401", rec.Code)
	}
}

// TestUnknownRoute_FallsBackToSPA confirms a client-side route the backend
// itself doesn't know about (e.g. a Next.js page) still resolves through
// the SPA fallback rather than a bare 404 -- deep-linking or refreshing on
// a frontend route depends on this.
func TestUnknownRoute_FallsBackToSPA(t *testing.T) {
	r, _, err := New(testConfig(t), nil, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/some/client/side/route", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /some/client/side/route = %d, want 200 (SPA fallback)", rec.Code)
	}
}
