// Package domain holds git-gateway-service's value objects. Per
// specs/backend-go/services/git-gateway-service.md §4, this package is
// deliberately light: value objects mirrored from the Dev Server Agent's
// wire protocol / local `git` output, not invariant-bearing entities — this
// service owns no persistent state to protect invariants over (§5, §6). No
// type here has a constructor that enforces a git invariant (e.g. "a commit
// must have a valid SHA") because this service never constructs a commit —
// it only relays and reflects what the Dev Server Agent or local `git`
// binary already produced.
package domain

// FileState enumerates the file-status values git-gateway-service's wire
// protocol carries (mirrors the generated proto's FileStatus.state string
// per gitgateway.proto's comment: "modified/added/deleted/untracked/conflicted").
// Kept as a string-backed type with a Valid() check rather than a
// constructor-enforced entity — this is the one "meaningfully pure" piece of
// validation logic this package has, per the design doc's guidance that a
// minimal domain package with just types is fine otherwise.
type FileState string

const (
	FileStateModified   FileState = "modified"
	FileStateAdded      FileState = "added"
	FileStateDeleted    FileState = "deleted"
	FileStateUntracked  FileState = "untracked"
	FileStateConflicted FileState = "conflicted"
	FileStateRenamed    FileState = "renamed"
)

// Valid reports whether s is one of the known file-status states.
func (s FileState) Valid() bool {
	switch s {
	case FileStateModified, FileStateAdded, FileStateDeleted, FileStateUntracked, FileStateConflicted, FileStateRenamed:
		return true
	default:
		return false
	}
}

// FileStatus is one file's position in `git status` output — path plus its
// state, translated 1:1 from either local `git status --porcelain` parsing
// or the Dev Server Agent's git.status JSON response.
type FileStatus struct {
	Path  string
	State FileState
}

// GitStatus is a worktree's full status: current branch plus its file list.
// Ahead/behind counts and a conflict flag are part of the design doc's
// sketch (§4) but are not part of the current generated proto surface
// (GetStatusResponse only carries files+branch); add them here alongside
// the corresponding proto fields if/when that RPC surface grows.
type GitStatus struct {
	Branch string
	Files  []FileStatus
}

// DiffResult is a unified-diff text blob returned by GetDiff — this service
// does not parse the diff into hunks (§2: "does not parse diffs beyond
// what's needed to pass them through"), it passes through whatever the
// local `git diff` binary or Dev Server Agent produced.
type DiffResult struct {
	UnifiedDiff string
}

// CommitResult holds the SHA produced by a Commit operation.
type CommitResult struct {
	CommitSHA string
}

// PushResult reflects whether a Push operation succeeded.
type PushResult struct {
	Success bool
}

// PullResult reflects whether a Pull operation succeeded, and whether it
// left the worktree with unresolved conflicts.
type PullResult struct {
	Success      bool
	HadConflicts bool
}
