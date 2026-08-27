# C4 HLD — Target Go Microservices System

**Model:** C4 Architecture (Simon Brown). Companion to
[`specs/backend/api/backend-hld-c4.md`](../../backend/api/backend-hld-c4.md)
(the TS system this replaces) — read that first if you haven't; this
document assumes the same actor/external-system set and only changes what's
inside the `backend` system boundary.

## C1 — System Context

Unchanged from the TS system at the context level: same actors (end user,
admin, mobile user, custom agent developer), same external systems (Dev
Server Agent, GitHub/GitLab/Jira/Linear/Bitbucket/Azure DevOps/Gitea, mobile
push, database). Two additions:

```mermaid
C4Context
  title System Context — Orca Backend (Go microservices target)

  Person(user, "End User", "Developer / Tech Lead")
  Person(admin, "Admin", "Manages users, sessions, policies, audit log")
  Person(mobile_user, "Mobile User", "Monitors + dispatches from a phone")
  Person(agentdev, "Custom Agent Developer", "Connects a self-written agent")

  System(backend, "Orca Backend (Go)", "17 microservices behind api-gateway. Same coordination/control-plane role as the TS system.")

  System_Ext(frontend, "Orca Frontend (React SPA)", "Unchanged — same client, new backend API")
  System_Ext(devagent, "Dev Server Agent", "Unchanged execution plane (agent/)")
  System_Ext(pgcluster, "PostgreSQL", "Database-per-service, one physical DB per service")
  System_Ext(vault, "HashiCorp Vault", "NEW — single source of truth for all secret material")
  System_Ext(scm, "GitHub/GitLab/Jira/Linear/Bitbucket/Azure DevOps/Gitea", "Same external SCM/PM systems")
  System_Ext(mobile, "Orca Mobile", "Unchanged")
  System_Ext(push, "APNs / FCM", "Unchanged")
  System_Ext(bus, "Event Bus (NATS JetStream)", "NEW — async domain events between services")

  Rel(user, frontend, "Uses", "Browser UI")
  Rel(frontend, backend, "Calls", "HTTPS + WSS, JSON over REST, WebSocket for real-time")
  Rel(admin, backend, "Manages platform", "HTTPS /admin (routed to auth-service)")
  Rel(mobile_user, mobile, "Monitors + dispatches", "")
  Rel(mobile, backend, "Status + dispatch", "WSS, TweetNaCl E2E preserved at the edge")
  Rel(agentdev, devagent, "Writes and deploys", "")

  Rel(backend, devagent, "Relays git/fs/pty/AI-spawn work", "Same 3-mode relay contract as TS (or a gRPC-streaming successor — see 08)")
  Rel(backend, pgcluster, "Reads/writes, one DB per service", "SQL, Vault-issued dynamic credentials")
  Rel(backend, vault, "Fetches DB creds, encrypts/decrypts secrets", "Vault API, Kubernetes auth")
  Rel(backend, scm, "Direct per-tenant OAuth API calls", "HTTPS REST/GraphQL — no shared CLI shell-out")
  Rel(backend, bus, "Publishes/consumes domain events", "NATS")
  Rel(backend, push, "Push delivery", "HTTPS VAPID")
```

**What changed vs. the TS context:** Vault and the event bus are new
first-class external dependencies. GitHub/GitLab move from
"self-executed CLI in-process" to "direct API calls" (closing TS Gap 1 by
construction — there's no CLI shell-out option in the Go design at all).

## C2 — Container (service topology)

```mermaid
C4Container
  title Container Diagram — Orca Backend (Go), service topology

  Container_Ext(frontend, "Frontend/Mobile/CLI clients", "")

  Container_Boundary(edge, "Edge") {
    Container(gw, "api-gateway", "Go / chi + gRPC-Gateway", "TLS termination, JWT validation, REST/WS<->gRPC translation, rate limiting")
  }

  Container_Boundary(identity, "Identity & Tenancy") {
    Container(auth, "auth-service", "Go", "Users, sessions, RBAC, admin, audit")
    Container(tenant, "tenant-service", "Go", "Company/dept/user-profile hierarchy, teams")
  }

  Container_Boundary(workspace, "Workspace Coordination") {
    Container(proj, "project-service", "Go", "Project CRUD, membership, dev-server binding")
    Container(infra, "infra-fleet-service", "Go", "SSH/dev-server registry, fleet health, provider registry, terminal/PTY routing")
    Container(git, "git-gateway-service", "Go", "git.* dispatch: local vs relay")
  }

  Container_Boundary(scm_bound, "SCM & PM Integration") {
    Container(scm, "scm-integration-service", "Go", "GitHub/GitLab/Bitbucket/Azure DevOps/Gitea")
    Container(issue, "issue-tracking-service", "Go", "Jira/Linear")
  }

  Container_Boundary(ai_orch, "AI & Orchestration") {
    Container(aiprov, "ai-provider-service", "Go", "Provider account metadata, credential orchestration")
    Container(wf, "workflow-service", "Go", "Template + DAG execution")
    Container(task, "task-service", "Go", "Task DAG, grants, AI decompose")
    Container(orch, "orchestration-service", "Go", "Multi-agent coordination gates")
    Container(auto, "automation-service", "Go", "Scheduled/triggered runs")
  }

  Container_Boundary(support, "Supporting") {
    Container(annot, "annotation-service", "Go", "Review comments")
    Container(notif, "notification-service", "Go", "WS fan-out + push")
    Container(usage, "usage-service", "Go", "AI-CLI usage/cost tracking")
    Container(cred, "credential-broker-service", "Go", "Secret metadata + Vault mediation")
  }

  ContainerDb(pg, "PostgreSQL (per-service DBs)", "17 logical databases")
  Container_Ext(vault_c, "HashiCorp Vault", "")
  Container_Ext(bus_c, "NATS JetStream", "")
  Container_Ext(devagent_c, "Dev Server Agent", "")
  System_Ext(scm_ext, "GitHub/GitLab/Jira/Linear/…", "")

  Rel(frontend, gw, "HTTPS/WSS", "")
  Rel(gw, auth, "gRPC", "")
  Rel(gw, tenant, "gRPC", "")
  Rel(gw, proj, "gRPC", "")
  Rel(gw, infra, "gRPC + WS proxy for terminal streams", "")
  Rel(gw, git, "gRPC", "")
  Rel(gw, scm, "gRPC", "")
  Rel(gw, issue, "gRPC", "")
  Rel(gw, aiprov, "gRPC", "")
  Rel(gw, wf, "gRPC", "")
  Rel(gw, task, "gRPC", "")
  Rel(gw, orch, "gRPC", "")
  Rel(gw, auto, "gRPC", "")
  Rel(gw, annot, "gRPC", "")
  Rel(gw, notif, "gRPC + WS push", "")
  Rel(gw, usage, "gRPC", "")

  Rel(auth, pg, "", "")
  Rel(tenant, pg, "", "")
  Rel(proj, pg, "", "")
  Rel(infra, pg, "", "")
  Rel(aiprov, pg, "", "")
  Rel(wf, pg, "", "")
  Rel(task, pg, "", "")
  Rel(orch, pg, "", "")
  Rel(auto, pg, "", "")
  Rel(annot, pg, "", "")
  Rel(notif, pg, "", "")
  Rel(usage, pg, "", "")
  Rel(cred, pg, "Metadata only", "")

  Rel(cred, vault_c, "All secret reads/writes", "")
  Rel(aiprov, cred, "", "")
  Rel(scm, cred, "", "")
  Rel(issue, cred, "", "")
  Rel(infra, cred, "", "")
  Rel(auth, vault_c, "Dynamic DB creds for itself", "")

  Rel(git, infra, "Resolve connectionId", "")
  Rel(git, devagent_c, "Relay when connected", "")
  Rel(infra, devagent_c, "3-mode relay + fleet health", "")
  Rel(scm, scm_ext, "Direct OAuth API", "")
  Rel(issue, scm_ext, "Direct OAuth API", "")

  Rel(task, bus_c, "Publish task.* events", "")
  Rel(wf, bus_c, "Publish workflow.* events", "")
  Rel(auto, bus_c, "Publish automation.* events", "")
  Rel(notif, bus_c, "Consume, fan out", "")
```

## C3 — Component

Component-level breakdowns live in each service's own doc under
[`services/`](../services/) — every service doc includes a Clean
Architecture package diagram (domain/usecase/adapter layers) per the
convention in
[`03-clean-architecture-guidelines.md`](./03-clean-architecture-guidelines.md).
This document doesn't duplicate 17 component diagrams; it stops at the
container level, matching how `backend-hld-c4.md` scoped its own C3 section
to the two richest containers rather than every one.
