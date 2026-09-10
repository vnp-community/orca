# SOL-003: Fix BUG-003 — copy the `orca-authz` Rego bundle into every embedded-OPA service's container image, fail fast if it's missing

**Resolves:** BUG-003
**Service:** `project-service`, `auth-service`, `task-service`, `annotation-service` (all 4 confirmed affected — same `common/policy.Evaluator` consumer, same `Dockerfile` template)
**Affected files:** `services/{project,auth,task,annotation}-service/deploy/Dockerfile` (4 files, identical change), `common/policy/evaluator.go`, each service's `cmd/server/main.go` (readiness wiring)
**Priority:** High — systemic, blocks every OPA-gated RPC in 4 services
**Status:** 🟡 Proposed — not yet implemented

---

## Grounding in `specs/backend-go/tdd/`

- `architecture/07-security-architecture.md`'s AuthZ section: *"Each
  service calls OPA (embedded, in-process — no network hop, policies
  loaded at startup and hot-reloaded on bundle update)"* — the design is
  explicit that the bundle is a build/deploy artifact each service carries
  with it, not something fetched over the network at runtime (which would
  have a different, more forgiving failure mode — a retryable connection
  error, not "file not found"). Packaging it into the image is the only
  way to satisfy this requirement; `common/policy/evaluator.go`'s own doc
  comment says as much ("no sidecar, no network hop").
- `architecture/09-observability-reliability.md`'s Health checks section:
  *"Every service exposes `/healthz` (liveness)... `/readyz` (readiness —
  can serve traffic: DB pool healthy, Vault lease valid, NATS connection
  established)... `readyz` failing pulls a pod out of the Service's
  endpoint list without restarting it."* A service that can't evaluate
  authorization decisions cannot safely serve traffic — the same category
  of precondition as a healthy DB pool. BUG-003 exists in the form it does
  (a normal-looking `200`-shaped RPC error, discovered only by actually
  calling an authorization-gated method) specifically because bundle
  loading happens **lazily**, on first `Decision()` call
  (`evaluator.go`'s own doc comment: *"prepared query is cached... The
  bundle is compiled once per distinct query string"* — i.e., not at
  startup). Under the target design's `/readyz` contract, this should have
  failed the readiness probe at boot, keeping a misconfigured replica out
  of rotation entirely, not served requests that fail one RPC at a time.

## Design

### 1. Package the bundle into every affected image

```dockerfile
# services/project-service/deploy/Dockerfile (and the identical 3 siblings)
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.work go.work.sum* ./
COPY common ./common
COPY proto ./proto
COPY policy ./policy                      # NEW — the orca-authz bundle lives at repo-root policy/
COPY services/project-service ./services/project-service
WORKDIR /src/services/project-service
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /out/project-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/project-service /project-service
COPY --from=build /src/policy/orca-authz /policy/orca-authz   # NEW
COPY services/project-service/migrations /migrations
USER nonroot:nonroot
ENTRYPOINT ["/project-service"]
```

Copying from the `build` stage (not the build context directly into the
final stage) avoids a second `COPY policy ./policy` reading off disk twice
and keeps the multi-stage pattern consistent with how the binary itself is
carried over.

### 2. Set an absolute, container-correct default — don't rely on the relative path lining up

`internal/config/config.go:41`'s default,
`commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz")`,
should become an absolute path matching where the Dockerfile now places
it — `/policy/orca-authz` — so the **default** is correct in the
container, and the relative path becomes something a local-dev override
opts into explicitly (e.g. `docker-compose.yml`'s dev environment, or a
direct `go run` from the service directory, can still set
`OPA_BUNDLE_PATH=../../policy/orca-authz` via its own env, rather than the
compiled-in default silently being wrong for the deployed case — matching
`architecture/10-deployment-infrastructure.md`'s "Local development" vs.
Kubernetes-topology distinction: the *default* should target where the
binary actually runs in the target environment, with local dev overriding
via its own compose/env file, not the other way around).

### 3. Fail fast at startup, matching the `/readyz` contract

`common/policy.Evaluator` currently compiles lazily on first `Decision()`
call. Add an explicit `Evaluator.Warm(ctx, queries ...string) error` (or
equivalent) that each service's `main.go` calls once at startup — right
after `policy.NewEvaluator(cfg.OPABundlePath)` — for every query name that
service actually uses (one, in `project-service`'s case:
`"data.orca.authz.project.allow"`). A failure here should prevent the
service from reporting `/readyz` healthy at all — consistent with how a
failed DB-pool ping already gates readiness per `09`'s doc.

```go
// common/policy/evaluator.go — sketch addition
// Warm eagerly compiles every named query — call once at service startup
// so a missing/unreadable bundle fails the service's own readiness check
// instead of surfacing as a per-request PROJECT_POLICY_EVAL_FAILED-style
// error the first time a real caller happens to hit an OPA-gated RPC.
func (e *Evaluator) Warm(ctx context.Context, queries ...string) error {
	for _, q := range queries {
		if _, err := e.preparedQuery(ctx, q); err != nil {
			return fmt.Errorf("policy: warming query %q: %w", q, err)
		}
	}
	return nil
}
```

```go
// project-service/cmd/server/main.go — sketch
evaluator := policy.NewEvaluator(cfg.OPABundlePath)
if err := evaluator.Warm(ctx, "data.orca.authz.project.allow"); err != nil {
	log.Fatalf("project-service: OPA bundle failed to load at startup (bundle path %q): %v", cfg.OPABundlePath, err)
}
```

`log.Fatalf` (or wiring the failure into whatever this service's real
`/readyz` handler checks, if a graceful-not-crashing readiness-gate
pattern is preferred over a hard boot failure — the doc's own framing
("pulls a pod out of the endpoint list without restarting it") suggests
readiness-gating over crashing, so prefer wiring this into `/readyz`'s
check function over a hard `Fatalf` if `main.go`'s existing structure
supports it cleanly).

## Testing Plan

- **Docker-level regression test** (the actual bug is a packaging gap, not
  a Go bug — a Go unit test alone can't catch a missing `COPY`): a CI step
  that builds each of the 4 images and runs
  `docker run --rm <image> ls /policy/orca-authz` (or an equivalent
  container-inspection check) asserting the bundle directory is non-empty.
  This is the single highest-value test to add — it directly prevents this
  exact regression class, including for any future 5th service that adopts
  embedded OPA and copies the Dockerfile template without the bundle line.
- Unit test: `Evaluator.Warm` with a bundle path pointing at a directory
  with no `.rego` files → returns a non-nil error (proves the fail-fast
  path actually fails, not silently no-ops).
- Integration test per affected service: construct the real `Evaluator`
  against the real `policy/orca-authz` bundle checked into the repo, call
  `Decision` with a known-allow and a known-deny input, assert both resolve
  correctly — this is the "does the bundle actually evaluate" smoke test
  that would have caught BUG-003 in CI before it ever reached
  `172.20.2.39`.
- Re-run `tests/client/rpc-catalog.spec.ts`'s `repo.list`/`worktree.list`
  cases (with a valid `projectId`) against the fixed deployment — should
  move from `PROJECT_POLICY_EVAL_FAILED` to a clean allow/deny decision.
