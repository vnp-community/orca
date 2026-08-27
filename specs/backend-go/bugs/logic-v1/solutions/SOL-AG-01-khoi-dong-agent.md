# SOL-AG-01: Add an agent-session spawn path to `infra-fleet-service`, reusing the Dev Server Agent's already-real `agent.spawn` RPC

**Resolves:** [BUG-AG-01](../BUG-AG-01-khoi-dong-agent-partial.md)
**Service:** `infra-fleet-service` (extended) + `ai-provider-service` (caller only, no code change) + `api-gateway`
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new `AgentSession` message + `StartAgentSession` RPC)
- `backend-go/services/infra-fleet-service/internal/domain/agent_session.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend `DevServerAgentClient`; add `AgentSessionRepository`, `AIProviderResolverClient`)
- `backend-go/services/infra-fleet-service/internal/usecase/start_agent_session.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/agent_methods.go` (new — `SpawnAgent`/typed wrapper over the real `agent.spawn`)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/agent_session_repository.go` (new) + migration `NNNN_agent_sessions.up/down.sql`
- `backend-go/services/infra-fleet-service/internal/adapter/grpcclient/aiprovider_client.go` (new — mirrors `git-gateway-service`'s existing client of the same name)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (wire `StartAgentSession`)
- `backend-go/services/infra-fleet-service/cmd/server/main.go` (dial `ai-provider-service`, construct the new usecase)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go` (new — `agent.start`)
**Status:** 📋 Proposed — not yet implemented (credential injection **blocked**, see "Genuine architecture gap" below)

---

## Design rationale (grounded in TDD)

### Where this belongs: extend `infra-fleet-service`, not a new service

`infra-fleet-service.md` already owns exactly the machinery BL-AG-01 needs:
resolve `connectionId` → `DevServer`, dispatch to the Dev Server Agent over
the resolved transport, persist a session row keyed by `pty_id`
(`infra-fleet-service.md:117-137` §3's `SpawnTerminalSession`/`ResolveConnection`
RPCs; `:293-313` §6's `adapter/devserveragent/` as "this service's defining
adapter"). `usecase.SpawnTerminalSession`
(`backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:48-94`)
is the exact resolve→spawn→persist shape `StartAgentSession` needs — same
`ConnectionResolver`, same `DevServerAgentClient`, same
`TerminalSessionRepository`-shaped persistence step. Standing up an 18th
service would duplicate all three ports for a capability that is, at the
transport level, "spawn a PTY with a different binary and richer
bookkeeping" — the same reasoning `SOL-009` used to fold `files.*` into
`git-gateway-service` rather than mint a new service
(`02-microservices-decomposition.md:33-36` design principle 4, reversed).

**Genuine extension flagged:** `infra-fleet-service.md` §1-2 scopes this
service to "reach a dev server and know which one owns a given piece of
work" — coordination/routing only, explicitly not business rules. BR-AG-01
(one agent per worktree per user), BR-AG-03/04 (trust-preset-before-spawn,
30s startup timeout), and the resume/status/rate-limit rules the sibling
bugs need (BUG-AG-02..05) are real domain logic layered on top of PTY
routing — richer than what §1's charter contemplates. This solution treats
"AI agent session" as a **specialization of `TerminalSession`** owned by
this service (a new `agent_sessions` table referencing
`terminal_sessions.pty_id`, not a replacement for it) rather than inventing
a service boundary the TDD catalog doesn't have a slot for — but this is an
explicit scope addition to `infra-fleet-service.md`, the same way SOL-009
flagged `files.*` as an addition to `git-gateway-service.md`.

### The Dev Server Agent already implements `agent.spawn` — this is a pure backend-go gap

BUG-AG-01 states "No `agent.spawn` JSON-RPC method — the only spawn
primitive on the wire is `pty.create`." That is true of backend-go's
`devserveragent.Client` (confirmed: `methods.go` only wraps `pty.*`), but
**not** true of the Dev Server Agent itself. Reading `agent/`'s real
dispatch table:

- `agent/src/relay/agent-rpc-dispatch.ts` has live `case 'agent.spawn'`,
  `case 'agent.kill'`, `case 'agent.sendInput'` handlers (the "v5.0"
  comments date them as already-shipped, not planned).
- `agent/src/relay/agent-spawner.ts`'s `handleAgentSpawn` accepts almost
  exactly BL-AG-01's JSON-RPC contract: `{taskId, userId, modelId,
  accountId, cwd, resumeId, worktreePath, branchName, cols, rows,
  trustPreset}`, resolves a per-model `AgentBinarySpec` (`resolveAgentSpec`,
  `agent-spawner.ts:200-215`) covering claude/codex/gemini/opencode/ollama
  with **already-built** resume-arg logic (`buildArgs`, e.g. claude:
  `['--resume', resumeId]`), spawns via `node-pty`, and streams output back
  as JSON-RPC **notifications** `agent.output {ptyId, data}` /
  `agent.exited {ptyId, exitCode}` (`agent-spawner.ts:490-520`) — not a new
  RPC surface BL-AG-01/03 need built, just one backend-go has never called.
- This means **BL-AG-01's "Agent Config" concept (binary/args/resume-flag
  catalog) already exists agent-side** as `AGENT_SPECS`
  (`agent-spawner.ts:130-195`) — BUG-AG-01's "No `AgentConfig` concept...
  anywhere in backend-go" finding is correct for backend-go, but the fix is
  **not** porting a duplicate catalog into Go. `StartAgentSession` only
  needs to pass `modelId`/`trustPreset` through; the agent resolves binary
  + args itself. Two documented discrepancies worth carrying into the
  implementation, not silently reconciling: (1) BL-AG-01's `AgentConfig`
  describes `trustPresetEnvVars` (env vars); the real implementation applies
  trust preset as **CLI args** (`YOLO_TUI_AGENT_ARGS`), not env — BR-AG-03
  ("apply trust preset env vars before spawn") should be read as "apply the
  trust preset before spawn," full stop, the mechanism is args not env. (2)
  BL-AG-03's per-agent resume syntax table has at least one confirmed
  mismatch against the real agent (OpenCode resumes via `--session <id>`,
  not `resume <id>` — see `agent-spawner.ts`'s comment citing
  `agent-session-resume.ts:210`); flagged in SOL-AG-03 too since resume args
  are entirely agent-side, not something this solution builds.

This finding changes BUG-AG-01's remediation shape substantially: the
missing piece is Go-side wiring (typed client method, usecase, persistence,
proto/gRPC/wscompat plumbing) against an **already-real** agent capability —
not new `agent/` work, for everything except credential injection (next
section).

### Genuine architecture gap — credential injection is structurally blocked, not just unwired

This is the one piece of BL-AG-01 this solution **cannot** wire end-to-end,
and it is a real conflict between two things the TDD itself asserts, not a
missing implementation detail:

- `ai-provider-service.md` §9 states the Go rewrite's entire point is
  closing **TS Gap 2**: backend must never resolve a plaintext AI provider
  key; the Dev Server Agent decrypts ciphertext **locally**, via its own
  Vault Transit identity, at spawn time (`ai-provider-service.md:252-317`).
- `credential-broker-service.md` structurally enforces this:
  `ResolveCredential`'s per-category behavior table states `AI_PROVIDER_KEY`
  "**never plaintext**... Execution plane (Dev Server Agent) decrypts
  locally via its own Vault Transit access"
  (`credential-broker-service.md:184`), and its Vault ACL table gives the
  Dev Server Agent "Transit decrypt only, scoped to ciphertext it has
  already received via the push path" (`credential-broker-service.md:437`).
  There is **no RPC in this service's proto that returns a plaintext
  `AI_PROVIDER_KEY` value to any caller** — not a policy backend-go could
  choose to violate, a message shape that structurally can't carry one.
- But the **real, current** `agent/` implementation has no such Vault Transit
  capability at all. `agent-credential-store.ts` implements exactly the OLD
  Gap-2-shaped model: Layer 1 (browser `SubtleCrypto`-encrypted blob) +
  Layer 2 (agent's own local `scrypt`+AES-256-GCM re-encryption written to
  `~/.orca/credentials/<accountId>.enc`). `readDecryptedKey()` only strips
  Layer 2 — its return value is still Layer-1 ciphertext the agent cannot
  open. `buildAgentEnv()` (`agent-spawner.ts:246-318`) requires a **plaintext**
  `resolvedApiKey` passed in `agent.spawn`'s own params, and **throws** if
  it's absent for a keyed provider — there is no fallback path.

So: the TDD's target security model (agent decrypts locally via Vault
Transit) and the TDD's own credential-broker contract (never returns
plaintext) together make it **structurally impossible** for backend-go to
populate `agent.spawn`'s `resolvedApiKey` today, while the **real agent
code** has no alternative and requires exactly that field. Closing this gap
needs an `agent/` change — implementing Vault Transit decrypt in
`agent-credential-store.ts` and consuming `PushCiphertext` deliveries
(`credential-broker-service.md`'s `PushCiphertext` RPC already exists
service-side; nothing currently receives it agent-side) — which is outside
"the Go rewrite of `backend/`" as scoped by
`08-inter-service-communication.md`'s Option A ("the execution plane...
does not need to change at all"). **This is a genuine architecture decision
to flag to the user, not an implementation gap this solution can paper
over**: either (a) treat the agent-side Vault Transit work as a
prerequisite task before BL-AG-01/BL-AG-04 can spawn a keyed-provider agent
with real credentials, tracked explicitly, or (b) scope an initial cut to
`localInference` accounts only (`apiKeyEnvVar == nil`, e.g. Ollama — no
credential needed at all, per `agent-spawner.ts`'s `AGENT_SPECS` table),
which this solution's usecase can support today with zero credential
plumbing.

Everything else below (session persistence, resume, trust preset, startup
timeout) is implementable in backend-go now and is not blocked by this gap.

---

## Design — proto (`infrafleet.proto`)

```protobuf
service InfraFleetService {
  // ... existing RPCs unchanged ...

  // StartAgentSession spawns an AI-CLI agent (Claude Code, Codex, ...) via
  // the Dev Server Agent's real agent.spawn RPC — sibling to
  // SpawnTerminalSession, not a replacement (a bare shell PTY still uses
  // SpawnTerminalSession). See infra-fleet-service.md's package layout note
  // extended by this solution.
  rpc StartAgentSession(StartAgentSessionRequest) returns (AgentSession);
}

message StartAgentSessionRequest {
  string connection_id = 1;     // -> DevServer, same resolution as SpawnTerminalSession
  string worktree_id   = 2;     // logical FK -> project-service; BR-AG-01 scope key
  string user_id       = 3;     // BR-AG-01: one agent per worktree PER USER
  string cwd           = 4;     // worktree path on the dev server
  string model_id      = 5;     // e.g. "claude", "gpt-4o", "gemini", "opencode", "ollama"
  string account_id    = 6;     // ai_provider.accounts.id — from ResolveProvider; "" for localInference models
  string trust_preset  = 7;     // "standard" | "full" | "none" — forwarded as-is, applied agent-side as CLI args
  int32  cols          = 8;
  int32  rows          = 9;
  // resolved_api_key is DELIBERATELY ABSENT — see this solution's "Genuine
  // architecture gap" section. Adding it here would let a caller populate
  // agent.spawn's resolvedApiKey param, silently reintroducing Gap 2.
}

message AgentSession {
  string id              = 1;   // sessionId — orca_sessions.session_id equivalent
  string pty_id          = 2;
  string worktree_id     = 3;
  string dev_server_id   = 4;
  string user_id         = 5;
  string model_id        = 6;
  string account_id      = 7;
  string status          = 8;   // idle|running|waiting|completed|error|stopped — see SOL-AG-05
  int64  started_at_unix_ms      = 9;
  int64  last_active_at_unix_ms  = 10;
}
```

## Design — `usecase/ports.go` extensions

```go
// DevServerAgentClient — new methods alongside the existing pty.* wrappers,
// dispatching to the agent's real agent.spawn/agent.kill/agent.sendInput
// (confirmed live in agent/src/relay/agent-rpc-dispatch.ts).
type DevServerAgentClient interface {
	// ... existing SpawnPty/WritePty/.../InspectProcess unchanged ...

	// SpawnAgent calls agent.spawn. Returns immediately once the agent
	// accepts the request ({ok:true, ptyId}) — output/exit arrive later as
	// agent.output/agent.exited notifications over the same StreamPty
	// subscription used for plain PTYs (the wire shape is deliberately the
	// same notification mechanism).
	SpawnAgent(ctx context.Context, devServer domain.DevServer, in SpawnAgentInput) (SpawnAgentResult, error)
	// KillAgent calls agent.kill — signal is "SIGTERM" (graceful) or
	// "SIGKILL" (force), see SOL-AG-02.
	KillAgent(ctx context.Context, devServer domain.DevServer, ptyID, signal string) error
	// SendAgentInput calls agent.sendInput — used for graceful Ctrl+C, see
	// SOL-AG-02.
	SendAgentInput(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error
}

// SpawnAgentInput mirrors agent.spawn's real param set 1:1
// (agent-spawner.ts's AgentSpawnRequest) — resolvedApiKey intentionally
// absent, see "Genuine architecture gap".
type SpawnAgentInput struct {
	TaskID       string // this service's own session id, minted before calling — see below
	UserID       string
	ModelID      string
	AccountID    string
	Cwd          string
	ResumeID     string // "" for a fresh start; set by ResumeAgentSession (SOL-AG-03)
	WorktreePath string
	BranchName   string
	Cols, Rows   int32
	TrustPreset  string
}

type SpawnAgentResult struct {
	PtyID string
}

// AgentSessionRepository persists AgentSession — a specialization of
// TerminalSession (references terminal_sessions.pty_id), not a
// replacement. See domain/agent_session.go for the entity.
type AgentSessionRepository interface {
	// Create enforces BR-AG-01 (one non-terminal agent session per
	// worktree+user) via a partial unique constraint at the DB layer —
	// ErrAgentAlreadyRunning on conflict, not a race-prone check-then-insert.
	Create(ctx context.Context, s domain.AgentSession) (domain.AgentSession, error)
	Get(ctx context.Context, tenantID, sessionID string) (found bool, s domain.AgentSession, err error)
	// LatestForWorktree — BL-AG-03's `SELECT ... ORDER BY startedAt DESC
	// LIMIT 1`, used by ResumeAgentSession (SOL-AG-03).
	LatestForWorktree(ctx context.Context, tenantID, worktreeID string) (found bool, s domain.AgentSession, err error)
	UpdateStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error
	MarkStopped(ctx context.Context, tenantID, sessionID string, now time.Time) error
}

// AIProviderResolverClient — infra-fleet-service's own client of
// ai-provider-service.Resolve, mirroring git-gateway-service's existing
// grpcclient/aiprovider_client.go 1:1 (same RPC, new caller). This is a NEW
// edge on 02-microservices-decomposition.md's dependency graph
// (infra --> aiprov) — flagged explicitly, see "Design — wiring" below.
type AIProviderResolverClient interface {
	Resolve(ctx context.Context, userID, projectID string) (accountID, modelHint, status string, err error)
}
```

## Design — `usecase/start_agent_session.go`

Follows `SpawnTerminalSession.Execute`'s exact resolve→spawn→persist shape
(`spawn_terminal_session.go:48-94`), with BR-AG-01's single-agent guard and
credential-gap handling added:

```go
type StartAgentSession struct {
	resolver  ConnectionResolver
	agent     DevServerAgentClient
	sessions  AgentSessionRepository
	clock     func() time.Time
}

func (uc *StartAgentSession) Execute(ctx context.Context, in StartAgentSessionInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	connected, devServer, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err != nil || !connected {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "no dev server owns this connectionId", err)
	}

	sessionID := uuid.NewString() // minted here, passed as SpawnAgentInput.TaskID —
	                               // agent.spawn's ptyId embeds this (agent-spawner.ts:
	                               // `pty-${userId}-${taskId}-${Date.now()}`), so the
	                               // session<->pty linkage is derivable even before the
	                               // agent's response comes back.

	result, err := uc.agent.SpawnAgent(ctx, devServer, SpawnAgentInput{
		TaskID: sessionID, UserID: in.UserID, ModelID: in.ModelID, AccountID: in.AccountID,
		Cwd: in.Cwd, WorktreePath: in.Cwd, Cols: in.Cols, Rows: in.Rows, TrustPreset: in.TrustPreset,
	})
	if err != nil {
		// A resolvedApiKey-required failure (see "Genuine architecture gap")
		// surfaces from the agent as a JSON-RPC error; translateRelayError
		// maps its message to a dedicated apperrors kind so callers can
		// distinguish "credential injection unavailable" from a generic
		// spawn failure — see translateAgentSpawnError.
		return domain.AgentSession{}, translateAgentSpawnError(err)
	}

	now := uc.clock()
	session, err := uc.sessions.Create(ctx, domain.AgentSession{
		ID: sessionID, PtyID: result.PtyID, TenantID: tenantID, WorktreeID: in.WorktreeID,
		DevServerID: devServer.ID, UserID: in.UserID, ModelID: in.ModelID, AccountID: in.AccountID,
		Status: domain.AgentStatusSpawning, StartedAt: now, LastActiveAt: now,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAgentAlreadyRunning) {
			// BR-AG-01. The agent process is now orphaned on the dev server —
			// kill it rather than leave an untracked PTY running (mirrors
			// KillTerminalSession's "the persisted record must not lie"
			// discipline, applied to the inverse case: don't let an
			// unpersisted PTY outlive its record).
			_ = uc.agent.KillAgent(ctx, devServer, result.PtyID, "SIGKILL")
			return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_ALREADY_RUNNING", "an agent is already running for this worktree and user", err)
		}
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_AGENT_SESSION_FAILED", "failed to persist agent session", err)
	}
	// BR-AG-04 (30s startup timeout) is enforced by SOL-AG-05's
	// AgentOutputClassifier watching this session's StreamPty subscription
	// for the first idle signal, not inline here — Execute returns as soon
	// as the agent *accepts* the spawn, matching agent.spawn's own
	// fire-and-forget contract.
	return session, nil
}
```

`translateAgentSpawnError` pattern-matches the agent's
`"buildAgentEnv: ... no plaintext resolvedApiKey was provided"` /
`"no credential found for accountId=..."` messages (both distinct, fixed
strings in `agent-spawner.ts`) into
`apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE", ...)`
— an honest, distinguishable error rather than a generic internal failure,
until the agent-side Vault Transit work in "Genuine architecture gap" lands.

## Design — `domain/agent_session.go`

```go
type AgentStatus string

const (
	AgentStatusSpawning  AgentStatus = "spawning" // between SpawnAgent accept and first idle signal (SOL-AG-05)
	AgentStatusIdle       AgentStatus = "idle"
	AgentStatusRunning    AgentStatus = "running"
	AgentStatusWaiting    AgentStatus = "waiting"
	AgentStatusCompleted  AgentStatus = "completed"
	AgentStatusError      AgentStatus = "error"
	AgentStatusStopped    AgentStatus = "stopped"
)

var ErrAgentAlreadyRunning = errors.New("domain: an agent session is already running for this worktree+user")

type AgentSession struct {
	ID, PtyID, TenantID, WorktreeID, DevServerID string
	UserID, ModelID, AccountID                   string
	ResumeOfSessionID                             string // "" for a fresh start — SOL-AG-03
	AgentVersion                                   string // dev server's agent_version at spawn time — SOL-AG-03's BR-AG-09 check
	Status                                         AgentStatus
	StartedAt, LastActiveAt                        time.Time
	StoppedAt                                      *time.Time
}
```

## Design — schema (extends `infra-fleet-service.md` §5)

```sql
-- Extension beyond infra-fleet-service.md §5 — an AI-agent specialization of
-- terminal_sessions, not a replacement. This is the orca_sessions table
-- BL-AG-01/02/03 all reference; the doc's plain "orca_sessions" name is kept
-- as a column-comment cross-reference, table name follows this service's
-- existing snake_case convention.
CREATE TABLE agent_sessions (
  id                UUID PRIMARY KEY,                       -- sessionId
  tenant_id         UUID NOT NULL,
  pty_id            UUID NOT NULL REFERENCES terminal_sessions(pty_id),
  worktree_id       UUID NOT NULL,                          -- logical FK -> project-service
  dev_server_id     UUID NOT NULL REFERENCES dev_servers(id),
  user_id           UUID NOT NULL,
  model_id          TEXT NOT NULL,
  account_id        UUID,                                   -- logical FK -> ai_provider.accounts; NULL for localInference
  resume_of_session_id UUID REFERENCES agent_sessions(id),
  agent_version     TEXT,                                   -- BR-AG-09, see SOL-AG-03
  status            TEXT NOT NULL DEFAULT 'spawning' CHECK (status IN
                       ('spawning','idle','running','waiting','completed','error','stopped')),
  started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_active_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  stopped_at        TIMESTAMPTZ
);
-- BR-AG-01: one non-terminal agent session per worktree+user.
CREATE UNIQUE INDEX idx_agent_sessions_active_per_worktree_user
  ON agent_sessions(tenant_id, worktree_id, user_id)
  WHERE status NOT IN ('stopped', 'completed', 'error');
CREATE INDEX idx_agent_sessions_worktree_recent
  ON agent_sessions(tenant_id, worktree_id, started_at DESC); -- SOL-AG-03's resume lookup
```

## Design — wiring

- `cmd/server/main.go` gains a new outbound gRPC client dial to
  `ai-provider-service` (a **new dependency edge**,
  `infra --> aiprov`, not present in
  `02-microservices-decomposition.md`'s current dependency graph — flagged
  as a graph extension this solution requires, same class of change as
  SOL-009's "no new edges required" note, except here one genuinely is
  needed). `AIProviderResolverClient` wraps it exactly like
  `git-gateway-service/internal/adapter/grpcclient/aiprovider_client.go`
  already does — same RPC, second caller.
- `api-gateway`'s `wscompat/channels_agent.go` (new) registers `agent.start`,
  decoding `{worktreeId, connectionId, userId, modelId, accountId,
  trustPreset, cols, rows}` and calling `InfraFleetServiceClient.StartAgentSession`
  — same shape as `registerTerminalCreateChannel`
  (`channels_terminal.go`), new file rather than growing
  `channels_terminal.go` further, per this repo's file-naming discipline
  (agent sessions are a distinct concept from generic terminals, not a
  `terminal-helpers`-style catch-all).
- `account_id` for the request comes from a **prior** call to
  `aiProvider.resolve` (already wired per BUG-AG-01's "What backend-go has":
  `GET /v1/aiprovider/resolve`) — `channels_agent.go`'s `agent.start` handler
  does not itself call `Resolve`; the renderer resolves the account first
  (mirrors BL-AG-01's own flow, step 3c, which resolves the provider before
  building the spawn call) and passes `accountId` through, keeping
  `StartAgentSession`'s cross-service call graph to one hop
  (`infra-fleet-service --> agent`), not two serialized synchronous calls
  on the hot path.

## Test plan

- `domain/agent_session_test.go` — pure: status enum validity, `AgentSession` invariants.
- `usecase/start_agent_session_test.go` — fake `ConnectionResolver`/`DevServerAgentClient`/`AgentSessionRepository`:
  - resolved connection → `SpawnAgent` called with the right params → session persisted with `spawning` status.
  - `ErrAgentAlreadyRunning` from the repository → asserts `KillAgent` is called (cleanup) and the returned error is `INFRA_AGENT_ALREADY_RUNNING`.
  - `SpawnAgent` returns the agent's "no plaintext resolvedApiKey" error string → asserts the mapped error is `INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE`, not a generic internal error.
  - unresolved connection → `INFRA_CONNECTION_NOT_FOUND`, `SpawnAgent` never called.
- `adapter/devserveragent/agent_methods_test.go` — `SpawnAgent` sends
  `agent.spawn` with the exact param names `agent-spawner.ts`'s
  `handleAgentSpawn` reads (`taskId`, `userId`, `modelId` as `"model"` —
  confirm which key name the Go client uses matches the agent's dual
  accept of `model`/`modelId`), decodes `{ok, ptyId}`.
- `adapter/postgres/agent_session_repository_test.go` — `testcontainers-go`:
  the partial unique index actually rejects a second concurrent `Create`
  for the same `(tenant_id, worktree_id, user_id)` while one is
  non-terminal, and allows a new one once the prior row is `stopped`.
- Integration (`docker-compose`, cross-service): `localInference` model
  (`ollama`) end-to-end spawn with no `accountId` — the one path this
  solution can fully exercise without the blocked credential work.

## References

- `specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md` — problem statement
- `docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md` — spec, BR-AG-01..04/18-21, `AgentConfig`/JSON-RPC contract
- `specs/backend-go/tdd/services/infra-fleet-service.md:1-76` (§1-2 bounded context), `:117-137` (§3 RPC surface), `:293-349` (§6 package layout, `devserveragent` as defining adapter), `:525-559` (§10 Option A)
- `specs/backend-go/tdd/services/ai-provider-service.md:250-317` (§9 Gap-2-closing design, ciphertext-push)
- `specs/backend-go/tdd/services/credential-broker-service.md:120-186` (§3 API surface, `ResolveCredentialResponse`'s per-category plaintext table), `:430-438` (Vault ACL table, agent's Transit-decrypt-only scope)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:33-36` (design principle 4), `:110-166` (dependency graph — `infra --> aiprov` is a new edge)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:84-108` (Option A, agent unchanged)
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:48-94` — the resolve→spawn→persist shape this solution's `StartAgentSession` follows
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:129-264` — existing `DevServerAgentClient`/`TerminalSessionRepository` ports extended here
- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:1-100` — the already-correct cascade this solution calls, unmodified
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/aiprovider_client.go` — the existing client pattern this solution's new `infra-fleet-service` client mirrors
- `agent/src/relay/agent-rpc-dispatch.ts:1080-1110` — live `agent.spawn`/`agent.kill`/`agent.sendInput` cases
- `agent/src/relay/agent-spawner.ts:60-215,330-460` — `AgentSpawnRequest`, `AGENT_SPECS`/`resolveAgentSpec`, `handleAgentSpawn`, `agent.output`/`agent.exited` notifications
- `agent/src/relay/agent-credential-store.ts:1-10,347-365` — Layer-1/Layer-2 credential model, `readDecryptedKey`'s Layer-2-only scope
