package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestResolveAccessRequest_RequiresAdmin(t *testing.T) {
	uc := NewResolveAccessRequest(&fakeAccessRequestRepository{}, &fakeDevServerGroupGrantRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ResolveAccessRequestInput{RequestID: "req1", Approve: true})
	if err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestResolveAccessRequest_ApproveCreatesGrant(t *testing.T) {
	requests := &fakeAccessRequestRepository{byID: map[string]domain.DevServerAccessRequest{
		"req1": {
			ID: "req1", TenantID: "tenant-1", UserID: "user-1", DevServerGroupID: "g1",
			Status: domain.AccessRequestStatusPending, GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1",
		},
	}}
	grants := &fakeDevServerGroupGrantRepository{}
	uc := NewResolveAccessRequest(requests, grants)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveAccessRequestInput{RequestID: "req1", Approve: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Request.Status != domain.AccessRequestStatusApproved {
		t.Errorf("want request status=approved, got %q", out.Request.Status)
	}
	if len(grants.created) != 1 {
		t.Fatalf("expected 1 grant created, got %d", len(grants.created))
	}
	if grants.created[0].DevServerGroupID != "g1" || grants.created[0].GranteeID != "dept1" {
		t.Errorf("unexpected grant: %+v", grants.created[0])
	}
}

func TestResolveAccessRequest_RejectCreatesNoGrant(t *testing.T) {
	requests := &fakeAccessRequestRepository{byID: map[string]domain.DevServerAccessRequest{
		"req1": {
			ID: "req1", TenantID: "tenant-1", Status: domain.AccessRequestStatusPending,
			GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1", DevServerGroupID: "g1",
		},
	}}
	grants := &fakeDevServerGroupGrantRepository{}
	uc := NewResolveAccessRequest(requests, grants)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveAccessRequestInput{RequestID: "req1", Approve: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Request.Status != domain.AccessRequestStatusRejected {
		t.Errorf("want request status=rejected, got %q", out.Request.Status)
	}
	if len(grants.created) != 0 {
		t.Errorf("expected no grant created for a rejection, got %+v", grants.created)
	}
}

// TestResolveAccessRequest_AlreadyResolvedFails guards against double-
// resolving (double-granting) a request.
func TestResolveAccessRequest_AlreadyResolvedFails(t *testing.T) {
	requests := &fakeAccessRequestRepository{byID: map[string]domain.DevServerAccessRequest{
		"req1": {ID: "req1", Status: domain.AccessRequestStatusApproved},
	}}
	uc := NewResolveAccessRequest(requests, &fakeDevServerGroupGrantRepository{})

	ctx := withAdminTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, ResolveAccessRequestInput{RequestID: "req1", Approve: true}); err == nil {
		t.Fatal("expected an error for an already-resolved request")
	}
}
