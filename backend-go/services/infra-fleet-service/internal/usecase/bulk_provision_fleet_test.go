package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeProvisioner is an in-memory usecase.Provisioner — tracks concurrent
// in-flight calls (for the concurrency-bound assertion) and lets each test
// script a per-host outcome.
type fakeProvisioner struct {
	mu sync.Mutex

	inFlight    int32
	maxInFlight int32

	// alwaysErrHosts always return an error (used for the retry-exhaustion
	// assertion). prereqFailHosts return prereqsMet=false but no error.
	alwaysErrHosts  map[string]bool
	prereqFailHosts map[string]bool

	calls int32
}

func (f *fakeProvisioner) Provision(ctx context.Context, devServer domain.DevServer) (HandshakeInfo, bool, error) {
	atomic.AddInt32(&f.calls, 1)
	n := atomic.AddInt32(&f.inFlight, 1)
	defer atomic.AddInt32(&f.inFlight, -1)
	for {
		max := atomic.LoadInt32(&f.maxInFlight)
		if n <= max || atomic.CompareAndSwapInt32(&f.maxInFlight, max, n) {
			break
		}
	}

	if f.alwaysErrHosts[devServer.Host] {
		return HandshakeInfo{}, false, errors.New("deploy failed")
	}
	if f.prereqFailHosts[devServer.Host] {
		return HandshakeInfo{Platform: "linux"}, false, nil
	}
	return HandshakeInfo{Platform: "linux", NodeVersion: "22.3.0"}, true, nil
}

func newBulkProvisionFixture(n int) (*fakeSshTargetRepository, *fakeDevServerRepository, *fakeProvisioner) {
	targets := make([]domain.SshTarget, 0, n)
	for i := 0; i < n; i++ {
		targets = append(targets, domain.SshTarget{
			ID: fmt.Sprintf("ssht-%d", i), TenantID: "t1", Host: fmt.Sprintf("host-%d.example.com", i), UserName: "deploy",
		})
	}
	sshRepo := &fakeSshTargetRepository{targets: map[string][]domain.SshTarget{"t1": targets}}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{}}
	return sshRepo, devRepo, &fakeProvisioner{}
}

func TestBulkProvisionFleet_ConcurrencyBound(t *testing.T) {
	sshRepo, devRepo, prov := newBulkProvisionFixture(5)
	uc := NewBulkProvisionFleet(sshRepo, devRepo, prov)

	ctx := withTestTenant(context.Background())
	result, err := uc.Execute(ctx, BulkProvisionFleetInput{Concurrency: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success != 5 {
		t.Errorf("expected 5 successes, got %+v", result)
	}
	if prov.maxInFlight > 2 {
		t.Errorf("expected at most 2 concurrent Provision calls, saw %d", prov.maxInFlight)
	}
}

func TestBulkProvisionFleet_RetriesThenFailsAfterThreeAttempts(t *testing.T) {
	sshRepo, devRepo, _ := newBulkProvisionFixture(1)
	prov := &fakeProvisioner{alwaysErrHosts: map[string]bool{"host-0.example.com": true}}
	uc := NewBulkProvisionFleet(sshRepo, devRepo, prov)

	ctx := withTestTenant(context.Background())
	result, err := uc.Execute(ctx, BulkProvisionFleetInput{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 || result.Success != 0 {
		t.Errorf("expected 1 failure, got %+v", result)
	}
	if prov.calls != 3 {
		t.Errorf("expected exactly 3 attempts, got %d", prov.calls)
	}
	if devRepo.updateProvisionResultCalls != 1 || devRepo.lastProvisionStatus != domain.DevServerStatusUnhealthy {
		t.Errorf("expected UpdateProvisionResult called once with unhealthy, got calls=%d status=%q", devRepo.updateProvisionResultCalls, devRepo.lastProvisionStatus)
	}
}

func TestBulkProvisionFleet_PrereqShortfallDoesNotConsumeARetry(t *testing.T) {
	sshRepo, devRepo, _ := newBulkProvisionFixture(1)
	prov := &fakeProvisioner{prereqFailHosts: map[string]bool{"host-0.example.com": true}}
	uc := NewBulkProvisionFleet(sshRepo, devRepo, prov)

	ctx := withTestTenant(context.Background())
	result, err := uc.Execute(ctx, BulkProvisionFleetInput{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped (degraded), got %+v", result)
	}
	if prov.calls != 1 {
		t.Errorf("expected exactly 1 Provision call (no retry consumed), got %d", prov.calls)
	}
	if devRepo.lastProvisionStatus != domain.DevServerStatusDegraded {
		t.Errorf("expected degraded status, got %q", devRepo.lastProvisionStatus)
	}
}

func TestBulkProvisionFleet_ProjectFilterExcludesNonMatchingTargets(t *testing.T) {
	targets := []domain.SshTarget{
		{ID: "ssht-a", TenantID: "t1", Host: "a.example.com", UserName: "deploy", Project: "backend"},
		{ID: "ssht-b", TenantID: "t1", Host: "b.example.com", UserName: "deploy", Project: "frontend"},
	}
	sshRepo := &fakeSshTargetRepository{targets: map[string][]domain.SshTarget{"t1": targets}}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{}}
	prov := &fakeProvisioner{}
	uc := NewBulkProvisionFleet(sshRepo, devRepo, prov)

	ctx := withTestTenant(context.Background())
	result, err := uc.Execute(ctx, BulkProvisionFleetInput{Project: "backend"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Host != "a.example.com" {
		t.Errorf("expected only the backend-project target, got %+v", result.Outcomes)
	}
}

func TestBulkProvisionFleet_IdempotentRerunMatchesFirstRun(t *testing.T) {
	sshRepo, devRepo, prov := newBulkProvisionFixture(3)
	uc := NewBulkProvisionFleet(sshRepo, devRepo, prov)

	ctx := withTestTenant(context.Background())
	first, err := uc.Execute(ctx, BulkProvisionFleetInput{})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := uc.Execute(ctx, BulkProvisionFleetInput{})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if first.Success != second.Success {
		t.Errorf("expected idempotent success counts, got first=%d second=%d", first.Success, second.Success)
	}
}

func TestBulkProvisionFleet_RequiresTenantContext(t *testing.T) {
	sshRepo, devRepo, prov := newBulkProvisionFixture(1)
	uc := NewBulkProvisionFleet(sshRepo, devRepo, prov)

	_, err := uc.Execute(context.Background(), BulkProvisionFleetInput{})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
}
