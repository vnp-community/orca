# Domain / Capability → Go Service Mapping

Exhaustive mapping of every domain in
[`business-capabilities.md`](../../backend/api/business-capabilities.md) and
every RPC namespace in
[`backend-hld-c4.md`](../../backend/api/backend-hld-c4.md) to its target Go
service. Use this table to answer "where does capability X live in the Go
system" during implementation — it's the single source of truth for that
question, so keep it in sync if the decomposition in
[`../architecture/02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md)
changes.

| TS RPC namespace / capability | business-capabilities.md domain | Target Go service | Notes |
|-------------------------------|----------------------------------|---------------------|-------|
| `profile.*` | Profile hierarchy | `tenant-service` | |
| `team.*` | Team membership | `tenant-service` | |
| Admin console (`/admin/api/*`) | Admin console | `auth-service` | Folded into auth per decomposition doc |
| Auth (`POST /auth/local`, sessions) | Authentication & sessions | `auth-service` | |
| `project.*` (except `agentSpawn`) | Project management | `project-service` | |
| `project.agentSpawn` | Project management (exception) | `git-gateway-service` or `infra-fleet-service` (relays to execution plane) | Not a metadata op — dispatches like `task.execute` |
| `repo.*`, `folderWorkspace.*`, `projectGroup.*`, `worktree.*` | Repo / worktree lifecycle | `project-service` (metadata) + `infra-fleet-service` (relay dispatch) | Same coordination/execution split as TS: metadata here, git-touching ops go through `git-gateway-service` |
| `devServer.*`, `ssh.*`, `fleet.*` | Dev server / fleet management | `infra-fleet-service` | |
| `terminal.*` | Terminal / PTY session coordination | `infra-fleet-service` | Connection routing only, per TS design — actual PTY I/O stays with the execution plane |
| `git.*` | Git operations | `git-gateway-service` | |
| `github.*`, `gitlab.*` | GitHub/GitLab integration | `scm-integration-service` | **Rebuilt as direct API client, not CLI shell-out — closes TS Gap 1** |
| `hostedReview.*` | Hosted code review | `scm-integration-service` (GitHub/GitLab paths) + this service also owns Bitbucket/Azure/Gitea directly | |
| `jira.*` | Jira integration | `issue-tracking-service` | |
| `linear.*`, `linear-agent-access.*` | Linear integration | `issue-tracking-service` | |
| `annotation.*` | Annotation | `annotation-service` | |
| `aiProvider.*` | AI provider account management | `ai-provider-service` (metadata) + `credential-broker-service` (secret material) | |
| `task.*` | Task management & AI decomposition | `task-service` | `task.aiDecompose` calls out to whichever AI-inference path replaces `ai.complete` — TBD alongside the execution-plane relay decision in `08-inter-service-communication.md` |
| `workflow.*` | Workflow orchestration | `workflow-service` | |
| `orchestration.*`, `orchestration-gates.*` | Agent-team orchestration | `orchestration-service` | |
| `ai-vault.*` | AI vault / session history | **Not carried forward** | See decomposition doc — backend-host-local filesystem scan doesn't fit a stateless service; product decision needed |
| `accounts.*` | Provider account bridge (mobile) | `ai-provider-service` | Distinct from credential vault, same as TS |
| `automation.*` | Automations | `automation-service` | Execution actually wired this time (closes TS Gap 3) |
| `credentials.*` | External credential storage | `credential-broker-service` | Backed by Vault, not per-user files |
| `notifications.*` | Notifications (mobile push) | `notification-service` | |
| `browser.*`, `computer.*`, `emulator.*` | Browser/computer/emulator automation | **Not carried forward by default** | See decomposition doc Gap 6 — product decision |
| `workspacePorts.*` | Workspace port scanning | `infra-fleet-service` | Fixed to relay when `connectionId` set (closes TS Gap 7) |
| `settings.*`/`ui.*` (client state sync) | Client state sync | `tenant-service` (user-scoped settings) or a thin `api-gateway`-local concern if truly UI-only, decide per field | Generic KV — smallest-footprint mapping, revisit if it grows real structure |
| `runtime.clientEvents.subscribe` | Client state sync (event pub/sub) | `notification-service` | In-memory pub/sub becomes the WS layer `api-gateway` already needs for other real-time surfaces |
| `stats.*`, `diagnostics.*` | Diagnostics & stats | Per-service `/metrics` + `usage-service` for anything usage-shaped | No single "diagnostics service" — this is what Prometheus/OTel replace |
| `skills.discover` | Skills catalog | Not a backend concern in the target design — filesystem scan of what's on disk; revisit if this needs to be server-side at all for a multi-tenant deployment | |
| Claude/Codex/OpenCode usage tracking | AI vault / session history (usage sub-domain) | `usage-service` | |

## Storage-model carryover

The TS system's "one JSON blob table backs most domains" pattern
(`business-capabilities.md`'s Storage model summary) has **no equivalent**
in the Go target — every domain gets its own properly-modeled relational
schema in its owning service's database from day one. This is one of the
larger structural improvements this rewrite delivers, not just a language
port: the JSON-blob pattern existed in the TS system as *historical
accretion* (one `Store` class grew to own ~30 domains), not as a deliberate
design choice worth preserving.
