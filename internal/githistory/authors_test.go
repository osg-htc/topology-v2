package githistory

import (
	"crypto/sha1"
	"encoding/hex"
	"testing"
)

func TestAuthorKey(t *testing.T) {
	if got, want := AuthorKey("John Doe", "john@example.com"), "git-import:john@example.com"; got != want {
		t.Errorf("AuthorKey(normal) = %q, want %q", got, want)
	}

	// Case and whitespace shouldn't fragment what is really one author.
	if got, want := AuthorKey("John Doe", " JOHN@EXAMPLE.COM "), "git-import:john@example.com"; got != want {
		t.Errorf("AuthorKey(mixed case/whitespace) = %q, want %q", got, want)
	}

	sum := sha1.Sum([]byte("bot"))
	wantHash := "git-import:name:" + hex.EncodeToString(sum[:])
	if got := AuthorKey("Bot", ""); got != wantHash {
		t.Errorf("AuthorKey(empty email) = %q, want %q", got, wantHash)
	}
	if got := AuthorKey("Bot", "not-an-email"); got != wantHash {
		t.Errorf("AuthorKey(malformed email) = %q, want %q", got, wantHash)
	}
}
