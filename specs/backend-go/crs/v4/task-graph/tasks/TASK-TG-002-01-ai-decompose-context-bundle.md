# TASK-TG-002-01: `AIDecompose` 5-source context bundle + `TechStackDetector`

**From Solution:** BE-SOL-002
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/ai_decompose.go`, `backend-go/services/task-service/internal/usecase/tech_stack_detector.go` (new), `backend-go/services/task-service/internal/usecase/ports.go` (new `TechStackDetector` port, extend `EdgeRepository`/`TaskRepository` if `ListChildren` doesn't already exist)
**Depends on:** TASK-TG-001-02 (needs `Task.Description`/`Task.AIContext` fields to exist and be populated)
**Status:** `[x]` DONE

---

## Context

Verified directly (`internal/usecase/ai_decompose.go:1-118`, read in full):
`AIDecompose.Execute` (lines 42-73) already implements the two-step
review-before-commit shape correctly and is NOT a stub, matching BE-SOL-002's
own claim. The actual gap is narrower than a rewrite: `buildDecomposePrompt`
(lines 79-90) interpolates only `task.Title` and `providerCtx`, and
`domain.SubtaskProposal` (`internal/domain/subtask_proposal.go:1-12`, read in
full) is `{Title, Description}` with `Description` never populated anywhere
(`parseSubtaskProposals`, lines 98-118, only ever sets `Title`).

`ListChildren` does not exist on `TaskRepository` today — confirmed, `grep -n
"ListChildren" internal/` finds nothing; this task adds it (BE-SOL-002's
`buildContext` sketch calls `u.repo.ListChildren(ctx, task.ID)` as if it
already exists — it doesn't, add it here rather than treating it as given).

`TechStackDetector` is entirely new — no `git-gateway-service` client
exists in `task-service` today (confirmed, `grep -rn "gitgateway"
internal/` finds nothing in this service; `git-gateway-service`'s real file
RPCs are `ReadFile`/`ReadFileChunk`/`ReadFilePreview`/`ReadDir`
(`backend-go/proto/orca/gitgateway/v1/gitgateway.proto:68-71`, confirmed by
direct read) — BE-SOL-002 doesn't name an exact RPC, just "the same one
`git-gateway-service.md` §3.1 names for commit-message generation"; use
`ReadFile` (simplest: one call per candidate filename, tolerate
not-found) rather than `ReadDir`+`ReadFile` unless the candidate-file list
grows large enough that a directory listing first is clearly cheaper.

`AIProviderContextResolver`/`ProjectExecutionResolver` (used by
`AIDecompose` already) live in `internal/adapter/grpcclient` — put the new
`TechStackDetector`'s gRPC client implementation there too, for consistency
(`internal/usecase/tech_stack_detector.go` holds only the port interface
and/or the pure best-effort wrapper the usecase calls, matching this
codebase's port-in-usecase/impl-in-adapter split).

## Changes to make

**1. `internal/usecase/ports.go`** — new port + `TaskRepository` addition:

```go
// TechStackDetector inspects a project's worktree (via git-gateway-service)
// for common manifest files to build a coarse tech-stack hint for the AI
// decompose prompt — best-effort: AIDecompose must never fail because
// detection failed or found nothing.
type TechStackDetector interface {
	Detect(ctx context.Context, projectID string) ([]string, error)
}
```

```go
// ListChildren returns the direct children of taskID (parent_child edges'
// targets) — used by AIDecompose's context bundle to list already-existing
// subtasks so the AI doesn't propose duplicates.
ListChildren(ctx context.Context, tenantID, taskID string) ([]domain.Task, error)
```

**2. `internal/usecase/ai_decompose.go`** — widen `buildContext`/prompt:

```go
type decomposeContext struct {
	Title            string
	Description      string
	AIContext        string
	TechStack        []string
	ExistingSubtasks []string
}

func (uc *AIDecompose) buildContext(ctx context.Context, tenantID string, task domain.Task) decomposeContext {
	stack, _ := uc.techStack.Detect(ctx, task.ProjectID) // best-effort — nil on any failure, never blocks decompose
	existing, _ := uc.tasks.ListChildren(ctx, tenantID, task.ID)
	titles := make([]string, 0, len(existing))
	for _, t := range existing {
		titles = append(titles, t.Title)
	}
	return decomposeContext{
		Title: task.Title, Description: task.Description, AIContext: task.AIContext,
		TechStack: stack, ExistingSubtasks: titles,
	}
}
```

`AIDecompose` struct gains a `techStack TechStackDetector` field;
`NewAIDecompose`'s constructor gains that parameter — update
`cmd/server/main.go`'s wiring accordingly.

`buildDecomposePrompt` (currently lines 79-90) is rewritten to interpolate
`decomposeContext`'s 5 fields instead of bare `task.Title` +
`providerCtx` — keep `providerCtx` too (it's a distinct, already-correct
piece of context BE-SOL-002 doesn't propose removing).

**3. `internal/usecase/tech_stack_detector.go`** (new — port-side best-effort
wrapper, if the codebase's convention is to keep the interface + a thin
default implementation in `usecase/`; otherwise this file can be omitted and
the interface folds directly into `ports.go` above, with only the real
gRPC-backed implementation living in `adapter/grpcclient` — check
`AIProviderContextResolver`'s file layout for the exact precedent to match
before deciding).

**4. `internal/adapter/grpcclient/tech_stack_detector.go`** (new — real
implementation):

```go
type TechStackDetector struct {
	gitGateway gitgatewayv1.GitGatewayServiceClient
	resolver   usecase.ProjectExecutionResolver // reused, not duplicated — same connectionID resolution AIDecompose/SimpleExecutor already use
}

func (d *TechStackDetector) Detect(ctx context.Context, projectID string) ([]string, error) {
	connectionID, worktreePath, connected, err := d.resolver.ResolveConnection(ctx, tenantIDFrom(ctx), projectID)
	if err != nil || !connected {
		return nil, nil // best-effort — see usecase's own "never blocks decompose" note
	}
	var stack []string
	for _, candidate := range []string{"package.json", "go.mod", "requirements.txt"} {
		resp, err := d.gitGateway.ReadFile(ctx, &gitgatewayv1.ReadFileRequest{ConnectionId: connectionID, Path: path.Join(worktreePath, candidate)})
		if err == nil && resp.GetContent() != "" {
			stack = append(stack, candidateToStackName(candidate))
		}
	}
	return stack, nil
}
```

(Exact `ReadFileRequest`/`ReadFileResponse` field names need a direct check
of `gitgateway.proto` around line 68 before finalizing this call — this
task's sketch assumes `connection_id`/`path`/`content` by analogy with
`SimpleExecutor`'s `RelayRequest` shape, confirm before coding.)

## Test plan (from BE-SOL-002, re-stated for this task's split)

- `buildContext` includes all 5 sources.
- `TechStack` is empty (not an error) when the detector fails or the
  project has no connected dev server.
- `ListChildren` on a task with 2 existing subtasks returns exactly those 2
  titles in `ExistingSubtasks`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run "TestAIDecompose|TestTechStackDetector" -v
```

Expected: clean build; `AIDecompose`'s prompt-building test asserts all 5
context fields appear in the generated prompt; a `TechStackDetector` fake
returning an error never fails `AIDecompose.Execute` itself.

## Execution notes (2026-09-09)

Implemented per the task's own corrected shape, with one further real-code
divergence found and resolved during implementation (documented below).
`ListChildren` added to `TaskRepository` (`ports.go`) and implemented in
`adapter/postgres/repository.go` using the widened `taskSelectColumns`/
`scanTask` helpers; `decomposeContext`/`buildContext`/widened
`buildDecomposePrompt` added to `ai_decompose.go` exactly per the task's
sketch (5 sources: title, description, AI context, tech stack, existing
subtask titles); `TechStackDetector` interface added directly to
`ports.go` (not a separate `usecase/tech_stack_detector.go` file — checked
`AIProviderContextResolver`'s real precedent as the task itself instructed,
confirmed its port interface lives in `ports.go` too, no separate file, so
followed that exact convention).

**Further divergence found beyond what the task file already flags**:
`gitgateway.proto`'s real `ReadFileRequest`/`ReadFileResponse`
(`gitgateway.proto:444-451`) is `{worktree_id, path}` →
`{bytes content, string encoding}` — not the `{connection_id, absolute
path}` → `{string content}` shape this task's own sketch assumed by analogy
with `SimpleExecutor`. Bigger question this raised: `worktree_id` is
documented (`specs/backend-go/tdd/services/git-gateway-service.md:121`) as
"a logical FK into project-service" — a different registry than
`ProjectExecutionResolver`'s infra-fleet-service `connectionId`. Resolved
by following this codebase's own established convention instead of adding
a new project-service client: `ProjectExecutionResolver`'s own doc comment
states "Like git-gateway-service's worktreeID, task-service's projectID IS
the infra-fleet-service connectionId" — i.e. this external identifier is
passed through verbatim across every resource-scoped service's RPC
regardless of the receiving field's local name. `TechStackDetector.Detect`
follows that same convention: `task.ProjectID` is passed straight through
as `ReadFileRequest.WorktreeId`, no new resolver added. Flagged in the new
file's own doc comment for a future reader who might otherwise "fix" this
into a project-service lookup that doesn't exist as a port anywhere in this
service. `ProjectExecutionResolver` itself is therefore NOT used by
`TechStackDetector` (BE-SOL-002's own sketch reused it for connectionID
resolution, which doesn't apply once the real proto shape is known) — this
is a real design correction, not an oversight; noted for the record since
it changes the constructor shape from the task's literal code sample.

Wired `NewTechStackDetector` in `adapter/grpcclient/tech_stack_detector.go`
against `git-gateway-service`'s `ReadFile` RPC (6 candidate manifest
files: package.json/go.mod/requirements.txt/Cargo.toml/pom.xml/Gemfile),
fully best-effort (any error — including "no worktree" for `id == ""` — is
swallowed, returns `nil, nil`). Added `GitGatewayServiceAddr` to
`config.go` and dialed it in `main.go` alongside the other downstream
clients; `NewAIDecompose`'s constructor gained the `techStack` parameter,
updated at its one production call site.

Fixed test fallout beyond the task's own file list: `ListChildren` and
`GetSubtree`(already present)/`RecalculateAncestorProgress` needed adding
to all 3 `TaskRepository` test doubles; `ai_decompose_test.go` needed a new
`fakeTechStackDetector` and every existing `NewAIDecompose(...)` call site
updated for the new 5th parameter. Added exactly the 3 tests the task's own
"Test plan" section names:
`TestAIDecompose_PromptIncludesAllFiveContextSources`,
`TestAIDecompose_TechStackDetectorFailure_NeverFailsExecute`, and
`TestAIDecompose_BuildContext_ListsExistingSubtaskTitles`.

Verify: `go build`/`go vet ./services/task-service/...` both clean
(including `-tags=integration`); `go test ./services/task-service/... -run
"TestAIDecompose|TestTechStackDetector"` — all 9 `TestAIDecompose_*` cases
pass (no dedicated `TestTechStackDetector_*` unit tests were added for the
grpcclient adapter itself — it has no branching logic beyond "loop over
candidates, ignore errors," so its behavior is exercised indirectly through
`TestAIDecompose_TechStackDetectorFailure_NeverFailsExecute`'s fake at the
usecase layer); full `go test ./services/task-service/...` passes with no
regressions.
