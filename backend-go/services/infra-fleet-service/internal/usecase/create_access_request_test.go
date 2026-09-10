package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeAccessRequestRepository is an in-memory
// DevServerAccessRequestRepository, shared across this package's
// access-request tests.
type fakeAccessRequestRepository struct {
	created      []domain.DevServerAccessRequest
	createErr    error
	byID         map[string]domain.DevServerAccessRequest
	getErr       error
	pending      []domain.DevServerAccessRequest
	listErr      error
	updateErr    error
	lastUpdateID string
}

func (f *fakeAccessRequestRepository) Create(ctx context.Context, req domain.DevServerAccessRequest) (domain.DevServerAccessRequest, error) {
	if f.createErr != nil {
		return domain.DevServerAccessRequest{}, f.createErr
	}
	f.created = append(f.created, req)
	return req, nil
}

func (f *fakeAccessRequestRepository) Get(ctx context.Context, tenantID, id string) (domain.DevServerAccessRequest, error) {
	if f.getErr != nil {
		return domain.DevServerAccessRequest{}, f.getErr
	}
	return f.byID[id], nil
}

func (f *fakeAccessRequestRepository) ListPending(ctx context.Context, tenantID string) ([]domain.DevServerAccessRequest, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.pending, nil
}

func (f *fakeAccessRequestRepository) UpdateStatus(ctx context.Context, tenantID, id string, status domain.AccessRequestStatus) (domain.DevServerAccessRequest, error) {
	f.lastUpdateID = id
	if f.updateErr != nil {
		return domain.DevServerAccessRequest{}, f.updateErr
	}
	req := f.byID[id]
	req.Status = status
	return req, nil
}

func TestCreateAccessRequest_RequiresTenantAndUserContext(t *testing.T) {
	uc := NewCreateAccessRequest(&fakeAccessRequestRepository{})
	_, err := uc.Execute(context.Background(), CreateAccessRequestInput{
		DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1",
	})
	if err == nil {
		t.Fatal("expected an error when no tenant/user is in context")
	}
}

// TestCreateAccessRequest_NotAdminGated confirms any authenticated tenant
// user (no admin role needed) can file a request — the whole point of this
// flow.
func TestCreateAccessRequest_NotAdminGated(t *testing.T) {
	repo := &fakeAccessRequestRepository{}
	uc := NewCreateAccessRequest(repo)

	ctx := tenant.WithUserID(withTenant(context.Background(), "tenant-1"), "user-1")
	got, err := uc.Execute(ctx, CreateAccessRequestInput{
		DevServerGroupID: "g1", Message: "please", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1", NowUnixMs: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error (should not require admin): %v", err)
	}
	if got.Status != domain.AccessRequestStatusPending {
		t.Errorf("want new request pending, got %q", got.Status)
	}
	if got.UserID != "user-1" {
		t.Errorf("want UserID from context, got %q", got.UserID)
	}
}

func TestCreateAccessRequest_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeAccessRequestRepository{createErr: errors.New("db unavailable")}
	uc := NewCreateAccessRequest(repo)

	ctx := tenant.WithUserID(withTenant(context.Background(), "tenant-1"), "user-1")
	_, err := uc.Execute(ctx, CreateAccessRequestInput{
		DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1",
	})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
