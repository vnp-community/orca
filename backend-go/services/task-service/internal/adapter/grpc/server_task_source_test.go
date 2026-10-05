package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// memTaskSources is an in-memory usecase.TaskSourceRepository.
type memTaskSources struct {
	byKey map[string]string
	bySrc map[string]domain.TaskSource
}

func newMemTaskSources() *memTaskSources {
	return &memTaskSources{byKey: map[string]string{}, bySrc: map[string]domain.TaskSource{}}
}

func (m *memTaskSources) key(tenantID, projectID string, p domain.SourceProvider, site, ref string) string {
	return tenantID + "|" + projectID + "|" + string(p) + "|" + site + "|" + ref
}

func (m *memTaskSources) LinkSource(_ context.Context, s domain.TaskSource) error {
	k := m.key(s.TenantID, s.ProjectID, s.Provider, s.Site, s.Ref)
	if _, taken := m.byKey[k]; taken {
		return domain.ErrSourceAlreadyLinked
	}
	m.byKey[k] = s.TaskID
	m.bySrc[s.TaskID] = s
	return nil
}

func (m *memTaskSources) FindTaskIDBySource(_ context.Context, tenantID, projectID string, p domain.SourceProvider, site, ref string) (string, bool, error) {
	if id, ok := m.byKey[m.key(tenantID, projectID, p, site, ref)]; ok {
		return id, true, nil
	}
	id, ok := m.byKey[m.key(tenantID, projectID, p, "", ref)]
	return id, ok, nil
}

func (m *memTaskSources) GetSource(_ context.Context, _, taskID string) (domain.TaskSource, bool, error) {
	s, ok := m.bySrc[taskID]
	return s, ok, nil
}

func sourceServer(t *testing.T) (*Server, *fakeTaskRepository) {
	t.Helper()
	tasks := newFakeTaskRepository()
	s := newTestServer(tasks, &fakeEdgeRepository{})
	sources := newMemTaskSources()
	return s.WithTaskSources(usecase.NewCreateTaskFromSource(tasks, sources, usecase.NewCreateTask(tasks, tasks)), sources), tasks
}

func startFromJira(project string) *taskv1.CreateTaskFromSourceRequest {
	return &taskv1.CreateTaskFromSourceRequest{
		Create:   &taskv1.CreateTaskRequest{Title: "ENG-1 fix login", ProjectId: project},
		Provider: "jira", Ref: "ENG-1", Url: "https://x.atlassian.net/browse/ENG-1",
	}
}

func TestServer_CreateTaskFromSource_CreatorCreatesThenReusesSameTask(t *testing.T) {
	s, _ := sourceServer(t)
	ctx := ctxWithTenantAndUser(t, "user-1")

	first, err := s.CreateTaskFromSource(ctx, startFromJira("proj-1"))
	if err != nil || !first.GetCreated() {
		t.Fatalf("first call: created=%v err=%v", first.GetCreated(), err)
	}
	second, err := s.CreateTaskFromSource(ctx, startFromJira("proj-1"))
	if err != nil {
		t.Fatalf("the creator must be able to reuse their own task: %v", err)
	}
	if second.GetCreated() || second.GetTask().GetId() != first.GetTask().GetId() {
		t.Errorf("want the same task back, got created=%v id=%s (first %s)", second.GetCreated(), second.GetTask().GetId(), first.GetTask().GetId())
	}
}

// The regression for the permission gap: GetTask only checks the tenant, so the
// existing-task path must check the grant itself.
func TestServer_CreateTaskFromSource_StrangerCannotReceiveAnothersTask(t *testing.T) {
	s, _ := sourceServer(t)
	if _, err := s.CreateTaskFromSource(ctxWithTenantAndUser(t, "user-1"), startFromJira("proj-1")); err != nil {
		t.Fatal(err)
	}

	_, err := s.CreateTaskFromSource(ctxWithTenantAndUser(t, "user-2"), startFromJira("proj-1"))
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("a user with no grant must get PermissionDenied, got %v", err)
	}
}

func TestServer_GetTaskSource_OwnerSeesItStrangerDoesNot(t *testing.T) {
	s, _ := sourceServer(t)
	created, err := s.CreateTaskFromSource(ctxWithTenantAndUser(t, "user-1"), startFromJira("proj-1"))
	if err != nil {
		t.Fatal(err)
	}
	id := created.GetTask().GetId()

	got, err := s.GetTaskSource(ctxWithTenantAndUser(t, "user-1"), &taskv1.GetTaskSourceRequest{TaskId: id})
	if err != nil || !got.GetFound() || got.GetProvider() != "jira" || got.GetRef() != "ENG-1" {
		t.Fatalf("owner: %+v, %v", got, err)
	}
	if _, err := s.GetTaskSource(ctxWithTenantAndUser(t, "user-2"), &taskv1.GetTaskSourceRequest{TaskId: id}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("stranger must be denied, got %v", err)
	}
}

func TestServer_TaskSourceRPCs_UnimplementedUntilConfigured(t *testing.T) {
	s := newTestServer(newFakeTaskRepository(), &fakeEdgeRepository{}) // no WithTaskSources
	ctx := ctxWithTenantAndUser(t, "user-1")
	if _, err := s.CreateTaskFromSource(ctx, startFromJira("p")); status.Code(err) != codes.Unimplemented {
		t.Errorf("CreateTaskFromSource: want Unimplemented, got %v", err)
	}
	if _, err := s.GetTaskSource(ctx, &taskv1.GetTaskSourceRequest{TaskId: "t"}); status.Code(err) != codes.Unimplemented {
		t.Errorf("GetTaskSource: want Unimplemented, got %v", err)
	}
}

func TestServer_CreateTaskFromSource_SitePersistedAndReturnedByGetTaskSource(t *testing.T) {
	s, _ := sourceServer(t)
	ctx := ctxWithTenantAndUser(t, "user-1")

	req := startFromJira("proj-1")
	req.Site = "https://a.atlassian.net"
	res, err := s.CreateTaskFromSource(ctx, req)
	if err != nil || !res.GetCreated() {
		t.Fatalf("create: created=%v err=%v", res.GetCreated(), err)
	}
	got, err := s.GetTaskSource(ctx, &taskv1.GetTaskSourceRequest{TaskId: res.GetTask().GetId()})
	if err != nil || !got.GetFound() || got.GetSite() != "https://a.atlassian.net" {
		t.Fatalf("GetTaskSource = %+v, err=%v; want site echoed", got, err)
	}
}
