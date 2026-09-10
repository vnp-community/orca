package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func withTestTenant(ctx context.Context) context.Context {
	return tenant.WithTenantID(ctx, "t1")
}

func TestImportFleetInventory_AllNewBatch(t *testing.T) {
	repo := &fakeSshTargetRepository{}
	uc := NewImportFleetInventory(repo)

	result, err := uc.Execute(withTestTenant(context.Background()), ImportFleetInventoryInput{
		Servers: []FleetServerInput{
			{Host: "10.0.0.1", UserName: "deploy", VaultSSHRole: "role-1"},
			{Host: "10.0.0.2", UserName: "deploy", VaultSSHRole: "role-1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 2 || result.Updated != 0 || result.Skipped != 0 {
		t.Errorf("expected imported=2 updated=0 skipped=0, got %+v", result)
	}
	if len(repo.upserted) != 2 {
		t.Errorf("expected 2 Upsert calls, got %d", len(repo.upserted))
	}
}

func TestImportFleetInventory_PreExistingRowCountsAsUpdated(t *testing.T) {
	repo := &fakeSshTargetRepository{upsertUpdated: true}
	uc := NewImportFleetInventory(repo)

	result, err := uc.Execute(withTestTenant(context.Background()), ImportFleetInventoryInput{
		Servers: []FleetServerInput{
			{Host: "10.0.0.1", UserName: "deploy", VaultSSHRole: "role-1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Updated != 1 || result.Imported != 0 {
		t.Errorf("expected updated=1 imported=0, got %+v", result)
	}
}

func TestImportFleetInventory_InvalidRowSkippedButBatchContinues(t *testing.T) {
	repo := &fakeSshTargetRepository{}
	uc := NewImportFleetInventory(repo)

	result, err := uc.Execute(withTestTenant(context.Background()), ImportFleetInventoryInput{
		Servers: []FleetServerInput{
			{Host: "10.0.0.1", UserName: "deploy", VaultSSHRole: ""}, // invalid: empty vault role
			{Host: "10.0.0.2", UserName: "deploy", VaultSSHRole: "role-1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 1 || len(result.Errors) != 1 {
		t.Errorf("expected skipped=1 with 1 error, got %+v", result)
	}
	if result.Errors[0].Host != "10.0.0.1" || result.Errors[0].UserName != "deploy" {
		t.Errorf("expected error to identify the offending row, got %+v", result.Errors[0])
	}
	if result.Imported != 1 {
		t.Errorf("expected the second, valid row to still commit: imported=1, got %+v", result)
	}
}

func TestImportFleetInventory_UpsertErrorSkipsRowButContinuesBatch(t *testing.T) {
	repo := &fakeSshTargetRepository{upsertErr: errors.New("db unavailable")}
	uc := NewImportFleetInventory(repo)

	result, err := uc.Execute(withTestTenant(context.Background()), ImportFleetInventoryInput{
		Servers: []FleetServerInput{
			{Host: "10.0.0.1", UserName: "deploy", VaultSSHRole: "role-1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 1 || len(result.Errors) != 1 {
		t.Errorf("expected skipped=1 with 1 error, got %+v", result)
	}
}

func TestImportFleetInventory_DryRunNeverCallsUpsert(t *testing.T) {
	repo := &fakeSshTargetRepository{
		byHostUser: map[string]domain.SshTarget{
			sshTargetHostUserKey("t1", "10.0.0.1", "deploy"): {ID: "existing-1", TenantID: "t1", Host: "10.0.0.1", UserName: "deploy"},
		},
	}
	uc := NewImportFleetInventory(repo)

	result, err := uc.Execute(withTestTenant(context.Background()), ImportFleetInventoryInput{
		DryRun: true,
		Servers: []FleetServerInput{
			{Host: "10.0.0.1", UserName: "deploy", VaultSSHRole: "role-1"}, // pre-existing -> updated
			{Host: "10.0.0.9", UserName: "deploy", VaultSSHRole: "role-1"}, // new -> imported
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.upserted) != 0 {
		t.Errorf("expected DryRun to never call Upsert, got %d calls", len(repo.upserted))
	}
	if result.Updated != 1 || result.Imported != 1 {
		t.Errorf("expected updated=1 imported=1, got %+v", result)
	}
}

func TestImportFleetInventory_RequiresTenantContext(t *testing.T) {
	repo := &fakeSshTargetRepository{}
	uc := NewImportFleetInventory(repo)

	_, err := uc.Execute(context.Background(), ImportFleetInventoryInput{
		Servers: []FleetServerInput{{Host: "10.0.0.1", UserName: "deploy", VaultSSHRole: "role-1"}},
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
}
