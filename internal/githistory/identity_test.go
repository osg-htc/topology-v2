package githistory

import "testing"

func TestPathIdentity_AddModifyRename(t *testing.T) {
	r := NewPathIdentity()

	id1 := r.Touch('A', "", "p1", "n1")
	if got := r.FinalName(id1); got != "n1" {
		t.Fatalf("after add: FinalName = %q, want %q", got, "n1")
	}

	id2 := r.Touch('M', "p1", "p1", "n1b")
	if id2 != id1 {
		t.Fatalf("modify at the same path should keep the same identity: got %d, want %d", id2, id1)
	}
	if got := r.FinalName(id1); got != "n1b" {
		t.Fatalf("after modify: FinalName = %q, want %q", got, "n1b")
	}

	id3 := r.Touch('R', "p1", "p2", "n2")
	if id3 != id1 {
		t.Fatalf("rename should keep the same identity across paths: got %d, want %d", id3, id1)
	}
	if got := r.FinalName(id1); got != "n2" {
		t.Fatalf("after rename: FinalName = %q, want %q", got, "n2")
	}

	// The entity now lives at p2, not p1 -- touching p1 again must not
	// collide with the identity that moved away from it.
	id4 := r.Touch('A', "", "p1", "fresh")
	if id4 == id1 {
		t.Fatalf("a fresh add at the vacated old path must get a new identity, not reuse %d", id1)
	}
}

func TestPathIdentity_Delete(t *testing.T) {
	r := NewPathIdentity()
	id1 := r.Touch('A', "", "p1", "n1")
	r.Touch('D', "p1", "", "")

	// Re-adding at the same path after a delete is a genuinely new entity.
	id2 := r.Touch('A', "", "p1", "n1-again")
	if id2 == id1 {
		t.Fatalf("re-adding a deleted path must get a new identity, not reuse %d", id1)
	}
}

func TestPathIdentity_RenameFromUnseenPath(t *testing.T) {
	// The walk can start mid-history, so a rename's source path may never
	// have been Touch'd as an Add. Must not panic, and must still track the
	// entity going forward under a valid identity.
	r := NewPathIdentity()
	id := r.Touch('R', "never-seen", "p2", "n2")
	if id < 0 {
		t.Fatalf("rename from an unseen path returned an invalid id: %d", id)
	}
	if got := r.FinalName(id); got != "n2" {
		t.Fatalf("FinalName after rename-from-unseen = %q, want %q", got, "n2")
	}
}

func TestPathIdentity_DeleteOfUnseenPath(t *testing.T) {
	r := NewPathIdentity()
	id := r.Touch('D', "never-seen", "", "")
	if id < 0 {
		t.Fatalf("delete of an unseen path returned an invalid id: %d", id)
	}
}

func TestPathIdentity_IndependentEntitiesDontCollide(t *testing.T) {
	r := NewPathIdentity()
	idA := r.Touch('A', "", "pa", "a")
	idB := r.Touch('A', "", "pb", "b")
	if idA == idB {
		t.Fatalf("two unrelated adds got the same identity: %d", idA)
	}
}

func TestPathIdentity_UnknownStatus(t *testing.T) {
	r := NewPathIdentity()
	if id := r.Touch('X', "a", "b", "n"); id != -1 {
		t.Fatalf("unknown status: Touch returned %d, want -1", id)
	}
}
