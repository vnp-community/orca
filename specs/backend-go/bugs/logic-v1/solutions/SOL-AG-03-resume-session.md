# SOL-AG-03: Add `ResumeAgentSession`, and consume the already-real `agent.hook` provider-session notification to capture what to resume

**Resolves:** [BUG-AG-03](../BUG-AG-03-resume-session-not-implemented.md)
**Service:** `infra-fleet-service` (extended) — plus a **small, scoped `agent/` change** (see below; unlike SOL-AG-01/02, this one is not backend-go-only)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new `ResumeAgentSession` RPC)
- `backend-go/services/infra-fleet-service/internal/usecase/resume_agent_session.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/record_agent_hook_provider_session.go` (new — consumes `agent.hook` notifications)
- `backend-go/services/infra-fleet-service/internal/domain/agent_session.go` (extended — `ResumeProviderSessionKey/ID`, `ErrAgentSessionExpired`, `ErrAgentVersionMismatch`)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (extend the notification demux to also route `agent.hook`, alongside the existing `pty.data`/`pty.exit`/`pty.replay` demux)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/agent_session_repository.go` (extended: `UpdateProviderSession`)
- **`agent/src/relay/agent-hook-server.ts` (proposed change, cross-repo — see "Genuine gap")**: thread Orca's `ptyId` into `AgentHookRelayEnvelope` so a hook event can be correlated to the exact `AgentSession` it belongs to.
**Status:** 📋 Proposed — not yet implemented; correlation gap requires a scoped `agent/` change before this closes cleanly (see below)

---

## Design rationale (grounded in TDD)

### Where this belongs

Same reasoning as SOL-AG-01/02: `ResumeAgentSession` is `StartAgentSession`
(SOL-AG-01) called with a non-empty `ResumeID`, following BL-AG-03's own
framing ("Khởi động agent với resume flag" — step 3 reuses `agent.spawn`
verbatim, just with `resumeId` populated). No new service; extends the same
`infra-fleet-service` usecase family.

### The resume-arg-building logic is already real, agent-side — and already internally inconsistent in two places worth flagging, not silently resolving

BUG-AG-03 states "No per-agent resume-arg builder." Two agent-side
mechanisms already build resume args, and they **disagree with each other**
for one agent, and both **disagree with BL-AG-03's own table** in places:

| Agent | `agent-spawner.ts`'s `AGENT_SPECS` (used by `agent.spawn`, the Dev-Server-tier headless path this bug is about) | `agent-session-resume.ts`'s `getAgentResumeArgv` (used by the Desktop-tier "sleeping agent" TUI-resume feature) | BL-AG-03's table |
|---|---|---|---|
| Claude Code | `--resume <id>` | `--resume <id>` | `--resume <id>` — all three agree |
| Codex | `--session-file ~/.codex/<id>.json` | `resume <id>` | "session file" (matches `AGENT_SPECS`, not `getAgentResumeArgv`) |
| OpenCode | `--session <id>` | `--session <id>` | `resume <id>` — **BL doc is wrong**, both real implementations agree with each other and disagree with the doc |

Codex is the one true internal inconsistency (`agent-spawner.ts` and
`agent-session-resume.ts` build genuinely different argv for the same
agent) — this solution calls `agent.spawn`'s path
(`AGENT_SPECS`, since that's what the Dev-Server-tier `StartAgentSession`
actually invokes), but flags the Codex divergence as worth resolving in
`agent/` itself rather than silently picking a side; OpenCode's BL-doc
mismatch is a documentation fix, not a code decision (both real
implementations already agree).

**This confirms BUG-AG-03's "no resume-arg builder" finding is, once
again, accurate for backend-go but not for the system as a whole** — no Go
code needs to build `--resume <id>`; `StartAgentSession`
(SOL-AG-01) already threads `ResumeID` straight into `SpawnAgentInput`,
which the agent turns into the right argv itself.

### Genuine gap: capturing *which* id to resume needs a scoped `agent/` change

BL-AG-03 step 2 assumes Orca Server can `SELECT sessionId ... FROM
orca_sessions WHERE worktreeId=?` and pass that straight back as
`resumeId`. But the id `--resume <id>` actually needs is **not** Orca's own
`AgentSession.ID` (`SpawnAgentInput.TaskID`, embedded in the synthetic
`ptyId`) — it's the underlying CLI's **own** conversation/session id
(Claude Code's internal session UUID, Codex's own session id, etc.), which
Orca never sees unless the CLI reports it back.

That reporting mechanism **already exists**, just not wired to backend-go:

- Claude Code / Codex / Gemini / etc. call back into a local hook HTTP
  server the Dev Server Agent runs (`agent/src/relay/agent-hook-server.ts`),
  posting a `session_id`/`conversationId` field among other hook data.
- `extractAgentProviderSession`/`normalizeAgentProviderSession`
  (`agent/src/shared/agent-session-resume.ts:140-186`) already parse this
  into `AgentProviderSessionMetadata{key, id, transcriptPath?}` for every
  resumable agent kind (`RESUMABLE_TUI_AGENTS`,
  `agent-session-resume.ts:5-16` — covers claude/codex/gemini/opencode and
  more).
- `agent-hook-server.ts`'s `forwardEvent` already relays this to Orca as a
  **real, already-wired JSON-RPC notification**: `agent.hook`
  (`agent-hook-server.ts:8,104,239,361` — the envelope's `providerSession`
  field is populated exactly when a hook event carried one). This is a
  genuinely different notification than `agent.output`/`agent.exited`
  (SOL-AG-01) — `infra-fleet-service`'s `devserveragent` adapter has no
  handler for it today (confirmed: `StreamPty`'s notification demux only
  routes `pty.data`/`pty.exit`/`pty.replay`, per `ports.go:165-172`'s doc
  comment).

**The correlation problem** — and the reason this can't be closed with
Go-only work — is that `AgentHookRelayEnvelope` (`agent-hook-server.ts:341-364`)
identifies the originating pane by `paneKey`/`tabId`/`worktreeId`, a
**Desktop-UI addressing scheme**, and carries **no `ptyId` or Orca
`sessionId` field at all**. `infra-fleet-service`'s server-mode
`AgentSession` is keyed by `pty_id`/`id` (SOL-AG-01), not `paneKey` — there
is no reliable join key between an `agent.hook` notification and the
`AgentSession` row it belongs to today. Two options:

1. **(Recommended) Small, scoped `agent/` change**: thread the spawning
   `ptyId` (already known to `agent-spawner.ts` at spawn time, and unique
   per session) into `AgentHookRelayEnvelope`, sourced from whatever
   context the hook script's launching pane already carries. This is a
   materially smaller ask than SOL-AG-01's Vault Transit gap — one new
   optional envelope field, populated from data the spawning code already
   has — but it is still a cross-repo change outside "backend/ only," so
   flagged explicitly rather than assumed away.
2. **Interim, lower-confidence fallback (backend-go only, no `agent/`
   change)**: correlate by `(tenant_id, worktree_id)` to the most recent
   `AgentSession` in `spawning`/`running`/`idle`/`waiting` status — correct
   in the common case (BR-AG-01 already guarantees at most one non-terminal
   session per worktree+user), wrong if a hook event arrives for a session
   that has *just* transitioned out of that set (a narrow race, not a
   structural failure). This solution's code sketch below implements
   option 2 so `ResumeAgentSession` has *something* to build on now, while
   flagging option 1 as the correct fix to schedule.

## Design — proto

```protobuf
service InfraFleetService {
  // ... existing + SOL-AG-01/02's RPCs ...

  // ResumeAgentSession — BL-AG-03. Loads the latest AgentSession for
  // worktree_id, validates BR-AG-08 (7-day expiry) and BR-AG-09 (agent
  // version match), then re-spawns via the same path as StartAgentSession
  // with resume_id populated.
  rpc ResumeAgentSession(ResumeAgentSessionRequest) returns (AgentSession);
}

message ResumeAgentSessionRequest {
  string connection_id = 1;
  string worktree_id   = 2;
  string user_id       = 3;
  string cwd            = 4;
  int32  cols, rows      = 5, 6;
}
```

## Design — `usecase/resume_agent_session.go`

```go
const sessionExpiry = 7 * 24 * time.Hour // BR-AG-08

type ResumeAgentSession struct {
	sessions AgentSessionRepository
	start    *StartAgentSession // reused, not duplicated — see below
	clock    func() time.Time
}

func (uc *ResumeAgentSession) Execute(ctx context.Context, in ResumeAgentSessionInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	found, prior, err := uc.sessions.LatestForWorktree(ctx, tenantID, in.WorktreeID)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_RESUME_LOOKUP_FAILED", "failed to load prior agent session", err)
	}
	if !found {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND", "no prior session for this worktree", nil)
	}
	// [A1] BR-AG-08: 7-day inactivity expiry, measured from LastActiveAt
	// (not StartedAt) — a long-lived-but-recently-touched session should
	// not expire just because it started a while ago.
	if uc.clock().Sub(prior.LastActiveAt) > sessionExpiry {
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_SESSION_EXPIRED", "session expired — start a new session", domain.ErrAgentSessionExpired)
	}
	if prior.ResumeProviderSessionID == "" {
		// The agent.hook capture path (see this solution's "Genuine gap")
		// never reported a provider session id for this run — most likely
		// because it was killed before the CLI reported one, or the
		// correlation fallback missed it. Honest failure, not a silent
		// fresh-start substitution — BL-AG-03 makes resume-vs-fresh-start an
		// explicit user decision ([A3]'s "Bắt đầu session mới?" prompt), not
		// something this usecase should decide unilaterally.
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_NO_RESUMABLE_SESSION", "no resumable provider session id was captured for the prior run", nil)
	}

	// [A3] BR-AG-09: resume must use the same agent version as the
	// original session. dev_servers.agent_version (infra-fleet-service.md
	// §5) is this service's own record of the CURRENT connection's agent
	// build; compared against what was stored at spawn time.
	connected, devServer, _, err := uc.resolver().ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err == nil && connected && prior.AgentVersion != "" && devServer.AgentVersion != prior.AgentVersion {
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_VERSION_MISMATCH", "agent version differs from the original session — start a new session?", domain.ErrAgentVersionMismatch)
	}

	return uc.start.Execute(ctx, StartAgentSessionInput{
		ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID,
		Cwd: in.Cwd, ModelID: prior.ModelID, AccountID: prior.AccountID, Cols: in.Cols, Rows: in.Rows,
		ResumeID: prior.ResumeProviderSessionID, // the CLI's own id, not AgentSession.ID — see rationale
	})
}
```

`StartAgentSession.Execute` (SOL-AG-01) already accepts `ResumeID` in
`SpawnAgentInput` unchanged — `ResumeAgentSession` is a thin composition,
not a fork of the spawn logic, matching this codebase's existing preference
for small, single-purpose usecase types over branching mega-usecases.

## Design — `record_agent_hook_provider_session.go` (the `agent.hook` consumer)

```go
// RecordAgentHookProviderSession runs once per agent.hook notification
// carrying a providerSession field — subscribed alongside StreamPty's
// existing pty.data/pty.exit demux (client.go), not a new connection.
type RecordAgentHookProviderSession struct {
	sessions AgentSessionRepository
}

func (uc *RecordAgentHookProviderSession) Handle(ctx context.Context, tenantID string, hook AgentHookEvent) error {
	if hook.ProviderSession.ID == "" {
		return nil // most agent.hook events carry no providerSession — not every hook fires it
	}
	// Correlation fallback (option 2 in this solution's "Genuine gap"
	// section) — see that section for why this is best-effort pending an
	// agent/ change.
	found, session, err := uc.sessions.MostRecentActiveForWorktree(ctx, tenantID, hook.WorktreeID)
	if err != nil || !found {
		return err // nothing to attach this to yet — not an error worth failing the notification pump over
	}
	return uc.sessions.UpdateProviderSession(ctx, tenantID, session.ID, hook.ProviderSession.Key, hook.ProviderSession.ID)
}
```

## Design — schema (extends SOL-AG-01's `agent_sessions`)

```sql
ALTER TABLE agent_sessions
  ADD COLUMN resume_provider_session_key TEXT,  -- "session_id" | "conversation_id"
  ADD COLUMN resume_provider_session_id  TEXT;  -- the CLI's OWN id — distinct from agent_sessions.id
```

## Test plan

- `usecase/resume_agent_session_test.go`:
  - no prior session → `INFRA_AGENT_SESSION_NOT_FOUND`.
  - `LastActiveAt` > 7 days ago → `INFRA_AGENT_SESSION_EXPIRED`, `start.Execute` never called.
  - no `ResumeProviderSessionID` captured → `INFRA_AGENT_NO_RESUMABLE_SESSION`, distinct code from expiry.
  - `dev_servers.agent_version` mismatch → `INFRA_AGENT_VERSION_MISMATCH`.
  - happy path → asserts `start.Execute` called with `ResumeID = prior.ResumeProviderSessionID`, **not** `prior.ID` (regression guard against the id-confusion this solution's rationale calls out).
- `usecase/record_agent_hook_provider_session_test.go`: hook with no
  `providerSession` → no-op; hook with one, no active session for the
  worktree → no-op, no error; happy path → `UpdateProviderSession` called
  with the right key/id.
- Cross-repo integration test (once the `agent/` correlation field lands):
  spawn → CLI-side hook fires with a `session_id` → `agent.hook` arrives
  with `ptyId` → `AgentSession.ResumeProviderSessionID` populated → resume
  succeeds with the exact id.

## References

- `specs/backend-go/bugs/logic-v1/BUG-AG-03-resume-session-not-implemented.md`
- `docs/logic/agent-orchestration/BL-AG-03-resume-session.md` — BR-AG-08/09/10, resume-arg table, `[A1]`/`[A3]`
- `specs/backend-go/tdd/services/infra-fleet-service.md:190-206` (`dev_servers.agent_version` column, reused for BR-AG-09)
- `agent/src/relay/agent-spawner.ts:130-195` (`AGENT_SPECS`/`buildArgs`, the Dev-Server-tier resume path actually invoked)
- `agent/src/shared/agent-session-resume.ts:5-16,140-230` (`RESUMABLE_TUI_AGENTS`, `extractAgentProviderSession`, `getAgentResumeArgv` — the Desktop-tier resume path this solution flags as internally inconsistent with `AGENT_SPECS` for Codex)
- `agent/src/relay/agent-hook-server.ts:8,104,239,341-367` (`agent.hook` notification, `AgentHookRelayEnvelope`, the missing `ptyId` correlation field)
- `specs/backend-go/bugs/logic-v1/solutions/SOL-AG-01-khoi-dong-agent.md` — `StartAgentSession`/`SpawnAgentInput.ResumeID`, `AgentSessionRepository` this solution extends
