# BUG-TG-02: AI decompose/apply is a real round-trip but a minimal one — no rich context, no dependencies/estimates/prompts per subtask, no critical path

**Business Logic:** [BL-TG-02](../../../../docs/logic/task-graph/BL-TG-02-ai-task-planning.md) — AI-Assisted Task Planning & Decomposition
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A Developer clicking "AI: Plan this task" gets a real AI call and a real, transactional commit of accepted subtasks — this is not a CRUD-only stub. But the AI only ever sees the task's bare title (no description, no project/tech-stack context, no velocity data, no existing-subtask dedup), the proposals it gets back carry only a title (no type/estimate/dependency/prompt_template), the "dependency edges" in the spec's decomposition output are never produced (AIApply always creates `parent_child` edges only), and there is no critical-path calculation or tech-stack detection anywhere.

---

## Spec summary

BL-TG-02 wants two AI flows: (1) **AI Decompose** — collect task+project+tech-stack+velocity+existing-subtask context, ask an AI (preferably Claude Opus) for a structured JSON breakdown (`{subtasks: [{title, type, estimated_hours, depends_on, prompt_template}], dependencies, notes}`), show the user an editable plan modal, then commit accepted subtasks + `depends_on` edges on "Accept Selected"; and (2) **AI Generate Agent Prompt** — a separate flow producing one ready-to-use agent prompt for a single task. It also specifies a pure `calculateCriticalPath()` DAG algorithm over estimated hours, and tech-stack detection by reading `package.json`/`go.mod`/etc. from the dev server.

## What backend-go has

- **A real, working two-step decompose→apply pipeline** (TASK-224), not a stub:
  - `AIDecompose` usecase (`backend-go/services/task-service/internal/usecase/ai_decompose.go:42-73`) resolves an AI provider via `AIProviderContextResolver` (real gRPC call to ai-provider-service, `internal/adapter/grpcclient/ai_provider_resolver.go:26-42`), resolves the task's project connection via `ProjectExecutionResolver` (real gRPC to infra-fleet-service), builds a prompt, and relays it through `AICompleter.Complete` to the Dev Server Agent's `ai.complete` — a genuine external AI call, matching the spec's "review-before-commit" shape (`AIDecomposeResponse.proposals` is never written to `task_edges` until `AIApply`, see `task.proto:102-111`).
  - `AIApply` usecase (`internal/usecase/ai_apply.go:53-78`) commits proposals inside **one Postgres transaction** (`TxRunner.RunInTx`, `internal/adapter/postgres/repository.go:62-67`) — creating one subtask (`CreateTask`) + one `parent_child` edge (`AddEdge`) per proposal, with a real rollback-on-mid-loop-failure guarantee (per the file's own doc comment and `ai_apply_test.go`'s `TestAIApply_MidLoopFailure_RollsBackEntireSubtree`).
  - Both RPCs are fully wired: proto (`task.proto:76-111`), gRPC server (`internal/adapter/grpc/server.go:205-226`), composition root (`cmd/server/main.go:118-127`).

## What's missing

- **Context collection is a single field, not the spec's five-source context bundle.** `buildDecomposePrompt` (`internal/usecase/ai_decompose.go:79-90`) interpolates only `task.Title` plus a `providerCtx` traceability string — no `task.description`, no `task.aiContext`, no project name/repo, no tech stack, no velocity data (recent completed tasks), and no check of existing subtasks to avoid duplicates. All five of the spec's "Collect context" bullets are unimplemented; the `domain.Task` struct doesn't even have a `Description`/`AIContext` field to source them from (see BUG-TG-01).
- **No tech-stack detection.** The spec's `collectProjectContext()` (reading `package.json`/`go.mod`/`pom.xml`/`pyproject.toml`/`src` layout via the dev server relay) has no equivalent anywhere in `backend-go/services/task-service/`.
- **Proposals carry only a title, never type/estimate/dependency/prompt_template.** `domain.SubtaskProposal` (`internal/domain/subtask_proposal.go:9-12`) is `{Title, Description}` only (and `Description` is never populated — `parseSubtaskProposals`, `ai_decompose.go:98-118`, only ever sets `Title`, parsing a plain `"<n>. <title>"` numbered-list response, not the spec's structured JSON with `type`/`estimated_hours`/`depends_on`/`prompt_template` per subtask).
- **No dependency edges from AI proposals.** `AIApply` (`internal/usecase/ai_apply.go:67`) always adds `domain.EdgeKindParentChild` only — the spec's "INSERT dependency edges → orca_task_edges" step (subtasks depending on each other, e.g. "Task 2 depends on Task 1") has no code path at all; there is no `depends_on` field on `SubtaskProposal` to source it from.
- **No "AI Generate Agent Prompt" flow.** The spec's second flow (generate one ready-to-use agent prompt for a single task, editable, saved to `task.promptTemplate`) has no backend counterpart — there's no `prompt_template` field on `domain.Task` to save it to (see BUG-TG-01), and no RPC resembling it.
- **No critical-path calculation.** Zero hits for `CriticalPath`/`critical_path` anywhere in `backend-go/services/task-service/` (confirmed via grep) — the spec's `calculateCriticalPath()` DAG/topological-sort algorithm over `estimatedHours` has no equivalent, and `estimated_hours` doesn't exist on the domain model to compute it from in the first place.
- **No "raw AI response" persistence.** The spec's `task.aiPlanJson` field (storing the raw AI response for later reference) doesn't exist on `domain.Task`.
- **No structured-JSON error handling.** The spec calls for "AI timeout, invalid JSON response → show raw + retry"; the actual parser (`parseSubtaskProposals`) never expects JSON at all (deliberately, per its own doc comment — "no live agent in this environment to confirm ai.complete's response format... this parses the plain numbered-list format"), so there's no invalid-JSON case to even detect.

## See also

- None — `specs/backend-go/bugs/missing-v1/BUG-034-task-channels-not-implemented.md` covers the WS-channel wiring gap for `task.aiDecompose`/`task.aiApply` but is stale on RPC existence: both RPCs are real and implemented server-side today (`ai_decompose.go`, `ai_apply.go`), just not yet wired into `wscompat`'s `channels.go`.

## References

- `docs/logic/task-graph/BL-TG-02-ai-task-planning.md` — full spec (decompose flow, prompt-generation flow, critical path, tech-stack detection)
- `backend-go/services/task-service/internal/usecase/ai_decompose.go:42-133` — `AIDecompose.Execute`, `buildDecomposePrompt`, `parseSubtaskProposals`
- `backend-go/services/task-service/internal/usecase/ai_apply.go:53-78` — `AIApply.Execute` (transactional commit, `parent_child`-only edges)
- `backend-go/services/task-service/internal/domain/subtask_proposal.go:9-12` — `SubtaskProposal{Title, Description}`
- `backend-go/proto/orca/task/v1/task.proto:76-111` — `AIDecompose`/`AIApply` RPC + message definitions
- `backend-go/services/task-service/internal/adapter/grpcclient/ai_provider_resolver.go:26-42` — real `ResolveProvider` call
- `backend-go/services/task-service/README.md:242-245` — README's own (stale) "Known gaps" note claiming AIDecompose/AIApply "aren't implemented" — contradicted by the current code, which does implement both, just with the reduced scope this bug documents
- `specs/backend-go/bugs/logic-v1/BUG-TG-01-task-graph-structural-management-partial.md` — documents the missing `description`/`aiContext`/`estimated_hours`/`prompt_template`/`ai_plan_json` fields this bug's gaps depend on
