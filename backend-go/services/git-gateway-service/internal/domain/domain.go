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

import "errors"

// ErrForceDeleteBranchUnsupported is returned when the relay target's Dev
// Server Agent build predates a force-delete-branch method — the
// operational counterpart to GitExecutor.ForceDeleteBranch's compile-time
// guarantee (every implementation must have the method; this is "the
// method call fails cleanly" for an outdated agent build), per BUG-031's
// cited old-TS fallback comment ("older SSH relays predate
// git.forceDeletePreservedBranch").
//
// Lives in domain, not internal/adapter/grpcclient, deliberately: TASK-194
// Step 4's usecase.ForceDeleteBranch needs to check errors.Is against this
// sentinel, and internal/adapter/grpcclient already imports internal/usecase
// (for the port interfaces it implements) — usecase importing grpcclient
// back for just this sentinel would be a real import cycle. domain has no
// dependency on either package, so both grpcclient (which returns this
// error) and usecase (which checks it) can import domain safely.
var ErrForceDeleteBranchUnsupported = errors.New("git-gateway-service: relay target does not support force-delete-branch")

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

// SimpleResult is the bare-success-flag shape shared by Stage/Unstage
// (TASK-208) and Fetch — any operation with no richer result than
// "did it work".
type SimpleResult struct {
	Success bool
}

// CommitRef is one commit's metadata, returned by History. Mirrors
// gitgateway.proto's CommitRef message 1:1.
type CommitRef struct {
	SHA        string
	Author     string
	Committer  string
	Message    string
	Timestamp  int64
	ParentSHAs []string
}

// ForkSyncStatus reflects a worktree's ahead/behind/diverged state relative
// to its upstream fork's default branch.
type ForkSyncStatus struct {
	Ahead    int
	Behind   int
	Diverged bool
}

// UpstreamStatus reflects whether the current branch has a configured
// upstream and its ahead/behind counts if so.
type UpstreamStatus struct {
	HasUpstream bool
	Ahead       int
	Behind      int
}

// WorktreeCreateResult is what a successful `git worktree add` reports.
type WorktreeCreateResult struct {
	Path    string
	HeadSHA string
}

// WorktreeResult is CreateWorktree's usecase-level result: the saga's
// combined answer once both the git operation and project-service's
// bookkeeping record have succeeded.
type WorktreeResult struct {
	WorktreeID string
	Path       string
	HeadSHA    string
}

// RepoInfo is project-service's answer to "does this repo exist, and what
// project/URL does it belong to" — the minimal shape the worktree usecases
// need to validate a repo id before dispatching a git operation against it.
//
// Deviation from TASK-193's original sketch: project.proto's real Repo
// message (backend-go/proto/orca/project/v1/project.proto) has no
// dev_server_id or path field — those fields were this task's best-effort
// guess before checking the proto and don't exist. Dev-server/host
// resolution for a worktree op instead goes entirely through
// ConnectionResolver (see dispatchExecutor in usecase/ports.go), never
// through RepoInfo; RepoInfo here only carries what Repo actually has.
type RepoInfo struct {
	ID          string
	ProjectID   string
	URL         string
	DisplayName string
}

// WorktreeRecord mirrors project-service's Worktree message — the
// bookkeeping row RecordWorktreeCreated/SetWorktreeActivation return.
type WorktreeRecord struct {
	ID     string
	Path   string
	Branch string
}

// ResolvedBase is PrefetchCreateBase/ResolvePrBase/ResolveMrBase's answer:
// a base branch name plus the local SHA it resolved to once fetched.
type ResolvedBase struct {
	Branch string
	SHA    string
}
