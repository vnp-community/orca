# Service Catalog

One-page index. Full deep dive (API surface, domain model, DB schema,
dependencies, NFRs, migration notes) is in each service's own doc. See
[`../architecture/02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md)
for the reasoning behind these boundaries.

| Service | Category | Owns | Replaces (TS) | Migration phase |
|---------|----------|------|-----------------|-------------------|
| [api-gateway](./api-gateway.md) | Edge | Nothing (stateless routing) | `runtime/rpc/dispatcher.ts`, `WsSessionRouter`, `http-server.ts` | 5 (last) |
| [auth-service](./auth-service.md) | Identity | Users, sessions, RBAC, audit, admin console | `AuthManager`, `backend/src/main/admin/` | 4 |
| [tenant-service](./tenant-service.md) | Identity | Companies, departments, user profiles, teams | `ProfileResolver`, `TeamService` | 4 |
| [project-service](./project-service.md) | Workspace | Projects, membership, dev-server binding | `ProjectService` | 2 |
| [infra-fleet-service](./infra-fleet-service.md) | Workspace | SSH targets, dev servers, fleet health, provider registry, terminal routing | `ssh/`, `providers/`, `FleetHealthMonitor` | 2 |
| [git-gateway-service](./git-gateway-service.md) | Workspace | Nothing (dispatch only) | `git.ts` dynamic dispatch | 3 |
| [scm-integration-service](./scm-integration-service.md) | Integration | Nothing (external system of record) | `github/`, `gitlab/`, `bitbucket/`, `azure-devops/`, `gitea/`, `hosted-review/` | 3 |
| [issue-tracking-service](./issue-tracking-service.md) | Integration | Nothing (external system of record) | `jira/`, `linear/` | 1 |
| [ai-provider-service](./ai-provider-service.md) | AI | Provider account metadata, usage/quota | `AIProviderService`, `ProviderResolver`, `ProviderHealthChecker` | 2 |
| [workflow-service](./workflow-service.md) | AI | Templates, executions, step executions | `WorkflowOrchestrator`, `DAGBuilder`, `StepExecutors` | 2 |
| [task-service](./task-service.md) | AI | Task DAG, grants, comments | `TaskService`, `TaskDAGValidator`, `TaskGrantService`, `TaskAIPlanner` | 2 |
| [orchestration-service](./orchestration-service.md) | AI | Multi-agent coordination gates | `PgOrchestrationDb`, `runtime/orchestration/` | 2 |
| [automation-service](./automation-service.md) | AI | Automation definitions + runs | `AutomationService` | 2 |
| [annotation-service](./annotation-service.md) | Supporting | Review comments | `annotation-store.ts` | 1 |
| [notification-service](./notification-service.md) | Supporting | Push subscriptions, VAPID metadata, WS fan-out | `WebPushManager`, `PgWebPushStore` | 1 |
| [usage-service](./usage-service.md) | Supporting | AI-CLI usage/cost tracking | `ClaudeUsageStore`, `CodexUsageStore` | 1 (pilot) |
| [credential-broker-service](./credential-broker-service.md) | Supporting | Secret metadata + Vault mediation | 5 fragmented mechanisms — see [`05-credential-secret-stores.md`](../../backend/models/05-credential-secret-stores.md) | 2 |

**Not services** (Vault, PostgreSQL, NATS JetStream, the Dev Server Agent)
are external systems/infrastructure — see
[`../architecture/01-c4-hld.md`](../architecture/01-c4-hld.md)'s C2 diagram
for how they fit into the topology.
