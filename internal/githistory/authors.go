package githistory

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/bbockelm/topology-v2/internal/db"
)

// AuthorKey derives the users.legacy_contact_id dedup key for one commit
// author: "git-import:" + a lowercased/trimmed email, or a hashed fallback
// keyed by name when the email is empty or clearly not an email (a few very
// old commits in real history have blank or malformed author addresses).
func AuthorKey(name, email string) string {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" || !strings.Contains(e, "@") {
		sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(name))))
		return "git-import:name:" + hex.EncodeToString(sum[:])
	}
	return "git-import:" + e
}

// ResolveAuthors upserts one placeholder user per distinct (name, email)
// pair appearing anywhere in commits, in one pass, and returns a lookup from
// that pair's key to the resulting user id. Built once up front and held for
// the whole run, so resolving a commit's author during the main walk is a
// plain map read, never a query. Deliberately no fuzzy matching to existing
// real accounts -- see the plan's known limitations.
func ResolveAuthors(ctx context.Context, q *db.Queries, commits []Commit) (map[string]string, error) {
	displayNameByKey := map[string]string{}
	for _, c := range commits {
		key := AuthorKey(c.AuthorName, c.AuthorEmail)
		if _, ok := displayNameByKey[key]; !ok {
			displayNameByKey[key] = c.AuthorName
		}
	}
	ids := make(map[string]string, len(displayNameByKey))
	for key, name := range displayNameByKey {
		id, err := q.UpsertHistoricalAuthor(ctx, name, key)
		if err != nil {
			return nil, err
		}
		ids[key] = id
	}
	return ids, nil
}
