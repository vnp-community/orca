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
	GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error)
	Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
	Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
	Pull(ctx context.Context, repoPath string) (domain.PullResult, error)
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
