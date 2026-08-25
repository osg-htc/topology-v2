// package db_test: testsupport imports internal/db, so a same-package test
// file can't import testsupport without an import cycle.
package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/testsupport"
)

func newTestUser(t *testing.T, ctx context.Context, q *db.Queries) string {
	t.Helper()
	id, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-user", Status: "active", IsProvisioned: true})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return id
}

// TestConfirmEmailVerification covers the real correctness logic in
// ConfirmEmailVerification/UpsertEmailVerification (queries_email.go): a
// valid token confirms exactly once (no replay), an unknown token is
// rejected, an expired token is rejected even though it was never used, and
// re-requesting verification for the same (user, email) clears any prior
// verified state so the user must re-confirm.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise.
func TestConfirmEmailVerification(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	userID := newTestUser(t, ctx, q)

	t.Run("a valid token confirms once, and cannot be replayed", func(t *testing.T) {
		token := []byte("token-valid-once")
		if err := q.UpsertEmailVerification(ctx, db.UpsertEmailVerificationParams{
			UserID: userID, EmailSHA1: "sha1-a", EmailHint: "a***@example.org",
			TokenHash: token, ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("UpsertEmailVerification: %v", err)
		}

		hint, ok := q.ConfirmEmailVerification(ctx, token)
		if !ok || hint != "a***@example.org" {
			t.Fatalf("first confirm: got (%q, %v), want (a***@example.org, true)", hint, ok)
		}

		if _, ok := q.ConfirmEmailVerification(ctx, token); ok {
			t.Error("second confirm with the same token succeeded -- a token must not be usable twice")
		}
	})

	t.Run("an unknown token is rejected", func(t *testing.T) {
		if _, ok := q.ConfirmEmailVerification(ctx, []byte("never-issued")); ok {
			t.Error("confirming an unknown token succeeded")
		}
	})

	t.Run("an expired token is rejected even though never used", func(t *testing.T) {
		token := []byte("token-expired")
		if err := q.UpsertEmailVerification(ctx, db.UpsertEmailVerificationParams{
			UserID: userID, EmailSHA1: "sha1-b", EmailHint: "b***@example.org",
			TokenHash: token, ExpiresAt: time.Now().Add(-time.Hour), // already expired
		}); err != nil {
			t.Fatalf("UpsertEmailVerification: %v", err)
		}
		if _, ok := q.ConfirmEmailVerification(ctx, token); ok {
			t.Error("confirming an expired token succeeded")
		}
	})

	t.Run("re-requesting resets verified state and invalidates the old token", func(t *testing.T) {
		oldToken := []byte("token-c-first")
		if err := q.UpsertEmailVerification(ctx, db.UpsertEmailVerificationParams{
			UserID: userID, EmailSHA1: "sha1-c", EmailHint: "c***@example.org",
			TokenHash: oldToken, ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("UpsertEmailVerification (first request): %v", err)
		}
		if _, ok := q.ConfirmEmailVerification(ctx, oldToken); !ok {
			t.Fatalf("confirming the first token failed")
		}

		newToken := []byte("token-c-second")
		if err := q.UpsertEmailVerification(ctx, db.UpsertEmailVerificationParams{
			UserID: userID, EmailSHA1: "sha1-c", EmailHint: "c***@example.org",
			TokenHash: newToken, ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("UpsertEmailVerification (re-request): %v", err)
		}

		list, err := q.ListEmailVerifications(ctx, userID)
		if err != nil {
			t.Fatalf("ListEmailVerifications: %v", err)
		}
		var found bool
		for _, e := range list {
			if e.EmailHint == "c***@example.org" {
				found = true
				if e.Verified {
					t.Error("re-requesting verification for an already-verified email left it marked verified -- the user could skip re-confirming")
				}
			}
		}
		if !found {
			t.Fatal("re-requested email not found in ListEmailVerifications")
		}

		if _, ok := q.ConfirmEmailVerification(ctx, oldToken); ok {
			t.Error("the old token still confirmed after a re-request -- it should have been superseded")
		}
		if _, ok := q.ConfirmEmailVerification(ctx, newToken); !ok {
			t.Error("the new token from the re-request did not confirm")
		}
	})
}
