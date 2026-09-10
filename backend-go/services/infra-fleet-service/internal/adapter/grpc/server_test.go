package grpc

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// bulkFakeSshTargetRepositoryForServerTest is an in-memory
// usecase.SshTargetRepository — this package's own copy of the same fake
// shape usecase/bulk_provision_fleet_test.go uses (can't import a _test.go
// symbol across packages). Mutex-protected: BulkProvisionFleet.Execute
// calls Create/Delete from N concurrent per-server goroutines (its bounded
// concurrency semaphore) — confirmed by `go test -race` (this was a real,
// unguarded race in an earlier version of this fake).
type bulkFakeSshTargetRepositoryForServerTest struct {
	mu         sync.Mutex
	created    []domain.SshTarget
	deletedIDs []string
	errForHost map[string]error
}

func (f *bulkFakeSshTargetRepositoryForServerTest) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errForHost[target.Host]; ok {
		return domain.SshTarget{}, err
	}
	f.created = append(f.created, target)
	return target, nil
}
func (f *bulkFakeSshTargetRepositoryForServerTest) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	return domain.SshTarget{}, nil
}
func (f *bulkFakeSshTargetRepositoryForServerTest) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	return nil, nil
}
func (f *bulkFakeSshTargetRepositoryForServerTest) Delete(ctx context.Context, tenantID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}

// bulkFakeDevServerRepositoryForServerTest is an in-memory
// usecase.DevServerRepository, same rationale (and same mutex fix) as above.
type bulkFakeDevServerRepositoryForServerTest struct {
	mu         sync.Mutex
	registered []domain.DevServer
	errForHost map[string]error
}

func (f *bulkFakeDevServerRepositoryForServerTest) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errForHost[ds.Host]; ok {
		return domain.DevServer{}, err
	}
	f.registered = append(f.registered, ds)
	return ds, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}

// fakeBulkProvisionFleetStream implements
// infrafleetv1.InfraFleetService_BulkProvisionFleetServer (a
// grpc.ServerStreamingServer[BulkProvisionFleetEvent] alias) — records every
// Send call, optionally failing on a configured call index.
type fakeBulkProvisionFleetStream struct {
	ctx        context.Context
	sent       []*infrafleetv1.BulkProvisionFleetEvent
	failOnSend int // 1-based index of the Send call to fail; 0 = never fail
}

func (f *fakeBulkProvisionFleetStream) Send(e *infrafleetv1.BulkProvisionFleetEvent) error {
	f.sent = append(f.sent, e)
	if f.failOnSend != 0 && len(f.sent) == f.failOnSend {
		return errors.New("stream send failed")
	}
	return nil
}
func (f *fakeBulkProvisionFleetStream) Context() context.Context     { return f.ctx }
func (f *fakeBulkProvisionFleetStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeBulkProvisionFleetStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeBulkProvisionFleetStream) SetTrailer(metadata.MD)       {}
func (f *fakeBulkProvisionFleetStream) SendMsg(m interface{}) error  { return nil }
func (f *fakeBulkProvisionFleetStream) RecvMsg(m interface{}) error  { return nil }

func newBulkProvisionFleetTestServer(t *testing.T, sshRepo *bulkFakeSshTargetRepositoryForServerTest, devRepo *bulkFakeDevServerRepositoryForServerTest) *Server {
	t.Helper()
	bulkProvisionFleetUC := usecase.NewBulkProvisionFleet(
		usecase.NewCreateSshTarget(sshRepo),
		usecase.NewRegisterDevServer(devRepo),
		usecase.NewDeleteSshTarget(sshRepo),
	)
	// Only bulkProvisionFleet is populated — the rest of Server's usecase
	// fields stay nil, safe since this test only calls BulkProvisionFleet.
	return New(
		nil, nil, nil, bulkProvisionFleetUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func withTestTenant(ctx context.Context) context.Context {
	return tenant.WithTenantID(ctx, "tenant-1")
}

func TestServer_BulkProvisionFleet_StreamsEventPerServer(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepositoryForServerTest{}
	devRepo := &bulkFakeDevServerRepositoryForServerTest{}
	s := newBulkProvisionFleetTestServer(t, sshRepo, devRepo)

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{
			{Host: "h1", UserName: "orca", VaultSshRole: "role"},
			{Host: "h2", UserName: "orca", VaultSshRole: "role"},
		},
	}

	if err := s.BulkProvisionFleet(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 2 {
		t.Fatalf("expected 2 events sent, got %d", len(stream.sent))
	}
	for _, e := range stream.sent {
		if e.GetStatus() != infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED {
			t.Errorf("host %q: expected SUCCEEDED, got %v", e.GetHost(), e.GetStatus())
		}
	}
}

func TestServer_BulkProvisionFleet_MapsFailedStatusCorrectly(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepositoryForServerTest{}
	devRepo := &bulkFakeDevServerRepositoryForServerTest{errForHost: map[string]error{"h1": errors.New("boom")}}
	s := newBulkProvisionFleetTestServer(t, sshRepo, devRepo)

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
	}

	if err := s.BulkProvisionFleet(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 1 {
		t.Fatalf("expected 1 event sent, got %d", len(stream.sent))
	}
	got := stream.sent[0]
	if got.GetStatus() != infrafleetv1.BulkProvisionFleetEvent_FAILED {
		t.Errorf("expected FAILED, got %v", got.GetStatus())
	}
	if got.GetError() == "" {
		t.Error("expected a non-empty Error field")
	}
}

// TestServer_BulkProvisionFleet_MissingTenant_AllServersFailedNoTopLevelError
// documents actual usecase.BulkProvisionFleet.Execute behavior: it NEVER
// returns a non-nil top-level error (see bulk_provision_fleet.go's Execute —
// always `return result, nil`) — a per-server failure (like a missing
// tenant) surfaces as a FAILED event, not a returned error. The task doc's
// original TestServer_BulkProvisionFleet_UsecaseErrorReturnsGRPCStatus
// assumed Execute could fail generally; it cannot with the real
// implementation, so this test exercises the actually-reachable behavior
// instead, and TestServer_BulkProvisionFleet_StreamSendFails_ReturnsGRPCStatus
// below covers the handler's other general-error branch that IS reachable
// (stream.Send failing).
func TestServer_BulkProvisionFleet_MissingTenant_AllServersFailedNoTopLevelError(t *testing.T) {
	s := newBulkProvisionFleetTestServer(t, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: context.Background()} // no tenant in context
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
	}

	err := s.BulkProvisionFleet(req, stream)
	if err != nil {
		t.Fatalf("expected no top-level RPC error (failure surfaces per-server), got: %v", err)
	}
	if len(stream.sent) != 1 || stream.sent[0].GetStatus() != infrafleetv1.BulkProvisionFleetEvent_FAILED {
		t.Fatalf("expected 1 FAILED event, got %+v", stream.sent)
	}
}

// TestServer_BulkProvisionFleet_StreamSendFails_ReturnsGRPCStatus covers the
// handler's general (non-per-server) error path — a stream.Send failure
// (e.g. the client disconnected mid-stream) is wrapped via apperrors and
// returned as a gRPC status, the only way BulkProvisionFleet's handler can
// currently fail generally given Execute never returns a non-nil error.
func TestServer_BulkProvisionFleet_StreamSendFails_ReturnsGRPCStatus(t *testing.T) {
	s := newBulkProvisionFleetTestServer(t, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background()), failOnSend: 1}
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
	}

	err := s.BulkProvisionFleet(req, stream)
	if err == nil {
		t.Fatal("expected a gRPC status error when stream.Send fails")
	}
}

// --- ApplyTerraformPlan (TASK-BE-FLEET-008) ---

// fakeApplyTerraformPlanStream implements
// infrafleetv1.InfraFleetService_ApplyTerraformPlanServer.
type fakeApplyTerraformPlanStream struct {
	ctx     context.Context
	sent    []*infrafleetv1.ApplyTerraformPlanEvent
	sendErr error
}

func (f *fakeApplyTerraformPlanStream) Send(e *infrafleetv1.ApplyTerraformPlanEvent) error {
	f.sent = append(f.sent, e)
	return f.sendErr
}
func (f *fakeApplyTerraformPlanStream) Context() context.Context     { return f.ctx }
func (f *fakeApplyTerraformPlanStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeApplyTerraformPlanStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeApplyTerraformPlanStream) SetTrailer(metadata.MD)       {}
func (f *fakeApplyTerraformPlanStream) SendMsg(m interface{}) error  { return nil }
func (f *fakeApplyTerraformPlanStream) RecvMsg(m interface{}) error  { return nil }

// fakeTerraformRunnerForServerTest is an in-memory usecase.TerraformRunner.
type fakeTerraformRunnerForServerTest struct {
	outputJSON string
	applyErr   error
}

func (f *fakeTerraformRunnerForServerTest) Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (string, error) {
	if f.applyErr != nil {
		return "", f.applyErr
	}
	return f.outputJSON, nil
}

func newApplyTerraformPlanTestServer(t *testing.T, devRepo *bulkFakeDevServerRepositoryForServerTest, runner *fakeTerraformRunnerForServerTest) *Server {
	t.Helper()
	applyTerraformPlanUC := usecase.NewApplyTerraformPlan(devRepo, runner)
	return New(
		nil, nil, nil, nil, applyTerraformPlanUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func TestServer_ApplyTerraformPlan_SendsResultEvent(t *testing.T) {
	devRepo := &bulkFakeDevServerRepositoryForServerTest{}
	// Get() on this fake always returns a zero-value DevServer with no
	// error, so ControlDevServerID resolution always "succeeds" here.
	runner := &fakeTerraformRunnerForServerTest{outputJSON: `{"instance_hosts":{"value":["h1"]}}`}
	s := newApplyTerraformPlanTestServer(t, devRepo, runner)

	stream := &fakeApplyTerraformPlanStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.ApplyTerraformPlanRequest{ControlDevServerId: "d1", WorkingDir: "/infra"}

	if err := s.ApplyTerraformPlan(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 1 {
		t.Fatalf("expected 1 event sent, got %d", len(stream.sent))
	}
	got := stream.sent[0]
	if got.GetType() != "result" || got.GetOutputJson() != `{"instance_hosts":{"value":["h1"]}}` {
		t.Errorf("unexpected event: %+v", got)
	}
}

func TestServer_ApplyTerraformPlan_UsecaseErrorReturnsGRPCStatus(t *testing.T) {
	devRepo := &bulkFakeDevServerRepositoryForServerTest{}
	runner := &fakeTerraformRunnerForServerTest{applyErr: errors.New("terraform apply failed")}
	s := newApplyTerraformPlanTestServer(t, devRepo, runner)

	stream := &fakeApplyTerraformPlanStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.ApplyTerraformPlanRequest{ControlDevServerId: "d1", WorkingDir: "/infra"}

	err := s.ApplyTerraformPlan(req, stream)
	if err == nil {
		t.Fatal("expected a gRPC status error when the runner fails")
	}
	if len(stream.sent) != 0 {
		t.Errorf("expected no events sent, got %d", len(stream.sent))
	}
}

// --- FleetDefinition CRUD (TASK-BE-FLEET-012) ---

func newFleetDefinitionTestServer(t *testing.T, repo *fakeFleetDefinitionRepositoryForServerTest) *Server {
	t.Helper()
	return New(
		nil, nil, nil, nil, nil,
		usecase.NewCreateFleetDefinition(repo),
		usecase.NewUpdateFleetDefinition(repo),
		usecase.NewGetFleetDefinition(repo),
		usecase.NewListFleetDefinitions(repo),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

// fakeFleetDefinitionRepositoryForServerTest is an in-memory
// usecase.FleetDefinitionRepository — this package's own copy (can't reuse
// usecase's _test.go fake across packages).
type fakeFleetDefinitionRepositoryForServerTest struct {
	byID      map[string]domain.FleetDefinition
	createErr error
	updateErr error
}

func fleetDefKeyForServerTest(tenantID, id string) string { return tenantID + "/" + id }

func (f *fakeFleetDefinitionRepositoryForServerTest) Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	if f.createErr != nil {
		return domain.FleetDefinition{}, f.createErr
	}
	if f.byID == nil {
		f.byID = map[string]domain.FleetDefinition{}
	}
	f.byID[fleetDefKeyForServerTest(def.TenantID, def.ID)] = def
	return def, nil
}
func (f *fakeFleetDefinitionRepositoryForServerTest) Update(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	if f.updateErr != nil {
		return domain.FleetDefinition{}, f.updateErr
	}
	f.byID[fleetDefKeyForServerTest(def.TenantID, def.ID)] = def
	return def, nil
}
func (f *fakeFleetDefinitionRepositoryForServerTest) Get(ctx context.Context, tenantID, id string) (domain.FleetDefinition, error) {
	def, ok := f.byID[fleetDefKeyForServerTest(tenantID, id)]
	if !ok {
		return domain.FleetDefinition{}, domain.ErrFleetDefinitionNotFound
	}
	return def, nil
}
func (f *fakeFleetDefinitionRepositoryForServerTest) List(ctx context.Context, tenantID string) ([]domain.FleetDefinition, error) {
	var out []domain.FleetDefinition
	for _, def := range f.byID {
		if def.TenantID == tenantID {
			out = append(out, def)
		}
	}
	return out, nil
}

func TestServer_CreateFleetDefinition_ReturnsFleetDefinitionProto(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	ctx = tenant.WithUserID(ctx, "user-1")
	req := &infrafleetv1.CreateFleetDefinitionRequest{
		Name:      "fleet-1",
		Servers:   []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
		Provision: &infrafleetv1.ProvisionConfigProto{Iac: "terraform", WorkingDir: "/infra"},
	}

	got, err := s.CreateFleetDefinition(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetName() != "fleet-1" || got.GetVersion() != 1 {
		t.Errorf("unexpected response: %+v", got)
	}
	if len(got.GetServers()) != 1 || got.GetServers()[0].GetHost() != "h1" {
		t.Errorf("unexpected servers: %+v", got.GetServers())
	}
	if got.GetProvision().GetWorkingDir() != "/infra" {
		t.Errorf("unexpected provision: %+v", got.GetProvision())
	}
}

func TestServer_GetFleetDefinition_UsecaseErrorReturnsGRPCStatus(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	_, err := s.GetFleetDefinition(ctx, &infrafleetv1.GetFleetDefinitionRequest{Id: "unknown"})
	if err == nil {
		t.Fatal("expected a gRPC status error for an unknown fleet definition")
	}
}

func TestServer_ListFleetDefinitions_ReturnsAllForTenant(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {ID: "d1", TenantID: "tenant-1", Name: "fleet-1"},
	}}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	got, err := s.ListFleetDefinitions(ctx, &infrafleetv1.ListFleetDefinitionsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetDefinitions()) != 1 || got.GetDefinitions()[0].GetName() != "fleet-1" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestServer_UpdateFleetDefinition_IncrementsVersion(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {
			ID: "d1", TenantID: "tenant-1", Name: "fleet-1", Version: 1,
			Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	req := &infrafleetv1.UpdateFleetDefinitionRequest{
		Id:      "d1",
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h2", UserName: "orca", VaultSshRole: "role"}},
	}
	got, err := s.UpdateFleetDefinition(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetVersion() != 2 {
		t.Errorf("expected version 2, got %d", got.GetVersion())
	}
}

// --- ExportFleetDefinitionYaml (TASK-BE-FLEET-013) ---

func newExportFleetDefinitionYamlTestServer(t *testing.T, repo *fakeFleetDefinitionRepositoryForServerTest) *Server {
	t.Helper()
	return New(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		usecase.NewExportFleetDefinitionYaml(repo),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func TestServer_ExportFleetDefinitionYaml_ReturnsYamlContent(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {
			ID: "d1", TenantID: "tenant-1", Name: "fleet-1",
			Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	s := newExportFleetDefinitionYamlTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	got, err := s.ExportFleetDefinitionYaml(ctx, &infrafleetv1.ExportFleetDefinitionYamlRequest{Id: "d1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetYamlContent() == "" || !strings.Contains(got.GetYamlContent(), "host: h1") {
		t.Errorf("unexpected yaml content: %q", got.GetYamlContent())
	}
}

func TestServer_ExportFleetDefinitionYaml_NotFoundReturnsGRPCStatus(t *testing.T) {
	s := newExportFleetDefinitionYamlTestServer(t, &fakeFleetDefinitionRepositoryForServerTest{})
	ctx := withTestTenant(context.Background())
	_, err := s.ExportFleetDefinitionYaml(ctx, &infrafleetv1.ExportFleetDefinitionYamlRequest{Id: "unknown"})
	if err == nil {
		t.Fatal("expected a gRPC status error for an unknown fleet definition")
	}
}

// --- DeployFleetDefinition (TASK-BE-FLEET-014) ---

func newDeployFleetDefinitionTestServer(
	t *testing.T,
	fleetRepo *fakeFleetDefinitionRepositoryForServerTest,
	controlDevRepo *bulkFakeDevServerRepositoryForServerTest,
	runner *fakeTerraformRunnerForServerTest,
	sshRepo *bulkFakeSshTargetRepositoryForServerTest,
	devRepo *bulkFakeDevServerRepositoryForServerTest,
) *Server {
	t.Helper()
	applyTerraformPlan := usecase.NewApplyTerraformPlan(controlDevRepo, runner)
	bulkProvisionFleet := usecase.NewBulkProvisionFleet(
		usecase.NewCreateSshTarget(sshRepo), usecase.NewRegisterDevServer(devRepo), usecase.NewDeleteSshTarget(sshRepo),
	)
	deployFleetDefinition := usecase.NewDeployFleetDefinition(fleetRepo, applyTerraformPlan, bulkProvisionFleet)
	return New(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		deployFleetDefinition,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func TestServer_DeployFleetDefinition_ProvisionNil_StreamsBulkProvisionFleetEvent(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {
			ID: "d1", TenantID: "tenant-1", Name: "fleet-1",
			Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	s := newDeployFleetDefinitionTestServer(t, fleetRepo, &bulkFakeDevServerRepositoryForServerTest{}, &fakeTerraformRunnerForServerTest{}, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.DeployFleetDefinitionRequest{FleetDefinitionId: "d1"}

	if err := s.DeployFleetDefinition(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 1 || stream.sent[0].GetHost() != "h1" || stream.sent[0].GetStatus() != infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED {
		t.Errorf("unexpected events: %+v", stream.sent)
	}
}

func TestServer_DeployFleetDefinition_UsecaseErrorReturnsGRPCStatus(t *testing.T) {
	s := newDeployFleetDefinitionTestServer(t, &fakeFleetDefinitionRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{}, &fakeTerraformRunnerForServerTest{}, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.DeployFleetDefinitionRequest{FleetDefinitionId: "unknown"}

	err := s.DeployFleetDefinition(req, stream)
	if err == nil {
		t.Fatal("expected a gRPC status error for an unknown fleet definition")
	}
	if len(stream.sent) != 0 {
		t.Errorf("expected no events sent, got %d", len(stream.sent))
	}
}
