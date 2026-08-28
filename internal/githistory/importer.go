package githistory

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/proposalschema"
)

// pathPrefixes restricts every diff-tree call to the two directories this
// importer understands. virtual-organizations/ is deliberately excluded:
// VOs aren't a proposal kind in this system at all, imported directly,
// never through the proposal workflow. Downtime (*_downtime.yaml, under
// topology/) is filtered out later, per-path, in classifyPath -- it's
// cheaper to exclude by suffix there than to add a third top-level prefix.
var pathPrefixes = []string{"topology", "projects"}

// Options configures one import-history run.
type Options struct {
	RepoPath string
	Ref      string // the v1 repo's default branch, e.g. "master"
	// Since bounds the walk to commits at or after this git-recognized date
	// (e.g. "2019-01-01"), or "" for the entire history. History before the
	// modern YAML shapes stabilized genuinely can't be parsed by the current
	// model structs (confirmed: a "list of single-key maps" project shape and
	// a plain-string ContactLists.Primary shape both appear in the repo's
	// first year) -- rather than open-endedly chasing every old format
	// variant, Since draws a line and only walks history at or after it.
	Since string
	Limit int // 0 = no limit; otherwise stop after writing this many new proposals -- for cheap incremental test runs against a slice of history
}

// Result summarizes one run for the CLI's log line.
type Result struct {
	CommitsConsidered int // total first-parent commits found on Ref
	CommitsSkipped    int // already imported (source_commit_sha), from a prior run
	ProposalsWritten  int
}

// pathEvent is one changed, entity-classified path within one commit,
// carrying just enough to run pass 2 without re-invoking git.
type pathEvent struct {
	commitIdx    int
	cp           ChangedPath
	oldC, newC   classified
	oldOK, newOK bool
	identityID   int
}

// Run walks repoPath's history on Ref and replays it into change_proposals.
// Must run after import-tree has already loaded the current snapshot --
// resource identity resolution and name normalization both assume the live
// database already reflects the same final state this walk is building
// toward. Never touches live entity tables itself.
func Run(ctx context.Context, q *db.Queries, opts Options) (Result, error) {
	commits, err := ListCommits(ctx, opts.RepoPath, opts.Ref, opts.Since)
	if err != nil {
		return Result{}, fmt.Errorf("listing commits: %w", err)
	}
	result := Result{CommitsConsidered: len(commits)}

	already, err := q.ListImportedCommitSHAs(ctx)
	if err != nil {
		return result, fmt.Errorf("loading already-imported commits: %w", err)
	}

	authorIDs, err := ResolveAuthors(ctx, q, commits)
	if err != nil {
		return result, fmt.Errorf("resolving historical authors: %w", err)
	}

	// Pass 1: metadata-only walk across every not-yet-imported commit (path
	// statuses only, no blob content) so the rename/identity registry is
	// fully built -- and every entity's FINAL name known -- before pass 2
	// writes anything. A commit near the start of history can only be given
	// its correct (HEAD-time) target_name once every later rename of that
	// same entity has already been seen.
	identity := NewPathIdentity()
	var events []pathEvent
	for i, c := range commits {
		if already[c.SHA] {
			result.CommitsSkipped++
			continue
		}
		paths, err := ChangedPaths(ctx, opts.RepoPath, c.ParentSHA, c.SHA, pathPrefixes)
		if err != nil {
			return result, fmt.Errorf("commit %s: %w", c.SHA, err)
		}
		for _, cp := range paths {
			oldC, oldOK := classifyPath(cp.OldPath)
			newC, newOK := classifyPath(cp.NewPath)
			if !oldOK && !newOK {
				continue // not an entity file this importer understands
			}
			name := newC.Name
			if !newOK {
				name = oldC.Name
			}
			id := identity.Touch(cp.Status, cp.OldPath, cp.NewPath, name)
			events = append(events, pathEvent{
				commitIdx: i, cp: cp, oldC: oldC, newC: newC, oldOK: oldOK, newOK: newOK, identityID: id,
			})
		}
	}

	// Pass 2: fetch blob content, decompose, resolve target names (identity
	// is now fully built), and write one proposal (or one bundle) per
	// commit that actually produced a change.
	cat, err := OpenBlobCat(ctx, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("opening git cat-file: %w", err)
	}
	defer cat.Close()

	i := 0
	for i < len(events) {
		commitIdx := events[i].commitIdx
		var group []pathEvent
		for i < len(events) && events[i].commitIdx == commitIdx {
			group = append(group, events[i])
			i++
		}
		commit := commits[commitIdx]

		var changes []entityChange
		for _, ev := range group {
			oldBlob, err := cat.Get(ev.cp.OldBlob)
			if err != nil {
				return result, fmt.Errorf("commit %s: %w", commit.SHA, err)
			}
			newBlob, err := cat.Get(ev.cp.NewBlob)
			if err != nil {
				return result, fmt.Errorf("commit %s: %w", commit.SHA, err)
			}
			found, err := decomposeChangedPath(ev.oldC, ev.newC, ev.oldOK, ev.newOK, oldBlob, newBlob)
			if err != nil {
				return result, fmt.Errorf("commit %s, path %s/%s: %w", commit.SHA, ev.cp.OldPath, ev.cp.NewPath, err)
			}
			for _, ch := range found {
				if ch.Kind == models.KindResource {
					ch.TargetName = strconv.FormatInt(ch.ResourceTopologyID, 10)
				} else {
					ch.TargetName = identity.FinalName(ev.identityID)
				}
				changes = append(changes, ch)
			}
		}
		if len(changes) == 0 {
			continue
		}

		authorID := authorIDs[AuthorKey(commit.AuthorName, commit.AuthorEmail)]
		if err := writeCommitProposal(ctx, q, commit, changes, authorID); err != nil {
			return result, fmt.Errorf("commit %s: %w", commit.SHA, err)
		}
		result.ProposalsWritten++
		if opts.Limit > 0 && result.ProposalsWritten >= opts.Limit {
			break
		}
	}
	return result, nil
}

// decomposeChangedPath dispatches one changed path to the right per-kind
// decompose function, based on however the old and new sides classify.
// The two sides classifying to different kinds is a pathological case (git
// rename-detected a path across two totally different entity-file shapes,
// which real content-similarity detection essentially never does in
// practice) handled defensively as an independent delete on the old side
// plus a create on the new side, rather than silently picking one.
func decomposeChangedPath(oldC, newC classified, oldOK, newOK bool, oldBlob, newBlob []byte) ([]entityChange, error) {
	if oldOK && newOK && oldC.Kind != newC.Kind {
		del, err := decomposeChangedPath(oldC, classified{}, true, false, oldBlob, nil)
		if err != nil {
			return nil, err
		}
		add, err := decomposeChangedPath(classified{}, newC, false, true, nil, newBlob)
		if err != nil {
			return nil, err
		}
		return append(del, add...), nil
	}
	kind := newC.Kind
	if !newOK {
		kind = oldC.Kind
	}
	oldName, newName := "", ""
	if oldOK {
		oldName = oldC.Name
	}
	if newOK {
		newName = newC.Name
	}
	switch kind {
	case models.KindFacility:
		return decomposeFacility(oldName, newName, oldBlob, newBlob)
	case models.KindSite:
		oldFacility, newFacility := "", ""
		if oldOK {
			oldFacility = oldC.Facility
		}
		if newOK {
			newFacility = newC.Facility
		}
		return decomposeSite(oldName, newName, oldFacility, newFacility, oldBlob, newBlob)
	case models.KindResourceGroup:
		oldSite, newSite := "", ""
		if oldOK {
			oldSite = oldC.Site
		}
		if newOK {
			newSite = newC.Site
		}
		return decomposeResourceGroup(oldName, newName, oldSite, newSite, oldBlob, newBlob)
	case models.KindProject:
		return decomposeProject(oldName, newName, oldBlob, newBlob)
	default:
		return nil, nil
	}
}

// writeCommitProposal writes one commit's worth of changes as a single
// change_proposals row: a plain single-entity proposal when the commit
// touched exactly one entity (the common case, and the one that matters for
// discoverability -- ListProposalsByEntity matches only on a proposal's own
// top-level entity_kind/target_name, never looking inside a bundle's
// operations, so a single-entity commit MUST NOT be wrapped in a bundle or
// it would vanish from that entity's own history page); a bundle otherwise.
func writeCommitProposal(ctx context.Context, q *db.Queries, commit Commit, changes []entityChange, authorID string) error {
	if len(changes) == 1 {
		ch := changes[0]
		proposed, base, err := proposedAndBase(ch)
		if err != nil {
			return err
		}
		_, err = q.InsertHistoricalProposal(ctx, db.InsertHistoricalProposalParams{
			EntityKind: ch.Kind, TargetName: ch.TargetName, Operation: ch.Operation,
			ProposedState: proposed, BaseVersion: base,
			SchemaVersion: proposalschema.CurrentVersion(ch.Kind),
			CreatedBy:     authorID, CommittedAt: commit.AuthorDate, SourceCommitSHA: commit.SHA,
		})
		return err
	}

	ops := make([]bundleOpPayload, 0, len(changes))
	for _, ch := range changes {
		proposed, _, err := proposedAndBase(ch)
		if err != nil {
			return err
		}
		ops = append(ops, bundleOpPayload{
			EntityKind: ch.Kind, Operation: ch.Operation, TargetName: ch.TargetName,
			ProposedState: proposed, SchemaVersion: proposalschema.CurrentVersion(ch.Kind),
		})
	}
	proposed, err := json.Marshal(bundlePayload{Operations: ops})
	if err != nil {
		return err
	}
	_, err = q.InsertHistoricalProposal(ctx, db.InsertHistoricalProposalParams{
		EntityKind: models.KindBundle, Operation: models.OpCreate, ProposedState: proposed,
		SchemaVersion: proposalschema.CurrentVersion(models.KindBundle),
		CreatedBy:     authorID, CommittedAt: commit.AuthorDate, SourceCommitSHA: commit.SHA,
	})
	return err
}

// proposedAndBase turns one entityChange into (proposed_state, base_version)
// matching exactly what the live create/revise path would have stored:
// create -> (after, nil); update -> (after, before); delete -> a minimal
// {"name": ...} (a delete carries no real payload -- see
// entityActions.tsx's DeleteButton) plus the full before-snapshot as base,
// same as CreateProposal always snapshotting base for any non-create op.
func proposedAndBase(ch entityChange) (proposed, base json.RawMessage, err error) {
	switch ch.Operation {
	case models.OpCreate:
		return ch.After, nil, nil
	case models.OpUpdate:
		return ch.After, ch.Before, nil
	case models.OpDelete:
		name, err := json.Marshal(map[string]string{"name": ch.OldName})
		if err != nil {
			return nil, nil, err
		}
		return name, ch.Before, nil
	default:
		return nil, nil, fmt.Errorf("unknown operation %q", ch.Operation)
	}
}
