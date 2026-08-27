# TASK-FLEET-02-05: `usecase.BulkProvisionFleet` (fan-out, retry, status write)

**From Solution:** SOL-FLEET-02
**Priority:** P1
**Service:** `infra-fleet-service` (usecase)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go` (new)
**Depends on:** TASK-FLEET-02-01, TASK-FLEET-02-03, TASK-FLEET-02-04
**Status:** `[ ]` TODO

---

## Context

Fans out `Provisioner.Provision` across a tenant's (optionally
project-filtered) SSH targets with bounded concurrency, 3x retry on deploy
failure, and a per-server status write — the coordination logic
`infra-fleet-service.md` §3's `BootstrapFleetTarget` sketch anticipated, now
shaped as a unary batch RPC instead of a per-target progress stream (see
SOL-FLEET-02's "Why unary, not streaming" section).

## Changes to make

```go
// internal/usecase/bulk_provision_fleet.go
type BulkProvisionFleetInput struct {
    Project     string // "" = all of tenant's relay-ssh dev servers
    Concurrency int    // default 5
}

type ProvisionOutcome struct {
    DevServerID, Host, Status string
    Error                     string // "" on success
}

type BulkProvisionFleetResult struct {
    Success, Failed, Skipped int
    Outcomes                 []ProvisionOutcome
}

type Provisioner interface {
    Provision(ctx context.Context, devServer domain.DevServer) (domain.Connection, devserveragent.HandshakeInfo, error)
}

type BulkProvisionFleet struct {
    sshTargets  SshTargetRepository
    devServers  DevServerRepository
    provisioner Provisioner
}

func NewBulkProvisionFleet(sshTargets SshTargetRepository, devServers DevServerRepository, provisioner Provisioner) *BulkProvisionFleet {
    return &BulkProvisionFleet{sshTargets: sshTargets, devServers: devServers, provisioner: provisioner}
}

func (uc *BulkProvisionFleet) Execute(ctx context.Context, in BulkProvisionFleetInput) (BulkProvisionFleetResult, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }
    targets, err := uc.sshTargets.List(ctx, tenantID)
    if err != nil {
        return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindInternal, "INFRA_FLEET_LIST_FAILED", "failed to list ssh targets", err)
    }
    if in.Project != "" {
        targets = filterByProject(targets, in.Project)
    }
    concurrency := in.Concurrency
    if concurrency <= 0 {
        concurrency = 5
    }

    sem := make(chan struct{}, concurrency)
    var wg sync.WaitGroup
    outcomes := make([]ProvisionOutcome, len(targets))
    for i, target := range targets {
        wg.Add(1)
        sem <- struct{}{}
        go func(i int, target domain.SshTarget) {
            defer wg.Done()
            defer func() { <-sem }()
            outcomes[i] = uc.bulkProvisionOne(ctx, tenantID, target)
        }(i, target)
    }
    wg.Wait()

    var result BulkProvisionFleetResult
    for _, o := range outcomes {
        switch o.Status {
        case string(domain.DevServerStatusHealthy):
            result.Success++
        case string(domain.DevServerStatusDegraded):
            result.Skipped++
        default:
            result.Failed++
        }
    }
    result.Outcomes = outcomes
    return result, nil
}

func (uc *BulkProvisionFleet) bulkProvisionOne(ctx context.Context, tenantID string, target domain.SshTarget) ProvisionOutcome {
    devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, target.ID)
    if !found {
        devServer, _ = domain.NewDevServer(uuid.NewString(), tenantID, target.Host, domain.ConnectionModeRelaySSH, target.ID)
        devServer, err = uc.devServers.Register(ctx, devServer)
    }
    if err != nil {
        return ProvisionOutcome{Host: target.Host, Status: "unhealthy", Error: err.Error()}
    }

    var lastErr error
    for attempt := 1; attempt <= 3; attempt++ { // retry deploy failures 3x; a prereq shortfall (ErrPrerequisitesNotMet) does not consume an attempt
        var info devserveragent.HandshakeInfo
        _, info, lastErr = uc.provisioner.Provision(ctx, devServer)
        if errors.Is(lastErr, sshrelay.ErrPrerequisitesNotMet) {
            _ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusDegraded, info, time.Now())
            return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(domain.DevServerStatusDegraded), Error: lastErr.Error()}
        }
        if lastErr == nil {
            status := domain.DevServerStatusHealthy
            _ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, status, info, time.Now())
            return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(status)}
        }
    }
    _ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusUnhealthy, devserveragent.HandshakeInfo{}, time.Now())
    return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: "unhealthy", Error: lastErr.Error()}
}
```

Idempotent re-runs fall out naturally: a server already `healthy` is
re-provisioned harmlessly since `deploy.go`'s checksum verify is itself
idempotent.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestBulkProvisionFleet -v
```

Expected (fake `SshTargetRepository`/`DevServerRepository`/`Provisioner`): 5
targets, concurrency=2 → no more than 2 concurrent `Provisioner.Provision`
calls in flight; one target's `Provisioner.Provision` always errors → after
3 attempts, `Failed++`, `UpdateProvisionResult` called with `unhealthy`; one
target's prereq check fails but deploy succeeds → `Skipped++` with
`degraded` status and no retry consumed; `--project` filter excludes
non-matching targets. Also: run `BulkProvisionFleet` twice against the same
fixture set → second run's `Success` count matches the first.
