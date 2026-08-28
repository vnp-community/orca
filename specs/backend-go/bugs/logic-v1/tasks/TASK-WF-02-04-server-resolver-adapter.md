# TASK-WF-02-04: Implement `ServerResolver` adapter + wire into step executors

**From Solution:** SOL-WF-02
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/adapter/serverresolver/resolver.go` (new)
**Depends on:** TASK-WF-02-02, TASK-WF-02-03
**Status:** `[x]` DONE — new `internal/adapter/serverresolver` package implements all four `Target` prefixes + legacy-bare-string + empty, tenant forwarded via outbound gRPC metadata (matching `infrafleetclient.withTenantMetadata`'s convention); `server:`/`project:`/`fleet:tag:` resolution now fails loudly (not silent-local-execution) when `ResolveConnection` reports `Connected=false`, a deliberate strengthening beyond the literal pseudocode. Wired into all three `infrafleetclient` executors (`ServerResolver` injected, `cfg.EffectiveTarget()` resolved before `relay()`) and into `cmd/server/main.go` (new `PROJECT_SERVICE_ADDR` config + dial). `domain.*StepConfig.effectiveTarget` renamed to exported `EffectiveTarget` (required for cross-package use — see TASK-WF-02-02's updated note). New `resolver_test.go`: one test per prefix + not-connected + no-dev-server-bound + zero-healthy-servers + list-failure-propagates, plus a 20-goroutine concurrent round-robin test asserting both fake healthy servers get selected. `go build/vet/test -race` green for `serverresolver` (10/10); full-package `-race` surfaced a pre-existing, unrelated data race in `wave_dispatcher_test.go`'s `fakeStepExecutor` (confirmed via `git diff --stat` — file untouched by this task) at the time — since fixed in TASK-WF-02-06 (that fake gained a mutex); full-package `go test ./... -race` is now green.

---

## Context

BUG-WF-02 finds no server-resolution logic anywhere — every `agent`-type
step's `Target`/`ConnectionID` passes straight through to the relay call
unresolved. This adds the concrete `ServerResolver` implementation
covering all four `Target` prefixes and wires it into every
`StepExecutor`.

## Changes to make

Create `backend-go/services/workflow-service/internal/adapter/serverresolver/resolver.go`:

```go
package serverresolver

type resolver struct {
    projects projectv1.ProjectServiceClient
    infra    infrafleetv1.InfraFleetServiceClient
    // fleetTagCounters round-robins fleet:tag:<tag> targets — per-replica,
    // not globally coordinated; a perfectly even global distribution is
    // not a correctness requirement here.
    fleetTagCounters sync.Map // map[string]*atomic.Uint64
}

func New(projects projectv1.ProjectServiceClient, infra infrafleetv1.InfraFleetServiceClient) usecase.ServerResolver {
    return &resolver{projects: projects, infra: infra}
}

func (r *resolver) Resolve(ctx context.Context, tenantID, target string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // §8's intra-cluster default
    defer cancel()

    switch {
    case target == "":
        return "", nil // execute locally — unchanged default
    case strings.HasPrefix(target, "connection:"):
        return strings.TrimPrefix(target, "connection:"), nil
    case strings.HasPrefix(target, "server:"):
        devServerID := strings.TrimPrefix(target, "server:")
        resp, err := r.infra.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerID})
        if err != nil {
            return "", err
        }
        return resp.GetConnectionId(), nil
    case strings.HasPrefix(target, "project:"):
        projectID := strings.TrimPrefix(target, "project:")
        proj, err := r.projects.GetProject(ctx, &projectv1.GetProjectRequest{Id: projectID})
        if err != nil {
            return "", fmt.Errorf("serverresolver: resolve project %s: %w", projectID, err)
        }
        resp, err := r.infra.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: proj.GetProject().GetDevServerId()})
        if err != nil {
            return "", err
        }
        return resp.GetConnectionId(), nil
    case strings.HasPrefix(target, "fleet:tag:"):
        return r.resolveFleetTag(ctx, tenantID, strings.TrimPrefix(target, "fleet:tag:"))
    default:
        return target, nil // legacy bare connectionId
    }
}

func (r *resolver) resolveFleetTag(ctx context.Context, tenantID, tag string) (string, error) {
    resp, err := r.infra.ListDevServersByTag(ctx, &infrafleetv1.ListDevServersByTagRequest{Tag: tag, HealthyOnly: true})
    if err != nil {
        return "", err
    }
    servers := resp.GetDevServers()
    if len(servers) == 0 {
        return "", fmt.Errorf("serverresolver: no healthy dev server tagged %q", tag)
    }
    counter, _ := r.fleetTagCounters.LoadOrStore(tag, new(atomic.Uint64))
    chosen := servers[counter.(*atomic.Uint64).Add(1)%uint64(len(servers))]
    connResp, err := r.infra.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: chosen.GetId()})
    if err != nil {
        return "", err
    }
    return connResp.GetConnectionId(), nil
}
```

Wire it into every `StepExecutor` (`agent_step_executor.go`,
`shell_step_executor.go`, `notification_step_executor.go`): call
`serverResolver.Resolve(ctx, tenantID, cfg.effectiveTarget())` **before**
building the relay call's params, and use the resolved `connectionId` in
place of today's raw `cfg.ConnectionID`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/serverresolver/... -race
```

Expected: one test per `Target` prefix (`connection:`, `server:`,
`project:`, `fleet:tag:`, bare-legacy, empty) against fake
`ProjectServiceClient`/`InfraFleetServiceClient`; `fleet:tag:`
round-robins across ≥2 fake healthy servers over repeated calls (assert
both get selected); zero healthy servers for a tag returns a clear error,
never an empty-string connectionId.
