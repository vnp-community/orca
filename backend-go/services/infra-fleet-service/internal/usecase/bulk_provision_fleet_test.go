package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// bulkFakeSshTargetRepository is an in-memory SshTargetRepository scoped to
// this file's tests — separate from fakeSshTargetRepository
// (create_ssh_target_test.go) because BulkProvisionFleet's tests need
// concurrency-safe, per-host error injection that fake doesn't support.
type bulkFakeSshTargetRepository struct {
	mu         sync.Mutex
	created    []domain.SshTarget
	deletedIDs []string
	errForHost map[string]error
	deleteErr  error
}

func (f *bulkFakeSshTargetRepository) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errForHost[target.Host]; ok {
		return domain.SshTarget{}, err
	}
	f.created = append(f.created, target)
	return target, nil
}

func (f *bulkFakeSshTargetRepository) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	return domain.SshTarget{}, nil
}

func (f *bulkFakeSshTargetRepository) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	return nil, nil
}

func (f *bulkFakeSshTargetRepository) Delete(ctx context.Context, tenantID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedIDs = append(f.deletedIDs, id)
	return f.deleteErr
}

// bulkFakeDevServerRepository is an in-memory DevServerRepository scoped to
// this file's tests, same rationale as bulkFakeSshTargetRepository above.
type bulkFakeDevServerRepository struct {
	mu         sync.Mutex
	registered []domain.DevServer
	errForHost map[string]error

	// concurrency tracking for TestBulkProvisionFleet_RespectsConcurrencyLimit.
	inFlight    int32
	maxInFlight int32
	block       <-chan struct{} // if non-nil, Register waits on this before returning
}

func (f *bulkFakeDevServerRepository) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	cur := atomic.AddInt32(&f.inFlight, 1)
	defer atomic.AddInt32(&f.inFlight, -1)
	for {
		max := atomic.LoadInt32(&f.maxInFlight)
		if cur <= max || atomic.CompareAndSwapInt32(&f.maxInFlight, max, cur) {
			break
		}
	}
	if f.block != nil {
		<-f.block
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errForHost[ds.Host]; ok {
		return domain.DevServer{}, err
	}
	f.registered = append(f.registered, ds)
	return ds, nil
}

func (f *bulkFakeDevServerRepository) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *bulkFakeDevServerRepository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *bulkFakeDevServerRepository) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *bulkFakeDevServerRepository) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *bulkFakeDevServerRepository) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *bulkFakeDevServerRepository) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}

func newBulkProvisionFleetForTest(sshRepo *bulkFakeSshTargetRepository, devRepo *bulkFakeDevServerRepository) *BulkProvisionFleet {
	return NewBulkProvisionFleet(
		NewCreateSshTarget(sshRepo),
		NewRegisterDevServer(devRepo),
		NewDeleteSshTarget(sshRepo),
	)
}

func TestBulkProvisionFleet_AllServersSucceed(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newBulkProvisionFleetForTest(sshRepo, devRepo)

	spec := FleetSpec{Servers: []domain.FleetSpecServer{
		{Host: "h1", UserName: "orca", VaultSSHRole: "role"},
		{Host: "h2", UserName: "orca", VaultSSHRole: "role"},
		{Host: "h3", UserName: "orca", VaultSSHRole: "role"},
	}}

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, spec, 5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}
	for _, r := range result.Results {
		if r.Status != "SUCCEEDED" {
			t.Errorf("host %q: expected SUCCEEDED, got %q (err=%q)", r.Host, r.Status, r.Error)
		}
		if r.DevServerID == "" {
			t.Errorf("host %q: expected a non-empty DevServerID", r.Host)
		}
	}
}

func TestBulkProvisionFleet_OneServerEmptyHost_OthersSucceed(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newBulkProvisionFleetForTest(sshRepo, devRepo)

	spec := FleetSpec{Servers: []domain.FleetSpecServer{
		{Host: "", UserName: "orca", VaultSSHRole: "role"},
		{Host: "h2", UserName: "orca", VaultSSHRole: "role"},
	}}

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, spec, 5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var succeeded, failed int
	for _, r := range result.Results {
		switch r.Status {
		case "SUCCEEDED":
			succeeded++
		case "FAILED":
			failed++
			if r.Host != "" {
				t.Errorf("expected the empty-host server to be the failure, got failure for %q", r.Host)
			}
		}
	}
	if succeeded != 1 || failed != 1 {
		t.Errorf("expected 1 succeeded + 1 failed, got %d succeeded, %d failed", succeeded, failed)
	}
}

func TestBulkProvisionFleet_RegisterFails_CompensatingDeleteCalled(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{errForHost: map[string]error{"h1": errors.New("register failed")}}
	uc := newBulkProvisionFleetForTest(sshRepo, devRepo)

	spec := FleetSpec{Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}}
	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, spec, 5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Status != "FAILED" {
		t.Fatalf("expected 1 FAILED result, got %+v", result.Results)
	}
	if len(sshRepo.deletedIDs) != 1 {
		t.Fatalf("expected DeleteSshTarget.Execute called exactly once, got %d calls", len(sshRepo.deletedIDs))
	}
	if len(sshRepo.created) != 1 || sshRepo.deletedIDs[0] != sshRepo.created[0].ID {
		t.Errorf("expected the deleted id to match the created ssh target's id")
	}
}

func TestBulkProvisionFleet_RegisterFails_CompensatingDeleteAlsoFails(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepository{deleteErr: errors.New("delete also failed")}
	devRepo := &bulkFakeDevServerRepository{errForHost: map[string]error{"h1": errors.New("register failed")}}
	uc := newBulkProvisionFleetForTest(sshRepo, devRepo)

	spec := FleetSpec{Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}}
	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, spec, 5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	got := result.Results[0]
	if got.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %q", got.Status)
	}
	if !strings.Contains(got.Error, "register failed") || !strings.Contains(got.Error, "cleanup also failed") {
		t.Errorf("expected error to mention both failures, got %q", got.Error)
	}
}

func TestBulkProvisionFleet_RespectsConcurrencyLimit(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepository{}
	block := make(chan struct{})
	devRepo := &bulkFakeDevServerRepository{block: block}
	uc := newBulkProvisionFleetForTest(sshRepo, devRepo)

	var servers []domain.FleetSpecServer
	for i := 0; i < 10; i++ {
		servers = append(servers, domain.FleetSpecServer{Host: fmt.Sprintf("h%d", i), UserName: "orca", VaultSSHRole: "role"})
	}
	spec := FleetSpec{Servers: servers}

	const concurrency = 3
	done := make(chan BulkProvisionResult, 1)
	ctx := withTenant(context.Background(), "tenant-1")
	go func() {
		result, _ := uc.Execute(ctx, spec, concurrency, nil)
		done <- result
	}()

	// Wait until exactly `concurrency` goroutines are blocked inside
	// Register (the semaphore should let no more than that run at once),
	// then release them all — this is what actually exercises the limit
	// rather than just hoping timing works out.
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(&devRepo.inFlight) < concurrency && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(block)

	result := <-done
	if len(result.Results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(result.Results))
	}
	if devRepo.maxInFlight > concurrency {
		t.Errorf("expected at most %d concurrent Register calls, observed %d", concurrency, devRepo.maxInFlight)
	}
}

func TestBulkProvisionFleet_ZeroConcurrency_UsesDefault5(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newBulkProvisionFleetForTest(sshRepo, devRepo)

	spec := FleetSpec{Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}}
	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, spec, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error (zero concurrency must not panic on channel size 0): %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Status != "SUCCEEDED" {
		t.Fatalf("expected 1 SUCCEEDED result, got %+v", result.Results)
	}
	if uc.concurrencyDefault != 5 {
		t.Errorf("expected default concurrency 5, got %d", uc.concurrencyDefault)
	}
}

// TestBulkProvisionFleet_DuplicateHost_ReturnsAlreadyExistsNotFailed covers
// TASK-BE-FLEET-004's migration-0017 idempotency constraint interaction:
// CreateSshTarget failing with a unique_violation (23505) — the DB error
// migration 0017's (tenant_id, host) constraint produces on a re-run of the
// same FleetSpec — must surface as SUCCEEDED/"already_exists", not FAILED.
func TestBulkProvisionFleet_DuplicateHost_ReturnsAlreadyExistsNotFailed(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepository{
		errForHost: map[string]error{"h1": &pgconn.PgError{Code: "23505", Message: "duplicate key"}},
	}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newBulkProvisionFleetForTest(sshRepo, devRepo)

	spec := FleetSpec{Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}}
	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, spec, 5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	got := result.Results[0]
	if got.Status != "SUCCEEDED" || got.Error != "already_exists" {
		t.Errorf("expected {Status: SUCCEEDED, Error: already_exists}, got %+v", got)
	}
	if len(devRepo.registered) != 0 {
		t.Errorf("expected RegisterDevServer to never be called for an already-existing ssh target, got %d calls", len(devRepo.registered))
	}
}

// TestIsUniqueViolation_WrapsThroughAppErrors confirms errors.As reaches
// through apperrors.AppError's Unwrap() to find the underlying
// *pgconn.PgError — the exact wrapping CreateSshTarget.Execute applies.
func TestIsUniqueViolation_WrapsThroughAppErrors(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate key"}
	wrapped := apperrors.New(apperrors.KindInternal, "INFRA_CREATE_SSH_TARGET_FAILED", "failed to create ssh target", pgErr)
	if !isUniqueViolation(wrapped) {
		t.Error("expected isUniqueViolation to see through apperrors.AppError's wrapping")
	}
	if isUniqueViolation(errors.New("some other db error")) {
		t.Error("expected isUniqueViolation to return false for an unrelated error")
	}
}
