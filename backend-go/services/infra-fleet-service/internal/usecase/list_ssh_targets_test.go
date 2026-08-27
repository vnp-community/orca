package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestListSshTargets_ScopesByTenant(t *testing.T) {
	repo := &fakeSshTargetRepository{
		targets: map[string][]domain.SshTarget{
			"t1": {{ID: "s1", TenantID: "t1", Host: "h1"}},
			"t2": {{ID: "s2", TenantID: "t2", Host: "h2"}},
		},
	}
	uc := NewListSshTargets(repo)

	got, err := uc.Execute(withTenant(context.Background(), "t1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Errorf("expected only t1's target, got %+v", got)
	}
}

func TestListSshTargets_RequiresTenantContext(t *testing.T) {
	uc := NewListSshTargets(&fakeSshTargetRepository{})
	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
