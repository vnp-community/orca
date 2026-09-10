package usecase

import (
	"context"
<<<<<<< HEAD
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
		case string(domain.DevServerHealthHealthy):
			result.Success++
		case string(domain.DevServerHealthDegraded):
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
		return ProvisionOutcome{Host: target.Host, Status: string(domain.DevServerHealthUnhealthy), Error: err.Error()}
	}
	if !found {
		devServer, err = domain.NewDevServer(uuid.NewString(), tenantID, target.Host, domain.ConnectionModeRelaySSH, target.ID, nil)
		if err == nil {
			devServer, err = uc.devServers.Register(ctx, devServer)
		}
		if err != nil {
			return ProvisionOutcome{Host: target.Host, Status: string(domain.DevServerHealthUnhealthy), Error: err.Error()}
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
			_ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerHealthDegraded, info, time.Now())
			return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(domain.DevServerHealthDegraded), Error: "remote host does not meet minimum prerequisites"}
		}
		_ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerHealthHealthy, info, time.Now())
		return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(domain.DevServerHealthHealthy)}
	}

	_ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerHealthUnhealthy, HandshakeInfo{}, time.Now())
	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(domain.DevServerHealthUnhealthy), Error: errMsg}
}

func filterByProject(targets []domain.SshTarget, project string) []domain.SshTarget {
	out := make([]domain.SshTarget, 0, len(targets))
	for _, t := range targets {
		if t.Project == project {
			out = append(out, t)
		}
	}
	return out
=======
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// FleetSpec's Servers field uses domain.FleetSpecServer — moved there by
// TASK-BE-FLEET-010 (BE-FLEET-SOL-003 §2) so domain.FleetDefinition can
// reference the same type without domain importing usecase. Originally
// defined here (CR-FLEET-001, TASK-BE-FLEET-001).
type FleetSpec struct {
	Version string
	Servers []domain.FleetSpecServer
}

type BulkProvisionServerResult struct {
	Host        string
	Status      string // "SUCCEEDED" | "FAILED"
	DevServerID string
	Error       string
}

type BulkProvisionResult struct {
	Results []BulkProvisionServerResult
}

// BulkProvisionFleet fans out CreateSshTarget+RegisterDevServer over N
// servers in a FleetSpec, bounded by a semaphore, with a per-server
// compensating rollback (DeleteSshTarget) when RegisterDevServer fails
// after CreateSshTarget already committed — see CR-FLEET-001's "Rollback
// semantics". Reuses CreateSshTarget/RegisterDevServer as-is; does not
// modify either.
type BulkProvisionFleet struct {
	createSshTarget    *CreateSshTarget
	registerDevServer  *RegisterDevServer
	deleteSshTarget    *DeleteSshTarget
	concurrencyDefault int
}

func NewBulkProvisionFleet(createSshTarget *CreateSshTarget, registerDevServer *RegisterDevServer, deleteSshTarget *DeleteSshTarget) *BulkProvisionFleet {
	return &BulkProvisionFleet{
		createSshTarget:    createSshTarget,
		registerDevServer:  registerDevServer,
		deleteSshTarget:    deleteSshTarget,
		concurrencyDefault: 5,
	}
}

func (uc *BulkProvisionFleet) Execute(ctx context.Context, spec FleetSpec, concurrency int, emit func(BulkProvisionServerResult)) (BulkProvisionResult, error) {
	if concurrency <= 0 {
		concurrency = uc.concurrencyDefault
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	result := BulkProvisionResult{Results: make([]BulkProvisionServerResult, 0, len(spec.Servers))}

	for _, server := range spec.Servers {
		server := server
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r := uc.provisionOne(ctx, server)
			mu.Lock()
			result.Results = append(result.Results, r)
			mu.Unlock()
			if emit != nil {
				emit(r)
			}
		}()
	}
	wg.Wait()
	return result, nil
}

// isUniqueViolation reports whether err is (or wraps) Postgres'
// unique_violation (23505) — added alongside migration 0017's
// (tenant_id, host) constraint (TASK-BE-FLEET-004) so re-running the same
// FleetSpec treats an already-registered host as idempotent success, not a
// hard failure. errors.As reaches through apperrors.AppError's Unwrap()
// (common/apperrors) to the underlying *pgconn.PgError CreateSshTarget's
// Postgres adapter returns.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (uc *BulkProvisionFleet) provisionOne(ctx context.Context, server domain.FleetSpecServer) BulkProvisionServerResult {
	if server.Host == "" {
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: "host is required"}
	}

	sshTarget, err := uc.createSshTarget.Execute(ctx, CreateSshTargetInput{
		Host:         server.Host,
		UserName:     server.UserName,
		VaultSSHRole: server.VaultSSHRole,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return BulkProvisionServerResult{Host: server.Host, Status: "SUCCEEDED", Error: "already_exists"}
		}
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: err.Error()}
	}

	devServer, err := uc.registerDevServer.Execute(ctx, RegisterDevServerInput{
		Host:        server.Host,
		Mode:        domain.ConnectionModeRelaySSH,
		SSHTargetID: sshTarget.ID,
		Kind:        server.Kind,
	})
	if err != nil {
		if delErr := uc.deleteSshTarget.Execute(ctx, sshTarget.ID); delErr != nil {
			return BulkProvisionServerResult{
				Host: server.Host, Status: "FAILED",
				Error: fmt.Sprintf("register failed: %v; cleanup also failed: %v", err, delErr),
			}
		}
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: err.Error()}
	}

	return BulkProvisionServerResult{Host: server.Host, Status: "SUCCEEDED", DevServerID: devServer.ID}
>>>>>>> feat/team-rbac-implementation
}
