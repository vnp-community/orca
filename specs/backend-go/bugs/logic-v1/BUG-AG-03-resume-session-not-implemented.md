# BUG-AG-03: No agent-session persistence or resume-flag logic exists — "Resume" has nothing to resume

**Business Logic:** [BL-AG-03](../../../../docs/logic/agent-orchestration/BL-AG-03-resume-session.md) — Resume Agent Session
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** There is no "Resume" action a user can take against backend-go: no stored session id to look up, no per-agent resume-flag construction (`--resume <id>` / session-file / `resume <id>`), and no expiry/version-mismatch handling. A worktree with a previously stopped agent has no way to continue that conversation via backend-go today.

---

## Spec summary

Resume loads the most recent session id for a worktree from `orca_sessions` (Server DB), builds agent-specific resume args (Claude Code `--resume <id>`, Codex session-file, OpenCode `resume <id>`), and re-spawns via the same `agent.spawn` JSON-RPC used for a fresh start (BL-AG-01), over the existing Dev Server WS connection. Sessions expire after 7 days of inactivity (BR-AG-08); resume must use the same agent version as the original session (BR-AG-09); an incompatible version prompts "start new session?".

## What backend-go has

Nothing directly implements this flow. The closest adjacent, unrelated pieces:

- `SpawnTerminalSession` (`backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:37-94`) can spawn a fresh shell PTY, but takes no session id / resume flag and has no way to express "continue conversation X" — it only accepts `{ConnectionID, Cwd, Shell, Cols, Rows}` (spawn_terminal_session.go:14-20).
- `workflow-service`'s `Resume`/`ResumeExecution` (`backend-go/services/workflow-service/internal/domain/execution.go:125-131`, `backend-go/services/workflow-service/internal/usecase/resume_execution.go`) is a same-named but semantically unrelated concept — resuming a paused **workflow DAG execution**, not an AI agent PTY/conversation session. Do not conflate the two.

## What's missing

- No `orca_sessions` table (or equivalent) recording `{sessionId, worktreeId, devServerId, startedAt}` for a stopped agent to resume from — confirmed by grepping every `.sql` migration under `backend-go/` for `orca_session`/`agent_session`/`pty_session` (zero hits) and grepping all non-test `.go` files for the word `resume` outside `workflow-service` (zero agent-session hits — see raw grep output; every match is workflow-execution-related).
- No `SELECT sessionId, devServerId FROM orca_sessions WHERE worktreeId=? ORDER BY startedAt DESC` equivalent query anywhere.
- No per-agent resume-arg builder (`--resume <id>` for Claude Code, session-file path for Codex, `resume <id>` for OpenCode).
- No 7-day session expiry check (BR-AG-08) or "session expired, start new?" user-facing error path.
- No agent-version compatibility check (BR-AG-09) or version-mismatch error propagation ([A3]).
- No dependency on BL-AG-01's `agent.spawn` RPC, since that RPC itself does not exist in backend-go either (see BUG-AG-01) — resume would need to build on it once it exists.

## See also

- specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md — the fresh-spawn flow this resume flow is meant to extend; neither the session persistence nor the `agent.spawn` contract exists yet.
- specs/backend-go/bugs/logic-v1/BUG-AG-02-dung-agent-partial.md — stop-agent's missing "session id retained for resume" postcondition (BR-AG-07), the direct upstream dependency of this gap.

## References

- `docs/logic/agent-orchestration/BL-AG-03-resume-session.md`
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:14-94`
- `backend-go/services/workflow-service/internal/domain/execution.go:125-131` — unrelated "resume" concept, cited to rule it out
- `backend-go/services/workflow-service/internal/usecase/resume_execution.go` — unrelated "resume" concept, cited to rule it out
