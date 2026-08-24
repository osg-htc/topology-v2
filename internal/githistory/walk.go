// Package githistory replays a v1 topology repo's git history into a
// backlog of already-applied change_proposals rows, so a fresh v2 import
// comes with real edit history instead of a blank slate. It never touches
// live entity tables -- import-tree already seeds those from the current
// snapshot; this only ever writes change_proposals (and, once, a handful of
// placeholder users for historical authors).
//
// Everything here talks to git purely through plumbing commands (no working
// tree checkout, no full-repo reparse per commit) -- diff-tree drives cost
// down to "proportional to what actually changed across history," which is
// what makes walking tens of thousands of commits practical at all.
package githistory

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// emptyTree is git's well-known hash of the empty tree, used as the "old"
// side of a diff-tree for a repo's very first commit (which has no parent).
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// zeroBlob is the all-zero SHA diff-tree uses in place of a real blob hash
// on the side of a change that doesn't have one (an add has no old blob, a
// delete has no new blob).
const zeroBlob = "0000000000000000000000000000000000000000"

// Commit is one first-parent-mainline commit, with the parent to diff
// against (empty for the repo's root commit).
type Commit struct {
	SHA         string
	ParentSHA   string
	AuthorName  string
	AuthorEmail string
	AuthorDate  time.Time
}

// unitSep separates fields in the log format string below -- chosen because
// it can never appear in a commit's own metadata, unlike a comma or tab.
const unitSep = "\x1f"

// ListCommits returns every commit on ref's first-parent mainline, oldest
// first, optionally starting only at since (a git-recognized date like
// "2019-01-01", or "" for the repo's entire history). First-parent, not the
// full DAG: in a repo whose PRs land as real two-parent merge commits
// (confirmed for this one), diffing each mainline commit against its
// immediate parent already yields exactly one record per landed PR (or per
// direct push) -- walking every branch commit too would double-count
// content the merge commit already carries.
func ListCommits(ctx context.Context, repoPath, ref, since string) ([]Commit, error) {
	format := "%H" + unitSep + "%P" + unitSep + "%an" + unitSep + "%ae" + unitSep + "%aI"
	args := []string{"-C", repoPath, "log", "--first-parent", "--reverse", "--format=" + format}
	if since != "" {
		args = append(args, "--since="+since)
	}
	args = append(args, ref)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var commits []Commit
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, unitSep)
		if len(fields) != 5 {
			return nil, fmt.Errorf("git log: unexpected line shape: %q", line)
		}
		parent := ""
		if ps := strings.Fields(fields[1]); len(ps) > 0 {
			parent = ps[0] // first parent only, even if git printed more
		}
		date, err := time.Parse(time.RFC3339, fields[4])
		if err != nil {
			return nil, fmt.Errorf("git log: parsing author date %q: %w", fields[4], err)
		}
		commits = append(commits, Commit{
			SHA: fields[0], ParentSHA: parent,
			AuthorName: fields[2], AuthorEmail: fields[3], AuthorDate: date,
		})
	}
	return commits, nil
}

// ChangedPath is one file-level change within a single commit's diff,
// straight from diff-tree's raw output -- including both blob SHAs, so blob
// content can be fetched directly with no extra path->blob lookup step.
type ChangedPath struct {
	Status  byte // 'A' add, 'M' modify, 'D' delete, 'R' rename, 'C' copy
	OldPath string
	NewPath string
	OldBlob string
	NewBlob string
}

// ChangedPaths returns every changed path in commit sha (diffed against
// parentSHA, or the empty tree if parentSHA is ""), restricted to the given
// path prefixes. Uses raw diff-tree output specifically because it carries
// both blob SHAs per line -- no separate "what blob does this path have"
// lookup is needed.
func ChangedPaths(ctx context.Context, repoPath, parentSHA, sha string, pathPrefixes []string) ([]ChangedPath, error) {
	old := parentSHA
	if old == "" {
		old = emptyTree
	}
	args := []string{"-C", repoPath, "diff-tree", "--no-commit-id", "-r", "-M", "-C", "-z", "--raw", old, sha, "--"}
	args = append(args, pathPrefixes...)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree %s..%s: %w", old, sha, err)
	}
	return parseRawDiffTree(out)
}

// parseRawDiffTree parses -z --raw output: each entry is
// ":<old-mode> <new-mode> <old-blob> <new-blob> <status>\0<path>\0" and, for
// a rename/copy, one extra "\0<newpath>\0".
func parseRawDiffTree(out []byte) ([]ChangedPath, error) {
	fields := strings.Split(string(out), "\x00")
	var changes []ChangedPath
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "" || !strings.HasPrefix(f, ":") {
			continue
		}
		parts := strings.Fields(f)
		if len(parts) != 5 {
			return nil, fmt.Errorf("diff-tree: unexpected raw entry %q", f)
		}
		statusField := parts[4]
		status := statusField[0]
		oldMode, newMode := strings.TrimPrefix(parts[0], ":"), parts[1]
		oldBlob, newBlob := parts[2], parts[3]
		i++
		if i >= len(fields) {
			return nil, fmt.Errorf("diff-tree: missing path after %q", f)
		}
		path := fields[i]
		// A symlink (mode 120000, real in this repo's history -- e.g. a
		// legacy filename kept as a redirect to its renamed real file) has
		// no YAML content of its own; its "blob" is just the link target
		// text. Skip it as a changed path entirely, same as an unrecognized
		// file shape -- there's nothing here for classifyPath to parse.
		if status == 'R' || status == 'C' {
			if oldMode == "120000" || newMode == "120000" {
				i++ // still consume the second path field for a rename/copy
				continue
			}
		} else if oldMode == "120000" || newMode == "120000" {
			continue
		}
		switch status {
		case 'R', 'C':
			i++
			if i >= len(fields) {
				return nil, fmt.Errorf("diff-tree: missing new path after rename %q", f)
			}
			newPath := fields[i]
			changes = append(changes, ChangedPath{Status: status, OldPath: path, NewPath: newPath, OldBlob: oldBlob, NewBlob: newBlob})
		case 'A':
			changes = append(changes, ChangedPath{Status: 'A', NewPath: path, OldBlob: oldBlob, NewBlob: newBlob})
		case 'D':
			changes = append(changes, ChangedPath{Status: 'D', OldPath: path, OldBlob: oldBlob, NewBlob: newBlob})
		default: // 'M' and anything else diff-tree might report
			changes = append(changes, ChangedPath{Status: 'M', OldPath: path, NewPath: path, OldBlob: oldBlob, NewBlob: newBlob})
		}
	}
	return changes, nil
}

// BlobCat fetches blob content via one long-lived `git cat-file --batch`
// subprocess -- spawning a fresh process per blob, across the tens of
// thousands of file-versions a full history walk touches, would make
// process-spawn overhead the dominant cost.
type BlobCat struct {
	cmd      *exec.Cmd
	stdinRaw io.WriteCloser // closed to signal EOF so cmd.Wait() in Close returns
	stdin    *bufio.Writer
	stdout   *bufio.Reader
}

// OpenBlobCat starts the batch cat-file process. Call Close when done.
func OpenBlobCat(ctx context.Context, repoPath string) (*BlobCat, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "cat-file", "--batch")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &BlobCat{cmd: cmd, stdinRaw: stdin, stdin: bufio.NewWriter(stdin), stdout: bufio.NewReader(stdout)}, nil
}

// Get returns the content of blob sha, or nil for the all-zero SHA
// diff-tree uses when a change has no blob on that side (an add's old blob,
// a delete's new blob).
func (b *BlobCat) Get(sha string) ([]byte, error) {
	if sha == zeroBlob || sha == "" {
		return nil, nil
	}
	if _, err := b.stdin.WriteString(sha + "\n"); err != nil {
		return nil, err
	}
	if err := b.stdin.Flush(); err != nil {
		return nil, err
	}
	header, err := b.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("cat-file: reading header for %s: %w", sha, err)
	}
	header = strings.TrimSuffix(header, "\n")
	parts := strings.Fields(header)
	if len(parts) != 3 || parts[1] != "blob" {
		return nil, fmt.Errorf("cat-file: unexpected header %q for %s (missing?)", header, sha)
	}
	size, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("cat-file: bad size in header %q: %w", header, err)
	}
	data := make([]byte, size)
	if _, err := readFull(b.stdout, data); err != nil {
		return nil, fmt.Errorf("cat-file: reading %d bytes for %s: %w", size, sha, err)
	}
	if _, err := b.stdout.Discard(1); err != nil { // trailing newline after the blob content
		return nil, err
	}
	return data, nil
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// Close stops the cat-file subprocess.
func (b *BlobCat) Close() error {
	_ = b.stdin.Flush()
	_ = b.stdinRaw.Close()
	return b.cmd.Wait()
}
