package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestCreatePortForward_AllocatesAndPersists(t *testing.T) {
	repo := &fakePortForwardRepository{}
	alloc := &fakePortAllocator{}
	uc := NewCreatePortForward(repo, alloc)

	ctx := withTenant(context.Background(), "tenant-1")
	pf, err := uc.Execute(ctx, CreatePortForwardInput{ConnectionID: "conn-1", RemotePort: 3000})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if pf.ConnectionID != "conn-1" || pf.RemotePort != 3000 {
		t.Errorf("unexpected PortForward: %+v", pf)
	}
	if pf.LocalPort == 0 {
		t.Error("expected a non-zero allocated LocalPort")
	}
	if pf.Status != domain.PortForwardStatusActive {
		t.Errorf("expected Status=active, got %q", pf.Status)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.created) != 1 {
		t.Errorf("expected exactly one Create call, got %d", len(repo.created))
	}
}

func TestCreatePortForward_ReleasesPortOnRepoFailure(t *testing.T) {
	repo := &fakePortForwardRepository{createErr: errors.New("db write failed")}
	alloc := &fakePortAllocator{}
	uc := NewCreatePortForward(repo, alloc)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreatePortForwardInput{ConnectionID: "conn-1", RemotePort: 3000})
	if err == nil {
		t.Fatal("expected the repo error to propagate")
	}
	alloc.mu.Lock()
	defer alloc.mu.Unlock()
	if len(alloc.released) != 1 {
		t.Errorf("expected the allocated port to be released on failure, got %v", alloc.released)
	}
}

func TestCreatePortForward_RequiresTenantContext(t *testing.T) {
	uc := NewCreatePortForward(&fakePortForwardRepository{}, &fakePortAllocator{})
	if _, err := uc.Execute(context.Background(), CreatePortForwardInput{}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListPortForwards_DelegatesToRepository(t *testing.T) {
	repo := &fakePortForwardRepository{}
	repo.created = append(repo.created, domain.PortForward{ID: "pf-1", ConnectionID: "conn-1"})
	uc := NewListPortForwards(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ListPortForwardsInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestListPortForwards_RequiresTenantContext(t *testing.T) {
	uc := NewListPortForwards(&fakePortForwardRepository{})
	if _, err := uc.Execute(context.Background(), ListPortForwardsInput{}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeletePortForward_MarksClosed(t *testing.T) {
	repo := &fakePortForwardRepository{}
	uc := NewDeletePortForward(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, DeletePortForwardInput{ID: "pf-1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0].id != "pf-1" || repo.statusUpdates[0].status != domain.PortForwardStatusClosed {
		t.Errorf("expected UpdateStatus(pf-1, closed), got %+v", repo.statusUpdates)
	}
}

func TestDeletePortForward_RequiresTenantContext(t *testing.T) {
	uc := NewDeletePortForward(&fakePortForwardRepository{})
	if err := uc.Execute(context.Background(), DeletePortForwardInput{ID: "pf-1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
