# BUG-PRF-04: Profile-aware agent execution routing does not exist — agent spawn is a bare relay passthrough with no profile/env injection

**Business Logic:** [BL-PRF-04](../../../../docs/logic/profile/BL-PRF-04-profile-aware-agent-execution.md) — Profile-Aware Agent Execution Routing
**Priority (per spec):** P0
**Status:** NOT_IMPLEMENTED
**Severity:** Critical
**Symptom:** When a workflow step spawns an agent through backend-go, the model and trust preset used come only from the workflow step's own static config (`AgentStepConfig.TrustPreset`) — never from the user's resolved profile (`tenant-service.GetResolvedProfile`). Nothing injects `shell.envVars`/`shell.pathAdditions` into the spawned process's environment, nothing sets `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` for per-user credential isolation, nothing sets `ORCA_PROJECT_ID`/`ORCA_PROJECT_NAME`/`ANTHROPIC_MODEL`, and no project-context preamble is built or passed to the agent. A user with a customized profile (preferred model, PATH additions, env vars) sees none of it take effect when an agent runs.

---

## Spec summary

Starting an agent for a project should: (1) load the project and resolve its bound dev server; (2) check server health; (3) resolve the user's effective profile via `ProfileResolver`; (4) create the worktree on the dev server; (5) build an agent environment merging `resolved.shell.envVars`, extending `PATH` with `resolved.shell.pathAdditions`, setting per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`, `ANTHROPIC_MODEL` from `profile.agent.preferredModel`, and `ORCA_PROJECT_ID`/`ORCA_PROJECT_NAME`; (6) build and inject a project-context preamble into the agent's initial prompt; (7) spawn the agent binary resolved from the model, with CLI args resolved from `profile.agent.trustPreset`; (8) stream PTY output back, with defined handling for degraded/unreachable dev servers and relay reconnects.

## What backend-go has

- A relay-dispatch path exists that reaches the Dev Server Agent for an agent step: `AgentExecutor.Execute` (`backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go:51-67`) unmarshals `domain.AgentStepConfig` and relays `{prompt, worktreePath, trustPreset}` to `agent.exec`/`agent.execPrompt` via `infra-fleet-service`.
- `task-service`'s `SimpleExecutor` (`backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`) follows the same shape for task-service's own ad-hoc agent-exec path, also passing `trustPreset`/`model` straight from step/task config (not a resolved profile) — see its own extensive doc comment (lines 15-80) acknowledging the param-shape is copied from the TS reference and not independently profile-aware.
- Worktree creation on a dev server (spec step 4) is handled elsewhere and does exist for real: `git-gateway-service`'s `CreateWorktree` usecase (`backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`) — out of scope for this bug's gap, cited only to show step 4 alone isn't the missing piece.
- Some relay-connection-level reconnect/backoff logic exists in `infra-fleet-service`'s Dev Server Agent adapter (`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`, `client.go`) — this is generic WebSocket-transport resilience, not specific to an agent-execution session's PTY-buffering-during-reconnect behavior the spec describes, but is worth noting as partial infrastructure toward spec step 8's reconnect requirement.

## What's missing

- **No call to `tenant-service.GetResolvedProfile` anywhere in the agent-execution path.** `grep -rln "GetResolvedProfile\|tenantv1\." backend-go/` outside `tenant-service`/`api-gateway` shows only `task-service`'s `team_scope_resolver.go` (a still-stubbed team-membership resolver, unrelated to profile settings) — no workflow-service or task-service code resolves a user's profile before spawning an agent.
- **No environment injection at all.** `AgentStepConfig`/`agentExecParams` (`agent_step_executor.go:30-34`) carry only `Prompt`, `WorktreePath`, `TrustPreset` — no `env`/`envVars` field, so `resolved.shell.envVars` and `resolved.shell.pathAdditions` have no path into the spawned process.
- **No `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user isolation.** `grep -rln "GH_CONFIG_DIR\|GLAB_CONFIG_DIR" backend-go/` returns zero matches anywhere in the Go services.
- **No `ANTHROPIC_MODEL`/`ORCA_PROJECT_ID`/`ORCA_PROJECT_NAME` env vars.** Zero matches for any of these three names across `backend-go/`.
- **No agent-binary resolution from `profile.agent.preferredModel`.** No `AGENT_MAP`/`resolveAgentBinary`-equivalent function exists; the `trustPreset`-to-CLI-args mapping (spec's `buildAgentArgs`/`TRUST_ARGS`) also has no backend-go equivalent — `trustPreset` is passed through as a raw string to the relay call with no server-side interpretation.
- **No project-context preamble construction.** `grep -rln "ProjectContext\|buildProjectContext\|systemPreamble"` over `backend-go/` (excluding docs) returns zero code hits — nothing assembles the "You are working on project: ..." preamble the spec's `buildProjectContext` describes, nor injects it via `initFile`/stdin.
- **No dev-server-health gate before spawn** (spec's "Check Server Availability" step, with modal/retry on unreachable and warn-and-continue on degraded) tied into the agent-execution path specifically — the closest infra (`infra-fleet-service`'s device status) is not called from `AgentExecutor`/`SimpleExecutor`.
- The relay method name itself is flagged unverified: `agent_step_executor.go:14-25`'s own doc comment says the `"agent.exec"` vs `"agent.execPrompt"` reconciliation against the real Dev Server Agent handler was not confirmed, i.e. even the bare passthrough this bug credits as "existing" isn't proven to work end-to-end yet.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-PRF-02-profile-inheritance-approvedmodels-servertags-missing.md` — even if this routing existed, `agent.preferredModel`'s approved-list fallback isn't computed by the resolver it would need to call.
- `specs/backend-go/bugs/missing-v1/BUG-036-git-relay-methods-unreachable-on-agent.md` — related class of "relay method name/reachability not verified against the live Dev Server Agent" concern.

## References

- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go:1-67` — the entire agent-execution relay path, and its own doc-comment caveats
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go:1-80` — task-service's parallel ad-hoc agent-exec path, same gap
- `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go` — the only non-tenant-service/api-gateway caller of tenant-service concepts near this path, and it's a stub unrelated to profile settings
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`, `client.go` — generic transport-level reconnect (not agent-session-specific)
- `docs/logic/profile/BL-PRF-04-profile-aware-agent-execution.md:19-168` — full 8-step flow, `resolveAgentBinary`/`buildAgentArgs`, `buildProjectContext`, server-unavailability handling
