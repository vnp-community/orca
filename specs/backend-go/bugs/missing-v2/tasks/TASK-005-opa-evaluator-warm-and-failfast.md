# TASK-005: Add `Evaluator.Warm`, call it at startup in all 4 embedded-OPA services, fail fast if the bundle doesn't load

**From Solution:** SOL-003
**Priority:** P0 — systemic, blocks every OPA-gated RPC in 4 services
**Service:** `common/policy` (shared), `auth-service`, `task-service`, `annotation-service`, `project-service`
**File:** `common/policy/evaluator.go`, each service's `cmd/server/main.go`
**Depends on:** none (independent of TASK-006; land together for one coherent PR per SOL-003's own recommendation)
**Status:** `[ ]` TODO

---

## Context

`Evaluator.Decision` compiles the bundle lazily, on first call per distinct
query — a missing/broken bundle currently surfaces only the first time a
real caller hits an OPA-gated RPC, as an opaque
`PROJECT_POLICY_EVAL_FAILED`-shaped error (BUG-003). Add an explicit
eager-compile step every service calls once at boot, so a broken bundle
fails the service's startup, not a random later request.

## Changes to make

### Step 1 — `common/policy/evaluator.go`: add `Warm`

```go
// Warm eagerly compiles every named query — call once at service startup,
// right after NewEvaluator, so a missing/unreadable/invalid bundle fails
// loudly at boot instead of surfacing as an opaque per-request evaluation
// error the first time a real caller happens to hit an OPA-gated RPC (see
// specs/backend-go/bugs/missing-v2/BUG-003).
func (e *Evaluator) Warm(ctx context.Context, queries ...string) error {
	for _, q := range queries {
		if _, err := e.preparedQuery(ctx, q); err != nil {
			return fmt.Errorf("policy: warming query %q: %w", q, err)
		}
	}
	return nil
}
```

No new imports needed — `context`/`fmt` are already imported.

### Step 2 — wire `Warm` into each service's `main.go`, right after `NewEvaluator`

Each service already constructs its evaluator at a known line; each has
exactly one `decisionQuery` constant in its own `internal/adapter/opaclient/client.go`.

**`auth-service/cmd/server/main.go`** (near line 104):
```go
opaEvaluator := policy.NewEvaluator(cfg.OPABundlePath)
if err := opaEvaluator.Warm(ctx, "data.orca.authz.admin.allow"); err != nil {
	return fmt.Errorf("auth-service: OPA bundle failed to load at startup (bundle path %q): %w", cfg.OPABundlePath, err)
}
```

**`task-service/cmd/server/main.go`** (near line 110):
```go
opaEvaluator := policy.NewEvaluator(cfg.OPABundlePath)
if err := opaEvaluator.Warm(ctx, "data.orca.authz.task.allow"); err != nil {
	return fmt.Errorf("task-service: OPA bundle failed to load at startup (bundle path %q): %w", cfg.OPABundlePath, err)
}
```

**`annotation-service/cmd/server/main.go`** (near line 77):
```go
evaluator := policy.NewEvaluator(cfg.OPABundlePath)
if err := evaluator.Warm(ctx, "data.orca.authz.annotation.allow"); err != nil {
	return fmt.Errorf("annotation-service: OPA bundle failed to load at startup (bundle path %q): %w", cfg.OPABundlePath, err)
}
opa := opaclient.New(evaluator)
```

**`project-service/cmd/server/main.go`** (near line 115):
```go
evaluator := policy.NewEvaluator(cfg.OPABundlePath)
if err := evaluator.Warm(ctx, "data.orca.authz.project.allow"); err != nil {
	return fmt.Errorf("project-service: OPA bundle failed to load at startup (bundle path %q): %w", cfg.OPABundlePath, err)
}
opa := projectopaclient.New(evaluator)
```

Each `main.go`'s outer function already returns `error` on other startup
failures (dial errors, etc. — see the `return fmt.Errorf("dialing ...: %w", err)`
pattern already used throughout each file) — `return fmt.Errorf(...)` here
matches that existing convention and causes the process to exit non-zero,
which is the fail-fast behavior this task wants. **No `/readyz` handler
exists in any of these 4 services yet** (checked directly — this is a
separate, larger gap tracked by `09-observability-reliability.md`'s health
check requirement, out of this task's scope) — a hard startup failure is
the correct fallback until one exists; revisit wiring this into `/readyz`
instead of a hard exit once that endpoint is built.

## Verify

```bash
cd backend-go
go build ./common/policy/... ./services/auth-service/... ./services/task-service/... ./services/annotation-service/... ./services/project-service/...
go vet ./common/policy/... ./services/auth-service/... ./services/task-service/... ./services/annotation-service/... ./services/project-service/...
go test ./common/policy/... -count=1
```

Full per-service test runs happen in TASK-007 alongside TASK-006's
Dockerfile fix — `Warm` against the DEFAULT `OPABundlePath`
(`../../policy/orca-authz`, still relative at this point in the task
sequence) will actually succeed in local `go test`/`go run` from each
service's own directory (that's why BUG-003 wasn't caught by existing
tests — the relative path is only wrong inside the container). This task's
own verify step confirms the code compiles and `Warm` itself behaves
correctly in isolation; TASK-006 is what actually reproduces and fixes the
container-specific failure.
