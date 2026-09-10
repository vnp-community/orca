# task-graph tasks — index

Executable task breakdown of the `task-graph` (F37) CR series' solutions
([`../solutions/`](../solutions/)). Every task below is `Status: [ ] TODO`
— none of this is implemented yet. Each task cites real, current
`backend-go` file:line locations (re-verified against the on-disk source
while writing these tasks, not copied blind from the solutions — several
solution-sketch citations turned out stale or the sketched code shape
didn't match the real ports, flagged inline in each affected task's
Context section) so an implementing agent can work from the task file
alone without re-reading the parent solution end to end.

## Solution → Task ID map

| Solution | Task IDs | Notes |
|---|---|---|
| [BE-SOL-001](../solutions/BE-SOL-001-orcatask-data-model-widening.md) — `Task` widening, `GetSubtree`, `RecalculateProgress`, auto-block, atomic `AddEdge` | `TASK-TG-001-01` (migration), `TASK-TG-001-02` (`domain.Task`/`Status` widening), `TASK-TG-001-03` (`RecalculateProgress`), `TASK-TG-001-04` (auto-block + atomic `AddEdge`), `TASK-TG-001-05` (`GetSubtree`), `TASK-TG-001-06` (**new, not a BE-SOL-001 task** — `api-gateway`'s `task.create` channel drops `projectId`, a standalone bug discovered while implementing the frontend's New Task dialog) | `TASK-TG-001-04` **corrects** BE-SOL-001's `ports.Tx`/`*Tx`-suffixed-method sketch — a working `TxRunner` port already exists (`ports.go:167-184`, already used by the real, already-transactional `AIApply`) and this task reuses it instead of inventing new port surface. `TASK-TG-001-02` flags that widening `Task.Status` to a named type ripples into `ports.go`/`repository.go`/`server.go`, not just `domain/task.go`. `TASK-TG-001-06` is P0 — it silently breaks every task created through the UI once the frontend's New Task dialog ships. |
| [BE-SOL-002](../solutions/BE-SOL-002-ai-decompose-context-and-dependency-edges.md) — 5-source context bundle, structured JSON proposals, dependency edges, critical path | `TASK-TG-002-01` (context bundle + `TechStackDetector`), `TASK-TG-002-02` (structured proposal + `AIApply` dependency edges), `TASK-TG-002-03` (`CalculateCriticalPath` + `GenerateAgentPrompt`) | `TASK-TG-002-02` **corrects** BE-SOL-002's assumption that `AIApply` needs to be written from scratch with `CreateTx`/`InsertEdgeTx` — it's already real and already transactional (`ai_apply.go:53-73`); this task only adds `depends_on` edges to the existing loop, reusing TASK-TG-001-04's corrected `AddEdge`. |
| [BE-SOL-003](../solutions/BE-SOL-003-task-access-control-team-scope-and-sharing.md) — real `TeamScopeResolver`, owner grant, expiry, Revoke/List grants, share-link, `action` wire field | `TASK-TG-003-01` (real `TeamScopeResolver`), `TASK-TG-003-02` (owner grant on create), `TASK-TG-003-03` (grant expiry), `TASK-TG-003-04` (Revoke/List grants), `TASK-TG-003-05` (public share-link — **security review required**), `TASK-TG-003-06` (`action` wire field — smallest task in the series) | `TASK-TG-003-01` resolves BE-SOL-003's own "verify before implementing" flag: the real `tenant-service` RPC is **`ListTeamsForUser`**, not `ListUserTeams` as BE-SOL-003 guessed, and its request carries only `user_id` (no `tenant_id` field) — confirmed by direct read of `tenant.proto:221-231`. `TASK-TG-003-05` corrects the `Task.share_token` proto field number (BE-SOL-003 guessed `20`; the real next-free number depends on how many fields TASK-TG-001-02 actually lands, expected `27`). |
| [BE-SOL-004](../solutions/BE-SOL-004-orchestration-service-coordinator-run-lifecycle.md) — `CoordinatorRun` lifecycle RPCs + advance loop | `TASK-TG-004-01` (`CoordinatorRunRepository` port + adapter), `TASK-TG-004-02` (5 lifecycle RPCs), `TASK-TG-004-03` (`AdvancePendingRuns` tick loop + `FOR UPDATE SKIP LOCKED` — **flag for a short design review before merge**, per the solution's own risk note) | `TASK-TG-004-01` flags a real gap in BE-SOL-004's "no new migration" claim: `RecordHeartbeat` needs a `heartbeat_at` column that doesn't exist on `orchestration.coordinator_runs` today, so a small additive migration (`0004_coordinator_run_heartbeat`) is needed after all for that one RPC. |
| [BE-SOL-005](../solutions/BE-SOL-005-task-agent-execution-permission-and-complex-executor.md) — permission precheck, status revert, real `ComplexExecutor`, env injection, richer prompt | `TASK-TG-005-01` (permission precheck + status revert), `TASK-TG-005-02` (real `ComplexExecutor` + `ReportTaskExecutionResult` callback), `TASK-TG-005-03` (env injection + richer `buildExecutePrompt`) | `TASK-TG-005-02` flags that `ReportTaskExecutionResult` doesn't exist in either this series or [`docs/crs/v3/flow-task/`](../../../../../../docs/crs/v3/flow-task/)'s `TASK-FT-002-04` yet — whichever lands first should add it with an `engine` field, per that CR's own reuse note; check before adding a second, colliding definition. `TASK-TG-005-03` confirms via [SOL-AG-TG-001](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-001-agent-env-contract-assessment.md) that zero `agent/`-side change is needed for the env-injection half. |
| [BE-SOL-006](../solutions/BE-SOL-006-task-execute-streaming-relay.md) — `AttachAgentSpawn` streaming RPC (backend-go portion) | `TASK-TG-006-01` | Confirms BE-SOL-006's "verify before implementing" flag: the relay-connection registry `AttachPty` subscribes to lives in `internal/adapter/devserveragent/client.go` (`sessions map[string]*session`, a `RouteNotification`-style method) — read that file in full before coding, the exact subscribe-by-`spawn_id` API still needs direct confirmation against its real method signatures. |

## Dependency order

```
BE-SOL-001 (task-service data model)
  TASK-TG-001-01 (migration: 0004_task_fields_and_comments)
        │  ⚠ shared migration file — TASK-TG-003-03/-05 append to this
        │    same file later, coordinate rather than creating 0005/0006
        ▼
  TASK-TG-001-02 (domain.Task widening + Status type)
        │
        ├──────────────┬──────────────────────┐
        ▼              ▼                      ▼
  TASK-TG-001-03   TASK-TG-001-04         TASK-TG-001-05
  (RecalculateProgress) (auto-block +     (GetSubtree)
                     atomic AddEdge —
                     corrected TxRunner
                     shape)
        │              │                      │
        └──────────────┴──────────────────────┘
                        ▼
BE-SOL-002 (AI decompose — needs BE-SOL-001's fields + AddEdge)
  TASK-TG-002-01 (5-source context bundle + TechStackDetector)
        │  needs TASK-TG-001-02 (Description/AIContext fields)
        ▼
  TASK-TG-002-02 (structured JSON proposal + AIApply dependency edges)
        │  needs TASK-TG-001-04 (AddEdge's corrected 2-arg constructor)
        ▼
  TASK-TG-002-03 (CalculateCriticalPath + GenerateAgentPrompt)

BE-SOL-003 (task-service access control — independent of BE-SOL-002)
  TASK-TG-003-06 (action wire field — smallest task, no deps, land first)
  TASK-TG-003-01 (real TeamScopeResolver — no deps)
  TASK-TG-003-02 (owner grant on create — needs TASK-TG-001-02 landed for
                  the shared GrantRepository wiring, though no new Task field)
        │
        ▼
  TASK-TG-003-03 (grant expiry — shares migration file with TASK-TG-001-01)
        ▼
  TASK-TG-003-04 (Revoke/ListGrants — needs -003's expires_at field first,
                  to avoid a second wire-message revision)
  TASK-TG-003-05 (public share-link — shares migration file, needs
                  TASK-TG-001-02's final field count for share_token's
                  proto number; SECURITY REVIEW REQUIRED before merge)

BE-SOL-004 (orchestration-service coordinator — independent, no data-model dep)
  TASK-TG-004-01 (CoordinatorRunRepository port + adapter; adds a small
                  heartbeat_at migration BE-SOL-004 didn't anticipate)
        ▼
  TASK-TG-004-02 (5 lifecycle RPCs)
        ▼
  TASK-TG-004-03 (AdvancePendingRuns tick loop + FOR UPDATE SKIP LOCKED —
                  design review flagged; dispatch step deferred to BE-SOL-005/006)

BE-SOL-005 (needs BE-SOL-001 + BE-SOL-003 + BE-SOL-004)
  TASK-TG-005-01 (permission precheck + status revert)
        │  needs TASK-TG-003-01/-06 (ResolvePermission's TeamScopeResolver +
        │    action field) to be MEANINGFUL, not just present
        │  needs TASK-TG-001-02 (domain.StatusBlocked)
        ▼
  TASK-TG-005-02 (real ComplexExecutor + ReportTaskExecutionResult)
        │  needs TASK-TG-004-02 (StartCoordinatorRun must exist)
        ▼
  TASK-TG-005-03 (env injection + richer buildExecutePrompt)
        │  needs TASK-TG-001-02/TASK-TG-002-01 (Description/AIContext/
        │    PromptTemplate fields)

BE-SOL-006 (needs a real dispatch path from BE-SOL-004/005)
  TASK-TG-006-01 (AttachAgentSpawn streaming RPC, infra-fleet-service)
        │  needs TASK-TG-004-03/TASK-TG-005-02 (a real dispatch call to
        │    stream from — this RPC has no callers until then)
```

`BE-SOL-001 → BE-SOL-002` and `BE-SOL-001 → BE-SOL-003` are the only
strict solution-level dependencies for the first three solutions (matching
[`../solutions/README.md`](../solutions/README.md)'s own dependency
note); `BE-SOL-004` is independent of all three. `BE-SOL-005` needs
`BE-SOL-001` (prompt-building fields), `BE-SOL-003` (permission precheck),
and `BE-SOL-004` (`StartCoordinatorRun`) all landed. `BE-SOL-006` needs a
real dispatch path from `BE-SOL-004`/`BE-SOL-005` to have anything to
stream.

## Grounding corrections found while writing these tasks

Re-verifying each solution's file:line citations against the live
`backend-go` source (rather than copying them blind) surfaced several real
divergences, each flagged in its task's own Context section:

1. **`TASK-TG-001-04`/`TASK-TG-002-02`** — BE-SOL-001's `AddEdge` sketch and
   BE-SOL-002's `AIApply` sketch both invent a `ports.Tx` /
   `*Tx`-suffixed-method transaction shape that doesn't exist. The real,
   already-working `usecase.TxRunner` port (`ports.go:167-184`) hands its
   `fn` a plain `TaskRepository`/`EdgeRepository` pair instead, and
   `AIApply` (`ai_apply.go`) is **already fully implemented** against it —
   not a stub, contrary to what BE-SOL-002's design section implies.
2. **`TASK-TG-003-01`** — BE-SOL-003 explicitly flagged its assumed
   `tenant-service` RPC name (`ListUserTeams`) as unverified. It's wrong:
   the real RPC is `ListTeamsForUser`, and its request has no `tenant_id`
   field at all (confirmed, `tenant.proto:221-231`).
3. **`TASK-TG-001-01`** — BE-SOL-001's own migration sketch includes a line
   (`CREATE TABLE task.task_comments_index();`) that is prose-as-SQL, not
   real SQL to run; flagged explicitly so it isn't copied verbatim into the
   actual migration file.
4. **`TASK-TG-004-01`** — BE-SOL-004's headline "no new migration" claim
   holds for 4 of its 5 RPCs, but `RecordHeartbeat` needs a `heartbeat_at`
   column that doesn't exist on `orchestration.coordinator_runs` today —
   a small additive migration is needed after all.
5. **Proto field numbers** — several solutions' code sketches hardcode
   field numbers (e.g. BE-SOL-003's `share_token = 20`) that don't match
   the real, current message's field count once BE-SOL-001's ~18 new
   `Task` fields are counted. Each affected task instructs re-counting the
   live `.proto` file at implementation time rather than trusting the
   solution's guess.
6. **`task_grant.rego` line citation** — BE-SOL-003 cites
   `level_actions` at lines 16-24; the real map is at lines 25-31 (a
   16-line file-header comment precedes it). Corrected in
   `TASK-TG-003-02`/`TASK-TG-003-06`.
