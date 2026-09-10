//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	testUser1      = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	testAccessReq1 = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
)

func setupDevServerAccessRequestStore(t *testing.T) (*DevServerAccessRequestStore, *DevServerGroupStore) {
	t.Helper()
	repo, groupStore := setupDevServerGroupStore(t)
	return NewDevServerAccessRequestStore(repo.pool), groupStore
}

func TestDevServerAccessRequestStore_CreateGetListPendingUpdateStatus(t *testing.T) {
	reqStore, groupStore := setupDevServerAccessRequestStore(t)
	ctx := context.Background()

	group, _ := domain.NewDevServerGroup(testGroupParent, testTenant1, "Backend Team", "")
	if _, err := groupStore.Create(ctx, group); err != nil {
		t.Fatalf("creating group: %v", err)
	}

	req, err := domain.NewDevServerAccessRequest(testAccessReq1, testTenant1, testUser1, testGroupParent, "please", domain.GranteeKindDepartment, "dept-1", 1_700_000_000_000)
	if err != nil {
		t.Fatalf("building access request: %v", err)
	}
	if _, err := reqStore.Create(ctx, req); err != nil {
		t.Fatalf("creating access request: %v", err)
	}

	got, err := reqStore.Get(ctx, testTenant1, testAccessReq1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.AccessRequestStatusPending || got.Message != "please" || got.GranteeID != "dept-1" {
		t.Errorf("unexpected access request: %+v", got)
	}
	if got.CreatedAtUnixMs != 1_700_000_000_000 {
		t.Errorf("want CreatedAtUnixMs round-tripped, got %d", got.CreatedAtUnixMs)
	}

	pending, err := reqStore.ListPending(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != testAccessReq1 {
		t.Fatalf("unexpected pending list: %+v", pending)
	}

	updated, err := reqStore.UpdateStatus(ctx, testTenant1, testAccessReq1, domain.AccessRequestStatusApproved)
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.Status != domain.AccessRequestStatusApproved {
		t.Errorf("want status=approved, got %q", updated.Status)
	}

	stillPending, err := reqStore.ListPending(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list pending after resolve: %v", err)
	}
	if len(stillPending) != 0 {
		t.Errorf("expected 0 pending after resolving, got %+v", stillPending)
	}
}
