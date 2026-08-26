# TASK-209: Add Group C — history/compare RPCs to `git-gateway-service` (9 methods)

**From Solution:** SOL-032 (Part 2, Group C)
**Priority:** P0/P1 — **reclassified up from P2 per SOL-032's revised
phased rollout**: 8 of this group's 9 methods (`history`, `checkIgnored`,
`forkSync`, `commitCompare`, `branchCompare`, `commitDiff`, `branchDiff`,
`submoduleStatus`) already had a dedicated Dev Server Agent RPC handler in
the old TS backend and are reachable on Part A **today, with no TASK-227
dependency** — making this the highest-value, lowest-risk group in the
whole proposal for the 4 methods that are also shape-correct (see Contract
correction below). Only `upstreamStatus` needs TASK-227.
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `internal/domain/domain.go`, `internal/usecase/ports.go`, `internal/usecase/history.go` (+8 new usecase files), `internal/adapter/localgit/executor.go`, `internal/adapter/grpcclient/relay_executor.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-227 only for `upstreamStatus` (not among the 8
already-reachable methods); the other 8 methods (`history`, `checkIgnored`,
`forkSync`, `commitCompare`, `branchCompare`, `commitDiff`, `branchDiff`,
`submoduleStatus`) have no reachability dependency and can ship as soon as
their own shape issues (see Contract correction below) are resolved.
Otherwise touches the same shared files as TASK-207/208/210/211 — rebase
onto whichever has already merged.
**Status:** `[x]` DONE. All 9 methods are now implemented for real. The 4 previously-shippable methods (`history`, `checkIgnored`, `forkSync`, `upstreamStatus`) are unchanged except `upstreamStatus`, whose `pushTarget` placeholder string is now the real, structured `domain.PushTargetInput`/proto `PushTargetInput` (TASK-207's type) instead of a bare string — resolving this task's own open question #1 the same way `FastForward` already had it resolved; `upstreamStatus`'s relay reachability is also now unblocked since TASK-227 landed. The 5 previously ❌ BLOCKED methods are implemented for real against the confirmed agent contract (`specs/agent/api/agent-rpc-catalog-git-fs.md:49-55/143-147`, `agent/src/relay/git-handler-commit-diff-ops.ts`, `agent/src/relay/git-handler-ops.ts`, `agent/src/relay/agent-git-handler-extended.ts`): `commitCompare`(`worktreePath,commitId`→`{summary,entries[]}`)/`branchCompare`(`worktreePath,baseRef`→`{summary,entries[]}`) diff against ONE fixed ref (a commit's own parent, or HEAD's merge-base with baseRef), never two arbitrary refs; `commitDiff`/`branchDiff` are per-file (`filePath` required, `parentOid`/`oldPath` optional) returning a `{kind,originalContent,modifiedContent,...}` diff-result object — a new `FileDiffResponse`/`domain.FileDiffResult` type, NOT `GetDiffResponse`'s `unified_diff` (the original sketch's response-shape assumption didn't survive contact with the real `buildDiffResult` shape); `submoduleStatus` is per-SINGLE-submodule (`worktreePath,submodulePath,area?`) — SOL-032 §0 open question #3 fully resolved (not just worked around) by reading the real frontend caller (`frontend/src/renderer/src/runtime/runtime-git-client.ts:152-176`, `useSourceControlSubmoduleStatus.ts`): the frontend already knows every dirty submodule's path from the parent `git.status` response and never calls a "list every submodule" API, so no aggregation step was needed at all. Proto, domain types (`GitChangeEntry`/`CommitCompareResult`/`BranchCompareResult`/`FileDiffResult`), usecases, `localgit.Executor` (real git-binary, exercised against 13 new tests incl. root-commit/invalid-ref/flag-injection-guard edge cases), `RelayExecutor`, gRPC adapter, and `main.go` wiring all done; `go build`/`go vet`/`go test` clean across `localgit`/`grpcclient`/`grpc`/`usecase`.

---

## ⚠️ Contract correction (read before implementing)

SOL-032 §0 traced this group's 9 methods against the REAL agent contract
(`specs/agent/api/agent-rpc-catalog-git-fs.md`). Unlike Group A, most of
this group's reachability is already fine — but **5 of the 9 methods
still need a genuine shape redesign**, not a mechanical fix. To keep the
4 genuinely shippable-now methods clear of the 5 broken ones, they're
split into two clearly separated subsections below.

Per this task's per-file assignment, the shape-redesign methods stay in
this same file (not split into a separate task file) — kept together with
very clear visual separation, since these 3 corrections instructions
explicitly say not to create new planning-doc files.

### ✅ Shippable now (mechanical fixes only, no TASK-227 dependency except where noted)

- **`history`** — drop the `cursor`/pagination field entirely; the real
  agent has no pagination concept. Rename `ref` → `baseRef` (the field the
  real agent actually reads). Fixed directly below (proto, usecase, and
  `RelayExecutor.History`).
- **`checkIgnored`** — real agent response is `string[]` of just the
  ignored paths, not `{path, ignored}` for every input path. Fixed
  directly below (proto `CheckIgnoredResponse`, `RelayExecutor.CheckIgnored`,
  and the `domain.IgnoredPath`-per-input assumption) — this response-shape
  fix is mechanical enough to apply directly, unlike the 5 BLOCKED
  redesigns below.
- **`forkSync`** — real method REQUIRES `expectedUpstream` as a param;
  this task's original design has no such field at all. Added directly
  below (proto field + Go param + relay call).
- **`upstreamStatus`** — param rename, **and** this is the one method in
  this group that still needs TASK-227 (not among the 8 already-reachable
  methods). Real contract also allows an optional `pushTarget` field —
  added below alongside the `worktree_id`→`worktreePath` rename.

### ❌ BLOCKED — genuine shape redesigns (do NOT implement as originally designed below without resolving these first)

These 5 methods make up roughly half this task's scope. None of them are
param-rename fixes — each assumes an operation shape the real agent does
not have. Flagged inline with `⚠️ BLOCKED` comments at each wrong code
block; **not fixed**, per this correction pass's explicit instruction not
to fabricate a redesign without the real agent source
(`git-handler-push-target.ts`, `git-handler-status-ops.ts`, etc.) open.

- **`commitCompare`** — real op is ONE commit vs. its own parent
  (`worktreePath, commitId`), not two arbitrary commits (`base_sha,
  head_sha`) as originally designed. Matches
  `backend-agent-execution-boundary.md`'s own description ("Diffs a
  commit against its parent"), which the original design contradicted.
- **`branchCompare`** — real op is current HEAD vs. ONE `baseRef`
  (`worktreePath, baseRef`), not two arbitrary branches (`base_branch,
  head_branch`).
- **`commitDiff`** — real op is per-file (`worktreePath, commitOid,
  parentOid?, filePath` **required**, `oldPath?`); the original design has
  no `filePath` at all and assumes a whole-commit diff. Same class of bug
  as `git.diff` itself (BUG-036's "Companion fix" / TASK-228).
- **`branchDiff`** — same per-file issue as `commitDiff`, plus the same
  base-ref-only-vs-two-sided issue as `branchCompare`.
- **`submoduleStatus`** — **SOL-032 §0 open question #3.** Real op is
  per-SINGLE-submodule (`worktreePath, submodulePath, area?`), not "list
  every submodule" as this task's `repeated SubmoduleInfo` response
  assumes. Needs either a `.gitmodules`-enumeration step client-side, or a
  narrower per-submodule RPC with the frontend calling it once per
  submodule — a real design decision, not something to fabricate here.

---

## Context

`history`/`commitCompare`/`branchCompare`/`commitDiff`/`branchDiff`/
`submoduleStatus`/`checkIgnored`/`forkSync` all had a dedicated Dev Server
Agent RPC handler in the old TS backend per BUG-032 — the relay leg for
these 8 is low-risk. `upstreamStatus` is **not** among those 8 — confirm
against `specs/agent/api/agent-rpc-catalog-git-fs.md` whether an agent
handler already exists before assuming `RelayExecutor.UpstreamStatus`'s
relay call below will succeed against a real agent; if not, file a
follow-up to add the Dev Server Agent-side handler rather than silently
shipping a relay call with nothing on the other end.

## Changes to make

### Step 1: Proto

Add to the `GitGatewayService` service block:

```protobuf
  rpc History(HistoryRequest) returns (HistoryResponse);
  rpc CommitCompare(CommitCompareRequest) returns (CompareResponse);
  rpc BranchCompare(BranchCompareRequest) returns (CompareResponse);
  rpc CommitDiff(CommitDiffRequest) returns (GetDiffResponse);
  rpc BranchDiff(BranchDiffRequest) returns (GetDiffResponse);
  rpc SubmoduleStatus(SubmoduleStatusRequest) returns (SubmoduleStatusResponse);
  rpc CheckIgnored(CheckIgnoredRequest) returns (CheckIgnoredResponse);
  rpc ForkSync(ForkSyncRequest) returns (ForkSyncResponse);
  rpc UpstreamStatus(UpstreamStatusRequest) returns (UpstreamStatusResponse);
```

Append messages:

```protobuf
message CommitRef {
  string sha = 1;
  string author = 2;
  string committer = 3;
  string message = 4;
  int64 timestamp = 5; // unix seconds
  repeated string parent_shas = 6;
}

// cursor/pagination dropped per this task's Contract correction section —
// the real agent has no pagination concept. ref renamed to base_ref to
// match the field name the real agent actually reads.
message HistoryRequest {
  string worktree_id = 1;
  string base_ref = 2; // empty = current branch
  int32 limit = 3;
}
message HistoryResponse {
  repeated CommitRef commits = 1;
}

// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.commitCompare diffs ONE commit against its own parent
// (worktreePath, commitId), not two arbitrary commits. base_sha/head_sha
// below do not match the real agent contract — do not implement as
// designed until this shape is redesigned.
message CommitCompareRequest {
  string worktree_id = 1;
  string base_sha = 2;
  string head_sha = 3;
}
// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.branchCompare compares current HEAD against ONE base_ref, not two
// arbitrary branches. base_branch/head_branch below do not match the real
// agent contract — do not implement as designed until this shape is
// redesigned.
message BranchCompareRequest {
  string worktree_id = 1;
  string base_branch = 2;
  string head_branch = 3;
}
message CompareResponse {
  repeated CommitRef commits = 1;
  repeated string files_changed = 2;
}

// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.commitDiff is per-file (worktreePath, commitOid, parentOid?,
// filePath REQUIRED, oldPath?). This message has no filePath at all and
// assumes a whole-commit diff — same class of bug as git.diff itself
// (TASK-228). Do not implement as designed until this shape is redesigned.
message CommitDiffRequest {
  string worktree_id = 1;
  string sha = 2;
}
// ⚠️ BLOCKED — same per-file-missing issue as CommitDiffRequest above,
// plus the same base-ref-only-vs-two-sided issue as BranchCompareRequest.
// Do not implement as designed until this shape is redesigned.
message BranchDiffRequest {
  string worktree_id = 1;
  string base_branch = 2;
  string head_branch = 3;
}
// Both reuse GetDiffResponse's {unified_diff} shape — declared above.

// ⚠️ BLOCKED — see this task's Contract correction section and SOL-032 §0
// open question #3: real git.submoduleStatus operates on ONE submodule
// per call (worktreePath, submodulePath, area?), not "list every
// submodule" as this repeated-response shape assumes. Do not implement as
// designed until this shape is redesigned (either a .gitmodules
// enumeration step or a narrower per-submodule RPC).
message SubmoduleInfo {
  string path = 1;
  string current_sha = 2;
  string tracked_sha = 3;
  bool dirty = 4;
}
message SubmoduleStatusRequest {
  string worktree_id = 1;
}
message SubmoduleStatusResponse {
  repeated SubmoduleInfo submodules = 1;
}

// Response shape fixed per this task's Contract correction section: real
// git.checkIgnored returns only the ignored subset (string[]), not a
// {path, ignored} pair for every input path.
message CheckIgnoredRequest {
  string worktree_id = 1;
  repeated string paths = 2;
}
message CheckIgnoredResponse {
  repeated string ignored_paths = 1;
}

// expected_upstream added per this task's Contract correction section —
// real git.forkSync requires it; the original design omitted it entirely.
message ForkSyncRequest {
  string worktree_id = 1;
  string expected_upstream = 2;
}
message ForkSyncResponse {
  int32 ahead = 1;
  int32 behind = 2;
  bool diverged = 3;
}

// push_target added per this task's Contract correction section — real
// git.upstreamStatus accepts an optional structured pushTarget alongside
// worktreePath. Typed as a placeholder string field here since the real
// PushTargetInput shape is SOL-032 §0 open question #1, still unresolved
// — replace with the real structured type once that question closes,
// don't ship this as a bare string.
message UpstreamStatusRequest {
  string worktree_id = 1;
  string push_target = 2; // placeholder — see SOL-032 §0 open question #1
}
message UpstreamStatusResponse {
  bool has_upstream = 1;
  int32 ahead = 2;
  int32 behind = 3;
}
```

### Step 2: Domain value objects — `internal/domain/domain.go`

```go
// CommitRef is one commit's metadata, returned by History/CommitCompare/
// BranchCompare. Mirrors gitgateway.proto's CommitRef message 1:1.
type CommitRef struct {
	SHA         string
	Author      string
	Committer   string
	Message     string
	Timestamp   int64
	ParentSHAs  []string
}

// CompareResult is the shared shape for CommitCompare/BranchCompare.
type CompareResult struct {
	Commits      []CommitRef
	FilesChanged []string
}

// ⚠️ BLOCKED — see this task's Contract correction section and SOL-032 §0
// open question #3: this "list every submodule" shape doesn't match the
// real per-submodule agent contract (worktreePath, submodulePath, area?).
// Kept here unchanged pending that redesign decision — do not build
// SubmoduleStatus's usecase/executor around this type as-is.
//
// SubmoduleStatus is one submodule's tracked-vs-current SHA state.
type SubmoduleStatus struct {
	Path       string
	CurrentSHA string
	TrackedSHA string
	Dirty      bool
}

// IgnoredPath removed per this task's Contract correction section — real
// git.checkIgnored returns only the ignored subset (string[]), so
// CheckIgnored's GitExecutor method now returns []string directly
// instead of a {path, ignored} pair per input path. (Deleted, not kept
// unused, since nothing else in this group references it.)

// ForkSyncStatus reflects a worktree's ahead/behind/diverged state
// relative to its upstream fork's default branch.
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
```

### Step 3: Extend `GitExecutor` — `internal/usecase/ports.go`

```go
	// cursor param dropped, ref renamed to baseRef — see this task's
	// Contract correction section (History is now shippable-now).
	History(ctx context.Context, repoPath, baseRef string, limit int) ([]domain.CommitRef, error)
	// ⚠️ BLOCKED — CommitCompare/BranchCompare/CommitDiff/BranchDiff/
	// SubmoduleStatus signatures below are UNCHANGED from the original,
	// wrong design — see this task's Contract correction section. Do not
	// implement their usecases/executors against these signatures without
	// resolving the cited shape redesigns first.
	CommitCompare(ctx context.Context, repoPath, baseSHA, headSHA string) (domain.CompareResult, error)
	BranchCompare(ctx context.Context, repoPath, baseBranch, headBranch string) (domain.CompareResult, error)
	CommitDiff(ctx context.Context, repoPath, sha string) (domain.DiffResult, error)
	BranchDiff(ctx context.Context, repoPath, baseBranch, headBranch string) (domain.DiffResult, error)
	SubmoduleStatus(ctx context.Context, repoPath string) ([]domain.SubmoduleStatus, error)
	// CheckIgnored now returns []string (the ignored subset) per this
	// task's Contract correction section, not []domain.IgnoredPath.
	CheckIgnored(ctx context.Context, repoPath string, paths []string) ([]string, error)
	// expectedUpstream added — real git.forkSync requires it.
	ForkSync(ctx context.Context, repoPath, expectedUpstream string) (domain.ForkSyncStatus, error)
	// pushTarget added (placeholder type — see SOL-032 §0 open question
	// #1) — this method also still needs TASK-227, unlike the rest of
	// this group.
	UpstreamStatus(ctx context.Context, repoPath, pushTarget string) (domain.UpstreamStatus, error)
```

### Step 4: Usecases — `internal/usecase/`

`history.go` (representative — the rest of this group follow the identical
validate/dispatch/call/wrap shape):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// Cursor field dropped, Ref renamed to BaseRef — see this task's Contract
// correction section; the real agent has no pagination concept.
type HistoryInput struct {
	WorktreeID string
	BaseRef    string
	Limit      int
}

// NextCursor field dropped along with HistoryRequest's cursor — nothing
// downstream produces one anymore.
type HistoryResult struct {
	Commits []domain.CommitRef
}

type History struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewHistory(resolver ConnectionResolver, local, relay GitExecutor) *History {
	return &History{resolver: resolver, local: local, relay: relay}
}

func (uc *History) Execute(ctx context.Context, in HistoryInput) (HistoryResult, error) {
	if in.WorktreeID == "" {
		return HistoryResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return HistoryResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	commits, err := executor.History(ctx, repoPath, in.BaseRef, in.Limit)
	if err != nil {
		return HistoryResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_HISTORY_FAILED", "failed to fetch history", err)
	}
	return HistoryResult{Commits: commits}, nil
}
```

`commit_compare.go`/`branch_compare.go`/`commit_diff.go`/`branch_diff.go`/
`submodule_status.go` are the 5 ❌ BLOCKED methods — do not write these 5
usecase files against this section's described shapes until the redesigns
in this task's Contract correction section are resolved.
`check_ignored.go` returns `[]string` (the ignored subset, per the
Contract correction section — not a domain type); `fork_sync.go` returns
`domain.ForkSyncStatus` and now also validates `ForkSyncInput.ExpectedUpstream`
is non-empty; `upstream_status.go` returns `domain.UpstreamStatus`. Each
constructs its input's `apperrors.KindInvalidArgument` guard for whichever
fields are required (e.g. `CheckIgnoredInput.Paths` must be non-empty),
`dispatchExecutor`, call the matching `GitExecutor` method, wrap the error
with a `GITGATEWAY_<OP>_FAILED` code — identical pattern to `history.go`
above and to `checkout.go` in TASK-207.

### Step 5: `localgit.Executor`

```go
// History runs `git log` with a stable, tab-delimited format. cursor
// support removed — see this task's Contract correction section; the real
// agent has no pagination concept, so this local implementation matches
// it rather than offering a richer feature the relay side can't provide.
func (e *Executor) History(ctx context.Context, repoPath, baseRef string, limit int) ([]domain.CommitRef, error) {
	target := baseRef
	if target == "" {
		target = "HEAD"
	}
	args := []string{"log", target, `--format=%H%x09%an%x09%cn%x09%at%x09%P%x09%s`}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-%d", limit))
	}
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		return nil, err
	}
	var commits []domain.CommitRef
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 6 {
			continue
		}
		var ts int64
		fmt.Sscanf(f[3], "%d", &ts)
		var parents []string
		if f[4] != "" {
			parents = strings.Split(f[4], " ")
		}
		commits = append(commits, domain.CommitRef{
			SHA: f[0], Author: f[1], Committer: f[2], Timestamp: ts,
			ParentSHAs: parents, Message: f[5],
		})
	}
	return commits, nil
}

// ⚠️ BLOCKED — see this task's Contract correction section: CommitCompare/
// BranchCompare/compare below are UNCHANGED from the original, wrong
// two-arbitrary-refs design (real git.commitCompare/git.branchCompare
// compare against one fixed ref — a commit's own parent, or HEAD — not
// two caller-supplied refs). This also means compare()'s call into
// History below is now stale: it used History's cursor param as a
// "base..head" bound, but that param was removed from History's signature
// above per the Contract correction section — this helper won't even
// compile against the corrected History signature, which is itself
// evidence this whole approach needs a real redesign, not a patch.
func (e *Executor) CommitCompare(ctx context.Context, repoPath, baseSHA, headSHA string) (domain.CompareResult, error) {
	return e.compare(ctx, repoPath, baseSHA, headSHA)
}

func (e *Executor) BranchCompare(ctx context.Context, repoPath, baseBranch, headBranch string) (domain.CompareResult, error) {
	return e.compare(ctx, repoPath, baseBranch, headBranch)
}

func (e *Executor) compare(ctx context.Context, repoPath, base, head string) (domain.CompareResult, error) {
	commits, _, err := e.History(ctx, repoPath, head, 0, base)
	if err != nil {
		return domain.CompareResult{}, err
	}
	filesOut, err := e.run(ctx, repoPath, "diff", "--name-only", base+".."+head)
	if err != nil {
		return domain.CompareResult{}, err
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(filesOut), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return domain.CompareResult{Commits: commits, FilesChanged: files}, nil
}

// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.commitDiff is per-file (worktreePath, commitOid, parentOid?,
// filePath REQUIRED, oldPath?). This whole-commit `git show` below is the
// wrong operation, same class of bug as GetDiff before TASK-228's fix — do
// not implement as designed until a filePath param is added.
func (e *Executor) CommitDiff(ctx context.Context, repoPath, sha string) (domain.DiffResult, error) {
	out, err := e.run(ctx, repoPath, "show", "--format=", sha)
	if err != nil {
		return domain.DiffResult{}, err
	}
	return domain.DiffResult{UnifiedDiff: out}, nil
}

// ⚠️ BLOCKED — see this task's Contract correction section: same
// per-file-missing issue as CommitDiff above, plus real git.branchDiff
// compares HEAD against one baseRef, not two arbitrary branches as this
// `base..head` diff assumes.
func (e *Executor) BranchDiff(ctx context.Context, repoPath, baseBranch, headBranch string) (domain.DiffResult, error) {
	out, err := e.run(ctx, repoPath, "diff", baseBranch+".."+headBranch)
	if err != nil {
		return domain.DiffResult{}, err
	}
	return domain.DiffResult{UnifiedDiff: out}, nil
}

// ⚠️ BLOCKED — see this task's Contract correction section and SOL-032 §0
// open question #3: real git.submoduleStatus operates on ONE submodule
// per call. This "list every submodule via `git submodule status`" below
// is a reasonable LOCAL implementation on its own, but the GitExecutor
// method signature it satisfies (repoPath only, no submodulePath) doesn't
// match what the relay side can ever provide against the real agent — do
// not treat this as done until the per-submodule vs. list decision is
// made.
//
// SubmoduleStatus runs `git submodule status` and parses its stable
// one-line-per-submodule format (" <sha> <path> (<describe>)", a leading
// '+' means the checked-out SHA differs from the index's tracked SHA).
func (e *Executor) SubmoduleStatus(ctx context.Context, repoPath string) ([]domain.SubmoduleStatus, error) {
	out, err := e.run(ctx, repoPath, "submodule", "status")
	if err != nil {
		return nil, err
	}
	var subs []domain.SubmoduleStatus
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		dirty := strings.HasPrefix(line, "+")
		fields := strings.Fields(strings.TrimLeft(line, "+- "))
		if len(fields) < 2 {
			continue
		}
		subs = append(subs, domain.SubmoduleStatus{CurrentSHA: fields[0], Path: fields[1], TrackedSHA: fields[0], Dirty: dirty})
	}
	return subs, nil
}

// CheckIgnored runs `git check-ignore` per path (its own exit code — 0 for
// ignored, 1 for not — makes a single multi-path invocation ambiguous
// about which paths matched, so this loops rather than parsing combined
// output). Returns only the ignored subset per this task's Contract
// correction section — matches the real agent's response shape.
func (e *Executor) CheckIgnored(ctx context.Context, repoPath string, paths []string) ([]string, error) {
	var ignored []string
	for _, p := range paths {
		if _, err := e.run(ctx, repoPath, "check-ignore", "--quiet", p); err == nil {
			ignored = append(ignored, p)
		}
	}
	return ignored, nil
}

// ForkSync compares HEAD against expectedUpstream (a caller-supplied
// remote-tracking ref, e.g. "origin/main") — expectedUpstream is now a
// required param per this task's Contract correction section (the real
// agent requires it), replacing the old auto-discovery via `git remote
// show origin` below.
func (e *Executor) ForkSync(ctx context.Context, repoPath, expectedUpstream string) (domain.ForkSyncStatus, error) {
	ahead, behind, err := e.aheadBehind(ctx, repoPath, "HEAD", expectedUpstream)
	if err != nil {
		return domain.ForkSyncStatus{}, err
	}
	return domain.ForkSyncStatus{Ahead: ahead, Behind: behind, Diverged: ahead > 0 && behind > 0}, nil
}

// UpstreamStatus reads the current branch's configured @{upstream}.
// pushTarget is accepted for signature parity with the relay path (real
// git.upstreamStatus takes an optional pushTarget) but unused locally —
// local execution reads the branch's actual git config directly rather
// than resolving a push target.
func (e *Executor) UpstreamStatus(ctx context.Context, repoPath, pushTarget string) (domain.UpstreamStatus, error) {
	upstream, err := e.run(ctx, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		// No upstream configured is a domain outcome, not a Go error.
		return domain.UpstreamStatus{HasUpstream: false}, nil
	}
	ahead, behind, err := e.aheadBehind(ctx, repoPath, "HEAD", strings.TrimSpace(upstream))
	if err != nil {
		return domain.UpstreamStatus{}, err
	}
	return domain.UpstreamStatus{HasUpstream: true, Ahead: ahead, Behind: behind}, nil
}

// defaultRemoteBranch removed — ForkSync now takes expectedUpstream
// directly per this task's Contract correction section, so this helper's
// auto-discovery-via-`git remote show` is no longer needed here (it was
// only ever called from ForkSync above).

// aheadBehind runs `git rev-list --left-right --count a...b` and parses its
// two-column tab-separated output.
func (e *Executor) aheadBehind(ctx context.Context, repoPath, a, b string) (ahead, behind int, err error) {
	out, err := e.run(ctx, repoPath, "rev-list", "--left-right", "--count", a+"..."+b)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("localgit: unexpected rev-list output %q", out)
	}
	fmt.Sscanf(fields[0], "%d", &ahead)
	fmt.Sscanf(fields[1], "%d", &behind)
	return ahead, behind, nil
}
```

### Step 6: `RelayExecutor`

```go
// History: cursor param dropped, ref renamed to baseRef/"baseRef" wire key
// — see this task's Contract correction section.
func (r *RelayExecutor) History(ctx context.Context, repoPath, baseRef string, limit int) ([]domain.CommitRef, error) {
	var result struct {
		Commits []domain.CommitRef `json:"commits"`
	}
	err := r.relay(ctx, repoPath, "git.history", map[string]any{
		"worktreePath": repoPath, "baseRef": baseRef, "limit": limit,
	}, &result)
	return result.Commits, err
}

// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.commitCompare diffs ONE commit against its own parent
// (worktreePath, commitId), not two arbitrary commits. baseSha/headSha
// below do not exist on the real agent contract — do not wire this relay
// call until the shape is redesigned.
func (r *RelayExecutor) CommitCompare(ctx context.Context, repoPath, baseSHA, headSHA string) (domain.CompareResult, error) {
	var result domain.CompareResult
	err := r.relay(ctx, repoPath, "git.commitCompare", map[string]any{
		"repoPath": repoPath, "baseSha": baseSHA, "headSha": headSHA,
	}, &result)
	return result, err
}

// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.branchCompare compares current HEAD against ONE baseRef, not two
// arbitrary branches. baseBranch/headBranch below do not exist on the
// real agent contract — do not wire this relay call until the shape is
// redesigned.
func (r *RelayExecutor) BranchCompare(ctx context.Context, repoPath, baseBranch, headBranch string) (domain.CompareResult, error) {
	var result domain.CompareResult
	err := r.relay(ctx, repoPath, "git.branchCompare", map[string]any{
		"repoPath": repoPath, "baseBranch": baseBranch, "headBranch": headBranch,
	}, &result)
	return result, err
}

// ⚠️ BLOCKED — see this task's Contract correction section: real
// git.commitDiff is per-file (worktreePath, commitOid, parentOid?,
// filePath REQUIRED, oldPath?). This call has no filePath at all and
// assumes a whole-commit diff — same class of bug as git.diff itself
// (TASK-228). Do not wire this relay call until filePath is added.
func (r *RelayExecutor) CommitDiff(ctx context.Context, repoPath, sha string) (domain.DiffResult, error) {
	var result domain.DiffResult
	err := r.relay(ctx, repoPath, "git.commitDiff", map[string]any{
		"repoPath": repoPath, "sha": sha,
	}, &result)
	return result, err
}

// ⚠️ BLOCKED — see this task's Contract correction section: same
// per-file-missing issue as CommitDiff above, plus the same
// base-ref-only-vs-two-sided issue as BranchCompare. Do not wire this
// relay call until the shape is redesigned.
func (r *RelayExecutor) BranchDiff(ctx context.Context, repoPath, baseBranch, headBranch string) (domain.DiffResult, error) {
	var result domain.DiffResult
	err := r.relay(ctx, repoPath, "git.branchDiff", map[string]any{
		"repoPath": repoPath, "baseBranch": baseBranch, "headBranch": headBranch,
	}, &result)
	return result, err
}

// ⚠️ BLOCKED — see this task's Contract correction section and SOL-032 §0
// open question #3: real git.submoduleStatus takes ONE submodulePath
// (plus optional area) and returns that single submodule's status — not
// "list every submodule" as this repoPath-only call assumes. Do not wire
// this relay call until the per-submodule vs. list decision is made.
func (r *RelayExecutor) SubmoduleStatus(ctx context.Context, repoPath string) ([]domain.SubmoduleStatus, error) {
	var result struct {
		Submodules []domain.SubmoduleStatus `json:"submodules"`
	}
	err := r.relay(ctx, repoPath, "git.submoduleStatus", map[string]any{"repoPath": repoPath}, &result)
	return result.Submodules, err
}

// CheckIgnored now returns only the ignored subset (string[]) per this
// task's Contract correction section — matches the real agent's response
// shape instead of a {path, ignored} pair per input path.
func (r *RelayExecutor) CheckIgnored(ctx context.Context, repoPath string, paths []string) ([]string, error) {
	var result struct {
		Ignored []string `json:"ignoredPaths"`
	}
	err := r.relay(ctx, repoPath, "git.checkIgnored", map[string]any{
		"worktreePath": repoPath, "paths": paths,
	}, &result)
	return result.Ignored, err
}

// ForkSync now sends the required expectedUpstream param — the real agent
// rejects calls without it. See this task's Contract correction section.
func (r *RelayExecutor) ForkSync(ctx context.Context, repoPath, expectedUpstream string) (domain.ForkSyncStatus, error) {
	var result domain.ForkSyncStatus
	err := r.relay(ctx, repoPath, "git.forkSync", map[string]any{
		"worktreePath": repoPath, "expectedUpstream": expectedUpstream,
	}, &result)
	return result, err
}

// UpstreamStatus: still needs TASK-227 (unlike this group's other 8
// methods) — no confirmed agent handler is reachable from backend-go's
// transport until then, see this task's header Depends-on line. Param key
// renamed to worktreePath and an optional pushTarget added per the real
// contract (placeholder type — see SOL-032 §0 open question #1). Wire the
// relay call now; if it turns out no agent handler exists at all even
// post-TASK-227, this becomes a FailedPrecondition at runtime (relay()
// returns the agent's "unknown method" error) rather than a compile-time
// gap, which is an acceptable interim state per git-gateway-service's
// existing "relay and see" posture for unverified methods.
func (r *RelayExecutor) UpstreamStatus(ctx context.Context, repoPath, pushTarget string) (domain.UpstreamStatus, error) {
	var result domain.UpstreamStatus
	err := r.relay(ctx, repoPath, "git.upstreamStatus", map[string]any{
		"worktreePath": repoPath, "pushTarget": pushTarget,
	}, &result)
	return result, err
}
```

### Step 7: gRPC adapter — `internal/adapter/grpc/server.go`

Add fields/params and translation methods following `GetDiff`/`Commit`'s
shape for the 4 ✅ shippable-now methods (`History`, `CheckIgnored`,
`ForkSync`, `UpstreamStatus`), translating `[]domain.CommitRef` and
`[]string` (checkIgnored's ignored-subset result — not `[]domain.IgnoredPath`,
removed per this task's Contract correction section) to their proto
message-slice equivalents via small `toProto*` helpers (mirroring
`toProtoFileStatuses`). Do NOT add the other 5 (`CommitCompare`,
`BranchCompare`, `CommitDiff`, `BranchDiff`, `SubmoduleStatus`) until their
shape redesigns land.

### Step 8: Composition root — `cmd/server/main.go`

Wire the 4 ✅ shippable-now usecases now; hold off on the 5 ❌ BLOCKED
ones until their redesigns land (per this task's Contract correction
section):

```go
	historyUC := usecase.NewHistory(resolver, local, relay)
	checkIgnoredUC := usecase.NewCheckIgnored(resolver, local, relay)
	forkSyncUC := usecase.NewForkSync(resolver, local, relay)
	upstreamStatusUC := usecase.NewUpstreamStatus(resolver, local, relay)

	// ⚠️ BLOCKED — do not wire these 5 until their Contract correction
	// section shape redesigns land:
	// commitCompareUC := usecase.NewCommitCompare(resolver, local, relay)
	// branchCompareUC := usecase.NewBranchCompare(resolver, local, relay)
	// commitDiffUC := usecase.NewCommitDiff(resolver, local, relay)
	// branchDiffUC := usecase.NewBranchDiff(resolver, local, relay)
	// submoduleStatusUC := usecase.NewSubmoduleStatus(resolver, local, relay)
```

Extend `gitgatewaygrpc.New(...)` with all 9.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/git-gateway-service
go build ./... && go vet ./...
```

Expected: clean build, `buf breaking` reports only additions.

**This does not confirm any of these 9 RPCs work end-to-end.** For the 4
✅ shippable-now methods (`history`, `checkIgnored`, `forkSync`,
`upstreamStatus`): a clean build confirms the Go compiles, not that the
relay calls succeed against a real agent — `history`/`checkIgnored`/
`forkSync` are reachable today (no TASK-227 dependency), but
`upstreamStatus` won't produce a working result until TASK-227
(agent reachability) lands. For the 5 ❌ BLOCKED methods
(`commitCompare`, `branchCompare`, `commitDiff`, `branchDiff`,
`submoduleStatus`): do not implement their usecase/executor/gRPC-adapter
layers from this file's designs at all until the shape redesigns in this
task's Contract correction section are resolved — a clean build of their
code as originally sketched here would only prove the Go compiles against
the wrong contract, not that it works.
