# SOL-PW-03: Merge, stash, branch create/soft-delete via scoped `git.exec` RPCs; push/pull progress via `git.execStream`

**Resolves:** [BUG-PW-03](../BUG-PW-03-remote-git-operations-merge-stash-branch-gaps.md)
**Service:** `git-gateway-service` (all new RPCs/usecases) + `api-gateway` (wscompat wiring only)
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (5 new unary RPCs + 2 new server-streaming RPCs)
- `backend-go/services/git-gateway-service/internal/usecase/merge_branch.go`, `stash_push.go`, `stash_pop.go`, `create_branch.go`, `delete_branch.go` (soft), `push_stream.go`, `pull_stream.go` (new)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` (`GitExecutor` additions; new `StreamingGitExecutor` port)
- `backend-go/services/git-gateway-service/internal/adapter/localgit/` (implement all 7 directly against the local `git` binary)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (implement the 5 unary ops via `git.exec`; implement the 2 streaming ops via `git.execStream`; all 7 gated on `ConnectionMode`)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` (`ResolvedConnection` gains a `Mode` field)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/resolver.go` (thread `dev_server.mode` through `ResolveConnection`'s translation)
- `backend-go/services/git-gateway-service/internal/domain/` (`ErrGitOpUnsupportedOverSSHRelay` sentinel)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go` (`git.merge`, `git.stash.push`, `git.stash.pop`, `git.branch.create`, `git.branch.delete`, plus server-streaming forwarding for `git.push`/`git.pull` progress)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

BUG-PW-03 frames the branch-create gap as blocked on `git.exec`, and
explicitly notes `git.exec` "was deliberately never carried forward to
backend-go" (citing missing-v1/BUG-032). That framing is about **exposing
a raw, generic `git.exec` passthrough RPC to clients** — this solution
does not do that. Instead it follows the exact precedent
`git-gateway-service`'s own code already sets for `ListLocalBranches`
(`internal/adapter/grpcclient/relay_executor.go:589-602`): a **scoped,
purpose-built RPC** whose usecase composes a specific `git.exec` call
internally, never exposing the generic `args: []string` shape to a
caller. `ports.go`'s own doc comment for the Group A methods
(`Checkout`/`ListLocalBranches`/`FastForward`/`ConflictOperation`, lines
~130-145) states the pattern directly: each RPC is "redesigned against
the real agent contract," not a pass-through. This solution's five new
RPCs (`MergeBranch`, `StashPush`, `StashPop`, `CreateBranch`,
`DeleteBranch`) are five more instances of that same pattern — matching
`git-gateway-service.md` §3's "one usecase per RPC" convention
(`03-clean-architecture-guidelines.md`) and §10's framing of this
service's Go work as "a straightforward one-to-one translation" of the
TS `git.*` namespace, which per BL-PW-03's own relay/command map
(`docs/logic/project-workspace/BL-PW-03-remote-git-operations.md:19-44`)
already lists `git.branch.create`/`git.branch.delete`/`git.merge`/
`git.stash.push`/`git.stash.pop` as first-class method names, not
`git.exec` calls — this solution restores that 1:1 mapping rather than
leaving the gap TS's dynamic dispatch never had.

### The critical grounding fact this solution rests on: the Dev Server Agent already supports these operations, but only over one of its two relay transports

`specs/agent/api/agent-rpc-catalog-git-fs.md` documents **two independent
implementations** of `git.exec` (the "Part A / Part B" split, per that
doc's own framing, §"Two independent implementations"):

- **Part A — WS-connected Dev Server Agent** (`direct-websocket`/
  `relay-websocket` connection modes): `ALLOWED_GIT_SUBCOMMANDS` is 21
  subcommands wide, explicitly including `branch, checkout, merge,
  rebase, stash` "with real arguments" (catalog doc lines 62-69) — no
  per-subcommand shape restriction beyond the injection-flag validator
  (`agent-git-exec-validator.ts`'s `assertNoGitInjectionFlags`).
- **Part B — SSH Relay Daemon** (`relay-ssh` connection mode):
  `ALLOWED_GIT_SUBCOMMANDS` is only 14 subcommands, and the doc's own
  "Not allowed at all" list (catalog doc lines ~226-227) **explicitly
  names** `merge, rebase, stash` and branch-mutating operations among the
  rejected subcommands — confirmed by `git-gateway-service`'s own
  `ListLocalBranches` doc comment (`relay_executor.go:582-588`), which
  already notes "the whitelist RelayExecutor's SSH-relay calls actually
  reach (Part B, not Part A's separate, broader `git.exec` surface)".

This means the five new RPCs are **fully supported for the local-exec
case and for `relay-websocket`/`direct-websocket` connections, but
genuinely unsupported for `relay-ssh` connections** — a real, protocol-
level asymmetry, not a Go-side implementation gap. This is not a new
constraint this solution invents; it's the existing agent contract,
confirmed by reading its own whitelist source of truth rather than
assumed. The design below makes this asymmetry a typed, checked error
(mirroring `ErrForceDeleteBranchUnsupported`/
`ErrConflictResolveUnsupportedOverRelay`'s existing precedent in
`internal/domain/`), not a silent failure or an attempted call that the
agent would reject anyway.

### Push/pull progress streaming: `git.execStream` already exists for exactly this

BUG-PW-03 states flatly "every git RPC... is a plain unary
request/response" and there is no streaming mechanism to back
`pushWithProgress()`. The agent-side catalog shows this is **not quite
true of the agent's own capability** — `git.execStream` (Part A only,
catalog doc line 47 and §"`git.execStream` streaming shape") already
streams `stdout`/`stderr` line-by-line via WS frames for the same
21-subcommand whitelist as `git.exec`, which includes `push`/`pull`/
`fetch`. The gap BUG-PW-03 correctly identifies is that
`git-gateway-service` never exposes a server-streaming RPC that relays to
it — this solution adds exactly that, gated the same way as the merge/
stash RPCs (Part A only; `relay-ssh` connections fall back to the
existing unary `Push`/`Pull` with no live progress, a documented
degradation rather than a silent one).

### Branch soft-delete

`ForceDeleteBranchRequest` (`gitgateway.proto:634-637`) already exists
for `git branch -D`. This solution's `DeleteBranch` RPC is the `git
branch -d` sibling BUG-PW-03 names as missing — same shape, same
dispatch pattern, distinguished by a `force` bool rather than a second
near-duplicate RPC (matching `CreateDirRequest`'s existing
`no_clobber` bool-field precedent for "same operation family, one flag
distinguishes the variant" from `SOL-009`).

## Design — Proto additions (`gitgateway.proto`)

```protobuf
service GitGatewayService {
  // ... existing RPCs unchanged ...

  // ── SOL-PW-03: merge/stash/branch-create/soft-delete. All five are
  // FAILED_PRECONDITION for a relay-ssh-mode connection — see
  // ErrGitOpUnsupportedOverSSHRelay's doc comment. ─────────────────────
  rpc MergeBranch(MergeBranchRequest) returns (MergeBranchResponse);
  rpc StashPush(StashPushRequest) returns (StashPushResponse);
  rpc StashPop(StashPopRequest) returns (StashPopResponse);
  rpc CreateBranch(CreateBranchRequest) returns (CreateBranchResponse);
  rpc DeleteBranch(DeleteBranchRequest) returns (DeleteBranchResponse); // soft (-d); ForceDeleteBranch remains the -D path

  // ── SOL-PW-03: push/pull progress streaming. Server-streaming; falls
  // back to the existing unary Push/Pull for relay-ssh connections (the
  // caller gets ErrGitOpUnsupportedOverSSHRelay on the *stream* RPC and
  // is expected to retry against the unary one — see wscompat wiring). ──
  rpc PushStream(PushRequest) returns (stream GitProgressEvent);
  rpc PullStream(PullRequest) returns (stream GitProgressEvent);
}

message MergeBranchRequest {
  string worktree_id = 1;
  string branch = 2;      // branch to merge INTO the current branch
  bool   no_ff = 3;        // matches BL-PW-03's "Merge branch (no-ff)" AC; default true when unset at the usecase layer
}
message MergeBranchResponse {
  bool success = 1;
  bool had_conflicts = 2;  // mirrors PullResponse's existing had_conflicts shape
}

message StashPushRequest {
  string worktree_id = 1;
  string message = 2;      // optional; empty = git's default stash message
  bool   include_untracked = 3;
}
message StashPushResponse { bool success = 1; }

message StashPopRequest {
  string worktree_id = 1;
  // stash_ref empty = pop the most recent (stash@{0}), matching `git
  // stash pop` with no ref argument — most callers never need a specific
  // ref, per BL-PW-03's "Stash push + pop" AC naming no index selection.
  string stash_ref = 2;
}
message StashPopResponse {
  bool success = 1;
  bool had_conflicts = 2;  // `git stash pop` can conflict, same as a merge
}

message CreateBranchRequest {
  string worktree_id = 1;
  string branch = 2;
  string base_ref = 3;     // empty = branch from current HEAD
  bool   checkout = 4;     // true = also switch to it (composes CreateBranch+Checkout server-side in one round trip, matching the "checkout -b" UX the CheckoutRequest doc comment explicitly deferred to a separate step)
}
message CreateBranchResponse { string branch = 1; }

message DeleteBranchRequest {
  string worktree_id = 1;
  string branch = 2;
}
message DeleteBranchResponse { bool success = 1; }

// GitProgressEvent mirrors the agent's git.execStream frame shape
// (catalog doc: {type:'stream.chunk',line,source?} / {type:'stream.end',exitCode}).
message GitProgressEvent {
  string line = 1;
  string source = 2;      // "stdout" | "stderr"
  bool   is_final = 3;
  int32  exit_code = 4;   // only meaningful when is_final
}
```

## Design — `ResolvedConnection` gains connection-mode awareness

```go
// internal/usecase/ports.go — extended
type ResolvedConnection struct {
    Connected    bool
    ConnectionID string
    RepoPath     string
    // Mode is empty when Connected is false (host-local — the distinction
    // doesn't apply). Populated from infra-fleet-service's
    // ResolveConnectionResponse.dev_server.mode (already returned today,
    // per resolver.go's existing translation — just not threaded through
    // this struct before now, the same class of "field exists upstream,
    // not forwarded" gap relay_executor.go's own doc comment already
    // flags for ConnectionID/RepoPath's relationship).
    Mode string // "relay-ssh" | "relay-websocket" | "direct-websocket"
}
```

```go
// internal/domain/ — new sentinel, same home/rationale as the two
// existing ones (ErrForceDeleteBranchUnsupported,
// ErrConflictResolveUnsupportedOverRelay): both grpcclient (returns it)
// and usecase (checks it via errors.Is) need it without an import cycle.
var ErrGitOpUnsupportedOverSSHRelay = errors.New(
    "git-gateway-service: this operation requires a relay-websocket or " +
        "direct-websocket connection; relay-ssh's git.exec whitelist does " +
        "not permit merge/stash/branch-write subcommands")
```

## Design — `usecase/` layer

Representative usecase (all five unary ops follow this shape; the two
streaming ones follow the pattern in the next section):

```go
// internal/usecase/merge_branch.go
type MergeBranchInput struct {
    WorktreeID string
    Branch     string
    NoFF       bool
}

type MergeBranch struct {
    resolver ConnectionResolver
    local    GitExecutor
    relay    GitExecutor
}

func NewMergeBranch(resolver ConnectionResolver, local, relay GitExecutor) *MergeBranch {
    return &MergeBranch{resolver: resolver, local: local, relay: relay}
}

func (uc *MergeBranch) Execute(ctx context.Context, in MergeBranchInput) (domain.MergeResult, error) {
    if in.WorktreeID == "" || in.Branch == "" {
        return domain.MergeResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_ARGS", "worktree_id and branch are required", nil)
    }
    conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
    if err != nil {
        return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
    }
    // Fail closed on the one connection mode the agent's own git.exec
    // whitelist rejects outright — checked here, before ever attempting
    // the relay call, mirroring RenameFile/CopyFile's existing
    // check-before-call precedent in SOL-009.
    if conn.Connected && conn.Mode == "relay-ssh" {
        return domain.MergeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_MERGE_UNSUPPORTED_SSH_RELAY", "merge is not supported over an SSH-relay connection", domain.ErrGitOpUnsupportedOverSSHRelay)
    }
    executor := uc.local
    if conn.Connected {
        executor = uc.relay
    }
    result, err := executor.MergeBranch(ctx, conn.RepoPath, in.Branch, in.NoFF)
    if err != nil {
        return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_MERGE_FAILED", "failed to merge branch", err)
    }
    return result, nil
}
```

`StashPush`/`StashPop`/`CreateBranch`/`DeleteBranch` are the same body
shape with different `GitExecutor` methods and result types — one file
each, per this service's existing one-usecase-per-RPC convention.

### `GitExecutor` interface additions (`ports.go`)

```go
MergeBranch(ctx context.Context, repoPath, branch string, noFF bool) (domain.MergeResult, error)
StashPush(ctx context.Context, repoPath, message string, includeUntracked bool) (domain.SimpleResult, error)
StashPop(ctx context.Context, repoPath, stashRef string) (domain.MergeResult, error) // reuses MergeResult's had_conflicts shape
CreateBranch(ctx context.Context, repoPath, branch, baseRef string, checkout bool) (string, error)
DeleteBranch(ctx context.Context, repoPath, branch string) error // soft; ForceDeleteBranch (existing) stays the -D path
```

### `localgit` implementation (host-local case — always supported, no mode gate)

Straightforward `os/exec` calls against the local `git` binary, e.g.
`git merge --no-ff <branch>` / `git stash push [-u] [-m <message>]` /
`git stash pop [<ref>]` / `git branch <name> [<base_ref>]` (+ separate
`git checkout <name>` if `checkout=true`) / `git branch -d <name>` — all
five subcommands are baseline-compatible per
[`docs/reference/git-compatibility.md`](../../../../docs/reference/git-compatibility.md)'s
Git 2.25 floor (branch/merge/stash predate that baseline by years), so no
`GitCapabilityCache` fallback logic is needed for this addition
specifically — flagged as checked, not assumed, per this repo's Git
Binary Compatibility rule.

### `grpcclient.RelayExecutor` implementation (relay case — mode-gated)

```go
// MergeBranch relays via git.exec's merge subcommand — only reachable
// when the target connection is Part A (relay-websocket/direct-websocket);
// Part B's (relay-ssh) git.exec whitelist rejects `merge` outright
// (agent-rpc-catalog-git-fs.md's "Not allowed at all" list). The usecase
// layer's ConnectionResolver check (above) is expected to prevent this
// method from ever being called against a relay-ssh connection — this
// method does not re-check mode itself, matching RenameFile/CopyFile's
// existing division of responsibility (usecase checks, executor trusts).
func (r *RelayExecutor) MergeBranch(ctx context.Context, repoPath, branch string, noFF bool) (domain.MergeResult, error) {
    args := []string{"merge"}
    if noFF {
        args = append(args, "--no-ff")
    }
    args = append(args, branch)
    var result gitExecResult
    err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result)
    if err != nil {
        return domain.MergeResult{}, err
    }
    return domain.MergeResult{Success: true, HadConflicts: strings.Contains(result.Stderr, "CONFLICT")}, nil
}
```

`StashPush`/`StashPop`/`CreateBranch`/`DeleteBranch` follow the identical
`args := []string{...}` + `r.relay(ctx, repoPath, "git.exec", ...)` shape,
reusing the existing `gitExecResult{Stdout, Stderr}` type
`ListLocalBranches` already defined (`relay_executor.go:570-577`) — no new
wire-shape type needed. `CreateBranch` composes two `git.exec` calls when
`checkout=true` (`branch` then `checkout`), sequentially, not as one
`git checkout -b` invocation — `checkout -b`'s combined form is not on
either Part's exec whitelist as a single flag-shape today; confirm this
before implementation, since if it is allowlisted through `checkout`'s
own flag rules a single call would be simpler and should be preferred.

### `PushStream`/`PullStream` (server-streaming, Part A only)

```go
// internal/usecase/push_stream.go
type PushStream struct {
    resolver ConnectionResolver
    local    StreamingGitExecutor // adapter/localgit: pipes os/exec's Stdout/Stderr line-by-line
    relay    StreamingGitExecutor // adapter/grpcclient: relays to git.execStream
}

func (uc *PushStream) Execute(ctx context.Context, in PushInput, sink func(domain.GitProgressLine) error) error {
    conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
    if err != nil {
        return apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
    }
    if conn.Connected && conn.Mode == "relay-ssh" {
        return apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_PUSH_STREAM_UNSUPPORTED_SSH_RELAY", "push progress streaming is not supported over an SSH-relay connection; retry against the unary Push RPC", domain.ErrGitOpUnsupportedOverSSHRelay)
    }
    executor := uc.local
    if conn.Connected {
        executor = uc.relay
    }
    return executor.PushStream(ctx, conn.RepoPath, in.Remote, in.Branch, sink)
}
```

`adapter/grpc`'s server implementation calls `Execute` with a `sink`
closure that writes each line onto the gRPC server-stream
(`stream.Send(&gitgatewayv1.GitProgressEvent{...})`) — the same
"usecase stays transport-agnostic, adapter/grpc does the wire
translation" split this service already uses everywhere else. The relay
implementation subscribes to the agent's `git.execStream` frames
(`{type:'stream.chunk',line,source}`/`{type:'stream.end',exitCode}`,
per the catalog doc) via whatever `infra-fleet-service` relay-streaming
mechanism exists for that — **flagged as needing verification against
`infra-fleet-service`'s actual client interface**: today's `Relay` RPC
(`services/infra-fleet-service/internal/usecase/relay.go`) is unary
request/response only (`uc.agent.Exec(ctx, devServer, method, params)`
returns one `map[string]any`, not a stream). Exposing `git.execStream`'s
line-by-line frames therefore needs a **new server-streaming RPC on
`infra-fleet-service`** (`RelayStream`, mirroring `Relay`'s shape but
returning `stream map[string]any` or a typed frame message) — this is a
genuine scope addition beyond `infra-fleet-service`'s current RPC surface,
not something this solution can complete inside `git-gateway-service`
alone. Flagged explicitly, matching this task's instruction to call out
extensions beyond the TDD as-is.

## Design — wiring (wscompat)

```go
// channels_git.go — five new unary channels, identical shape to the
// existing git.checkout/git.abortMerge entries this file already has.
r.Register("git.merge", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type mergeArgs struct {
		WorktreeID string `json:"worktreeId"`
		Branch     string `json:"branch"`
		NoFF       bool   `json:"noFf"`
	}
	in, err := decodeArg[mergeArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.MergeBranch(ctx, &gitgatewayv1.MergeBranchRequest{WorktreeId: in.WorktreeID, Branch: in.Branch, NoFf: in.NoFF})
	if err != nil {
		return nil, err // FAILED_PRECONDITION over relay-ssh surfaces as-is, per this solution's typed-error design
	}
	return resp, nil
})
// git.stash.push / git.stash.pop / git.branch.create / git.branch.delete
// follow the same shape — omitted for brevity.

// git.push/git.pull gain a streaming variant. wscompat's existing
// request/response WS-compat model needs a subscription-style channel for
// this (matching whatever pattern workflow.execute's StreamExecutionEvents
// already uses for wscompat, if one exists — verify against
// registerWorkflowChannels before assuming a new mechanism is needed) —
// the gRPC server-stream's GitProgressEvent frames get forwarded as
// incremental WS frames under a new event name (e.g. "git.push.progress"),
// terminated by a final frame carrying the unary-equivalent
// {success, hadConflicts} once GitProgressEvent.is_final is seen.
```

## Test plan

- `merge_branch_test.go` / `stash_push_test.go` / `stash_pop_test.go` /
  `create_branch_test.go` / `delete_branch_test.go` — fake
  `ConnectionResolver`/`GitExecutor`: local dispatch calls the local
  executor; relay dispatch with `Mode="relay-websocket"` calls the relay
  executor; relay dispatch with `Mode="relay-ssh"` returns
  `ErrGitOpUnsupportedOverSSHRelay` **without** calling the relay executor
  at all (assert the fake records zero calls) — the regression guard this
  service's existing known-gap tests already establish the pattern for.
- `adapter/localgit/` — real temp-repo integration tests: merge with a
  genuine conflict reports `had_conflicts=true`; stash push/pop round-trip
  restores working-tree state; create-branch-with-checkout leaves HEAD on
  the new branch; soft-delete of an unmerged branch fails (matching `git
  branch -d`'s own safety behavior, distinct from `-D`).
- `adapter/grpcclient` — assert the exact `git.exec` `args` slice sent for
  each of the five ops (contract test against the agent's real subcommand
  shape, not just "some call happened").
- `push_stream_test.go`/`pull_stream_test.go` — fake `StreamingGitExecutor`
  emitting a sequence of lines then a final frame; assert the usecase
  forwards each line to `sink` in order and returns after the final frame;
  `relay-ssh` mode returns the typed error before ever constructing a
  `StreamingGitExecutor` call.
- `wscompat/channels_git_test.go` — one test per new unary channel; a
  `FAILED_PRECONDITION` from the client surfaces unmodified to the WS
  response (not swallowed into a generic 500-equivalent).

## Dev Server Agent (`agent/`) impact

**No `agent/` change is required for the five unary RPCs or for local-exec
push/pull streaming** — Part A's `git.exec`/`git.execStream` already
support every subcommand this solution needs, confirmed by reading the
agent's own whitelist source of truth
(`specs/agent/api/agent-rpc-catalog-git-fs.md`), not assumed.

**`infra-fleet-service` does need a new `RelayStream` RPC** (§"PushStream/
PullStream" above) to carry `git.execStream`'s frame sequence across the
gRPC boundary — this is a Go-service-only change (no `agent/` change), but
it is a real scope addition beyond `infra-fleet-service`'s TDD-sketched
RPC surface (`08-inter-service-communication.md`'s "Talking to the Dev
Server Agent" section describes the relay contract at the protocol level,
not per-RPC) and beyond this solution's own service boundary
(`git-gateway-service`) — flagged for whoever picks up implementation to
coordinate with `infra-fleet-service`'s owner before starting.

Push/pull progress streaming over a `relay-ssh` connection remains
unsupported after this solution (Part B has no `execStream` equivalent at
all per the catalog doc) — the unary `Push`/`Pull` RPCs remain the only
path for that connection mode, a documented degradation matching this
service's existing `ErrHostUnreachable`-style typed-failure posture
(`git-gateway-service.md` §8's "Failure mode on relay failure"), not a
silent capability loss.

## References

- `docs/logic/project-workspace/BL-PW-03-remote-git-operations.md:19-44,97-114,174-190` — relay git command map (naming `git.branch.create`/`git.merge`/`git.stash.push`/`git.stash.pop` as first-class methods), push/pull progress-stream contract, acceptance criteria
- `specs/agent/api/agent-rpc-catalog-git-fs.md` §"Two independent implementations", `ALLOWED_GIT_SUBCOMMANDS` for Part A (21 subcommands including merge/rebase/stash) vs. Part B (14 subcommands, explicit "Not allowed at all" list including merge/rebase/stash), and §"`git.execStream` streaming shape" — the load-bearing agent-side contract this entire solution is grounded in
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go:582-602` — `ListLocalBranches`'s existing doc comment, the direct precedent for "scoped RPC composes `git.exec` internally, Part A vs. Part B whitelist awareness already exists in this codebase"
- `backend-go/services/git-gateway-service/internal/usecase/abort_merge.go` — the `resolver`/`local`/`relay` dispatch shape every new usecase in this solution follows verbatim
- `backend-go/services/git-gateway-service/internal/domain/*.go` — `ErrForceDeleteBranchUnsupported`/`ErrConflictResolveUnsupportedOverRelay`, the existing typed-unsupported-operation precedent `ErrGitOpUnsupportedOverSSHRelay` follows
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:112-123,158-173` — `DevServer.mode`/`ResolveConnectionResponse` already carrying connection-mode data this solution threads through
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go:9-24` — `ConnectionMode` enum (`relay-ssh`/`relay-websocket`/`direct-websocket`) this solution's mode gate checks against
- `backend-go/services/infra-fleet-service/internal/usecase/relay.go` — the existing unary-only `Relay` usecase, confirming `RelayStream` is a genuine new RPC, not an existing capability this solution merely calls
- `specs/backend-go/tdd/services/git-gateway-service.md:36-74` (§2 dispatch shape), `:75-119` (§3 RPC surface convention), `:258-297` (§8 non-functional — connection-resolution caching, `ErrHostUnreachable` failure-mode precedent), `:324-352` (§10 migration notes — "straightforward one-to-one translation" framing)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:84-108` — "Talking to the Dev Server Agent" section, Option A (existing wire protocol) framing this solution's `RelayStream` addition stays within
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md` §"Known gaps carried forward" — the check-before-call / typed-sentinel-error pattern this solution's SSH-relay gate follows
