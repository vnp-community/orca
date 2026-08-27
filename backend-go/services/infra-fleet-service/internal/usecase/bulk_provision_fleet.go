package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// BulkProvisionFleetInput is BL-FLEET-02's fan-out batch-provision request.
type BulkProvisionFleetInput struct {
	Project     string // "" = all of tenant's relay-ssh dev servers
	Concurrency int    // default 5
}

// ProvisionOutcome is one server's result within a BulkProvisionFleet run.
type ProvisionOutcome struct {
	DevServerID, Host, Status string
	Error                     string // "" on success
}

// BulkProvisionFleetResult is BulkProvisionFleet's batch summary.
type BulkProvisionFleetResult struct {
	Success, Failed, Skipped int
	Outcomes                 []ProvisionOutcome
}

// BulkProvisionFleet fans out Provisioner.Provision across a tenant's
// (optionally project-filtered) SSH targets with bounded concurrency, 3x
// retry on deploy failure, and a per-server status write — the
// coordination logic infra-fleet-service.md §3's BootstrapFleetTarget
// sketch anticipated, shaped as a unary batch RPC (see SOL-FLEET-02's "Why
// unary, not streaming" section). Idempotent re-runs fall out naturally: a
// server already healthy is re-provisioned harmlessly since deploy.go's
// checksum verify is itself idempotent.
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

// bulkProvisionOne find-or-creates devServer for target, then attempts
// Provision up to 3 times. A prereq shortfall (Provision's prereqsMet=false
// on an otherwise-successful call) does not consume a retry attempt — it
// returns immediately as Degraded, matching "deploy succeeded, prereqs
// marginal" rather than "deploy failed".
func (uc *BulkProvisionFleet) bulkProvisionOne(ctx context.Context, tenantID string, target domain.SshTarget) ProvisionOutcome {
	devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, target.ID)
	if err != nil {
		return ProvisionOutcome{Host: target.Host, Status: string(domain.DevServerStatusUnhealthy), Error: err.Error()}
	}
	if !found {
		devServer, err = domain.NewDevServer(uuid.NewString(), tenantID, target.Host, domain.ConnectionModeRelaySSH, target.ID)
		if err == nil {
			devServer, err = uc.devServers.Register(ctx, devServer)
		}
		if err != nil {
			return ProvisionOutcome{Host: target.Host, Status: string(domain.DevServerStatusUnhealthy), Error: err.Error()}
		}
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		info, prereqsMet, provisionErr := uc.provisioner.Provision(ctx, devServer)
		if provisionErr != nil {
			lastErr = provisionErr
			continue // retry deploy/handshake failures up to 3x
		}
		if !prereqsMet {
			_ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusDegraded, info, time.Now())
			return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(domain.DevServerStatusDegraded), Error: "remote host does not meet minimum prerequisites"}
		}
		_ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusHealthy, info, time.Now())
		return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(domain.DevServerStatusHealthy)}
	}

	_ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusUnhealthy, HandshakeInfo{}, time.Now())
	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(domain.DevServerStatusUnhealthy), Error: errMsg}
}

func filterByProject(targets []domain.SshTarget, project string) []domain.SshTarget {
	out := make([]domain.SshTarget, 0, len(targets))
	for _, t := range targets {
		if t.Project == project {
			out = append(out, t)
		}
	}
	return out
}
