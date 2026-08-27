// Package usecase holds git-gateway-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// Per git-gateway-service.md §2/§6, this service's only owned logic is
// "resolve host -> dispatch -> translate": every usecase here follows the
// same shape — resolve which host owns the target worktree via
// ConnectionResolver, then dispatch the actual git operation to whichever
// GitExecutor answers for that host (local binary vs. relay to the Dev
// Server Agent), then return the result for the gRPC adapter to translate
// to wire types.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ResolvedConnection is ConnectionResolver's answer for a worktree: whether
// its owning host is a remote dev server (Connected=true, ConnectionID
// populated) or the same host git-gateway-service itself runs on
// (Connected=false).
//
// RepoPath is the filesystem path GitExecutor operates against. In the full
// target architecture (git-gateway-service.md §7) this is resolved
// separately by project-service's WorktreeResolver and passed alongside the
// connection lookup; this scaffold folds it into ConnectionResolver's
// response to keep the port count to the two named in this service's build
// task (ConnectionResolver, GitExecutor) — see this service's README for the
// project-service integration this still needs.
type ResolvedConnection struct {
	Connected    bool
	ConnectionID string
	RepoPath     string
}

// ConnectionResolver resolves which host owns a worktree, by calling
// infra-fleet-service's ResolveConnection RPC (git-gateway-service.md §2
// step 2, §7). Implemented by internal/adapter/grpcclient in this scaffold
// as a stub that always answers Connected=false — see that package's doc
// comment for what real wiring needs.
type ConnectionResolver interface {
	ResolveConnection(ctx context.Context, worktreeID string) (ResolvedConnection, error)
}

// GitExecutor performs the actual git operation against a resolved worktree
// path. Two implementations exist per git-gateway-service.md §2:
//   - internal/adapter/localgit: a real os/exec-backed implementation used
//     when ConnectionResolver reports Connected=false (host-local case).
//   - internal/adapter/grpcclient: a stub relay-to-Dev-Server-Agent
//     implementation used when Connected=true, via infra-fleet-service's
//     provider-registry client (not wired in this scaffold).
//
// Each usecase is handed both implementations and selects between them
// based on ConnectionResolver's answer — that selection is the "dispatch"
// logic this service actually owns (§2), and is what this package's tests
// exercise with fakes, independent of which GitExecutor implementation is
// real vs. stubbed.
type GitExecutor interface {
	GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error)
	// GetDiff takes a required filePath — the real Dev Server Agent contract
	// is per-file, not whole-repo. See TASK-228.
	GetDiff(ctx context.Context, repoPath, filePath string, staged bool) (domain.DiffResult, error)
	Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
	Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
	Pull(ctx context.Context, repoPath string) (domain.PullResult, error)

	// Stage/Unstage (TASK-208) — both take a repeated path list; the wire
	// method underneath always targets the agent's bulk variant regardless
	// of len(paths), see RelayExecutor.Stage/Unstage's doc comment.
	Stage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error)
	Unstage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error)

	// History (TASK-209, shippable-now) — cursor/pagination dropped, ref
	// renamed to baseRef, per SOL-032 §0's contract correction.
	History(ctx context.Context, repoPath, baseRef string, limit int) ([]domain.CommitRef, error)
	// CheckIgnored (TASK-209, shippable-now) returns only the ignored
	// subset, matching the real agent's response shape.
	CheckIgnored(ctx context.Context, repoPath string, paths []string) ([]string, error)
	// ForkSync (TASK-209, shippable-now) — expectedUpstream is required,
	// the real agent rejects calls without it.
	ForkSync(ctx context.Context, repoPath, expectedUpstream string) (domain.ForkSyncStatus, error)
	// UpstreamStatus (TASK-209) — pushTarget is now the real, structured
	// domain.PushTargetInput (nil = let the agent resolve the worktree's
	// configured push target) instead of the original placeholder string
	// field, now that TASK-207 built PushTargetInput for real (SOL-032 §0
	// open question #1 resolved for this method the same way FastForward's
	// already was). Still needs TASK-227 for relay reachability, unlike
	// this group's other 3 shippable-now methods.
	UpstreamStatus(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.UpstreamStatus, error)

	// RemoteCommitURL/RemoteFileURL (TASK-210, shippable-now) — pure local
	// string construction from the worktree's configured remote URL, no
	// agent method on either side. See TASK-210's contract correction.
	RemoteCommitURL(ctx context.Context, repoPath, sha string) (string, error)
	RemoteFileURL(ctx context.Context, repoPath, path, ref string) (string, error)
	// Fetch (TASK-210) runs `git fetch --prune [remote]` — the real agent
	// always prunes and has no separate prune flag; pushTarget is optional
	// and only its RemoteName is consulted for which remote to fetch (see
	// localgit.Executor.Fetch and RelayExecutor.Fetch doc comments). Needed
	// TASK-227 for relay reachability and PushTargetInput for real (both
	// now resolved, unblocking this method per TASK-210's own contract
	// correction).
	Fetch(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.SimpleResult, error)

	// ── Group C — real compare/diff/submodule shapes (TASK-209's Contract
	// correction section resolved via specs/agent/api/agent-rpc-catalog-git-fs.md
	// lines 49-55/143-147 and the real handler source cited on each method
	// below — not the two-arbitrary-refs / whole-commit / list-every-submodule
	// shapes TASK-209's original design incorrectly sketched). ─────────────

	// CommitCompare diffs ONE commit against its own parent (or the empty
	// tree for a root commit) — matches git.commitCompare exactly
	// (agent/src/relay/git-handler-commit-diff-ops.ts:15-122,
	// agent/src/relay/git-handler.ts:915-919). commitID must be a full
	// 40/64-hex object id (assertFullGitObjectId).
	CommitCompare(ctx context.Context, repoPath, commitID string) (domain.CommitCompareResult, error)
	// BranchCompare diffs current HEAD against ONE baseRef's merge-base —
	// matches git.branchCompare exactly (agent/src/relay/git-handler-ops.ts:124-214,
	// agent/src/relay/git-handler.ts:891-913). baseRef must not start with
	// "-" (the real agent rejects that to avoid flag injection).
	BranchCompare(ctx context.Context, repoPath, baseRef string) (domain.BranchCompareResult, error)
	// CommitDiff is a single required file's before/after content at one
	// commit vs. its (optional) parent — matches git.commitDiff exactly
	// (agent/src/relay/git-handler-commit-diff-ops.ts:124-160,
	// agent/src/relay/git-handler.ts:1245-1265). Same class of per-file fix
	// as GetDiff's own TASK-228 correction: the original TASK-209 design had
	// no filePath at all and assumed a whole-commit diff. parentOID empty =
	// root commit (diffed against the empty tree via oldPath/filePath at
	// the empty blob).
	CommitDiff(ctx context.Context, repoPath, commitOID, parentOID, filePath, oldPath string) (domain.FileDiffResult, error)
	// BranchDiff is a single required file's before/after content vs. one
	// baseRef's merge-base — matches git.branchDiff exactly
	// (agent/src/relay/git-handler-ops.ts:218-288,
	// agent/src/relay/git-handler.ts:1213-1243). Same per-file-required fix
	// as CommitDiff, plus the same base-ref-only-vs-two-sided fix as
	// BranchCompare.
	BranchDiff(ctx context.Context, repoPath, baseRef, filePath, oldPath string) (domain.FileDiffResult, error)
	// SubmoduleStatus operates on ONE submodule per call — matches
	// git.submoduleStatus exactly (agent/src/relay/agent-git-handler-extended.ts:196-230,
	// specs/agent/api/agent-rpc-catalog-git-fs.md:55/123). SOL-032 §0 open
	// question #3 resolved by reading the real frontend caller
	// (frontend/src/renderer/src/runtime/runtime-git-client.ts:152-176,
	// frontend/src/renderer/src/components/right-sidebar/useSourceControlSubmoduleStatus.ts):
	// the frontend already knows every dirty submodule's path from the
	// parent git.status response and only ever fetches one submodule's
	// status on lazy expansion — there is no "list every submodule" caller
	// to build an aggregation for, so the original TASK-209 repeated-response
	// design was solving a problem the real frontend doesn't have. area is
	// "staged" | "untracked" | "unstaged" (default), matching the real
	// agent's own default.
	SubmoduleStatus(ctx context.Context, repoPath, submodulePath, area string) (domain.GitStatus, error)

	// Clone and InitRepo create a worktree that doesn't exist yet — unlike
	// every other GitExecutor method, they are not called with a repoPath
	// resolved from an existing worktreeId/connectionId. See DevServerReachability.
	Clone(ctx context.Context, url, destPath string) (worktreePath, defaultBranch string, err error)
	InitRepo(ctx context.Context, destPath, defaultBranch string) (path, resolvedDefaultBranch string, err error)

	BaseRefDefault(ctx context.Context, repoPath string) (ref string, err error)
	SearchRefs(ctx context.Context, repoPath, query string) (refs []string, err error)
	CheckHooks(ctx context.Context, repoPath string) (installedHooks []string, orcaHooksCurrent bool, err error)
	ReadIssueCommand(ctx context.Context, repoPath string) (content string, exists bool, err error)
	WriteIssueCommand(ctx context.Context, repoPath, content string) error
	ScanSetupScriptImports(ctx context.Context, repoPath string) (importedPaths []string, err error)

	// New, SOL-031 (TASK-193):
	CreateWorktree(ctx context.Context, repoPath, branch, baseRef string) (domain.WorktreeCreateResult, error)
	RemoveWorktree(ctx context.Context, worktreePath string, force bool) error
	// FetchAndResolveRef ensures ref is fetched/up to date in repoPath's
	// local clone and returns its resolved SHA — shared by
	// PrefetchCreateBase and ResolvePrBase/ResolveMrBase's "confirm the
	// platform's base branch actually exists locally" step.
	FetchAndResolveRef(ctx context.Context, repoPath, ref string) (sha string, err error)
	// ListWorktreePaths returns the raw on-disk worktree paths for repoPath
	// (`git worktree list --porcelain`, no bookkeeping join) — DetectWorktrees
	// needs this. Added directly to the interface rather than behind a
	// runtime type assertion (TASK-193 Step 8's own correction: a
	// compile-time-required method is available one edit away, so ship
	// that instead of a type-assertion sketch).
	ListWorktreePaths(ctx context.Context, repoPath string) ([]string, error)
	// ForceDeleteBranch is REQUIRED on every GitExecutor implementation —
	// deliberately not an optional/type-asserted method (TASK-194). This is
	// the structural fix for the old TS backend's forceDeletePreservedBranch?
	// crash-bug class (BUG-031): Go's compiler now refuses to build ANY
	// GitExecutor implementation missing this method, closing the "one
	// provider variant silently lacks it" gap by construction, not by
	// convention. The operational fallback for an outdated relay-side
	// agent build that genuinely doesn't support this is handled inside
	// RelayExecutor.ForceDeleteBranch's own body (a typed, caught error via
	// domain.ErrForceDeleteBranchUnsupported), independent of this
	// compile-time guarantee.
	ForceDeleteBranch(ctx context.Context, repoPath, branch string) error

	// ── Group A — branch/ref operations (TASK-207) ───────────────────────
	//
	// Checkout/ListLocalBranches/FastForward/ConflictOperation's shapes
	// below were redesigned against the real agent contract
	// (specs/agent/api/agent-rpc-catalog-git-fs.md), not implemented as
	// TASK-207's own original sketch — see each method's doc comment and
	// gitgateway.proto's matching message for citations. The other 5
	// (RebaseFromBase/AbortRebase/AbortMerge/Discard/BulkDiscard) are the
	// mechanical param-rename-only subset that sketch got right.

	// Checkout switches to an existing ref — the real agent has no
	// create-branch (-b) semantics, so unlike TASK-207's original sketch
	// there is no `create` param here. See CheckoutRequest's proto doc
	// comment.
	Checkout(ctx context.Context, repoPath, branch string) (domain.CheckoutResult, error)
	// ListLocalBranches returns richer per-branch data than the real
	// agent's own git.localBranches RPC — RelayExecutor composes it via
	// git.exec's for-each-ref subcommand instead of calling
	// git.localBranches directly. See BranchInfo's proto doc comment.
	ListLocalBranches(ctx context.Context, repoPath string) ([]domain.BranchInfo, error)
	// FastForward takes an optional structured pushTarget (nil = agent
	// resolves the worktree's configured push target), matching the real
	// git.fastForward/git.pull/git.push contract instead of TASK-207's
	// original plain branch-string sketch. See PushTargetInput's proto doc
	// comment.
	FastForward(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.FastForwardResult, error)
	RebaseFromBase(ctx context.Context, repoPath, baseRef string) (domain.RebaseResult, error)
	AbortRebase(ctx context.Context, repoPath string) (domain.SimpleResult, error)
	AbortMerge(ctx context.Context, repoPath string) (domain.SimpleResult, error)
	// ConflictOperation is a DETECTOR ONLY, matching the real agent exactly
	// — returns which operation (if any) left repoPath conflicted
	// ("merge"/"rebase"/"cherry-pick"/"unknown"). See ResolveConflict below
	// for the per-file resolve op TASK-207's original sketch conflated with
	// this one.
	ConflictOperation(ctx context.Context, repoPath string) (operation string, err error)
	// ResolveConflict has no real agent RPC backing it — see its proto doc
	// comment. RelayExecutor's implementation always returns
	// domain.ErrConflictResolveUnsupportedOverRelay; only localgit.Executor
	// does real work.
	ResolveConflict(ctx context.Context, repoPath, path, operation string) (domain.SimpleResult, error)
	Discard(ctx context.Context, repoPath, path string) (domain.SimpleResult, error)
	BulkDiscard(ctx context.Context, repoPath string, paths []string) (domain.BulkDiscardResult, error)
}

// DevServerReachability resolves whether devServerID is a live,
// agent-reachable remote host (relay branch) or this service should
// operate on its own filesystem (local branch) — used only by Clone/
// InitRepo, which have no worktree/connectionId yet to resolve through
// ConnectionResolver (a repo doesn't exist until one of these two calls
// creates it). Backed by infra-fleet-service's GetFleetHealth (per-dev-server
// reachability) — the closest existing read to "is this host up" without
// inventing a new infra-fleet-service RPC.
type DevServerReachability interface {
	IsReachable(ctx context.Context, devServerID string) (bool, error)
}

// AICompleter calls the Dev Server Agent's ai.complete method for
// GenerateCommitMessage (git-gateway-service.md §3.1: "this service never
// calls an LLM API directly"). Unlike GitExecutor there is no host-local
// implementation of this port — AI inference only ever runs on the Dev
// Server Agent's execution plane, reached through the same infra-fleet
// Relay RPC RelayExecutor already uses for git.* methods, so a worktree
// with no relay connection has no completion path at all (see
// GenerateCommitMessage's Execute for that decision).
type AICompleter interface {
	// Complete relays prompt to the Dev Server Agent's ai.complete method
	// over the connection identified by connectionID (a ResolvedConnection.
	// ConnectionID, not a repoPath — ai.complete has no filesystem path
	// argument) and returns the generated text.
	Complete(ctx context.Context, connectionID, prompt string) (string, error)
}

// AIProviderResolver resolves which AI provider account a tenant/user would
// use, by calling ai-provider-service's ResolveProvider RPC —
// DiscoverCommitMessageModels' only data source (TASK-211). Distinct from
// AICompleter (which relays the actual completion call through
// infra-fleet-service); this port talks to ai-provider-service directly,
// since account resolution is metadata, not an execution-plane call.
type AIProviderResolver interface {
	ResolveProvider(ctx context.Context, tenantID, userID string) (providerType, accountID, status string, err error)
}

// FilesystemExecutor performs file I/O against a resolved worktree path
// (TASK-050). Two implementations exist, selected by
// dispatchFilesystemExecutor the same way dispatchExecutor selects a
// GitExecutor:
//   - internal/adapter/localfs: real os-backed implementation, used when
//     ConnectionResolver reports Connected=false.
//   - internal/adapter/grpcclient's RelayExecutor: relays to the Dev Server
//     Agent's fs.* methods, used when Connected=true.
type FilesystemExecutor interface {
	ReadFile(ctx context.Context, repoPath, relPath string) ([]byte, error)
	ReadFilePreview(ctx context.Context, repoPath, relPath string, maxBytes int64) (content []byte, truncated bool, err error)
	ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error)
	WriteFile(ctx context.Context, repoPath, relPath string, content []byte, createParents bool) (bytesWritten int64, err error)
	WriteFileChunk(ctx context.Context, repoPath, relPath string, offsetBytes int64, content []byte, isFinal bool) (bytesWritten int64, err error)
	CreateDir(ctx context.Context, repoPath, relPath string, recursive, noClobber bool) error
	Delete(ctx context.Context, repoPath, relPath string, recursive bool) error
	Stat(ctx context.Context, repoPath, relPath string) (domain.FileStat, error)
	Search(ctx context.Context, repoPath string, opts domain.SearchOptions) ([]domain.SearchMatch, error)
	Glob(ctx context.Context, repoPath, pattern string, maxResults int) ([]string, error)
}

// LocalOnlyFilesystemExecutor covers Rename/Copy — BUG-009's known gap: the
// Dev Server Agent's fs.* surface implements
// stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep but not
// rename/copy. Only adapter/localfs implements this interface;
// adapter/grpcclient's RelayExecutor does not, so RenameFileUseCase/
// CopyFileUseCase (TASK-055) can compile-time-guarantee they never call a
// relay target.
type LocalOnlyFilesystemExecutor interface {
	Rename(ctx context.Context, repoPath, fromRel, toRel string) error
	Copy(ctx context.Context, repoPath, fromRel, toRel string) error
}

// dispatchFilesystemExecutor mirrors dispatchExecutor: resolve worktreeID's
// owning host, then return whichever FilesystemExecutor answers for it plus
// the resolved repo path and the raw ResolvedConnection (needed by
// ReadFileChunkUseCase/RenameFileUseCase/CopyFileUseCase to reject
// relay-only-unsupported operations before calling the executor at all).
func dispatchFilesystemExecutor(ctx context.Context, resolver ConnectionResolver, local, relay FilesystemExecutor, worktreeID string) (FilesystemExecutor, ResolvedConnection, error) {
	conn, err := resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return nil, ResolvedConnection{}, err
	}
	if conn.Connected {
		return relay, conn, nil
	}
	return local, conn, nil
}

// ProjectClient wraps project-service's worktree-bookkeeping RPCs — a new
// outbound call this service didn't previously need to make for writes
// (git-gateway-service.md §7 lists "Calls project-service" for reads only
// today). Implemented by internal/adapter/grpcclient against
// project-service's existing RecordWorktreeCreated/RecordWorktreeRemoved
// RPCs (no project-service proto change — those RPCs already exist).
//
// GetRepo is this task's own addition on top of that existing surface: see
// its doc comment on domain.RepoInfo and internal/adapter/grpcclient's
// project_client.go for the confirmed proto gap — project.proto has no
// single-repo-by-id lookup RPC (only ListRepos(project_id)), so today's
// grpcclient implementation returns a typed, catchable error rather than a
// real answer, until project-service grows one.
type ProjectClient interface {
	GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error)
	RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch string) (domain.WorktreeRecord, error)
	RecordWorktreeRemoved(ctx context.Context, worktreeID string) error
}

// ScrollbackCleaner wraps infra-fleet-service's DeleteTerminalScrollbackSnapshots
// RPC — called best-effort by RemoveWorktree; see that usecase's doc comment.
type ScrollbackCleaner interface {
	DeleteTerminalScrollbackSnapshots(ctx context.Context, worktreeID string) error
}

// SCMClient wraps scm-integration-service's PR/MR base-branch lookups — a
// new outbound dependency edge git-gateway-service --> scm-integration-service
// that git-gateway-service.md §7's current dependency list (project-service,
// infra-fleet-service only) doesn't yet document, flagged here as a scope
// addition (SOL-031).
//
// KNOWN GAP: scm-integration-service's current proto
// (proto/orca/scmintegration/v1/scmintegration.proto) has NO RPC to fetch
// a single PR/MR's base branch by number — only ListPullRequests/
// CreatePullRequest/ListIssues exist. internal/adapter/scmclient is
// therefore wired against a port that has no real backing RPC yet; its
// implementation returns a typed apperrors.KindInternal error until
// scm-integration-service adds one (out of this task's scope — a
// follow-up proto task, not assumed here).
type SCMClient interface {
	GetPullRequestBase(ctx context.Context, repoID string, prNumber int32) (baseBranch, baseSHA string, err error)
	GetMergeRequestBase(ctx context.Context, repoID string, mrNumber int32) (baseBranch, baseSHA string, err error)
}

// dispatchExecutor is the resolve-and-dispatch logic every RPC-shaped
// usecase in this package shares: ask ConnectionResolver which host owns
// worktreeID, then return whichever GitExecutor answers for that host plus
// the resolved repo path to operate against. Centralized here so the
// routing behavior — connected=false -> local, connected=true -> relay — is
// implemented and tested exactly once.
//
// worktreeID here is also reused, unchanged, as the dispatch key for the
// repo-scoped worktree usecases (CreateWorktree, DetectWorktrees,
// PrefetchCreateBase, ResolvePrBase, ResolveMrBase) — see those usecases'
// doc comments for why passing a bare, caller-supplied repoID straight
// through here (without first confirming it via ProjectClient.GetRepo)
// would silently conflate a repo id with a worktree/connection id;
// resolved by having each of them call ProjectClient.GetRepo first and
// pass its echoed-back repo.ID into dispatchExecutor, the same shape
// CreateWorktree's own uc.projects.GetRepo call establishes, rather than
// forwarding the raw request field.
func dispatchExecutor(ctx context.Context, resolver ConnectionResolver, local, relay GitExecutor, worktreeID string) (GitExecutor, string, error) {
	conn, err := resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return nil, "", err
	}
	if conn.Connected {
		return relay, conn.RepoPath, nil
	}
	return local, conn.RepoPath, nil
}
