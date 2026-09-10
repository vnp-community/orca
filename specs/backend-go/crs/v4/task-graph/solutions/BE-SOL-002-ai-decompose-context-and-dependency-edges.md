# BE-SOL-002: AI decompose — 5-source context, structured JSON proposals, dependency edges, critical path

**Resolves:** [CR-TG-002](../../../../../../docs/crs/v4/task-graph/CR-TG-002-ai-decompose-context-and-dependency-edges.md)
**Service:** `task-service` only
**Depends on:** [BE-SOL-001](./BE-SOL-001-orcatask-data-model-widening.md) (needs `description`/`aiContext`/`estimatedHours`/`promptTemplate` fields to exist)
**Affected files (proposed):**
- `backend-go/services/task-service/internal/usecase/ai_decompose.go` (context bundle, JSON parse)
- `backend-go/services/task-service/internal/usecase/ai_apply.go` (dependency-edge creation)
- `backend-go/services/task-service/internal/domain/subtask_proposal.go` (widen struct)
- `backend-go/services/task-service/internal/domain/critical_path.go` (new — pure function)
- `backend-go/services/task-service/internal/usecase/tech_stack_detector.go` (new — git-gateway-service client)
- `backend-go/proto/orca/task/v1/task.proto` (`GenerateAgentPrompt` RPC — named in TDD §3.2 area, see below)
**Status:** 📋 Proposed — not yet implemented

> **⚠️ Cập nhật sau khi viết task (2026-09-09):** §"Design — dependency
> edges + AIApply" bên dưới phác thảo cùng shape `Tx`-suffixed port không
> tồn tại như BE-SOL-001 (xem correction ở đó). `AIApply`'s transaction
> thật đã dùng `usecase.TxRunner` (`ports.go:167-184`) và **đã implement
> đầy đủ**, không phải điểm cần xây transaction từ đầu. Xem
> [TASK-TG-002-02](../tasks/TASK-TG-002-02-structured-proposal-dependency-edges.md)
> cho design đã sửa theo primitive thật.

---

## Design rationale (grounded in TDD + real code)

`task-service.md` §3.2 already frames the intended shape: *"Gathers task
context, calls `ai-provider-service` to resolve provider/account context,
then relays completion to the Dev Server Agent's `ai.complete`... `AIDecompose`
returns a proposed breakdown; `AIApply` is a separate write RPC that commits
it, matching TS's two-step review-before-commit shape"* (`task-service.md:89-96`).
The real `AIDecompose.Execute` (`internal/usecase/ai_decompose.go:42-73`)
already implements this two-step shape correctly and is not a stub — the
gap the CR identifies is entirely in **what context goes in** and **what
structure comes out**, confirmed directly: `buildDecomposePrompt`
(`ai_decompose.go:79-90`) interpolates only `task.Title`, and
`domain.SubtaskProposal` (`internal/domain/subtask_proposal.go:9-12`) is
`{Title, Description}` with `Description` never populated.

This solution does not touch the AI-provider-resolution or `ai.complete`
relay call — both already correct per the TDD's own description and the
CR's confirmation — it only widens the prompt input and the response
parsing/persistence around that call.

## Design — 5-source context bundle

```go
// ai_decompose.go
type decomposeContext struct {
    Title            string
    Description      string
    AIContext        string
    TechStack        []string
    ExistingSubtasks []string
}

func (u *AIDecompose) buildContext(ctx context.Context, task domain.Task) decomposeContext {
    stack, _ := u.techStack.Detect(ctx, task.ProjectID) // best-effort, nil on failure — never blocks decompose
    existing, _ := u.repo.ListChildren(ctx, task.ID)
    return decomposeContext{
        Title: task.Title, Description: task.Description, AIContext: task.AIContext,
        TechStack: stack, ExistingSubtasks: titlesOf(existing),
    }
}
```

`TechStackDetector` calls `git-gateway-service`'s existing file-read RPC
(the same one `git-gateway-service.md` §3.1 names for commit-message
generation, per `task-service.md:92-93`'s own cross-reference) to read
`package.json`/`go.mod`/`requirements.txt` at the worktree root — no new
RPC on `git-gateway-service`'s side, reuse only.

## Design — structured JSON proposal (replaces text-list parser)

```go
// domain/subtask_proposal.go
type SubtaskProposal struct {
    Title          string
    Description    string
    Type           string
    EstimatedHours float64
    DependsOnIndex []int // indices into the SAME batch, resolved to real edges in AIApply
    PromptTemplate string
}
```

```go
// ai_decompose.go — parseSubtaskProposals rewritten
func parseSubtaskProposals(raw string) ([]domain.SubtaskProposal, error) {
    var proposals []domain.SubtaskProposal
    if err := json.Unmarshal([]byte(raw), &proposals); err != nil {
        return nil, fmt.Errorf("ai_decompose: AI response is not valid JSON: %w", err)
        // explicit error — the current text-list parser silently degrades to
        // partial/empty results on malformed input; this must not.
    }
    return proposals, nil
}
```

Prompt template updated to request `format: 'json'` with an explicit schema
example in the prompt body (the `relay.call('ai.complete', {format: 'json'})`
call itself is unchanged — already correct).

## Design — dependency edges + `AIApply` (reuses BE-SOL-001's atomic `AddEdge`)

```go
// ai_apply.go
func (u *AIApply) Execute(ctx context.Context, parentID string, proposals []domain.SubtaskProposal) error {
    return u.tx.RunInTx(ctx, func(tx ports.Tx) error {
        createdIDs := make([]string, len(proposals))
        for i, p := range proposals {
            id, err := u.repo.CreateTx(ctx, tx, domain.Task{Title: p.Title, Description: p.Description,
                Type: p.Type, EstimatedHours: &p.EstimatedHours, PromptTemplate: p.PromptTemplate, ParentID: &parentID})
            if err != nil { return err }
            createdIDs[i] = id
            if err := u.repo.InsertEdgeTx(ctx, tx, id, parentID, domain.EdgeKindParentChild); err != nil { return err }
        }
        for i, p := range proposals {
            for _, depIdx := range p.DependsOnIndex {
                if err := u.repo.InsertEdgeTx(ctx, tx, createdIDs[i], createdIDs[depIdx], domain.EdgeKindDependsOn); err != nil { return err }
            }
        }
        return nil
    })
}
```

Note: this bypasses [BE-SOL-001](./BE-SOL-001-orcatask-data-model-widening.md)'s
`AddEdge` usecase (which runs its own transaction) because `AIApply` needs
all edges + all task creates in **one** transaction — it calls the same
`InsertEdgeTx`/cycle-check primitives at the repository layer directly,
not the `AddEdge` usecase wrapper. Cycle-check is still run per edge before
insert (AI-generated `DependsOnIndex` referencing a later index than itself
in a way that would cycle is rejected the same as a manual `AddEdge` would).

## Design — `CalculateCriticalPath` (pure domain function)

```go
// domain/critical_path.go
func CalculateCriticalPath(tasks []Task, edges []Edge) []string {
    // topological sort (Kahn's algorithm, same primitive style as
    // workflow-service's DAG.BuildWaves per workflow-service.md §4 — reused
    // conceptually, not code-shared across services per the microservices
    // boundary) + longest-path DP keyed on EstimatedHours
}
```

## Design — `GenerateAgentPrompt` (separate flow from decompose)

```protobuf
rpc GenerateAgentPrompt(GenerateAgentPromptRequest) returns (GenerateAgentPromptResponse);
```

Distinct usecase, same `ai.complete` relay primitive as `AIDecompose` but a
different prompt template (produces a `PromptTemplate` string for one task,
not a subtask breakdown) — writes result to `Task.PromptTemplate`, which
[BE-SOL-005](./BE-SOL-005-task-agent-execution-permission-and-complex-executor.md)'s
`SimpleExecutor.buildExecutePrompt` consumes.

## Test plan

- `buildContext` includes all 5 sources; `TechStack` empty (not error) when
  detector fails.
- Malformed AI JSON response → `AIDecompose` returns error, zero subtasks
  created (no partial-apply).
- `AIApply` with `DependsOnIndex` forming a cycle within the same batch →
  entire transaction rolled back, zero tasks created (test asserts DB has
  no orphaned partial rows).
- `CalculateCriticalPath` on a 5-node DAG with 2 branches — longest path by
  hours matches hand-computed expectation.

## Not in scope (per the CR)

- Velocity-based estimation (optional per CR-TG-002 §3) — not implemented
  this pass, no team-throughput data source identified yet.
- UI for critical-path highlighting — [FE-SOL-001](../../../../../frontend/crs/v4/task-graph/solutions/FE-SOL-001-task-crud-board-grant-ui.md).

## References

- [CR-TG-002](../../../../../../docs/crs/v4/task-graph/CR-TG-002-ai-decompose-context-and-dependency-edges.md)
- [SOL-TG-02](../../../../bugs/logic-v1/solutions/SOL-TG-02-ai-task-planning.md)
- `specs/backend-go/tdd/services/task-service.md` §3.2
