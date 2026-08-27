# Deployment & Infrastructure

## Environments

Standard three-tier promotion, each a separate Kubernetes namespace (or
cluster, for `production` vs. the rest, depending on isolation
requirements the operating org decides at implementation time):

| Environment | Purpose | Data |
|-------------|---------|------|
| `dev` | Fast-iteration, ephemeral, can be torn down/rebuilt at will | Synthetic/seeded data only |
| `staging` | Pre-production validation, production-like topology (real Vault, real Postgres instances, not `testcontainers`) | Anonymized snapshot or synthetic data at production scale |
| `production` | Live | Real tenant data |

## Kubernetes topology

- One Helm chart per service (`services/<name>/deploy/`), sharing a common
  library chart (`orca-go-common-chart`) for the boilerplate every service
  needs identically: Deployment, Service, ServiceAccount (for Vault K8s
  auth), HorizontalPodAutoscaler, PodDisruptionBudget, NetworkPolicy,
  ServiceMonitor (Prometheus scrape config). An umbrella chart composes all
  17 for a full-system deploy; individual services can also be deployed/
  upgraded independently (the whole point of microservices — a
  `workflow-service` fix shouldn't require redeploying `auth-service`).
- Each service: minimum 3 replicas in `production` (survives a single-node
  failure without capacity loss), HPA scaling on CPU + custom metric
  (gRPC request rate) where load is bursty (`api-gateway`, `git-gateway-service`).
- `NetworkPolicy` default-deny, explicit allow per the dependency graph in
  [`02-microservices-decomposition.md`](./02-microservices-decomposition.md)
  — a service can only reach the services it's declared to depend on.
- Service mesh: **Linkerd** by default (lower operational complexity, mTLS
  and basic traffic policy out of the box, sufficient for 17 services).
  **Istio** is the documented alternative if the deploying organization
  already standardizes on it or needs its more advanced traffic-shaping
  (canary via weighted routing, richer authorization policy) — the choice
  is infrastructure-layer and doesn't affect any service's application
  code either way.

## CI/CD

```
PR opened
  → lint (golangci-lint) + govulncheck + buf lint/breaking (only for changed proto)
  → unit tests (per changed service module)
  → integration tests (testcontainers-go: Postgres, Vault dev server, NATS)
  → build + scan container image (Trivy)
  → merge to main (only changed services rebuild — module-scoped CI, not a full monorepo rebuild every time)

main branch, per service
  → build + push image (tagged with git SHA + semver if tagged)
  → deploy to `dev` automatically
  → promote to `staging` on manual approval or scheduled batch
  → promote to `production` via GitOps PR (ArgoCD watches a config repo; merging the version bump is the deploy)
```

- **GitOps (ArgoCD)** for `staging`/`production` — the deployed state is
  whatever's declared in the GitOps config repo, not whatever someone
  `kubectl apply`'d by hand. Matches "enterprise level" auditability
  (every production change is a reviewed PR with a diff).
- Each service deploys independently — the CI pipeline is scoped per Go
  module (per service), so a change to `annotation-service` doesn't trigger
  a rebuild/redeploy of the other 16.

## Database & Vault infrastructure

- PostgreSQL: managed service (RDS/Cloud SQL/self-hosted with Patroni for
  HA) per the decision made per-service in
  [`05-data-architecture.md`](./05-data-architecture.md). Automated
  backups, point-in-time recovery enabled for every service database,
  tested restore procedure (see
  [`09-observability-reliability.md`](./09-observability-reliability.md)).
- Vault: HA deployment (Raft integrated storage, 3+ node cluster), auto-unseal
  via a cloud KMS (avoids the operational risk of manual unseal-key
  quorum in an incident). This is the single highest-availability-requirement
  component in the system, per the reasoning in
  [`09-observability-reliability.md`](./09-observability-reliability.md).

## Local development

- `docker-compose.yml` at the repo root of this Go workspace, bringing up:
  all 17 services, a local Postgres instance (one container, 17 databases —
  local dev doesn't need physical instance separation, only
  staging/production do), a local Vault in `dev` mode (auto-unsealed, seeded
  with dev policies), a local NATS.
- `make dev-up` / `make dev-down` wrapping the compose lifecycle;
  `skaffold` (or an equivalent) for developers who prefer iterating directly
  against a local/dev Kubernetes cluster instead of compose.

## Multi-region / DR (enterprise requirement, phased)

Not required for an initial production rollout, but the design shouldn't
preclude it:

- Stateless services (everything except the databases and Vault) scale
  horizontally across regions behind a global load balancer without
  architectural change — the Clean Architecture + gRPC design has no
  region-affinity built in.
- Cross-region database replication (read replicas at minimum, active-active
  only if a specific service's write latency requirements justify the
  added complexity — e.g. not needed for `usage-service`, worth evaluating
  for `auth-service` if user base spans regions with a meaningful latency
  gap).
- Vault supports multi-region via Performance Replication (Enterprise
  feature) — flagged here as a licensing/cost decision for whoever operates
  this system, not resolved by this document.
