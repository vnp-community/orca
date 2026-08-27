# TASK-224: Relay design — real `SimpleExecutor` + `aiDecompose`/`aiApply` (task-service's execution-plane relay)

**From Solution:** SOL-034 ("Making `SimpleExecutor` real" + "`aiDecompose` — relay design, explicit" sections)
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`, `internal/usecase/ports.go`, `internal/usecase/ai_decompose.go`, `ai_apply.go` (new), `internal/adapter/grpcclient/simple_executor.go` (real implementation, replaces `StubSimpleExecutor`), `internal/adapter/grpcclient/project_execution_resolver.go` (new), `internal/adapter/grpcclient/aidecompose_relay.go` (new), `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-223 (`AIApply` reuses `CreateTask`+`AddEdge`'s existing logic; `AIDecompose` reuses `TaskRepository.Get`, unaffected by TASK-223's new methods but grouped here since both this task and TASK-223 touch `ports.go`/`main.go` — rebase onto whichever merges first)
**Status:** `[x]` DONE. Both gaps this task previously flagged `[partial]` are now genuinely closed, not just re-documented:

- **Gap 1 (agent.exec → agent.execPrompt), closed for real.** Read both RPC
  handlers in full: `agent/src/relay/agent-rpc-dispatch.ts`'s `case
  'agent.exec'` (line 913, a generic `binary/args/cwd/stdin/env/timeoutMs`
  "run this literal binary" primitive, result
  `{stdout,stderr,exitCode,timedOut}`) and `case 'agent.execPrompt'` (line
  992, delegating to `agent/src/relay/agent-print-mode-exec.ts`'s
  `handleAgentExecPrompt`, lines 33-178: required `prompt`+`worktreePath`,
  optional `stepId`/`trustPreset`/`model` (defaults to `"claude"`, the only
  supported one)/`accountId`/`env`/`timeoutMs`, result
  `{stdout,stderr,exitCode,timedOut,stepId}` — no `executionRef` field on
  either method's REAL result shape; the prior wiring's `{executionRef}`
  unmarshal target was already a fabrication). Found the real, already-proven
  param-construction pattern at `backend/src/main/workflow/StepExecutors.ts:101-131`
  (`executeAgent()`): `relay.call('agent.execPrompt', { stepId, prompt,
  worktreePath, trustPreset, traceId, ...(resolved ? { accountId,
  model } : {}) })` — accountId/model omitted ENTIRELY when unresolved,
  falling back to the agent's own default account. `simple_executor.go`
  now calls `agent.execPrompt`, resolves `worktreePath` from
  infra-fleet-service's `ResolveConnectionResponse.repo_path` (the same
  field git-gateway-service's `ConnectionResolver` already reads for its own
  `RepoPath`, `internal/usecase/ports.go:26-37` in that service — extended
  `usecase.ProjectExecutionResolver`'s signature to return it), builds the
  prompt via a `buildExecutePrompt` helper following `ai_decompose.go`'s
  existing `buildDecomposePrompt` plain-text convention, and omits
  `accountId`/`model` per `StepExecutors.ts`'s own established
  omit-when-unresolved convention (task-service has no per-task
  AI-provider-account pin today). A non-zero exit code or `timedOut:true` is
  now a real error, not a synthesized success. See
  `internal/adapter/grpcclient/simple_executor.go`'s doc comment for the
  full citation trail and honest limits kept unchanged (still
  dispatch-only, no completion callback). Tests:
  `simple_executor_test.go` (method name, params shape, non-zero-exit,
  timed-out, no-worktree-path cases).
- **Gap 2 (AIApply's non-transactional loop), closed for real.**
  Re-searched the WHOLE `backend-go` tree (not just task-service) before
  writing any code — the "no `WithTx`/`UnitOfWork` precedent exists
  anywhere in this repo" premise this task previously stated is no longer
  true (and, per `git log`, wasn't checked broadly enough even when it was
  written): `project-service`, `issue-tracking-service`, `usage-service`,
  `orchestration-service`, and `automation-service` all call
  `pool.Begin(ctx)` directly, and `credential-broker-service` goes further
  with a named `TxRunner`/`RunInTx` port + `dbtx` pool-or-tx abstraction in
  its postgres adapter (`internal/usecase/ports.go`'s `TxRunner`,
  `internal/adapter/postgres/repository.go`'s `dbtx`/`RunInTx` via
  `pgx.BeginFunc`). Adopted credential-broker-service's `TxRunner` shape
  exactly: added `usecase.TxRunner` (`RunInTx(ctx, fn func(ctx, tasks
  TaskRepository, edges EdgeRepository) error) error`), implemented by
  `internal/adapter/postgres.Repository.RunInTx` via `pgx.BeginFunc` and a
  `dbtx` interface (`Exec`/`QueryRow`/`Query`) so every existing query
  method works unchanged whether `r.db` is the pool or an open `pgx.Tx`.
  `AIApply.Execute` now runs its entire create-subtask+add-edge loop inside
  one `RunInTx` call; `main.go` wires `repo` itself (which already
  implements `TxRunner`) into `NewAIApply`. Proof, not just an error path:
  `internal/usecase/ai_apply_test.go`'s
  `TestAIApply_MidLoopFailure_RollsBackEntireSubtree` (against an in-memory
  `fakeTxRunner` mirroring credential-broker-service's own rollback-simulating
  fake) AND `internal/adapter/postgres/repository_test.go`'s
  `TestRepository_RunInTx_RollsBackAllWritesOnError` (against a REAL
  Postgres transaction via testcontainers, forcing a genuine
  `task_edges_single_parent` unique-constraint violation) both assert the
  first proposal's subtask+edge do NOT survive after a later proposal
  fails — not merely that an error is returned. `TestRepository_RunInTx_CommitsAllWritesTogether`
  covers the happy path against real Postgres too. Both integration tests
  pass individually (`go test -tags=integration
  ./internal/adapter/postgres/... -run TestRepository_RunInTx -v`); the
  package's pre-existing testcontainers readiness check (port-open, not
  `pg_isready`) is flaky under back-to-back container churn independent of
  this change — confirmed by re-running `TestRepository_Update_WrongTenant_Fails`
  (an unrelated, pre-existing test) alone after it flaked in a full-package
  run, which then passed.

`go build ./... && go vet ./... && go test ./...` is clean for
`task-service`. `ComplexExecutor` stays a stub as specified — out of
scope, unchanged.

---

## Context

Per `task-service.md` §2/§3.1/§3.2 and
`08-inter-service-communication.md`'s "only two Go services that talk to
the execution plane" rule (`infra-fleet-service` and
`git-gateway-service`), **task-service cannot call the Dev Server Agent
directly** — both new relay paths in this task go through
`infra-fleet-service`'s gRPC surface as a client, exactly the way
`git-gateway-service`'s `ConnectionResolver`/`RelayExecutor` do
(`internal/adapter/grpcclient/resolver.go`, `relay_executor.go`), reusing
the *same* generic `Relay(connectionId, method, paramsJson)` RPC — not a
new relay mechanism.

`StubSimpleExecutor`'s existing doc comment names the gap: `Execute` only
receives `(tenantID, taskID, requestID)` — no `connectionId`. A real
implementation needs one more resolution step: `taskID` → `project_id`
(already a real column, `task.tasks.project_id`, added for Epic C) → the
project's bound dev server's `connectionId`. Per
`grpcclient/resolver.go`'s established convention (confirmed by reading
that file): `ResolveConnectionRequest` has no separate project/worktree
field, only `connection_id` — so `project_id` is passed through verbatim
as the connection id, exactly like `git-gateway-service`'s `worktreeID`.

`ComplexExecutor` is **explicitly out of scope** — it hands off to
`orchestration-service`'s coordinator, materially larger than a single
relay call. `StubComplexExecutor` stays a stub after this task ships; flag
this explicitly in the PR description.

## Changes to make

### Step 1: New port — `internal/usecase/ports.go`

```go
// ProjectExecutionResolver resolves a project's execution target
// (connectionId, or none for host-local) via infra-fleet-service —
// task-service never calls project-service or infra-fleet-service directly
// from this port's consumers (SimpleExecutor, AIDecompose); the resolution
// itself lives in internal/adapter/grpcclient, mirroring
// git-gateway-service's ConnectionResolver split.
type ProjectExecutionResolver interface {
	ResolveConnection(ctx context.Context, tenantID, projectID string) (connectionID string, connected bool, err error)
}

// AIProviderContextResolver resolves AI provider/account context for a
// tenant+user by calling ai-provider-service — distinct from
// ProjectExecutionResolver (execution target) and from the relay client
// below (the actual completion call).
type AIProviderContextResolver interface {
	ResolveContext(ctx context.Context, tenantID, userID string) (providerContext string, err error)
}

// AICompleter relays a prompt to the Dev Server Agent's ai.complete method
// over a resolved connectionID — same port shape as
// git-gateway-service.AICompleter, implemented here against
// infra-fleet-service's Relay RPC rather than duplicated per-service.
type AICompleter interface {
	Complete(ctx context.Context, connectionID, prompt string) (string, error)
}
```

Update `SimpleExecutor`'s doc comment (`ports.go:97-106`) to drop the "STUB
in this scaffold" note once this task's implementation lands (leave the
interface signature itself unchanged — `Execute(ctx, tenantID, taskID,
requestID) (executionRef string, err error)` — only the implementation
changes).

### Step 2: Real `SimpleExecutor` — `internal/adapter/grpcclient/simple_executor.go`

Replace the existing stub implementation:

```go
package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// SimpleExecutor implements usecase.SimpleExecutor for real, replacing the
// prior StubSimpleExecutor — dispatches Execute's simple path to
// infra-fleet-service's agent.exec relay method. See task-service.md §3.1
// and this task's Context note for why task-service goes through
// infra-fleet-service rather than dialing the Dev Server Agent itself.
type SimpleExecutor struct {
	tasks    usecase.TaskRepository
	resolver usecase.ProjectExecutionResolver
	relay    infrafleetv1.InfraFleetServiceClient
}

func NewSimpleExecutor(tasks usecase.TaskRepository, resolver usecase.ProjectExecutionResolver, relay infrafleetv1.InfraFleetServiceClient) *SimpleExecutor {
	return &SimpleExecutor{tasks: tasks, resolver: resolver, relay: relay}
}

func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	task, err := s.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: load task: %w", err)
	}
	connectionID, connected, err := s.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: resolve connection: %w", err)
	}
	if !connected {
		// Per git-gateway-service.md §8's precedent: a resolve failure or
		// not-connected connectionId is a real error, never a silent local
		// fallback — task-service has no local agent.exec equivalent of its
		// own (unlike git-gateway-service's §2 step 3, there is no "this
		// service's own host" case for task execution).
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", nil)
	}

	paramsJSON, err := json.Marshal(map[string]any{
		"taskId":    taskID,
		"title":     task.Title,
		"requestId": requestID,
	})
	if err != nil {
		return "", fmt.Errorf("simple_executor: marshal params: %w", err)
	}
	resp, err := s.relay.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: "agent.exec", ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("simple_executor: relay agent.exec: %w", err)
	}
	var result struct {
		ExecutionRef string `json:"executionRef"`
	}
	if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
		return "", fmt.Errorf("simple_executor: unmarshal agent.exec result: %w", err)
	}
	return result.ExecutionRef, nil
}
```

This is a real, non-stub implementation for the simple path. Two honest
limits carried over unchanged, not solved by this task: (1) task-service
still has no execution-completion callback — `agent.exec`'s result here is
the *dispatch* acknowledgment, not a completion signal, so
`HasActiveExecutions`' one-way-transition caveat is unaffected; (2) the
`agent.exec` param/result JSON shape above is a best-effort mirror of
`git-gateway-service`'s own `relay_executor.go` caveat — verify against
`specs/agent/api/agent-rpc-catalog-runtime.md`'s real `agent.exec` handler
contract before finalizing field names.

### Step 3: `ProjectExecutionResolver` implementation — new file `internal/adapter/grpcclient/project_execution_resolver.go`

```go
// Package grpcclient — this file mirrors git-gateway-service's
// ConnectionResolver (internal/adapter/grpcclient/resolver.go) exactly:
// project_id is passed through verbatim as infra-fleet-service's
// connection_id, per that file's confirmed convention
// (ResolveConnectionRequest has no separate project/worktree field).
package grpcclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type ProjectExecutionResolver struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewProjectExecutionResolver(client infrafleetv1.InfraFleetServiceClient) *ProjectExecutionResolver {
	return &ProjectExecutionResolver{client: client}
}

func (p *ProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, bool, error) {
	resp, err := p.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{ConnectionId: projectID})
	if err != nil {
		return "", false, fmt.Errorf("grpcclient: ResolveConnection(%q): %w", projectID, err)
	}
	if !resp.GetConnected() {
		return "", false, nil
	}
	return projectID, true, nil
}
```

### Step 4: `AICompleter` implementation — new file `internal/adapter/grpcclient/aidecompose_relay.go`

```go
package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// AICompleter relays to the Dev Server Agent's ai.complete method via
// infra-fleet-service's Relay RPC — same method name and best-effort
// param/result shape as git-gateway-service's RelayExecutor.Complete
// (relay_executor.go:137-158); duplicated per-service rather than shared,
// since each service's Relay client is a distinct generated gRPC stub over
// its own dialed connection to infra-fleet-service.
type AICompleter struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewAICompleter(client infrafleetv1.InfraFleetServiceClient) *AICompleter {
	return &AICompleter{client: client}
}

func (a *AICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	paramsJSON, err := json.Marshal(map[string]any{"prompt": prompt})
	if err != nil {
		return "", fmt.Errorf("grpcclient: marshal ai.complete params: %w", err)
	}
	resp, err := a.client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: "ai.complete", ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("grpcclient: relay ai.complete: %w", err)
	}
	var result struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
		return "", fmt.Errorf("grpcclient: unmarshal ai.complete result: %w", err)
	}
	return result.Content, nil
}
```

### Step 5: Proto — `AIDecompose`/`AIApply`

Add to the `TaskService` service block:

```protobuf
  rpc AIDecompose(AIDecomposeRequest) returns (AIDecomposeResponse);
  rpc AIApply(AIApplyRequest) returns (AIApplyResponse);
```

Append messages:

```protobuf
message AIDecomposeRequest {
  string task_id = 1;
}
message SubtaskProposal {
  string title = 1;
  string description = 2;
}
message AIDecomposeResponse {
  repeated SubtaskProposal proposals = 1; // review-before-commit — not yet written to task_edges
}

// AIApply commits a (possibly user-edited) proposal set from a prior
// AIDecompose call — the two-step review-before-commit shape.
message AIApplyRequest {
  string task_id = 1;
  repeated SubtaskProposal proposals = 2;
}
message AIApplyResponse {
  repeated Task created_subtasks = 1;
}
```

### Step 6: Usecases

`ai_decompose.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type AIDecomposeInput struct {
	TenantID string
	UserID   string
	TaskID   string
}

type AIDecompose struct {
	tasks      TaskRepository
	aiProvider AIProviderContextResolver
	resolver   ProjectExecutionResolver
	relay      AICompleter
}

func NewAIDecompose(tasks TaskRepository, aiProvider AIProviderContextResolver, resolver ProjectExecutionResolver, relay AICompleter) *AIDecompose {
	return &AIDecompose{tasks: tasks, aiProvider: aiProvider, resolver: resolver, relay: relay}
}

func (uc *AIDecompose) Execute(ctx context.Context, in AIDecomposeInput) ([]domain.SubtaskProposal, error) {
	task, err := uc.tasks.Get(ctx, in.TenantID, in.TaskID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	providerCtx, err := uc.aiProvider.ResolveContext(ctx, in.TenantID, in.UserID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PROVIDER_RESOLVE_FAILED", "failed to resolve AI provider context", err)
	}
	connectionID, connected, err := uc.resolver.ResolveConnection(ctx, in.TenantID, task.ProjectID)
	if err != nil || !connected {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "TASK_AI_DECOMPOSE_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}
	prompt := buildDecomposePrompt(task, providerCtx)
	content, err := uc.relay.Complete(ctx, connectionID, prompt)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_FAILED", "failed to generate subtask proposals via AI relay", err)
	}
	return parseSubtaskProposals(content), nil
}
```

`aiDecompose`'s not-connected case must return
`TASK_AI_DECOMPOSE_NO_CONNECTION` rather than silently returning an empty
proposal list — this is asserted explicitly in TASK-226's test plan.

`ai_apply.go` — a plain write usecase: for each `SubtaskProposal`, reuse
`CreateTask`'s existing insert-task-row logic plus an `AddEdge`
(`domain.EdgeKindParentChild`) call, invoked N times **in one
transaction** (one failed subtask insert should not leave a partial
subtree, per `task-service.md` §8's transactional-consistency posture for
`AddEdge`'s cycle-check-then-write). If `TaskRepository`/`EdgeRepository`
don't already expose a transaction-scoped variant, add a
`WithTx(ctx, fn) error` method to both ports (or a shared
`UnitOfWork` port) as part of this step — check `ports.go` for any
existing transaction primitive before adding a new one:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type AIApplyInput struct {
	TenantID  string
	TaskID    string
	Proposals []domain.SubtaskProposal
}

type AIApply struct {
	createTask *CreateTask
	addEdge    *AddEdge
}

func NewAIApply(createTask *CreateTask, addEdge *AddEdge) *AIApply {
	return &AIApply{createTask: createTask, addEdge: addEdge}
}

func (uc *AIApply) Execute(ctx context.Context, in AIApplyInput) ([]domain.Task, error) {
	// Wrap in a single transaction per this usecase's doc comment above —
	// exact mechanism (WithTx / UnitOfWork) depends on what ports.go
	// already exposes; do not ship this loop non-transactional.
	created := make([]domain.Task, 0, len(in.Proposals))
	for _, p := range in.Proposals {
		task, err := uc.createTask.Execute(ctx, CreateTaskInput{TenantID: in.TenantID, Title: p.Title, ParentID: in.TaskID})
		if err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to create subtask from AI proposal", err)
		}
		if err := uc.addEdge.Execute(ctx, AddEdgeInput{TenantID: in.TenantID, FromTaskID: in.TaskID, ToTaskID: task.ID, Type: domain.EdgeKindParentChild}); err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to link subtask to parent", err)
		}
		created = append(created, task)
	}
	return created, nil
}
```

Adjust `CreateTaskInput`/`AddEdgeInput` field names against this package's
actual existing types (`create_task.go`, `add_edge.go`) before finalizing
— the sketch above assumes their shape from `task.proto`'s
`CreateTaskRequest`/`AddEdgeRequest` fields.

### Step 7: gRPC adapter + composition root

Add `aiDecompose *usecase.AIDecompose`, `aiApply *usecase.AIApply` fields
to `Server`, translation methods following `ExecuteTask`'s existing shape,
and wire `simpleExecutorUC := grpcclient.NewSimpleExecutor(taskRepo,
projectExecutionResolver, infraFleetClient)` (replacing the stub
constructor call) plus `aiDecomposeUC`/`aiApplyUC` into
`cmd/server/main.go`, dialing `ai-provider-service` alongside the existing
`infra-fleet-service` dial (same pattern as TASK-211's git-gateway-service
change) for `AIProviderContextResolver`.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/task-service
go build ./... && go vet ./...
```
