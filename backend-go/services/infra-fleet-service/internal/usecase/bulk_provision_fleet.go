package usecase

import (
	"context"
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
}
