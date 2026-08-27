# SOL-TG-02: Rich decompose context, structured JSON proposals, dependency edges, critical path, and a prompt-generation flow

**Resolves:** [BUG-TG-02](../BUG-TG-02-ai-task-planning-partial.md)
**Service:** `task-service` (primary) + `git-gateway-service` (read-only, reused as-is for tech-stack detection)
**Affected files (proposed):**
- `backend-go/proto/orca/task/v1/task.proto` (`SubtaskProposal` fields, `GenerateAgentPrompt` RPC)
- `backend-go/services/task-service/internal/domain/subtask_proposal.go` (widen)
- `backend-go/services/task-service/internal/domain/critical_path.go` (new — pure `CalculateCriticalPath`)
- `backend-go/services/task-service/internal/usecase/ai_decompose.go` (context bundle, JSON parse)
- `backend-go/services/task-service/internal/usecase/ai_apply.go` (dependency edges from proposals)
- `backend-go/services/task-service/internal/usecase/generate_agent_prompt.go` (new)
- `backend-go/services/task-service/internal/usecase/ports.go` (`ProjectContextResolver`, `TechStackDetector`, `VelocityResolver`)
- `backend-go/services/task-service/internal/adapter/grpcclient/tech_stack_detector.go` (new — calls git-gateway-service `ReadFile`)
- `backend-go/services/task-service/internal/adapter/grpc/server.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`task-service.md §3.2` states the intended shape precisely: "Gathers task
context, calls `ai-provider-service` to resolve provider/account context,
then relays completion to the Dev Server Agent's `ai.complete`... the same
AI-inference-off-backend principle already applied to
`git-gateway-service`'s commit-message generation." The current
`buildDecomposePrompt` (`ai_decompose.go:79-90`) already follows that
relay-not-inference shape correctly — this solution only widens *what*
gets packaged into the prompt and *what* comes back, not the
architecture. §10's "one prescribed behavior change... `TaskAIPlanner`'s
decomposition call moves to the relay-to-execution-plane pattern" is
already implemented; nothing here revisits that decision.

This bug's context-source gaps are a direct consequence of BUG-TG-01's
field gaps: `task.Description`/`task.AIContext`/`estimated_hours` don't
exist on `domain.Task` today. **This solution depends on
[SOL-TG-01](./SOL-TG-01-task-graph-structural-management.md) landing
first** for `Description`, `AIContext`, `EstimatedHours`, and
`PromptTemplate` — every context-bundle and proposal-field change below
assumes those fields exist.

**Genuine extension beyond the TDD, flagged explicitly**: the spec's
`collectProjectContext()` reads `package.json`/`go.mod`/etc. directly off
the target host's filesystem. `task-service.md` never names a port for
this — the closest existing capability is `git-gateway-service`'s file-I/O
RPC group (`gitgateway.proto:57-74`; `ReadFile`/`ReadDir` in particular),
which already exists in the current proto (confirmed:
`backend-go/proto/orca/gitgateway/v1/gitgateway.proto:58-61` — the
`FileIO` group `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md`
proposed is already landed). This solution adds a new
`task-service → git-gateway-service` dependency edge, absent from
`02-microservices-decomposition.md`'s dependency graph
(`02-microservices-decomposition.md:110-166` lists `task --> tenant`,
`task --> aiprov`, `task --> infra`, `task --> orch`, but no `task -->
git`) — flag this as a scope addition to that graph, the same way
SOL-009 flagged its own proto extension, rather than silently adding an
undocumented edge.

## Design — domain

### `SubtaskProposal` — widen to match the spec's structured breakdown

```go
// internal/domain/subtask_proposal.go
type SubtaskProposal struct {
    Title          string
    Description    string
    Type           string   // task|bug|feature — mirrors Task.Type (SOL-TG-01)
    EstimatedHours *float64
    // DependsOnIndices names OTHER proposals in the SAME AIDecompose
    // response by their 0-based position, e.g. proposal[2] depends on
    // proposal[0] -> DependsOnIndices: []int{0}. Indices, not IDs, because
    // proposals have no Task.ID until AIApply creates them — this mirrors
    // how the AI response itself can only refer to sibling proposals by
    // their position in its own numbered output.
    DependsOnIndices []int
    PromptTemplate   string
}
```

### `CalculateCriticalPath` — pure domain function, `domain/critical_path.go`

Spec's DAG/topological-sort algorithm over `estimated_hours`. Operates on
already-fetched `TaskEdge`s (kind `depends_on`) and a task-ID→hours map —
same "pure function, no DB" discipline as `DetectCycle`/`ResolveGrant`:

```go
// CalculateCriticalPath returns the longest-duration path through the
// depends_on DAG rooted implicitly at whichever nodes have no incoming
// depends_on edge, plus its total duration — the standard CPM longest-path
// algorithm, not a new one invented for this codebase. Assumes the edge
// set is already acyclic (DetectCycle is the enforcement point, at
// AddEdge/AIApply time — this function does not re-validate).
func CalculateCriticalPath(edges []TaskEdge, hours map[string]float64) (path []string, totalHours float64) {
    // adjacency + in-degree, per-kind filtered exactly like DetectCycle
    // (task_edge.go:86-91's pattern reused)
    adjacency, indegree := buildDependsOnGraph(edges)
    order := topologicalSort(adjacency, indegree) // Kahn's algorithm; empty if a cycle slipped through (defensive, not expected)

    longest := map[string]float64{}   // longest path ending AT this node
    predecessor := map[string]string{}
    for _, id := range order {
        longest[id] = hours[id]
        for _, from := range incomingOf(id, edges) {
            if longest[from]+hours[id] > longest[id] {
                longest[id] = longest[from] + hours[id]
                predecessor[id] = from
            }
        }
    }
    // walk back from the max-longest node via predecessor to build `path`
    ...
}
```

Unit-tested exactly like `DetectCycle`/`ResolveGrant`: diamond graphs,
parallel branches, a chain with one long-pole node, zero-`estimated_hours`
nodes (default to 0, per spec's "if AI/user leaves estimate blank" — never
a computation error).

## Design — usecase

### `AIDecompose` — five-source context bundle

```go
// internal/usecase/ai_decompose.go
type AIDecompose struct {
    tasks        TaskRepository
    edges        EdgeRepository        // for existing-subtask dedup + velocity
    aiProvider   AIProviderContextResolver
    resolver     ProjectExecutionResolver
    projectCtx   ProjectContextResolver // new — project name/repo, via project-service
    techStack    TechStackDetector      // new — via git-gateway-service.ReadFile, see below
    relay        AICompleter
}

func (uc *AIDecompose) Execute(ctx context.Context, in AIDecomposeInput) ([]domain.SubtaskProposal, string, error) {
    // ...existing tenant/task/provider/connection resolution unchanged...

    existingSubtasks, err := uc.edges.ListFrom(ctx, tenantID, in.TaskID, domain.EdgeKindParentChild)
    // hydrate titles for the dedup instruction below

    project, err := uc.projectCtx.Resolve(ctx, tenantID, task.ProjectID) // name, repo URL
    stack, err := uc.techStack.Detect(ctx, tenantID, task.ProjectID)     // best-effort: log+continue on error, never block decompose on a file-read failure
    velocity, err := uc.velocityResolver.RecentCompletedTasks(ctx, tenantID, task.ProjectID, 10) // titles + estimated/actual hours of last N Done tasks in this project

    prompt := buildDecomposePrompt(task, providerCtx, project, stack, existingSubtaskTitles, velocity)
    content, err := uc.relay.Complete(ctx, connectionID, prompt)
    proposals, err := parseSubtaskProposalsJSON(content)
    if err != nil {
        // structured error, not a generic KindInternal — see error-handling section
        return nil, "", apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_INVALID_JSON", "AI response was not valid JSON", err)
    }
    return proposals, content, nil // raw content returned for AIPlanJSON persistence, see below
}
```

`buildDecomposePrompt` asks for **structured JSON** now instead of a
numbered list, since the response shape needs `type`/`estimated_hours`/
`depends_on`/`prompt_template` per subtask — a numbered list has no field
structure to carry those in. This is the one wire-format change flagged
per this file's "genuine extension" note: `parseSubtaskProposals`'s
existing numbered-list parser (`ai_decompose.go:98-118`) is fully replaced
by `parseSubtaskProposalsJSON`, not kept as a fallback — `AIDecompose`/
`AIApply` are unreleased RPCs with no wscompat/REST callers wired yet per
BUG-034, so there is no live wire-format consumer to keep backward
compatible.

```
Prompt shape (schematic):
  Task: {title}
  Description: {description}
  AI Context: {aiContext}
  Project: {project.name} ({project.repoURL})
  Tech stack: {stack.languages}, {stack.frameworks}
  Existing subtasks (do not duplicate): {existingSubtaskTitles}
  Recent team velocity: {velocity items, title + actual_hours}

  Respond with JSON matching:
  {"subtasks": [{"title","description","type","estimated_hours","depends_on":[<indices>],"prompt_template"}],
   "notes": "<string>"}
```

`parseSubtaskProposalsJSON` unmarshals into a wire struct matching that
shape 1:1, mapping into `[]domain.SubtaskProposal` — no ordinal-stripping
logic needed (JSON has real structure), replacing `parseOrdinal`/the
`.`/`)` index-scan entirely.

### `TechStackDetector` — via git-gateway-service, not a new file-I/O client

```go
// internal/adapter/grpcclient/tech_stack_detector.go
type TechStackDetector struct {
    git gitgatewayv1.GitGatewayServiceClient // dialed to git-gateway-service
}

func (d *TechStackDetector) Detect(ctx context.Context, tenantID, projectID string) (domain.TechStack, error) {
    // Resolves projectID -> its default worktree_id via project-service
    // (same ProjectExecutionResolver-adjacent lookup SimpleExecutor already
    // does for worktreePath, task-service/internal/adapter/grpcclient/simple_executor.go:137),
    // then probes a fixed candidate list — package.json, go.mod, pom.xml,
    // pyproject.toml, Cargo.toml — via ReadFile, treating NOT_FOUND as
    // "this ecosystem isn't present" rather than an error.
    for _, candidate := range techStackCandidates {
        resp, err := d.git.ReadFile(ctx, &gitgatewayv1.ReadFileRequest{WorktreeId: worktreeID, Path: candidate.path})
        if isNotFound(err) { continue }
        if err != nil { return domain.TechStack{}, err }
        candidate.parse(resp.GetContent()) // -> languages/frameworks accumulator
    }
    return stack, nil
}
```

Best-effort by design (per the usecase snippet above): a tech-stack
detection failure degrades the prompt's richness, it must never fail
`AIDecompose` outright — the spec's context bundle is an enrichment, not a
precondition for producing a plan at all.

### `AIApply` — dependency edges from proposal indices + `ai_plan_json` persistence

```go
func (uc *AIApply) Execute(ctx context.Context, in AIApplyInput) ([]domain.Task, error) {
    created := make([]domain.Task, 0, len(in.Proposals))
    return uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
        createTask := NewCreateTask(tasks)
        addEdge := NewAddEdge(edges) // now transaction-scoped per SOL-TG-01's AddEdge atomicity fix — reused unchanged
        for i, p := range in.Proposals {
            task, err := createTask.Execute(ctx, CreateTaskInput{
                Title: p.Title, Description: p.Description, ParentID: in.TaskID,
                Type: p.Type, EstimatedHours: p.EstimatedHours, PromptTemplate: p.PromptTemplate,
            })
            if err != nil { return ... }
            if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: in.TaskID, ToTaskID: task.ID, Kind: domain.EdgeKindParentChild}); err != nil { return ... }
            created = append(created, task)
        }
        // Second pass: proposal indices only resolve to real Task.IDs once
        // every proposal in this call has been created above — depends_on
        // edges (which trigger SOL-TG-01's auto-block) are added only
        // after all siblings exist.
        for i, p := range in.Proposals {
            for _, depIdx := range p.DependsOnIndices {
                if depIdx < 0 || depIdx >= len(created) { continue } // defensive: AI hallucinated an out-of-range index
                if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: created[i].ID, ToTaskID: created[depIdx].ID, Kind: domain.EdgeKindDependsOn}); err != nil { return ... }
            }
        }
        if in.RawAIResponse != "" {
            if err := tasks.UpdateAIPlanJSON(ctx, tenantID, in.TaskID, in.RawAIResponse); err != nil { return ... } // new repo method, SOL-TG-01's ai_plan_json column
        }
        return nil
    })
}
```

Still one transaction end-to-end (SOL-TG-01/TASK-224's existing
`TxRunner.RunInTx` shape, `ai_apply.go:53-78`, unchanged) — the dependency
pass is additional work inside the same transaction, not a second
round-trip, so `TestAIApply_MidLoopFailure_RollsBackEntireSubtree`'s
existing rollback guarantee extends to the new edges without a new test
concept.

### `GenerateAgentPrompt` — the second AI flow

New usecase, structurally a smaller sibling of `AIDecompose`:

```go
// internal/usecase/generate_agent_prompt.go
type GenerateAgentPrompt struct {
    tasks TaskRepository
    aiProvider AIProviderContextResolver
    resolver ProjectExecutionResolver
    relay AICompleter
}

func (uc *GenerateAgentPrompt) Execute(ctx context.Context, in GenerateAgentPromptInput) (string, error) {
    // same tenant/task/provider/connection resolution as AIDecompose
    prompt := buildPromptGenerationPrompt(task) // "write one ready-to-use agent prompt for completing this task, given its description/aiContext..."
    generated, err := uc.relay.Complete(ctx, connectionID, prompt)
    if err != nil { return "", ... }
    if in.Save {
        if err := uc.tasks.UpdatePromptTemplate(ctx, tenantID, in.TaskID, generated); err != nil { return "", ... } // SOL-TG-01's prompt_template column
    }
    return generated, nil
}
```

Editable-before-save per spec: `Save` is a caller-controlled bool so the
client can call this once to preview, let the user edit, then call
`UpdateTask{PromptTemplate: edited}` (SOL-TG-01) to persist the final
version — `GenerateAgentPrompt` itself only writes when `Save=true` (the
"generate and accept immediately" path), not on every call.

## Design — proto additions

```protobuf
message SubtaskProposal {
  string title = 1;
  string description = 2;
  string type = 3;
  google.protobuf.DoubleValue estimated_hours = 4;
  repeated int32 depends_on_indices = 5;
  string prompt_template = 6;
}
message AIApplyRequest {
  string task_id = 1;
  repeated SubtaskProposal proposals = 2;
  string raw_ai_response = 3; // echoed back from AIDecomposeResponse.raw_response, persisted to ai_plan_json
}

message AIDecomposeResponse {
  repeated SubtaskProposal proposals = 1;
  string raw_response = 2; // new — the unparsed AI JSON, for ai_plan_json persistence + "show raw on parse failure"
  string notes = 3;
}

message GenerateAgentPromptRequest { string task_id = 1; bool save = 2; }
message GenerateAgentPromptResponse { string prompt = 1; }
```

`raw_response` closes the spec's "AI timeout, invalid JSON response → show
raw + retry" requirement structurally: `AIDecompose`'s usecase returns
`TASK_AI_DECOMPOSE_INVALID_JSON` (a distinguishable error code, not a bare
`KindInternal`) precisely so a caller can show the raw completion text and
offer retry — the raw text itself would need to ride in the error detail
or a best-effort partial response; flagged as an open wire-contract
decision for whoever implements this (either an error-detail field via
`apperrors`, or a non-2xx response that still carries `raw_response`) —
not resolved further here since it's a client-UX contract choice outside
this bug's backend-only scope.

## Test plan

- `domain/critical_path_test.go` — diamond DAG, parallel independent
  chains (longest one wins), single-node graph, all-zero-hours graph
  (degenerates to path length = node count, not a divide-by-zero).
- `usecase/ai_decompose_test.go` — fake `ProjectContextResolver`/
  `TechStackDetector`/`VelocityResolver`: prompt-building test asserts
  every context source's value appears in the built prompt string;
  `TechStackDetector` returning an error doesn't fail `Execute` (best-effort
  assertion); malformed JSON from `AICompleter` surfaces
  `TASK_AI_DECOMPOSE_INVALID_JSON`, not a generic error.
- `usecase/ai_apply_test.go` — new case:
  `TestAIApply_DependsOnIndices_CreatesDependsOnEdgesAfterAllSiblingsExist`
  (assert edge `FromTaskID`/`ToTaskID` resolve to the right created IDs);
  out-of-range index is silently skipped, not a hard failure (AI
  hallucination shouldn't abort an otherwise-valid apply); mid-loop failure
  during the dependency pass still rolls back the whole transaction
  (extends the existing rollback test).
- `usecase/generate_agent_prompt_test.go` — `Save=false` never calls
  `UpdatePromptTemplate`; `Save=true` does, with the generated text.
- `adapter/grpcclient/tech_stack_detector_test.go` — fake
  `GitGatewayServiceClient`: `NOT_FOUND` on `package.json` continues to
  probe `go.mod`; a real read populates the expected `TechStack` fields.

## References

- `docs/logic/task-graph/BL-TG-02-ai-task-planning.md` — full spec
- `specs/backend-go/tdd/services/task-service.md:87-96` (§3.2 AI
  decomposition design, relay-not-inference principle), `:110`
  (`CycleDetector`/DAG framing this reuses for critical path's topological
  sort)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166`
  (dependency graph — flags the new `task --> git-gateway-service` edge
  this solution requires)
- `backend-go/services/task-service/internal/usecase/ai_decompose.go:42-133`,
  `ai_apply.go:1-78`
- `backend-go/services/task-service/internal/domain/subtask_proposal.go:9-12`,
  `task_edge.go:81-113` (`DetectCycle`, the pattern `CalculateCriticalPath`
  follows)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:57-74`
  (`FileIO` RPC group, already landed, reused for tech-stack detection)
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md`
  — the solution that landed `ReadFile`, cited as the mechanism
  `TechStackDetector` calls
- `specs/backend-go/bugs/logic-v1/BUG-TG-01-task-graph-structural-management-partial.md`
  — the missing `description`/`aiContext`/`estimated_hours`/
  `prompt_template`/`ai_plan_json` fields this solution's context bundle
  and proposal fields depend on (see
  [SOL-TG-01](./SOL-TG-01-task-graph-structural-management.md))
