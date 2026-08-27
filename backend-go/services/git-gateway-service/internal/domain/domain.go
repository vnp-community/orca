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

// ErrConflictResolveUnsupportedOverRelay is returned when ResolveConflict is
// called against a relay-connected (SSH) worktree — the real agent's
// git.exec whitelist for Part B (the surface RelayExecutor's SSH-relay
// calls reach) explicitly excludes both `checkout` and `add`
// (specs/agent/api/agent-rpc-catalog-git-fs.md:203-227's "Not allowed at
// all" list), so there is no whitelisted way to compose ours/theirs/
// markResolved remotely. Same operational-fallback shape as
// ErrForceDeleteBranchUnsupported above — lives in domain so both
// grpcclient (which returns it) and usecase (which checks it via
// errors.Is) can import it without an import cycle.
var ErrConflictResolveUnsupportedOverRelay = errors.New("git-gateway-service: relay target does not support per-file conflict resolution")

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

// GitChangeEntry is one changed file's status within a CommitCompare/
// BranchCompare result — mirrors parseBranchDiff's {path, status, oldPath?,
// added?, removed?} entry shape (agent/src/relay/git-handler-utils.ts:107-134,
// parseBranchStatusChar:15-30, agent/src/shared/git-uncommitted-line-stats.ts:56-76).
type GitChangeEntry struct {
	Path    string
	Status  string // "modified" | "added" | "deleted" | "renamed" | "copied"
	OldPath string
	Added   int
	Removed int
}

// CommitCompareResult mirrors the real agent's git.commitCompare response
// (agent/src/relay/git-handler-commit-diff-ops.ts:15-122,
// specs/agent/api/agent-rpc-catalog-git-fs.md:50/144): a commit diffed
// against its own parent (or the empty tree, for a root commit) — NOT two
// arbitrary commits, which TASK-209's original design incorrectly assumed.
// ParentOID is empty for a root commit (diffed against the empty tree).
type CommitCompareResult struct {
	CommitOID    string
	ParentOID    string
	CompareRef   string
	BaseRef      string
	ChangedFiles int
	Status       string // "ready" | "invalid-commit" | "error"
	ErrorMessage string
	Entries      []GitChangeEntry
}

// BranchCompareResult mirrors the real agent's git.branchCompare response
// (agent/src/relay/git-handler-ops.ts:124-214,
// specs/agent/api/agent-rpc-catalog-git-fs.md:49/143): current HEAD diffed
// against ONE baseRef's merge-base — NOT two arbitrary branches, which
// TASK-209's original design incorrectly assumed.
type BranchCompareResult struct {
	BaseRef      string
	BaseOID      string
	CompareRef   string
	HeadOID      string
	MergeBase    string
	ChangedFiles int
	CommitsAhead int
	Status       string // "ready" | "invalid-base" | "unborn-head" | "no-merge-base" | "loading" | "error"
	ErrorMessage string
	Entries      []GitChangeEntry
}

// FileDiffResult is a single file's before/after content — the real agent's
// git.commitDiff/git.branchDiff response shape (buildDiffResult,
// agent/src/relay/git-diff-result.ts:5-38), NOT a unified-diff text blob
// like DiffResult (GetDiff's shape, see that type's own doc comment) — the
// real agent composes these two ops from raw blob reads at each ref, not
// `git diff`'s textual output. See CommitDiff/BranchDiff's GitExecutor doc
// comments in ports.go for why this needed a different domain type than
// GetDiff's DiffResult.
type FileDiffResult struct {
	Kind             string // "text" | "binary"
	OriginalContent  string
	ModifiedContent  string
	OriginalIsBinary bool
	ModifiedIsBinary bool
	IsImage          bool
	MimeType         string
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
// bookkeeping row RecordWorktreeCreated/SetWorktreeActivation/ListWorktrees
// return. RepoID/Active added for BR-WT-04's per-repo active-count cap.
type WorktreeRecord struct {
	ID     string
	RepoID string
	Path   string
	Branch string
	Active bool
}

// WorktreeInfo is project-service's GetWorktree answer — the richer shape
// CompareWorktrees needs (RepoID + Branch + BaseRef), vs. WorktreeRecord's
// narrower ID/Path/Branch used by CreateWorktree's own bookkeeping call.
type WorktreeInfo struct {
	ID      string
	RepoID  string
	Branch  string
	BaseRef string // empty = never backfilled (worktree created before base_ref was added)
}

// TerminalSessionRef is one active PTY session, as reported by
// infra-fleet-service.ListTerminalSessions — the subset CheckWorktreeDeleteSafety/
// RemoveWorktree need to determine whether a session's cwd falls under a
// worktree's path.
type TerminalSessionRef struct {
	PtyID string
	Cwd   string
}

// DeleteSafetyReport is CheckWorktreeDeleteSafety's answer — see that
// usecase's doc comment for AgentRunning's heuristic-not-precise caveat.
type DeleteSafetyReport struct {
	UncommittedFiles int
	UntrackedFiles   int
	AgentRunning     bool
	ActivePtyIDs     []string
	SafeToDelete     bool
}

// RemoveWorktreeResult is RemoveWorktree's answer — UncommittedFilesDiscarded
// is only meaningful when Force was true (echoes what was overridden, for
// the UI's post-delete confirmation toast).
type RemoveWorktreeResult struct {
	UncommittedFilesDiscarded int
	StoppedPtyIDs             []string
}

// WorktreeComparison is one worktree's entry within CompareWorktrees'
// aggregated answer.
type WorktreeComparison struct {
	WorktreeID   string
	ChangedFiles int
	AddedLines   int
	RemovedLines int
	MergeBase    string
	Status       string
	ErrorMessage string
}

// CompareWorktreesResult is CompareWorktrees' full answer.
type CompareWorktreesResult struct {
	BaseRef   string
	Worktrees []WorktreeComparison
}

// MergeResult reflects a MergeBranch operation's outcome. A conflict is
// reported via HasConflicts, not an error — the repo is left in the
// conflicted state for the client to resolve via the existing
// ConflictOperation/ResolveConflict/AbortMerge RPCs (BR-WT-17: manual
// resolution only, never auto-resolved or auto-aborted).
type MergeResult struct {
	ResultSHA           string
	HasConflicts        bool
	ConflictedPaths     []string
	ConflictDispatchKey string
}

// ResolvedBase is PrefetchCreateBase/ResolvePrBase/ResolveMrBase's answer:
// a base branch name plus the local SHA it resolved to once fetched.
type ResolvedBase struct {
	Branch string
	SHA    string
}

// ── Group A — branch/ref operations (TASK-207) ─────────────────────────────

// BranchInfo is one local branch's tracking state, returned by
// ListLocalBranches. Mirrors gitgateway.proto's BranchInfo message 1:1 —
// see that message's doc comment for why this is richer than the real
// agent's own git.localBranches response.
type BranchInfo struct {
	Name      string
	Upstream  string
	Ahead     int
	Behind    int
	IsCurrent bool
	IsRemote  bool
}

// CheckoutResult reflects a Checkout operation's outcome.
type CheckoutResult struct {
	Success bool
	Branch  string
}

// PushTargetInput mirrors the real agent's GitPushTarget wire shape
// (agent/src/shared/types.ts:551-557) — see gitgateway.proto's
// PushTargetInput message doc comment for the full citation. Used by
// FastForward, UpstreamStatus, and Fetch (TASK-209/210's own contract
// corrections) — Push/Pull still take the open pushTarget redesign as a
// follow-up (see grpcclient.RelayExecutor's Push/Pull "KNOWN LIMITATION"
// comments), not resolved by this pass.
type PushTargetInput struct {
	RemoteName    string
	BranchName    string
	RemoteURL     string
	RemoteCreated bool
}

// FastForwardResult reflects whether a FastForward operation succeeded.
type FastForwardResult struct {
	Success   bool
	ResultSHA string // NEW — HEAD's SHA after a successful fast-forward
}

// RebaseResult reflects whether a RebaseFromBase operation succeeded, and
// whether it left the worktree with unresolved conflicts — mirrors
// PullResult's Success/HadConflicts shape for the same reason (a conflict
// is a real domain outcome, not a Go error).
type RebaseResult struct {
	Success      bool
	HadConflicts bool
}

// BulkDiscardResult reports partial failure across a multi-path discard —
// see BulkDiscardRequest's proto doc comment for why this isn't
// all-or-nothing.
type BulkDiscardResult struct {
	Success     bool
	FailedPaths []string
}
