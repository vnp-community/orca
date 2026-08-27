package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeSshTargetRepo is a minimal usecase.SshTargetRepository fake for this
// package's gRPC-level marshaling tests — only Upsert/GetByHostUser matter
// for ImportFleetInventory, the rest satisfy the interface unused.
type fakeSshTargetRepo struct {
	upsertErr error
	updated   bool
	targets   []domain.SshTarget
}

func (f *fakeSshTargetRepo) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	return target, nil
}
func (f *fakeSshTargetRepo) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	return domain.SshTarget{}, nil
}
func (f *fakeSshTargetRepo) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	return f.targets, nil
}
func (f *fakeSshTargetRepo) Upsert(ctx context.Context, target domain.SshTarget) (domain.SshTarget, bool, error) {
	if f.upsertErr != nil {
		return domain.SshTarget{}, false, f.upsertErr
	}
	return target, f.updated, nil
}
func (f *fakeSshTargetRepo) GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error) {
	return domain.SshTarget{}, false, nil
}

func TestServer_ImportFleetInventory_RequestToResponseMarshaling(t *testing.T) {
	repo := &fakeSshTargetRepo{}
	s := &Server{importFleetInventory: usecase.NewImportFleetInventory(repo)}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.ImportFleetInventory(ctx, &infrafleetv1.ImportFleetInventoryRequest{
		Servers: []*infrafleetv1.FleetServerInput{
			{Host: "10.0.0.1", User: "deploy", VaultSshRole: "role-1", Project: "team-a", Tags: []string{"prod"}},
			{Host: "10.0.0.1", User: "", VaultSshRole: "role-1"}, // invalid: empty user
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetImported() != 1 || resp.GetSkipped() != 1 || len(resp.GetErrors()) != 1 {
		t.Errorf("expected imported=1 skipped=1 with 1 error, got %+v", resp)
	}
	if resp.GetErrors()[0].GetHost() != "10.0.0.1" {
		t.Errorf("expected error to identify the offending host, got %+v", resp.GetErrors()[0])
	}
}

func TestServer_ImportFleetInventory_UsecaseErrorMapsToGRPCStatus(t *testing.T) {
	s := &Server{importFleetInventory: usecase.NewImportFleetInventory(&fakeSshTargetRepo{})}

	// No tenant in context -> usecase returns apperrors.KindUnauthenticated,
	// which apperrors.ToGRPCStatus must map to a non-nil gRPC status error.
	_, err := s.ImportFleetInventory(context.Background(), &infrafleetv1.ImportFleetInventoryRequest{
		Servers: []*infrafleetv1.FleetServerInput{{Host: "10.0.0.1", User: "deploy", VaultSshRole: "role-1"}},
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected codes.Unauthenticated, got %v", st.Code())
	}
}

func TestServer_ImportFleetInventory_UpsertErrorSurfacesAsSkipped(t *testing.T) {
	repo := &fakeSshTargetRepo{upsertErr: errors.New("db unavailable")}
	s := &Server{importFleetInventory: usecase.NewImportFleetInventory(repo)}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.ImportFleetInventory(ctx, &infrafleetv1.ImportFleetInventoryRequest{
		Servers: []*infrafleetv1.FleetServerInput{{Host: "10.0.0.1", User: "deploy", VaultSshRole: "role-1"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSkipped() != 1 || len(resp.GetErrors()) != 1 {
		t.Errorf("expected skipped=1 with 1 error, got %+v", resp)
	}
}

// fakeDevServerRepo is a minimal usecase.DevServerRepository fake for
// BulkProvisionFleet's gRPC-level marshaling test.
type fakeDevServerRepo struct{}

func (f *fakeDevServerRepo) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	return ds, nil
}
func (f *fakeDevServerRepo) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *fakeDevServerRepo) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *fakeDevServerRepo) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *fakeDevServerRepo) UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerStatus, info usecase.HandshakeInfo, provisionedAt time.Time) error {
	return nil
}
func (f *fakeDevServerRepo) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
	return nil, nil
}

// fakeBulkProvisioner is a minimal usecase.Provisioner fake.
type fakeBulkProvisioner struct{}

func (f *fakeBulkProvisioner) Provision(ctx context.Context, devServer domain.DevServer) (usecase.HandshakeInfo, bool, error) {
	return usecase.HandshakeInfo{Platform: "linux"}, true, nil
}

func TestServer_BulkProvisionFleet_RequestToResponseMarshaling(t *testing.T) {
	sshRepo := &fakeSshTargetRepo{targets: []domain.SshTarget{
		{ID: "ssht-1", TenantID: "t1", Host: "h1.example.com", UserName: "deploy"},
	}}
	s := &Server{bulkProvisionFleet: usecase.NewBulkProvisionFleet(sshRepo, &fakeDevServerRepo{}, &fakeBulkProvisioner{})}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.BulkProvisionFleet(ctx, &infrafleetv1.BulkProvisionFleetRequest{Concurrency: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSuccess() != 1 || len(resp.GetOutcomes()) != 1 {
		t.Errorf("expected success=1 with 1 outcome, got %+v", resp)
	}
	if resp.GetOutcomes()[0].GetHost() != "h1.example.com" || resp.GetOutcomes()[0].GetStatus() != string(domain.DevServerStatusHealthy) {
		t.Errorf("unexpected outcome: %+v", resp.GetOutcomes()[0])
	}
}

func TestServer_BulkProvisionFleet_UsecaseErrorMapsToGRPCStatus(t *testing.T) {
	s := &Server{bulkProvisionFleet: usecase.NewBulkProvisionFleet(&fakeSshTargetRepo{}, &fakeDevServerRepo{}, &fakeBulkProvisioner{})}

	_, err := s.BulkProvisionFleet(context.Background(), &infrafleetv1.BulkProvisionFleetRequest{})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected codes.Unauthenticated, got %v", st.Code())
	}
}
