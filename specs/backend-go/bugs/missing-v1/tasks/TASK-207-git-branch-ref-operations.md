# TASK-207: Add Group A — branch/ref RPCs to `git-gateway-service` (9 methods)

**From Solution:** SOL-032 (Part 2, Group A)
**Priority:** P1 — unblocks the core branch workflow, ship before Groups C/D/E
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `internal/domain/domain.go`, `internal/usecase/ports.go`, `internal/usecase/checkout.go` (+8 new usecase files), `internal/adapter/localgit/executor.go`, `internal/adapter/grpcclient/relay_executor.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-227 (agent reachability) — all 9 methods in this group
are currently unreachable on the agent process backend-go's transport
actually reaches (SOL-032 §0), so none of this group's relay legs will
produce a working result until TASK-227 lands; this task builds NEW RPCs,
so unlike TASK-206 (which had existing, already-shipped usecases to wrap)
there is nothing here "already implemented" to fall back on in the
meantime. TASK-207 and TASK-227 can still be developed in parallel since
they touch disjoint files. Otherwise independent of TASK-208/209/210/211 —
all extend the same `GitExecutor` interface and can land in any order, but
each PR must rebase onto whichever of these has already merged since they
all touch `ports.go`.
**Status:** `[x]` DONE — all 9 methods implemented (10 RPCs on the wire: `ConflictOperation` was split into a detector RPC + a new `ResolveConflict` RPC, see below). The 5 mechanical-rename methods (`AbortRebase`/`AbortMerge`/`Discard`/`BulkDiscard`/`RebaseFromBase`) are implemented exactly as this doc's own Steps 1-8 code blocks specify. The 4 originally-BLOCKED methods were redesigned against real agent evidence, not guessed:
- **`Checkout`** — `create` field dropped entirely (real `git.checkout` has no `-b` semantics: `agent/src/relay/git-handler.ts:702-719`, `specs/agent/api/agent-rpc-catalog-git-fs.md:132`); field renamed `ref`→`branch` to match the real param name.
- **`ListLocalBranches`** — kept the richer `BranchInfo` shape (upstream/ahead/behind), composed by `RelayExecutor` via `git.exec`'s `for-each-ref` subcommand instead of calling the real, narrower `git.localBranches` directly — confirmed `for-each-ref` has "no extra restriction" on Part B's exec whitelist (`agent-rpc-catalog-git-fs.md:203-206`), the whitelist this service's SSH-relay calls actually reach. Found and fixed a related bug while writing this: `strings.TrimSpace(out)` before line-splitting silently drops the alphabetically-last branch when it isn't the current one (its empty `%(HEAD)` trailing field gets eaten too) — fixed in both `localgit.Executor` and `RelayExecutor`.
- **`FastForward`** — added a real `domain.PushTargetInput`/`gitgateway.proto`'s `PushTargetInput` message modeled directly on the real agent's `GitPushTarget` (`agent/src/shared/types.ts:551-557`, validated by `agent/src/shared/git-push-target-validation.ts`), optional (nil = agent resolves the configured push target, matching `resolveRelayPushTarget`'s undefined branch, `agent/src/relay/git-handler-push-target.ts:159-166`). Also switched the local implementation from a bare `git merge --ff-only <branch>` to `git pull --ff-only [<remote> <branch>]`, matching the real agent's `pullWithArgs(['--ff-only'])` (`agent-rpc-catalog-git-fs.md:160`, `git-handler.ts:1190-1192`) instead of the original sketch's local-merge assumption. This `PushTargetInput` type is reusable for `Push`/`Pull`/`Fetch`'s same open pushTarget question (`relay_executor.go`'s existing "KNOWN LIMITATION" comments) but this task only wires it into `FastForward`.
- **`ConflictOperation`** — split into two RPCs: `ConflictOperation` is now a real DETECTOR ONLY (`worktree_id` → `'merge'|'rebase'|'cherry-pick'|'unknown'`), matching the real agent exactly (`git-handler.ts:886-889`, `git-handler-status-ops.ts:38-57`, `agent-rpc-catalog-git-fs.md:136`); a new `ResolveConflict` RPC covers the per-file ours/theirs/markResolved op the original design conflated with the detector. Confirmed Part B's `git.exec` whitelist explicitly excludes both `checkout` and `add` (`agent-rpc-catalog-git-fs.md:203-227`'s "Not allowed at all" list), so `ResolveConflict` cannot be composed over a relay connection at all — `RelayExecutor.ResolveConflict` returns a typed `domain.ErrConflictResolveUnsupportedOverRelay` (mapped to `FailedPrecondition`), following `domain.ErrForceDeleteBranchUnsupported`'s established pattern; `localgit.Executor.ResolveConflict` does real work (this doc's original Step 5 code, unchanged). No confirmed live frontend caller was found for the per-file resolve op in today's codebase (`backend/src/main/providers/*.ts` has no ours/theirs/markResolved call site) — this is a forward-looking surface for the source-control UI to grow into, not a fix for an existing broken call site.

All citations verified directly against `specs/agent/api/agent-rpc-catalog-git-fs.md` and the actual `agent/src/relay/git-handler.ts`, `agent/src/relay/git-handler-status-ops.ts`, `agent/src/relay/git-handler-push-target.ts`, `agent/src/shared/types.ts` source (not guessed). `go build`/`go vet`/`go test` clean on `git-gateway-service`; `buf generate proto` regenerated stubs successfully. `buf breaking` was not run against `main` — `gitgateway.proto` does not exist on `main` at all yet (git-gateway-service is new, unmerged work on this integration branch), so there is nothing to diff against. TASK-212's Group-A channel wiring is now wired too (see that task's own updated status).

---

## ⚠️ Contract correction (read before implementing)

SOL-032 §0 traced this group's 9 methods against the REAL agent contract
(`specs/agent/api/agent-rpc-catalog-git-fs.md`) and found the original
design below is only accurate for 5 of them. Do not implement any of the
4 BLOCKED methods as originally designed below without resolving the
cited open question first.

**Mechanical param-rename only (safe to implement as designed, once
TASK-227 lands) — fixed directly in this file's `RelayExecutor` code
blocks below:**

- `abortRebase` / `abortMerge` — `repoPath` → `worktreePath` only. Closest
  fixes in the whole set.
- `discard` — `repoPath` → `worktreePath`; `path` → `filePath`.
- `bulkDiscard` — `repoPath` → `worktreePath`; `paths` → `filePaths`.
- `rebaseFromBase` — `repoPath` → `worktreePath`; `baseBranch` → `baseRef`.

**BLOCKED — needs a genuine shape redesign, not a rename (flagged inline
below with `⚠️ BLOCKED` comments, NOT fixed — do not implement as
originally designed without resolving these first):**

- `checkout` — the real `git.checkout` has no `create`-branch (`-b`)
  semantics at all. This task's `create: bool` field/behavior needs to be
  redesigned (e.g. compose as two calls, or drop `create` from this RPC's
  scope entirely) before implementing. Flagged in SOL-032 §0's correction
  table (not one of the 4 numbered open design questions below, but still
  not safe to implement as-is).
- `localBranches` — **SOL-032 §0 open question #4.** Real agent response
  is `{current, branches[]}` (branch names only) vs. this task's much
  richer `BranchInfo{name, upstream, ahead, behind, is_current,
  is_remote}`. Decide whether to narrow the Go type to match, or compose
  the richer data via `git.exec`'s `for-each-ref` (present in Part A's
  whitelist but NOT Part B's stricter one — verify which whitelist this
  request actually reaches before choosing that path).
- `fastForward` — **SOL-032 §0 open question #1** (the same `pushTarget`
  question that blocks `push`/`pull`/`fetch`). Real `git.fastForward`
  takes an optional structured `pushTarget`, not this task's plain
  `branch` string. Read `git-handler-push-target.ts` and design
  `PushTargetInput` properly before implementing — don't guess a
  passthrough shape.
- `conflictOperation` — **SOL-032 §0 open question #2, the deepest
  mismatch in this group.** The real `git.conflictOperation` is a
  DETECTOR ONLY: `worktreePath` → `'merge'|'rebase'|'cherry-pick'|
  'unknown'`. It does not resolve individual conflicted files. This
  task's entire `ConflictOperationRequest`/response design below (and its
  `localgit.Executor.ConflictOperation` implementation, which executes
  ours/theirs/markResolved) assumes a fundamentally different operation
  that does not exist on the real agent. This needs either (a) composing
  the resolve step via `git.exec`'s whitelisted `checkout` subcommand, or
  (b) a genuinely new agent-side RPC beyond TASK-227's scope — a real
  design decision, not something to fabricate here. Do not implement this
  RPC as designed below until that decision is made.

---

## Context

`checkout`/`localBranches`/`fastForward`/`rebaseFromBase`/`abortRebase`/
`abortMerge`/`conflictOperation`/`discard`/`bulkDiscard` have no backing RPC
today (BUG-032). This task extends the existing `GitExecutor` port
(`ports.go:61-67`) and its two implementations
(`localgit.Executor`, `grpcclient.RelayExecutor`) with these 9 methods,
following `Commit`'s exact shape (`commit.go`, `executor.go:75-95`,
`relay_executor.go:109-117`) — no new dispatch mechanism, per SOL-032's
explicit instruction not to invent a second one.

## Changes to make

### Step 1: Proto — `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`

Add to the `GitGatewayService` service block:

```protobuf
  rpc Checkout(CheckoutRequest) returns (CheckoutResponse);
  rpc ListLocalBranches(ListLocalBranchesRequest) returns (ListLocalBranchesResponse);
  rpc FastForward(FastForwardRequest) returns (FastForwardResponse);
  rpc RebaseFromBase(RebaseFromBaseRequest) returns (RebaseFromBaseResponse);
  rpc AbortRebase(AbortRebaseRequest) returns (AbortRebaseResponse);
  rpc AbortMerge(AbortMergeRequest) returns (AbortMergeResponse);
  rpc ConflictOperation(ConflictOperationRequest) returns (ConflictOperationResponse);
  rpc Discard(DiscardRequest) returns (DiscardResponse);
  rpc BulkDiscard(BulkDiscardRequest) returns (BulkDiscardResponse);
```

Append messages to the bottom of the file:

```protobuf
// worktree_id + ref (branch/tag/commit-ish) + create (git checkout -b
// semantics). Collapses git-gateway-service.md §3's SwitchBranch/CreateBranch
// pair into one RPC since the frontend's checkout call already takes a
// create flag (rpc-catalog.md:196-234) rather than being two calls.
message CheckoutRequest {
  string worktree_id = 1;
  string ref = 2;
  bool create = 3;
}
message CheckoutResponse {
  bool success = 1;
  string branch = 2; // resolved current branch after the operation
}

message BranchInfo {
  string name = 1;
  string upstream = 2; // empty = no upstream configured
  int32 ahead = 3;
  int32 behind = 4;
  bool is_current = 5;
  bool is_remote = 6;
}
message ListLocalBranchesRequest {
  string worktree_id = 1;
}
message ListLocalBranchesResponse {
  repeated BranchInfo branches = 1;
}

message FastForwardRequest {
  string worktree_id = 1;
  string branch = 2;
}
message FastForwardResponse {
  bool success = 1;
}

message RebaseFromBaseRequest {
  string worktree_id = 1;
  string base_branch = 2;
}
message RebaseFromBaseResponse {
  bool success = 1;
  bool had_conflicts = 2;
}

message AbortRebaseRequest {
  string worktree_id = 1;
}
message AbortRebaseResponse {
  bool success = 1;
}

// AbortMerge matches git-gateway-service.md §3's own AbortMerge name exactly.
message AbortMergeRequest {
  string worktree_id = 1;
}
message AbortMergeResponse {
  bool success = 1;
}

// ConflictOperation collapses TS's ResolveConflict-per-file into one
// op-typed RPC matching the frontend's actual single-call shape.
message ConflictOperationRequest {
  string worktree_id = 1;
  string path = 2;
  string operation = 3; // "ours" | "theirs" | "markResolved"
}
message ConflictOperationResponse {
  bool success = 1;
}

message DiscardRequest {
  string worktree_id = 1;
  string path = 2;
}
message DiscardResponse {
  bool success = 1;
}

// BulkDiscard reports partial failure rather than all-or-nothing, since a
// mixed batch (some tracked, some already-clean) is a real case.
message BulkDiscardRequest {
  string worktree_id = 1;
  repeated string paths = 2;
}
message BulkDiscardResponse {
  bool success = 1;
  repeated string failed_paths = 2;
}
```

### Step 2: Domain value objects — `internal/domain/domain.go`

Append:

```go
// BranchInfo is one local branch's tracking state, returned by
// ListLocalBranches. Mirrors gitgateway.proto's BranchInfo message 1:1.
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

// FastForwardResult reflects whether a FastForward operation succeeded.
type FastForwardResult struct {
	Success bool
}

// RebaseResult reflects whether a RebaseFromBase operation succeeded, and
// whether it left the worktree with unresolved conflicts — mirrors
// PullResult's Success/HadConflicts shape for the same reason (a conflict
// is a real domain outcome, not a Go error; see executor.go's Pull doc
// comment).
type RebaseResult struct {
	Success      bool
	HadConflicts bool
}

// SimpleResult is a bare success flag, shared by every branch/ref op below
// that has no richer outcome to report (AbortRebase, AbortMerge,
// ConflictOperation, Discard) — kept as one type rather than four
// single-field structs since none of them carries op-specific state.
type SimpleResult struct {
	Success bool
}

// BulkDiscardResult reports partial failure across a multi-path discard —
// see BulkDiscardRequest's proto doc comment for why this isn't
// all-or-nothing.
type BulkDiscardResult struct {
	Success     bool
	FailedPaths []string
}
```

### Step 3: Extend `GitExecutor` — `internal/usecase/ports.go`

Add to the `GitExecutor` interface (after `Pull`):

```go
	Checkout(ctx context.Context, repoPath, ref string, create bool) (domain.CheckoutResult, error)
	ListLocalBranches(ctx context.Context, repoPath string) ([]domain.BranchInfo, error)
	FastForward(ctx context.Context, repoPath, branch string) (domain.FastForwardResult, error)
	RebaseFromBase(ctx context.Context, repoPath, baseBranch string) (domain.RebaseResult, error)
	AbortRebase(ctx context.Context, repoPath string) (domain.SimpleResult, error)
	AbortMerge(ctx context.Context, repoPath string) (domain.SimpleResult, error)
	ConflictOperation(ctx context.Context, repoPath, path, operation string) (domain.SimpleResult, error)
	Discard(ctx context.Context, repoPath, path string) (domain.SimpleResult, error)
	BulkDiscard(ctx context.Context, repoPath string, paths []string) (domain.BulkDiscardResult, error)
```

### Step 4: Usecase files — `internal/usecase/`

`checkout.go` (identical shape to `commit.go`):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CheckoutInput struct {
	WorktreeID string
	Ref        string
	Create     bool
}

type Checkout struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCheckout(resolver ConnectionResolver, local, relay GitExecutor) *Checkout {
	return &Checkout{resolver: resolver, local: local, relay: relay}
}

func (uc *Checkout) Execute(ctx context.Context, in CheckoutInput) (domain.CheckoutResult, error) {
	if in.WorktreeID == "" {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.Ref == "" {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REF", "ref is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.Checkout(ctx, repoPath, in.Ref, in.Create)
	if err != nil {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CHECKOUT_FAILED", "failed to checkout ref", err)
	}
	return result, nil
}
```

`list_local_branches.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ListLocalBranchesInput struct {
	WorktreeID string
}

type ListLocalBranches struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewListLocalBranches(resolver ConnectionResolver, local, relay GitExecutor) *ListLocalBranches {
	return &ListLocalBranches{resolver: resolver, local: local, relay: relay}
}

func (uc *ListLocalBranches) Execute(ctx context.Context, in ListLocalBranchesInput) ([]domain.BranchInfo, error) {
	if in.WorktreeID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	branches, err := executor.ListLocalBranches(ctx, repoPath)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_LIST_BRANCHES_FAILED", "failed to list local branches", err)
	}
	return branches, nil
}
```

`fast_forward.go`, `rebase_from_base.go`, `abort_rebase.go`, `abort_merge.go`,
`conflict_operation.go`, `discard.go`, `bulk_discard.go` follow the exact
same shape as `checkout.go`/`list_local_branches.go` — validate
`WorktreeID` (plus any other required field), `dispatchExecutor`, call the
matching `GitExecutor` method, wrap errors with a
`GITGATEWAY_<OP>_FAILED` code. Example for `bulk_discard.go` (the one with
a slightly richer input):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type BulkDiscardInput struct {
	WorktreeID string
	Paths      []string
}

type BulkDiscard struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewBulkDiscard(resolver ConnectionResolver, local, relay GitExecutor) *BulkDiscard {
	return &BulkDiscard{resolver: resolver, local: local, relay: relay}
}

func (uc *BulkDiscard) Execute(ctx context.Context, in BulkDiscardInput) (domain.BulkDiscardResult, error) {
	if in.WorktreeID == "" {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if len(in.Paths) == 0 {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATHS", "paths is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.BulkDiscard(ctx, repoPath, in.Paths)
	if err != nil {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_BULK_DISCARD_FAILED", "failed to discard paths", err)
	}
	return result, nil
}
```

### Step 5: `localgit.Executor` — `internal/adapter/localgit/executor.go`

Append (Git 2.5+ commands only, no `GitCapabilityCache` needed per this
package's existing baseline note — confirm each against
`docs/reference/git-compatibility.md` before merging):

```go
// Checkout runs `git checkout <ref>` or, when create is set, `git checkout
// -b <ref>` — both available since Git 2.5.
func (e *Executor) Checkout(ctx context.Context, repoPath, ref string, create bool) (domain.CheckoutResult, error) {
	args := []string{"checkout"}
	if create {
		args = append(args, "-b")
	}
	args = append(args, ref)
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.CheckoutResult{}, err
	}
	branch, err := e.run(ctx, repoPath, "branch", "--show-current")
	if err != nil {
		return domain.CheckoutResult{}, err
	}
	return domain.CheckoutResult{Success: true, Branch: strings.TrimSpace(branch)}, nil
}

// ListLocalBranches runs `git for-each-ref` against refs/heads with a
// machine-parseable format — ahead/behind and upstream come from
// %(upstream:short)/%(upstream:track) tokens.
func (e *Executor) ListLocalBranches(ctx context.Context, repoPath string) ([]domain.BranchInfo, error) {
	out, err := e.run(ctx, repoPath,
		"for-each-ref", "--format=%(refname:short)\t%(upstream:short)\t%(upstream:track)\t%(HEAD)",
		"refs/heads/")
	if err != nil {
		return nil, err
	}
	var branches []domain.BranchInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		ahead, behind := parseAheadBehind(fields[2])
		branches = append(branches, domain.BranchInfo{
			Name:      fields[0],
			Upstream:  fields[1],
			Ahead:     ahead,
			Behind:    behind,
			IsCurrent: fields[3] == "*",
		})
	}
	return branches, nil
}

// FastForward runs `git merge --ff-only <branch>`.
func (e *Executor) FastForward(ctx context.Context, repoPath, branch string) (domain.FastForwardResult, error) {
	if _, err := e.run(ctx, repoPath, "merge", "--ff-only", branch); err != nil {
		return domain.FastForwardResult{}, err
	}
	return domain.FastForwardResult{Success: true}, nil
}

// RebaseFromBase runs `git rebase <baseBranch>`. A conflict (nonzero exit
// with rebase state left behind) is a domain outcome, not a Go error — same
// posture as Pull's conflict handling above.
func (e *Executor) RebaseFromBase(ctx context.Context, repoPath, baseBranch string) (domain.RebaseResult, error) {
	out, err := e.run(ctx, repoPath, "rebase", baseBranch)
	if err != nil {
		if strings.Contains(out, "CONFLICT") || strings.Contains(err.Error(), "CONFLICT") {
			return domain.RebaseResult{Success: false, HadConflicts: true}, nil
		}
		return domain.RebaseResult{}, err
	}
	return domain.RebaseResult{Success: true}, nil
}

// AbortRebase runs `git rebase --abort`.
func (e *Executor) AbortRebase(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	if _, err := e.run(ctx, repoPath, "rebase", "--abort"); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// AbortMerge runs `git merge --abort`.
func (e *Executor) AbortMerge(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	if _, err := e.run(ctx, repoPath, "merge", "--abort"); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// ConflictOperation resolves one conflicted path: "ours"/"theirs" runs
// `git checkout --ours|--theirs -- <path>` then re-stages it (the checkout
// alone only updates the worktree copy); "markResolved" just stages the
// path as-is (the caller already edited it by hand).
func (e *Executor) ConflictOperation(ctx context.Context, repoPath, path, operation string) (domain.SimpleResult, error) {
	switch operation {
	case "ours":
		if _, err := e.run(ctx, repoPath, "checkout", "--ours", "--", path); err != nil {
			return domain.SimpleResult{}, err
		}
	case "theirs":
		if _, err := e.run(ctx, repoPath, "checkout", "--theirs", "--", path); err != nil {
			return domain.SimpleResult{}, err
		}
	case "markResolved":
		// no worktree change — the caller already resolved the content.
	default:
		return domain.SimpleResult{}, fmt.Errorf("localgit: unknown conflict operation %q", operation)
	}
	if _, err := e.run(ctx, repoPath, "add", "--", path); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// Discard restores a tracked path (`git checkout -- <path>`) or removes an
// untracked one (`git clean -f -- <path>`), mirroring TS git.discard's
// untracked-file handling. Which case applies is determined by asking `git
// status --porcelain` for that single path first.
func (e *Executor) Discard(ctx context.Context, repoPath, path string) (domain.SimpleResult, error) {
	out, err := e.run(ctx, repoPath, "status", "--porcelain=v1", "--", path)
	if err != nil {
		return domain.SimpleResult{}, err
	}
	if strings.HasPrefix(strings.TrimSpace(out), "??") {
		if _, err := e.run(ctx, repoPath, "clean", "-f", "--", path); err != nil {
			return domain.SimpleResult{}, err
		}
		return domain.SimpleResult{Success: true}, nil
	}
	if _, err := e.run(ctx, repoPath, "checkout", "--", path); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// BulkDiscard calls Discard per path, collecting failures rather than
// stopping at the first one — see BulkDiscardResult's doc comment.
func (e *Executor) BulkDiscard(ctx context.Context, repoPath string, paths []string) (domain.BulkDiscardResult, error) {
	var failed []string
	for _, p := range paths {
		if _, err := e.Discard(ctx, repoPath, p); err != nil {
			failed = append(failed, p)
		}
	}
	return domain.BulkDiscardResult{Success: len(failed) == 0, FailedPaths: failed}, nil
}

// parseAheadBehind parses %(upstream:track)'s "[ahead N, behind M]" (or
// "[ahead N]" / "[behind M]" / "") format.
func parseAheadBehind(track string) (ahead, behind int) {
	track = strings.Trim(track, "[]")
	for _, part := range strings.Split(track, ",") {
		part = strings.TrimSpace(part)
		var n int
		if _, err := fmt.Sscanf(part, "ahead %d", &n); err == nil {
			ahead = n
		}
		if _, err := fmt.Sscanf(part, "behind %d", &n); err == nil {
			behind = n
		}
	}
	return ahead, behind
}
```

Add `"fmt"` to the existing import block if not already present (it already
is, per `executor.go:18`'s current imports).

### Step 6: `RelayExecutor` — `internal/adapter/grpcclient/relay_executor.go`

Append, following `Commit`'s exact `relay()`-call shape:

```go
// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.checkout has no create-branch (-b) semantics at all. The `create`
// param below does not exist on the real agent contract; this needs a
// redesign (compose as two calls, or drop create from this RPC's scope)
// before implementing, not a param rename.
func (r *RelayExecutor) Checkout(ctx context.Context, repoPath, ref string, create bool) (domain.CheckoutResult, error) {
	var result domain.CheckoutResult
	err := r.relay(ctx, repoPath, "git.checkout", map[string]any{
		"repoPath": repoPath, "ref": ref, "create": create,
	}, &result)
	return result, err
}

// ⚠️ BLOCKED — see this task's Contract correction section and SOL-032 §0
// open question #4: real git.localBranches responds `{current, branches[]}`
// (names only), not this richer per-branch BranchInfo with
// upstream/ahead/behind. Narrow the type or compose via git.exec's
// for-each-ref before implementing this relay call as-is.
func (r *RelayExecutor) ListLocalBranches(ctx context.Context, repoPath string) ([]domain.BranchInfo, error) {
	var result struct {
		Branches []domain.BranchInfo `json:"branches"`
	}
	err := r.relay(ctx, repoPath, "git.localBranches", map[string]any{"repoPath": repoPath}, &result)
	return result.Branches, err
}

// ⚠️ BLOCKED — see this task's Contract correction section and SOL-032 §0
// open question #1: real git.fastForward takes an optional structured
// pushTarget, not this plain branch string. Do not implement this shape
// until PushTargetInput is designed from git-handler-push-target.ts.
func (r *RelayExecutor) FastForward(ctx context.Context, repoPath, branch string) (domain.FastForwardResult, error) {
	var result domain.FastForwardResult
	err := r.relay(ctx, repoPath, "git.fastForward", map[string]any{
		"repoPath": repoPath, "branch": branch,
	}, &result)
	return result, err
}

func (r *RelayExecutor) RebaseFromBase(ctx context.Context, repoPath, baseRef string) (domain.RebaseResult, error) {
	var result domain.RebaseResult
	err := r.relay(ctx, repoPath, "git.rebaseFromBase", map[string]any{
		"worktreePath": repoPath, "baseRef": baseRef,
	}, &result)
	return result, err
}

func (r *RelayExecutor) AbortRebase(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.abortRebase", map[string]any{"worktreePath": repoPath}, &result)
	return result, err
}

func (r *RelayExecutor) AbortMerge(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.abortMerge", map[string]any{"worktreePath": repoPath}, &result)
	return result, err
}

// ⚠️ BLOCKED — see this task's Contract correction section and SOL-032 §0
// open question #2: real git.conflictOperation is a DETECTOR ONLY
// (worktreePath → 'merge'|'rebase'|'cherry-pick'|'unknown'). It does not
// resolve individual conflicted files as this {path, operation} shape
// assumes. Do not implement this relay call, or localgit.Executor's
// matching ConflictOperation below, until that design decision is made.
func (r *RelayExecutor) ConflictOperation(ctx context.Context, repoPath, path, operation string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.conflictOperation", map[string]any{
		"repoPath": repoPath, "path": path, "operation": operation,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Discard(ctx context.Context, repoPath, filePath string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.discard", map[string]any{
		"worktreePath": repoPath, "filePath": filePath,
	}, &result)
	return result, err
}

func (r *RelayExecutor) BulkDiscard(ctx context.Context, repoPath string, filePaths []string) (domain.BulkDiscardResult, error) {
	var result domain.BulkDiscardResult
	err := r.relay(ctx, repoPath, "git.bulkDiscard", map[string]any{
		"worktreePath": repoPath, "filePaths": filePaths,
	}, &result)
	return result, err
}
```

These 9 relay method names/param shapes are best-effort, same caveat this
file's existing doc comment (`relay_executor.go:19-35`) already flags —
verify against `specs/agent/api/agent-rpc-catalog-git-fs.md` before
finalizing.

### Step 7: gRPC adapter — `internal/adapter/grpc/server.go`

Add 9 fields to `Server`, 9 params to `New`, and 9 translation methods
following `Commit`'s exact shape (`server.go:67-77`). Example for
`Checkout`:

```go
func (s *Server) Checkout(ctx context.Context, req *gitgatewayv1.CheckoutRequest) (*gitgatewayv1.CheckoutResponse, error) {
	result, err := s.checkout.Execute(ctx, usecase.CheckoutInput{
		WorktreeID: req.GetWorktreeId(), Ref: req.GetRef(), Create: req.GetCreate(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CheckoutResponse{Success: result.Success, Branch: result.Branch}, nil
}
```

Repeat for the other 8 (`ListLocalBranches` translates `[]domain.BranchInfo`
to `[]*gitgatewayv1.BranchInfo` with a small `toProtoBranches` helper,
mirroring `toProtoFileStatuses`).

### Step 8: Composition root — `cmd/server/main.go`

After the existing `generateCommitMessageUC := ...` line, add:

```go
	checkoutUC := usecase.NewCheckout(resolver, local, relay)
	listLocalBranchesUC := usecase.NewListLocalBranches(resolver, local, relay)
	fastForwardUC := usecase.NewFastForward(resolver, local, relay)
	rebaseFromBaseUC := usecase.NewRebaseFromBase(resolver, local, relay)
	abortRebaseUC := usecase.NewAbortRebase(resolver, local, relay)
	abortMergeUC := usecase.NewAbortMerge(resolver, local, relay)
	conflictOperationUC := usecase.NewConflictOperation(resolver, local, relay)
	discardUC := usecase.NewDiscard(resolver, local, relay)
	bulkDiscardUC := usecase.NewBulkDiscard(resolver, local, relay)
```

And extend the `gitgatewaygrpc.New(...)` call with all 9 new usecases,
matching `Server`'s new constructor param order from Step 7.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/git-gateway-service
go build ./... && go vet ./...
```

Expected: clean build, `buf breaking` reports only additions. Full test
coverage for this group is TASK-214/TASK-215/TASK-216, not this
task.

**This does not confirm any of these 9 RPCs work end-to-end for a
relay-dispatched (SSH-connected) worktree** — a clean build only confirms
the Go compiles, not that the relay calls succeed against a real agent.
That requires TASK-227 (agent reachability) to land first, and — for
`checkout`, `localBranches`, `fastForward`, and `conflictOperation`
specifically — the open design questions in this task's Contract
correction section to be resolved first. Local (unconnected) worktrees
are unaffected by the reachability gap but still blocked on those same 4
methods' design questions.
