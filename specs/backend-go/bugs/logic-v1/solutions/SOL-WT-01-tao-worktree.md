# SOL-WT-01: Enforce worktree creation business rules and add name/path input

**Resolves:** [BUG-WT-01](../BUG-WT-01-tao-worktree-partial.md)
**Service:** `git-gateway-service` (usecase/domain) + `project-service` (event publish only)
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `CreateWorktreeRequest` gains `name`/`path`; structured error detail for known alternate flows
- `backend-go/services/git-gateway-service/internal/domain/worktree_name.go` (new) — `ValidateWorktreeName`
- `backend-go/services/git-gateway-service/internal/domain/create_worktree_errors.go` (new) — typed sentinels for [A1]/[A2]/[A3]
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go` — pre-checks, alternate-name suggestion, branch-not-found handling
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` — `ProjectClient.ListWorktrees` (new method)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/project_client.go` — implement `ListWorktrees`
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go` — `CreateWorktree` accepts an explicit target path; local disk-space pre-check
- `backend-go/services/project-service/internal/usecase/record_worktree_created.go` + `adapter/eventbus/` — `worktree.created` outbox event
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`git-gateway-service.md` §2 states this service's *only* owned logic is
"resolve host → dispatch → translate" and explicitly disclaims git business
rules ("It does not decide merge strategies... that logic lives on the Dev
Server Agent... or in the locally invoked `git` binary"). BR-WT-01–04 are a
partial counterexample worth naming directly: charset validation, duplicate
detection, and a count cap are not git semantics — they're this system's own
product rules layered *on top of* a plain `git worktree add`, and the TDD's
"resolve → dispatch → translate" description doesn't preclude a fourth,
pre-dispatch step of "validate" the same way `RebindDevServer`
(`project-service.md` §3) runs its own guard checks before calling through.
This solution adds that step without changing the resolve/dispatch/translate
shape `CreateWorktree.Execute` already has (`create_worktree.go:41-71`).

- **BR-WT-01** (charset) is pure input validation — belongs in `domain/`
  per `03-clean-architecture-guidelines.md`'s "entities... with their
  invariants enforced in constructors/methods — not validated later in a
  handler." A `ValidateWorktreeName` free function (not a `Worktree` entity
  method, since git-gateway-service's domain is value-object-only per
  `git-gateway-service.md` §4) is the right shape.
- **BR-WT-04** (count cap) is answerable from data `git-gateway-service`
  already has a read path to: `project-service.ListWorktrees(project_id)`
  is a real, already-specified RPC (`project-service.md` §3). No proto
  change needed — this is a new call on an existing RPC, added to the
  `ProjectClient` port `git-gateway-service`'s `usecase/ports.go` already
  defines (`ports.go:341-345`).
- **[A1]/[A2]** (duplicate path / missing base branch) are answerable from
  `GitExecutor` methods that already exist: `ListWorktreePaths` (existence
  check + alternate-name generation) and `ListLocalBranches` (branch
  suggestion list) — both real, both already required on every
  `GitExecutor` implementation (`ports.go:183,216`). No new executor
  capability needed, just calling them from `CreateWorktree.Execute` and
  branching on the git failure's shape.
- **[A3]** (disk space) has no existing primitive on either side. Flag as a
  genuine, narrower extension: a **local-dispatch-only** soft check
  (`syscall`/`unix.Statfs` against the target's parent directory), skipped
  for relay dispatch since the Dev Server Agent's `fs.*` surface has no
  disk-usage RPC (same absence `BUG-009`/SOL-009 already documented for the
  agent's method set) — this is a warning, not a hard block, matching the
  spec's own "cảnh báo dung lượng disk" (warn) language rather than a
  BR-numbered hard rule.
- **Custom `name`/`path` input**: the spec's own Input contract
  (`docs/logic/worktree-management/BL-WT-01-tao-worktree.md:90-98`) requires
  it; `CreateWorktreeRequest` today has no field for it
  (`gitgateway.proto`'s `CreateWorktreeRequest`, confirmed by the bug). This
  is a straightforward proto/executor extension, not an architecture change
  — `git-gateway-service.md` §3's RPC sketch already implies request/response
  messages carry whatever the operation needs; it just never enumerated
  these two fields.
- **`worktree:created` event**: `git-gateway-service` owns no database
  (§5), so it cannot itself run `05-data-architecture.md`'s transactional
  outbox pattern (`event = DB row in the same transaction`). `project-service`
  *does* own the `worktrees` table and already writes the row
  (`RecordWorktreeCreated`, `worktree_repository.go:28-47`) — the outbox
  event belongs there, in the same transaction as the `INSERT`, per
  `05-data-architecture.md`'s "service A writes its domain state change and
  an outbox row in the same Postgres transaction" and
  `08-inter-service-communication.md`'s subject-naming convention
  (`orca.project.worktree.created`). `project-service.md` §6 already lists
  `adapter/eventbus/` with a `project.created, project.rebound, member.added`
  example list — `worktree.created` is a direct, unremarkable addition to
  that existing list, not a new adapter package.
- **Terminal PTY init postcondition**: deliberately **not** added to this
  saga. `CreateWorktree.Execute`'s own doc comment and this bug's own
  finding both note no terminal/PTY service is reachable from
  `git-gateway-service` today, and `project-service.md` §2's boundary
  decision ("`project.agentSpawn` does not port here... spawn RPC belongs
  to `infra-fleet-service`") establishes that execution-plane dispatch
  (which a PTY spawn is) does not belong inside a metadata/git saga.
  Recommend composing this at the `wscompat` edge instead (`worktree.create`
  channel optionally chaining `infra-fleet-service.SpawnTerminalSession`
  after a successful `CreateWorktree` — the same building block
  [SOL-WT-02](./SOL-WT-02-fan-out-worktree.md) uses at N=1) — a follow-up,
  not part of this fix.

---

## Design — proto additions (`gitgateway.proto`)

```protobuf
message CreateWorktreeRequest {
  string project_id = 1;
  string repo_id = 2;
  string branch = 3;
  string base_ref = 4;
  optional string name = 12;   // NEW — spec's `name?`; defaults to a sanitized branch name if empty
  optional string path = 13;   // NEW — spec's `path?`; defaults to the existing repoPath+"-"+name convention if empty
  optional string parent_worktree_id = 5;
  // ...existing lineage fields unchanged...
}

message CreateWorktreeResponse {
  string worktree_id = 1;
  string path = 2;
  string head_sha = 3;
  optional string suggested_name = 4; // NEW — populated only on WORKTREE_PATH_EXISTS, see below
}
```

Alternate-flow signaling uses `apperrors`' existing `Code` field
(`common/apperrors/apperrors.go:40-46`) rather than a new response shape —
consistent with how every other usecase in this package already reports
failure. Three new codes, all `KindFailedPrecondition` /
`KindInvalidArgument` as appropriate:

| Code | Kind | When | Client-usable detail |
|---|---|---|---|
| `WORKTREE_NAME_INVALID` | `KindInvalidArgument` | BR-WT-01 charset check fails | — |
| `WORKTREE_PATH_EXISTS` | `KindAlreadyExists` | [A1]: target path already in `ListWorktreePaths` | `CreateWorktreeResponse.suggested_name` still returned even on error (gRPC allows a response on non-OK only via error details in strict proto3; pragmatically: return the suggestion as part of the `AppError.Message`, structured as `"path already exists; try 'foo-2'"`, OR — cleaner — accept the minor proto deviation of returning `(CreateWorktreeResponse, error)` where `suggested_name` is set and error is still non-nil, since Go's dual-return idiom allows a caller to read both) |
| `WORKTREE_BASE_REF_NOT_FOUND` | `KindNotFound` | [A2]: `base_ref` not resolvable | attach available branch names via `AppError.Message` (`"branch 'xyz' not found; available: main, develop, ..."`) — a follow-up could promote this to a structured gRPC status detail, out of scope for this pass |
| `WORKTREE_LIMIT_EXCEEDED` | `KindFailedPrecondition` | BR-WT-04: count ≥ 20 for this repo | — |

---

## Design — domain

```go
// internal/domain/worktree_name.go
package domain

import (
	"errors"
	"regexp"
)

var worktreeNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

var ErrInvalidWorktreeName = errors.New("domain: worktree name must match [a-z0-9_-]+")

// ValidateWorktreeName enforces BR-WT-01. Free function, not a Worktree
// entity method — git-gateway-service's domain is value-object-only
// (git-gateway-service.md §4), it never constructs a Worktree entity.
func ValidateWorktreeName(name string) error {
	if !worktreeNamePattern.MatchString(name) {
		return ErrInvalidWorktreeName
	}
	return nil
}

// SuggestAlternateName appends "-2", "-3", ... until a name not present in
// taken is found — used for [A1]'s recovery suggestion.
func SuggestAlternateName(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := base + "-" + itoa(i)
		if !taken[candidate] {
			return candidate
		}
	}
}
```

## Design — usecase (`create_worktree.go`)

```go
func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
	name := in.Name
	if name == "" {
		name = sanitizeBranchForPath(in.Branch) // same sanitizer localgit already uses
	}
	if err := domain.ValidateWorktreeName(name); err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_NAME_INVALID", err.Error(), err)
	}

	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-WT-04 — count existing worktrees for this repo before attempting git.
	existing, err := uc.projects.ListWorktrees(ctx, in.ProjectID)
	if err == nil { // fail open on the count lookup itself — a transient
		           // ListWorktrees failure should not block worktree creation
		count := 0
		for _, w := range existing {
			if w.RepoID == in.RepoID && w.Active {
				count++
			}
		}
		if count >= 20 {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_LIMIT_EXCEEDED", "maximum 20 worktrees per repository", nil)
		}
	}

	// [A1] — duplicate path pre-check + alternate-name suggestion, using the
	// already-required ListWorktreePaths (ports.go:183) instead of letting
	// git's raw stderr be the only signal.
	onDisk, _ := executor.ListWorktreePaths(ctx, repoPath) // best-effort; git itself is still the final authority
	taken := make(map[string]bool, len(onDisk))
	for _, p := range onDisk {
		taken[p] = true
	}
	targetPath := in.Path
	if targetPath == "" {
		targetPath = repoPath + "-" + name
	}
	if taken[targetPath] {
		suggested := domain.SuggestAlternateName(name, taken)
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindAlreadyExists, "WORKTREE_PATH_EXISTS",
			fmt.Sprintf("path already exists; try %q", suggested), nil)
	}

	result, err := executor.CreateWorktree(ctx, repoPath, in.Branch, in.BaseRef, targetPath)
	if err != nil {
		if isBaseRefNotFoundErr(err) { // [A2] — classify git's stderr
			branches, listErr := executor.ListLocalBranches(ctx, repoPath)
			if listErr == nil {
				return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_BASE_REF_NOT_FOUND",
					fmt.Sprintf("branch %q not found; available: %s", in.BaseRef, joinBranchNames(branches)), err)
			}
		}
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", "git worktree add failed", err)
	}

	worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch, in.Lineage)
	// ...compensation logic unchanged...
}
```

`GitExecutor.CreateWorktree` gains a `targetPath string` parameter (empty =
derive as today) — a signature change affecting both `localgit.Executor`
and the relay executor; both already accept a `baseRef` string param today,
so this is a mechanical addition, not a new dispatch branch.

`isBaseRefNotFoundErr` classifies git's stderr (`"invalid reference"`,
`"unknown revision"`) — a small, explicit string-match helper, same
pragmatic approach `localgit`'s existing error-shape code already uses
elsewhere in this package (e.g. `strings.HasPrefix(baseRef, "-")` checks in
`BranchCompare`).

### [A3] disk-space check (local dispatch only)

```go
// internal/adapter/localgit/diskspace.go (new)
func checkFreeSpace(parentDir string, minBytes uint64) (ok bool, availableBytes uint64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(parentDir, &stat); err != nil {
		return true, 0, err // fail open — a disk-space check that itself fails should not block creation
	}
	available := stat.Bavail * uint64(stat.Bsize)
	return available >= minBytes, available, nil
}
```

Called from `CreateWorktree.Execute` only when `executor == uc.local`
(type-identity check against the injected local `GitExecutor`, matching the
existing `dispatchExecutor` return value) — surfaces as a warning field on
`CreateWorktreeResponse`, not a hard failure (`optional bool low_disk_warning
= 14`), matching the spec's "cảnh báo" (warn) language for [A3] vs. the hard
`FAILED_PRECONDITION` used for BR-WT-04.

## Design — `project-service` event

```go
// internal/usecase/record_worktree_created.go — after the successful INSERT
uc.events.Publish(ctx, tx, eventbus.Event{
	Subject: "orca.project.worktree.created",
	Payload: worktreeCreatedPayload{WorktreeID: wt.ID, ProjectID: wt.ProjectID, RepoID: wt.RepoID, Path: wt.Path, Branch: wt.Branch},
})
```

Written in the same transaction as the `INSERT` (`worktree_repository.go`'s
`RecordWorktreeCreated`), relayed by the existing outbox-polling process per
`05-data-architecture.md` — no new relay infrastructure, this is the same
pattern `project.created`/`member.added` already use per `project-service.md`
§6's `adapter/eventbus/` listing.

---

## Test plan

- `domain/worktree_name_test.go` — table test for `ValidateWorktreeName`
  (valid: `a-z0-9_-`; rejects uppercase, spaces, unicode, empty); `SuggestAlternateName` collision-walk test.
- `usecase/create_worktree_test.go` — new cases:
  - `_InvalidName_RejectsBeforeAnyExecutorCall` (assert zero calls to `local`/`relay` fakes)
  - `_PathAlreadyExists_ReturnsSuggestedName_NoGitCallAttempted`
  - `_LimitExceeded_RejectsBeforeGitCall` (fake `ProjectClient.ListWorktrees` returns 20 active rows for the repo)
  - `_LimitCheckFailsOpen_WhenListWorktreesErrors` (count lookup failing doesn't block creation)
  - `_BaseRefNotFound_ClassifiesGitError_AttachesBranchList`
  - `_CustomNameAndPath_PassedThroughToExecutor`
  - Existing happy-path/compensation tests updated for the new `targetPath` executor param.
- `adapter/localgit/executor_test.go` (or integration) — `CreateWorktree` with an explicit `targetPath` creates at that path, not the derived one; `checkFreeSpace` unit test with a stubbed statfs value.
- `project-service/internal/usecase/record_worktree_created_test.go` — asserts one outbox row is written in the same transaction as the insert (fake `EventPublisher`/outbox port).

## References

- `specs/backend-go/bugs/logic-v1/BUG-WT-01-tao-worktree-partial.md` — full gap list
- `specs/backend-go/tdd/services/git-gateway-service.md:2-73` (§2 bounded context, "no git business rules" framing this solution's validation step sits alongside), `:142-160` (§4 domain model, value-object-only), `:179-216` (§6 package layout)
- `specs/backend-go/tdd/services/project-service.md:42-53` (§2 boundary decision: agent spawn does not port here), `:280-290` (§6 `adapter/eventbus/` precedent)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md:61-72` (domain invariant placement), `:134-146` (testing implications)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:82-98` (transactional outbox pattern)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:30-45` (event subject naming)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:11-71`
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:170-183,216,341-345` (`ListWorktreePaths`, `ListLocalBranches`, `ProjectClient`)
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:466-482` (current `CreateWorktree`, path derivation)
- `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go:28-47` (`RecordWorktreeCreated`)
- `backend-go/common/apperrors/apperrors.go` (`Kind`/`Code` shape used for the new error codes)
- `docs/logic/worktree-management/BL-WT-01-tao-worktree.md`
