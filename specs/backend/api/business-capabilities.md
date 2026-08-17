# Backend Business Capabilities

Every business capability `backend/` (`backend/src/main/`) currently
provides, organized by domain — not a raw RPC method dump (see
[`specs/frontend/api/rpc-catalog.md`](../../frontend/api/rpc-catalog.md) for
that) but "what does the backend actually *do*." Each domain states: what it
does, how the frontend reaches it, where the real work happens today
(backend-local / Postgres / relayed to the Dev Server Agent), and which
Postgres table(s) back it. Dispatch detail beyond a one-line summary lives in
[`specs/frontend/api/backend-agent-execution-boundary.md`](../../frontend/api/backend-agent-execution-boundary.md) —
this doc doesn't repeat it.

Domain grouping matches `specs/backend/bugs/*`'s existing taxonomy where one
exists, so bug reports and capability docs use the same vocabulary.

## Identity & access

### Authentication & sessions
Local email+password login, session cookie issuance/revocation, SSO
kick-off (stub — always `501` today). `POST /auth/local`, `/auth/logout`,
`GET /auth/me`, `/auth/config`, `GET /auth/sso/:provider` (plain HTTP, not
RPC — session cookie `orca_session`, not a bearer token). Backend-local +
Postgres (`orca_users` and session state).

### Admin console
User CRUD (create/update/deactivate — no reactivate action despite the
field supporting it), session management (list/force-revoke, including
kill-all-sessions-for-user), access/rate policy CRUD, audit log query.
`/admin/api/*` (plain HTTP, `requireAdmin`-gated). Backend + Postgres.
**Known gap** (see the redesign proposal): `requireAdmin` was found this
session to only check "is logged in," not "is admin" — since patched per
`c88c918fa`, re-verify before relying on this doc for a security decision.

### Profile hierarchy (company → department → user)
3-tier settings resolution (company defaults ← department overrides ← user
overrides), company/department CRUD, user-department assignment.
`profile.*` RPC (11 methods). Backend + Postgres
(`orca_companies`/`orca_departments`/`orca_user_profiles`/`orca_users`). No
agent involvement — pure metadata.

### Team membership
Team CRUD, member add/remove, member listing. `team.*` RPC (5 methods).
Backend + Postgres (`orca_teams`/`orca_team_members`).

## Project & workspace coordination

### Project management (v5 collaborative projects)
Project CRUD, membership (add/remove/role-update), member listing, binding
a project to a dev server. `project.*` RPC (10 methods). Backend + Postgres
(`orca_v5_projects`/`orca_v5_project_members`). **One exception**:
`project.agentSpawn` relays entirely to the Dev Server Agent
(`agent.exec`) — the one place this namespace touches execution, not
metadata.

### Repo / worktree lifecycle
Repo catalog (add/clone/list/reorder/remove), folder-group organization
(project groups, nested-repo scanning), worktree lifecycle (create,
list/detect, activate, sleep, remove, rename/lineage tracking, branch
force-delete). `repo.*`, `folderWorkspace.*`, `projectGroup.*`, `worktree.*`
RPC. Backend coordinates + Postgres (one JSON state blob — see the
execution-boundary doc's storage-model note) for **all** metadata; the
actual git operations backing worktree create/remove/list **dynamically
relay** to the Dev Server Agent whenever the worktree's repo has a
`connectionId` (SSH target or dev server) — same dispatch pattern as
`git.*` below.

### Dev server / fleet management
Dev server registration (CRUD), connection establishment/teardown/health
check, SSH target registration and connection lifecycle, port-forward
CRUD, fleet bootstrap (install Node/git, deploy the relay binary over SSH),
fleet health polling. `devServer.*`, `ssh.*`, `fleet.*` RPC. Backend +
Postgres for all CRUD/metadata; the connect/disconnect/bootstrap/health-check
acts themselves are inherently "talk to the remote host directly" (raw SSH
exec or WS handshake) — this is Coordination Layer territory even though it
touches a remote machine, because it's establishing/monitoring the
connection itself, not doing dev-work on it.

### Terminal / PTY session coordination
PTY create/split/read/write/resize/close, session listing, agent-in-terminal
status detection, viewport/display-mode tracking. `terminal.*` RPC (~25
methods). Backend coordinates connection routing (which provider a given
`ptyId` belongs to); **actual PTY I/O always executes on the target
host** — locally via `node-pty` in Electron/desktop deployments, or always
relayed to the Dev Server Agent / SSH relay in the pure-Node multi-user
server deployment (which has no local-shell concept at all). No Postgres
involvement in PTY data itself.

## Source control

### Git operations
The full worktree git surface: status, diff, stage/unstage/commit/push/pull,
branch management, history, conflict resolution, AI-assisted commit
message/PR-description generation. `git.*` RPC (34 methods). **Dynamically
dispatched per call** — local git exec if the worktree has no
`connectionId`, relayed to the Dev Server Agent/SSH target otherwise. Zero
Postgres involvement in the git operations themselves (only reads
already-cached repo metadata).

### GitHub / GitLab integration
Issue/PR/MR CRUD, comments, reviewers, labels, checks, project-board views,
rate-limit status, "star Orca" prompt. `github.*` (46 methods), `gitlab.*`
(18 methods). **Currently backend self-executes** the `gh`/`glab` CLI
in-process using a shared OS keychain, not per-user-isolated, not relayed to
the Dev Server Agent — this is the domain the redesign proposal below
addresses. Auth login/logout (`startAuthLogin`/`revokeAuth`) is the one
exception that already relays correctly, per-user-isolated. No Postgres
(recent-project references only).

### Hosted code review
Cross-provider PR/MR creation and eligibility checks spanning GitHub,
GitLab, Bitbucket, Azure DevOps, Gitea. `hostedReview.*` RPC (3 methods).
Mixed: GitHub/GitLab branches go through the same self-executed CLI path as
above; Bitbucket/Azure/Gitea go through per-user HTTP clients with
Postgres/file-backed credentials.

### Jira & Linear integration
Issue/project/workflow querying and mutation, connection management.
`jira.*` (18 methods), `linear.*` + `linear-agent-access.*` (35 methods
combined). Cleanly backend-local: direct HTTPS REST (Jira) / GraphQL SDK
(Linear) calls, credentials in per-service encrypted files or
`WebCredentialStore` — **not** CLI-based, **not** relayed, **not**
Postgres-backed for credentials (one side effect: Linear issue-linking
writes a field onto the worktree's Postgres blob row).

### Annotation (code review comments)
Inline file/line comment CRUD. `annotation.*` RPC (2 methods, built this
session). Pure Postgres (`orca_annotations`), zero agent involvement.

## AI capability

### AI provider account management
Account metadata CRUD (which providers/accounts exist, quota/usage
tracking), credential write/rotate/test. `aiProvider.*` RPC (10 methods).
Backend + Postgres for metadata (`orca_ai_provider_accounts`/
`orca_provider_usage`); **credential material itself is deliberately
never stored or decrypted on backend** — `writeCredential`/`rotateKey`/
`testConnection` relay ciphertext-only to the Dev Server Agent, which holds
the decryption key (ADR-008). One known deviation from the ADR's stated
threat model: the agent-spawn path requires backend to forward a plaintext
resolved key when available — see the redesign proposal.

### Task management & AI decomposition
Task CRUD (hierarchical, with dependency edges), comments, grant-based
permissions, AI-assisted decomposition into subtasks, execution dispatch.
`task.*` RPC. Backend + Postgres (`orca_tasks`/`orca_task_edges`/
`orca_task_comments`/`orca_task_grants`) for everything except:
`task.aiDecompose` relays to the Dev Server Agent's `ai.complete` (AI
inference doesn't run on backend); `task.execute` branches by complexity —
simple tasks relay directly to `agent.exec`, complex ones (with
subtasks/dependencies) go through the orchestration coordinator
(`PgOrchestrationDb`, genuinely relational Postgres tables), which itself
eventually reaches the Dev Server Agent for worker execution.

### Workflow orchestration
Template CRUD, execution start/cancel/pause/resume/list, per-step dispatch
across 5 step types. `workflow.*` RPC. Backend + Postgres
(`orca_workflow_templates`/`orca_workflow_executions`/
`orca_workflow_step_executions`) for definitions/status; step *execution*
splits three ways — `agent`/`shell`/`notification` steps relay to the Dev
Server Agent, `webhook` runs a native `fetch()` on backend, `condition` is a
pure in-memory expression evaluator.

### Agent-team orchestration
Multi-agent coordination gates (dispatch context, message bus, task queue).
`orchestration.*`/`orchestration-gates.*` RPC. Entirely against the
genuinely-relational `PgOrchestrationDb` (migration 0020). One side effect
(terminal-agent-prompt injection) reaches into the PTY layer, where relay
may occur — that's the PTY layer's concern, not this domain's own logic.

### AI vault / session history
Local AI-CLI session transcript scanning/reading across supported CLIs
(Claude Code, Codex, Gemini, Kimi, Grok, OpenCode, …). `ai-vault.*` RPC.
Backend-local filesystem scan **on the backend host**, not the dev server —
restamped with an `executionHostId` for display but never dispatched
remotely.

### Provider account bridge (mobile)
Claude/Codex CLI account selection/removal for the mobile companion app.
`accounts.*` RPC. Backend-local filesystem (reads/writes the CLIs' own
config on the backend host) — distinct domain from AI-provider credential
vault above; no Postgres, no relay.

## Automation & scheduling

### Automations
Scheduled/triggered automation definition CRUD, manual run trigger,
external-manager integration. `automation.*` RPC. Backend + Postgres
(`PgAutomationStore`, same service class desktop uses). **Known gap**:
`runNow`/scheduled dispatch has no working execution path on the server
deployment — the dispatcher is intentionally left unwired (explicit TODO),
so triggered runs resolve `skipped_unavailable` rather than executing or
relaying anywhere.

## Cross-cutting / infrastructure-adjacent

### External credential storage
Encrypted per-user token storage for Bitbucket/Azure DevOps/Gitea/Linear/Jira
(explicitly **not** GitHub/GitLab, which use the shared CLI keychain
instead). `credentials.*` RPC. Backend-local AES-256-GCM encrypted files on
the backend host filesystem — **not Postgres** despite the "Web Server
mode" framing suggesting otherwise; gated behind `ORCA_MULTI_USER=1`.

### Notifications (mobile push)
In-app WS event fan-out plus mobile push delivery for events like
`agent-task-complete`. `notifications.*` RPC for the WS side;
`WebPushManager`/`PgWebPushStore` (genuinely Postgres-backed) for the push
side — a separate mechanism from both the WS fan-out and any Dev Server
Agent relay, easy to conflate with the latter but isn't one.

### Browser / computer / emulator automation
Embedded browser-pane control (CDP), macOS accessibility automation,
mobile device emulator control. `browser.*`, `computer.*`, `emulator.*`
RPC. **All entirely backend-host-local** — these automate the Orca backend
process's own Electron window / the backend machine itself, not any dev
server. No host branching, no Postgres, no relay — worth flagging in the
redesign proposal as likely out-of-model for a multi-user Postgres-backed
deployment (see below).

### Workspace port scanning
Detects dev-server-forwarded ports in use by a worktree's dev process.
`workspacePorts.*` RPC. Backend-local only — **silently excludes any
worktree whose repo has a `connectionId`** (returns empty rather than
relaying or erroring) — a real gap, not a deliberate local-only design like
browser/computer/emulator above.

### Client state sync
Generic key-value UI/client-settings persistence, feature-interaction
tracking, generic runtime event pub/sub. `client-ui.ts`
(`settings.*`/`ui.*`), `client-events.ts`
(`runtime.clientEvents.subscribe`). Backend + Postgres (the one JSON blob)
for settings; event pub/sub itself is in-memory, no persistence.

### Diagnostics & stats
Backend process memory snapshot, in-memory usage-stats counters. `stats.*`,
`diagnostics.*` RPC. Backend-local in-memory only, no Postgres.

### Skills catalog
Local filesystem scan for available skill definitions. `skills.discover`
RPC. Backend-local filesystem scan, no Postgres, no relay — sourced live
from disk each call rather than a persisted catalog.

---

## Storage model summary

With the confirmed exceptions of `orca_ai_provider_accounts`/
`orca_provider_usage`/`orca_annotations`/the v5 collaborative-project tables
(`orca_v5_projects`, `orca_teams`, `orca_tasks`, `orca_workflow_templates`,
etc.)/`PgOrchestrationDb`/`PgAutomationStore`/`PgWebPushStore` — all
genuinely relational — **everything else described as "Postgres" above goes
through one JSON blob table**, `core.orca_data_state_blob`, keyed by
`(tenant_id, user_id)`, loaded once at boot and persisted back on mutation.
See
[`backend-agent-execution-boundary.md`](../../frontend/api/backend-agent-execution-boundary.md)
for the full detail and file:line citations.

## Methodology

Synthesized from this session's exhaustive RPC-dispatch investigation
(5 parallel passes tracing every `backend/src/main/runtime/rpc/methods/*.ts`
handler into its real implementation) plus the pre-existing
`specs/agent/api/` audit (backend↔agent wire contract, dated 2026-08-15).
Not a fresh read of every line of `backend/src/main/` — capabilities with no
RPC surface at all (if any exist purely as internal plumbing) may be
under-represented; this catalogs what's reachable from the frontend or the
agent, which covers the overwhelming majority of backend's actual business
logic.
