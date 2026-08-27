# SOL-031: Give `git-gateway-service` the on-disk worktree half — `CreateWorktree`/`RemoveWorktree`/`ForceDeleteBranch`/PR-MR base resolution, coordinated with `project-service`'s existing bookkeeping via a synchronous saga

**Resolves:** [BUG-031](../BUG-031-worktree-channels-not-implemented.md)
**Service:** `git-gateway-service` (5 new RPCs, 1 new required interface method) + `project-service` (no proto change — existing `ListWorktrees` reused) + `api-gateway` (8 new `wscompat` channels, 1 of them a cross-service aggregation)
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
- `backend-go/services/git-gateway-service/internal/usecase/ports.go`
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`, `remove_worktree.go`, `force_delete_branch.go`, `detect_worktrees.go`, `prefetch_create_base.go`, `resolve_pr_base.go`, `resolve_mr_base.go` (all new)
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go`
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
- `backend-go/services/git-gateway-service/internal/adapter/scmclient/` (new package)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Status:** 📋 Proposed — not yet implemented

---

## Why this is the second highest-value proposal in this batch

BUG-031's key finding: `project-service` can answer "what worktrees exist"
and toggle activation/rename bookkeeping (`ListWorktrees`/
`SetWorktreeActivation`/`RenameWorktree`, all real, per
`project.proto:36-38`), but **nothing in backend-go can actually create or
remove an on-disk git worktree** — `git-gateway-service`'s proto has zero
worktree RPCs (`gitgateway.proto:10-16` has only status/diff/commit/push/
pull/commit-message), even though `RecordWorktreeCreated`'s own doc
comment (`record_worktree_created.go:22-24`, cited verbatim by BUG-031)
already describes the intended flow: "called by `git-gateway-service`
AFTER the real `git worktree add` succeeded on the Dev Server Agent." That
half was never built. This solution builds it, and — per this batch's
assignment — designs the exact coordination contract between the two
services' data, since `05-data-architecture.md` explicitly rejects
distributed transactions across service databases and requires a named
pattern (outbox or saga) for any cross-service consistency need.

---

## Coordination pattern: synchronous saga, git-gateway-service as the initiator, project-service as the last step — not outbox

`05-data-architecture.md`'s "Cross-service data consistency" section
offers exactly two patterns (§"Cross-service data consistency"): outbox
for "A does something, others need to eventually know" (async, no waiting
caller), or a synchronous saga for "A needs B to also succeed before A's
operation can be considered complete, **and the caller is waiting**."
Worktree create/remove is unambiguously the second shape — the user is
staring at a "creating worktree..." spinner and needs to know definitively
whether it succeeded, including the bookkeeping half, before the RPC
returns. Outbox's "eventually consistent, caller doesn't wait" semantics
don't fit; a saga does, and it's the **same** saga
`project-service.md` §2's own diagram already sketches (reproduced in
BUG-031's Description) — this solution's job is to make each step's
failure/compensation behavior explicit, which that diagram doesn't yet
do:

```mermaid
sequenceDiagram
  participant GW as api-gateway
  participant Git as git-gateway-service
  participant Proj as project-service
  participant Infra as infra-fleet-service
  participant Agent as Dev Server Agent

  GW->>Git: CreateWorktree(projectId, repoId, branch, baseRef)
  Git->>Proj: (resolve repo -> dev_server_id, via ListRepos/GetProject — existing RPCs)
  Git->>Infra: ResolveConnection(dev_server_id)
  Infra-->>Git: connectionId (or none)
  Git->>Agent: git worktree add (relayed, or local exec)
  Agent-->>Git: path, HEAD sha
  Git->>Proj: RecordWorktreeCreated(projectId, repoId, path, branch)
  alt RecordWorktreeCreated succeeds
    Proj-->>Git: Worktree
    Git-->>GW: CreateWorktreeResponse{path, headSha}
  else RecordWorktreeCreated fails
    Note over Git,Agent: Compensating step — the git op already succeeded<br/>on disk; project-service has no record of it.
    Git->>Agent: git worktree remove --force (best-effort rollback)
    alt rollback succeeds
      Git-->>GW: WORKTREE_BOOKKEEPING_FAILED (rolled back cleanly)
    else rollback ALSO fails
      Git-->>GW: WORKTREE_BOOKKEEPING_FAILED (orphaned on disk — surfaces via worktree.detectedList)
    end
  end
```

**Source-of-truth answer, stated explicitly** (the thing BUG-031 flagged
as needing a concrete design): `git-gateway-service` (via the Dev Server
Agent) is authoritative for **on-disk existence**; `project-service` is
authoritative for **bookkeeping metadata** (lineage, activation, rename
history, display). This is not a new rule — `project-service.md` §4
already states it for the read side ("Never authoritative for whether the
worktree still exists on disk — `git-gateway-service` reconciles on
demand, same as TS's detect/reconcile behavior") — this solution extends
it to the write side with the saga above, and gives the read-side
reconciliation a concrete RPC (`DetectWorktrees`, below) instead of
leaving "reconciles on demand" unspecified.

**Compensation is best-effort, not guaranteed** — a crash between the
agent's `git worktree add` succeeding and the compensating `git worktree
remove` running leaves a genuine orphan (on disk, no bookkeeping row).
This is why `worktree.detectedList` (§ below) isn't optional polish: it's
the reconciliation safety net for exactly this failure window, the same
role TS's "detect worktrees on disk not in bookkeeping" behavior already
played. `RemoveWorktree`'s failure-after-success case is the mirror image
(on-disk gone, bookkeeping row stale) and is covered by the same scan from
the other direction — no separate compensation logic needed there, since
"worktree doesn't exist" is a safe terminal state to leave a stale record
pointing at (unlike a create failure, which leaves live, unaccounted-for
disk usage and a dangling branch).

---

## Design — Proto additions (`gitgateway.proto`)

```protobuf
service GitGatewayService {
  // ... existing RPCs unchanged ...

  rpc CreateWorktree(CreateWorktreeRequest) returns (CreateWorktreeResponse);
  rpc RemoveWorktree(RemoveWorktreeRequest) returns (google.protobuf.Empty);
  // Required on every GitExecutor implementation from day one — see
  // "Fixing the old backend's crash bug" below.
  rpc ForceDeleteBranch(ForceDeleteBranchRequest) returns (google.protobuf.Empty);

  // Reconciliation read — the concrete form of project-service.md §4's
  // "git-gateway-service reconciles on demand."
  rpc DetectWorktrees(DetectWorktreesRequest) returns (DetectWorktreesResponse);

  rpc PrefetchCreateBase(PrefetchCreateBaseRequest) returns (PrefetchCreateBaseResponse);
  rpc ResolvePrBase(ResolvePrBaseRequest) returns (ResolveBaseResponse);
  rpc ResolveMrBase(ResolveMrBaseRequest) returns (ResolveBaseResponse);
}

message CreateWorktreeRequest {
  string project_id = 1;
  string repo_id = 2;
  string branch = 3;      // new branch name for the worktree
  string base_ref = 4;    // branch/tag/sha to branch from; typically pre-resolved via ResolvePrBase/ResolveMrBase/PrefetchCreateBase
}
message CreateWorktreeResponse {
  string worktree_id = 1; // project-service's Worktree.id, from the saga's RecordWorktreeCreated step
  string path = 2;
  string head_sha = 3;
}

message RemoveWorktreeRequest {
  string worktree_id = 1;
  bool   force = 2;       // maps to `git worktree remove --force` (uncommitted changes present)
}

message ForceDeleteBranchRequest {
  string worktree_id = 1;
  string branch = 2;
}

message DetectWorktreesRequest  { string repo_id = 1; }
message DetectWorktreesResponse {
  repeated string on_disk_paths = 1; // raw `git worktree list --porcelain` result for the repo — no bookkeeping join here, see wscompat aggregation below
}

message PrefetchCreateBaseRequest  { string repo_id = 1; string base_ref = 2; }
message PrefetchCreateBaseResponse { string resolved_sha = 1; } // ensures base_ref is fetched/up to date, warms the ref for a fast subsequent CreateWorktree

message ResolvePrBaseRequest { string repo_id = 1; int32 pr_number = 2; }
message ResolveMrBaseRequest { string repo_id = 1; int32 mr_number = 2; }
message ResolveBaseResponse  { string base_branch = 1; string base_sha = 2; }
```

Additive — `buf breaking` passes per `08-inter-service-communication.md`.

### Why `DetectWorktrees` returns raw paths, not a diff

Per `05-data-architecture.md`'s "Read models / query needs across service
boundaries" section: "Where the frontend needs data assembled from
multiple services in one view... `api-gateway` performs the aggregation
(parallel gRPC calls, merge in the edge layer) rather than any service
reaching into another's database." `worktree.detectedList` is exactly this
shape — it needs `git-gateway-service`'s on-disk truth AND
`project-service`'s bookkeeping truth merged into one "which worktrees are
orphaned/stale" view. Diffing inside `git-gateway-service` would mean it
reaching into `project-service`'s data for a read-only view, which this
doc's own bounded-context rule (§2: "It owns... No data") argues against
as much as a shared database would. The diff belongs at `api-gateway`
(below), not in either service.

---

## Design — `usecase/` layer (`git-gateway-service`)

### `CreateWorktree` — the saga

```go
// internal/usecase/create_worktree.go
type CreateWorktree struct {
    resolver ConnectionResolver
    projects ProjectClient // extends the existing projectclient port — see below
    local    GitExecutor
    relay    GitExecutor
}

func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
    repo, err := uc.projects.GetRepo(ctx, in.RepoID) // resolves repo path template + dev_server_id
    if err != nil {
        return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
    }
    executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID) // existing dispatch helper, ports.go:85-93
    if err != nil {
        return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
    }

    result, err := executor.CreateWorktree(ctx, repoPath, in.Branch, in.BaseRef)
    if err != nil {
        return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", "git worktree add failed", err)
    }

    worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch)
    if err != nil {
        // Compensating step (05-data-architecture.md's saga pattern: "if
        // step 2 fails, step 1's compensating action runs").
        if compErr := executor.RemoveWorktree(ctx, result.Path, true); compErr != nil {
            return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED",
                fmt.Sprintf("worktree created but bookkeeping failed (%v) and rollback also failed (%v) — orphaned at %s, will surface via worktree.detectedList", err, compErr, result.Path), err)
        }
        return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED", "worktree created but bookkeeping failed; rolled back cleanly", err)
    }
    return domain.WorktreeResult{WorktreeID: worktree.ID, Path: result.Path, HeadSHA: result.HeadSHA}, nil
}
```

`ProjectClient` (new port, `internal/usecase/ports.go`) wraps the
`project-service` gRPC client this service didn't previously need to call
for writes — `git-gateway-service.md` §7 already lists "Calls
`project-service`" for reads (worktree/repo resolution); this solution
adds `RecordWorktreeCreated`/`RecordWorktreeRemoved` as new outbound calls
on that same client, per that doc comment's own described (but
previously unbuilt) flow.

### `RemoveWorktree` — no compensation needed, by design

```go
// internal/usecase/remove_worktree.go
func (uc *RemoveWorktree) Execute(ctx context.Context, worktreeID string, force bool) error {
    executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
    if err != nil {
        return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
    }
    if err := executor.RemoveWorktree(ctx, repoPath, force); err != nil {
        return apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
    }
    // If this fails, the on-disk state (already gone) is the correct
    // terminal state — a stale bookkeeping row pointing at a removed
    // worktree is safe to leave for worktree.detectedList's scan to catch,
    // unlike CreateWorktree's failure direction. No compensating action.
    if err := uc.projects.RecordWorktreeRemoved(ctx, worktreeID); err != nil {
        return apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
    }
    return nil
}
```

### Fixing the old backend's crash bug: `ForceDeleteBranch` is a required `GitExecutor` method, not optional

BUG-031 cites the exact defect to avoid: the old TS `GitProvider`
interface declared `forceDeletePreservedBranch?` with a `?` — optional —
and only `SshGitProvider` implemented it
(`backend/src/main/providers/types.ts:389`,
`ssh-git-provider.ts:735-750`, with an explicit "older SSH relays predate
`git.forceDeletePreservedBranch`" fallback comment), so calling it against
a provider variant that didn't implement it crashed. Go's static interface
satisfaction makes the equivalent mistake structurally impossible **if**
the method is added to `GitExecutor` itself, not bolted onto one
implementation with a runtime type-assertion escape hatch:

```go
// internal/usecase/ports.go — GitExecutor grows a REQUIRED method.
// Both internal/adapter/localgit.Executor and internal/adapter/grpcclient's
// relay executor MUST implement it — the compiler enforces this the moment
// GitExecutor gains the method, closing the "one provider variant silently
// lacks it" gap by construction rather than by convention.
type GitExecutor interface {
    GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error)
    GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error)
    Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
    Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
    Pull(ctx context.Context, repoPath string) (domain.PullResult, error)
    // New, this solution:
    CreateWorktree(ctx context.Context, repoPath, branch, baseRef string) (domain.WorktreeCreateResult, error)
    RemoveWorktree(ctx context.Context, worktreePath string, force bool) error
    ForceDeleteBranch(ctx context.Context, repoPath, branch string) error
}
```

For the relay-executor implementation (`grpcclient/relay_executor.go`),
`ForceDeleteBranch` relays to whatever `git.forceDeletePreservedBranch`-
equivalent method the Dev Server Agent's git handler exposes — if the
target agent build genuinely predates that method (the exact scenario the
old TS fallback comment describes), the failure must surface as a typed,
caught error from the relay call (`ErrAgentMethodUnsupported`, mapped to a
clear `WORKTREE_FORCE_DELETE_UNSUPPORTED` gRPC status), not a nil-pointer
crash from an unimplemented interface method — the structural fix is
"every `GitExecutor` has the method," the operational fallback for an
outdated agent build is "the method call fails cleanly," and the two are
independent: Go's compiler guarantees the first; explicit error handling
in the relay executor's method body guarantees the second.

### `ResolvePrBase`/`ResolveMrBase` — a new dependency on `scm-integration-service`, flagged explicitly

Resolving a PR/MR's base branch requires calling GitHub's/GitLab's API
(the PR/MR's configured base ref and its current SHA) — that's platform
metadata, not a git operation, and `scm-integration-service` is
`backend-go`'s owner of GitHub/GitLab API access
(`git-gateway-service.md`'s own migration notes name it, in the context of
the `gh`/`glab` OAuth API client Gap 1 fix, §10). This solution adds a new
outbound dependency edge `git-gateway-service --> scm-integration-service`
that `git-gateway-service.md` §7's current dependency list (`project-service`,
`infra-fleet-service` only) doesn't yet document — flagged here as a scope
addition beyond that doc, analogous to how SOL-001 flagged
`GetAdminStats` as beyond `auth-service.md`'s scope.

```go
// internal/usecase/ports.go
type SCMClient interface {
    // GetPullRequestBase / GetMergeRequestBase return the platform's
    // recorded base branch + its current head SHA for a PR/MR — a thin
    // read, no mutation. Implemented by internal/adapter/scmclient against
    // scm-integration-service's own GitHub/GitLab RPCs (out of this
    // solution's scope to design — reuses whatever scm-integration-service
    // already exposes for PR/MR metadata reads).
    GetPullRequestBase(ctx context.Context, repoID string, prNumber int32) (baseBranch, baseSHA string, err error)
    GetMergeRequestBase(ctx context.Context, repoID string, mrNumber int32) (baseBranch, baseSHA string, err error)
}

// internal/usecase/resolve_pr_base.go
func (uc *ResolvePrBase) Execute(ctx context.Context, repoID string, prNumber int32) (domain.ResolvedBase, error) {
    baseBranch, baseSHA, err := uc.scm.GetPullRequestBase(ctx, repoID, prNumber)
    if err != nil {
        return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_PR_BASE_LOOKUP_FAILED", "failed to resolve PR base", err)
    }
    // Confirm the base ref actually exists (and is up to date) in the
    // LOCAL repo — this is git-gateway-service's own "resolve -> dispatch
    // -> translate" mandate (§2) applied to a ref instead of a worktree:
    // the platform might report a base branch this repo's clone hasn't
    // fetched yet.
    executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repoID)
    if err != nil {
        return domain.ResolvedBase{}, err
    }
    resolvedSHA, err := executor.FetchAndResolveRef(ctx, repoPath, baseBranch) // ensures the ref is fetched; returns its local SHA
    if err != nil {
        return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BASE_REF_UNRESOLVABLE", "PR base branch not resolvable in local repo", err)
    }
    return domain.ResolvedBase{Branch: baseBranch, SHA: resolvedSHA}, nil
}
```

(`FetchAndResolveRef` is a small addition to `GitExecutor`, shared by
`PrefetchCreateBase`'s usecase too — both need "make sure this ref is
fetched and return its SHA.")

---

## Design — `wscompat` wiring (`api-gateway`)

7 of the 8 channels are direct 1:1 unary wrappers, same shape as
`registerGitChannels` (`channels.go:221-252`) — `worktree.rm` →
`GitGatewayServiceClient.RemoveWorktree`, `worktree.forceDeleteBranch` →
`.ForceDeleteBranch`, `worktree.prefetchCreateBase` → `.PrefetchCreateBase`,
`worktree.resolvePrBase`/`worktree.resolveMrBase` → `.ResolvePrBase`/
`.ResolveMrBase`, `worktree.list` → `ProjectServiceClient.ListWorktrees`
(already-real wrapper, per BUG-031), `worktree.set` →
`ProjectServiceClient.SetWorktreeActivation` (already-real wrapper).

`worktree.detectedList` is the one aggregation, per this solution's
"why `DetectWorktrees` returns raw paths" design decision above:

```go
r.Register("worktree.detectedList", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type detectedListArgs struct {
        ProjectID string `json:"projectId"`
        RepoID    string `json:"repoId"`
    }
    in, err := decodeArg[detectedListArgs](args, 0)
    if err != nil {
        return nil, err
    }
    ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})

    // Parallel calls, merged at the edge — 05-data-architecture.md's
    // explicit prescription for a multi-service view, not a job for
    // either owning service to do internally.
    var onDisk *gitgatewayv1.DetectWorktreesResponse
    var known *projectv1.ListWorktreesResponse
    g, gctx := errgroup.WithContext(ctx)
    g.Go(func() (err error) { onDisk, err = gitClient.DetectWorktrees(gctx, &gitgatewayv1.DetectWorktreesRequest{RepoId: in.RepoID}); return })
    g.Go(func() (err error) { known, err = projectClient.ListWorktrees(gctx, &projectv1.ListWorktreesRequest{ProjectId: in.ProjectID}); return })
    if err := g.Wait(); err != nil {
        return nil, err
    }

    knownPaths := make(map[string]bool, len(known.GetWorktrees()))
    for _, w := range known.GetWorktrees() {
        knownPaths[w.GetPath()] = true
    }
    var orphaned []string // on disk, not in bookkeeping — exactly what TS's "detect" behavior surfaced
    for _, p := range onDisk.GetOnDiskPaths() {
        if !knownPaths[p] {
            orphaned = append(orphaned, p)
        }
    }
    return map[string]any{"orphanedPaths": orphaned}, nil
})
```

`worktree.set`/`worktree.list` remain 🏠 always-local bookkeeping per
BUG-031's dispatch-model finding — no `git-gateway-service` involvement,
matching `SetWorktreeActivation` already being pure Postgres.

`RegisterRealChannels` (`channels.go:64-82`) grows
`registerWorktreeChannels(r, gitClient, projectClient)`; `projectClient`
is a new dependency this file doesn't currently have (only
`gitgatewayv1`/`infrafleetv1`/`taskv1`/etc. are wired today) — flagged as
an additional composition-root change, same class SOL-028/SOL-030 each
flag for their own new clients.

---

## Test plan

- `services/git-gateway-service/internal/usecase/create_worktree_test.go`
  — happy path returns `{worktreeID, path, headSHA}`; `RecordWorktreeCreated`
  failure triggers `RemoveWorktree` compensation on the fake executor,
  asserted via a call-count/argument check; compensation-also-fails path
  returns an error whose message names both failures and the orphaned
  path (regression guard: never silently swallow the second failure).
- `services/git-gateway-service/internal/usecase/remove_worktree_test.go`
  — happy path; `RecordWorktreeRemoved` failure does NOT trigger any
  compensating git operation (asserted: fake executor's `CreateWorktree`/
  `RemoveWorktree` called exactly once, not twice) and returns
  `WORKTREE_BOOKKEEPING_STALE`, not a generic internal error.
- `services/git-gateway-service/internal/usecase/force_delete_branch_test.go`
  — table-driven over BOTH `localgit.Executor` and the relay executor
  (fakes), asserting the same `GitExecutor.ForceDeleteBranch` call
  succeeds against each — the direct regression test for the old
  optional-method crash bug class (a test that would have failed to even
  compile against the old TS-style optional-interface design, since Go's
  interface satisfaction is checked at compile time, not per-call).
- `services/git-gateway-service/internal/usecase/resolve_pr_base_test.go`
  — fake `SCMClient` + fake `GitExecutor`: happy path; `SCMClient` success
  but the base ref not fetchable locally returns
  `WORKTREE_BASE_REF_UNRESOLVABLE`, not the SCM-side data.
- `services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go`
  — `worktree.detectedList` against fake `GitGatewayServiceClient`/
  `ProjectServiceClient`: a path present on disk but absent from
  bookkeeping appears in `orphanedPaths`; a path present in both does not;
  one client erroring fails the whole call (via `errgroup`), not a partial
  silent result.
- Integration test (testcontainers-go `project-service` Postgres + fake
  Dev Server Agent, mirrors `provisioner_test.go`'s fake-server pattern) —
  full `CreateWorktree` → `worktree.detectedList` (empty) →
  `RemoveWorktree` → `worktree.detectedList` (still empty) round trip,
  confirming the saga's steps actually leave bookkeeping and disk state in
  agreement in the non-failure case.

## References

- `specs/backend-go/tdd/architecture/05-data-architecture.md` — "Cross-service data consistency" (outbox vs. saga), "Read models / query needs across service boundaries" (edge-layer aggregation) — both directly cited design decisions
- `specs/backend-go/tdd/services/project-service.md:26-69` (§2) — metadata-vs-execution split, the sequence diagram this solution's saga extends with explicit compensation
- `specs/backend-go/tdd/services/project-service.md:176-180` (§4) — "Never authoritative for whether the worktree still exists on disk — `git-gateway-service` reconciles on demand," the source-of-truth statement this solution makes concrete
- `specs/backend-go/tdd/services/git-gateway-service.md:36-79` (§2, §3) — bounded context ("resolve host → dispatch → translate"), current RPC surface
- `specs/backend-go/tdd/services/git-gateway-service.md:243-256` (§7) — current dependency list (`project-service`, `infra-fleet-service` only) — this solution adds `scm-integration-service`, flagged as a documented addition
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:10-16` — current `GitGatewayService` surface (no worktree RPCs)
- `backend-go/proto/orca/project/v1/project.proto:31-38,175-224` — `ProjectService`'s existing worktree bookkeeping RPCs/messages, reused as-is (no proto change proposed for `project-service`)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:61-93` — current `GitExecutor` interface and `dispatchExecutor` helper this solution extends
- `backend-go/services/project-service/internal/usecase/record_worktree_created.go:22-24` — doc comment describing the intended (previously unbuilt) flow this solution implements
- `backend-go/backend/src/main/providers/types.ts:389`, `ssh-git-provider.ts:735-750` — old backend's optional `forceDeletePreservedBranch` crash-bug precedent this solution's required-interface-method design fixes
- [BUG-031](../BUG-031-worktree-channels-not-implemented.md) — full findings this solution builds on
