// Contract tests for the grpc.Server translation layer — assert wire
// request/response field mapping is correct, using real usecases backed by
// small local fakes (this package's own, distinct from internal/usecase's
// _test.go-only fakes, which aren't visible outside that package).
package grpc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// fakeTaskRepository is a minimal usecase.TaskRepository backed by a plain
// map — enough for this file's contract tests, not a fidelity replica of
// Postgres.
type fakeTaskRepository struct {
	tasks  map[string]domain.Task
	grants []domain.Grant
}

func newFakeTaskRepository() *fakeTaskRepository {
	return &fakeTaskRepository{tasks: map[string]domain.Task{}}
}

// Grant/ListGrantsForAncestors satisfy usecase.GrantRepository too — this
// fake stands in for both ports at once, same as postgres.Repository does
// for real, so newTestServer can pass it to both NewGrant/NewResolvePermission
// and NewCreateTask/NewListTasks/etc.
func (f *fakeTaskRepository) Grant(ctx context.Context, tenantID string, grant domain.Grant) (string, error) {
	grant.ID = fmt.Sprintf("grant-%d", len(f.grants)+1)
	f.grants = append(f.grants, grant)
	return grant.ID, nil
}
func (f *fakeTaskRepository) Revoke(ctx context.Context, tenantID, grantID string) error {
	for i, g := range f.grants {
		if g.ID == grantID {
			f.grants = append(f.grants[:i], f.grants[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}
func (f *fakeTaskRepository) ListGrantsForTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error) {
	var out []domain.Grant
	for _, g := range f.grants {
		if g.TaskID == taskID {
			out = append(out, g)
		}
	}
	return out, nil
}
func (f *fakeTaskRepository) ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error) {
	ids := map[string]bool{}
	for _, id := range taskIDs {
		ids[id] = true
	}
	out := map[string][]domain.Grant{}
	for _, g := range f.grants {
		if ids[g.TaskID] {
			out[g.TaskID] = append(out[g.TaskID], g)
		}
	}
	return out, nil
}

func (f *fakeTaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	f.tasks[task.ID] = task
	return task, nil
}
func (f *fakeTaskRepository) Get(ctx context.Context, tenantID, id string) (domain.Task, error) {
	t, ok := f.tasks[id]
	if !ok || t.TenantID != tenantID {
		return domain.Task{}, errors.New("not found")
	}
	return t, nil
}
func (f *fakeTaskRepository) GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error) {
	current, ok := f.tasks[id]
	if !ok || current.TenantID != tenantID {
		return nil, errors.New("not found")
	}
	var chain []domain.Task
	for i := 0; maxDepth <= 0 || i < maxDepth; i++ {
		chain = append(chain, current)
		if current.ParentID == "" {
			break
		}
		parent, ok := f.tasks[current.ParentID]
		if !ok {
			break
		}
		current = parent
	}
	return chain, nil
}
func (f *fakeTaskRepository) UpdateStatus(ctx context.Context, tenantID, id, status string) error {
	t, ok := f.tasks[id]
	if !ok {
		return errors.New("not found")
	}
	t.Status = status
	f.tasks[id] = t
	return nil
}
func (f *fakeTaskRepository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	return false, nil
}
func (f *fakeTaskRepository) List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error) {
	var out []domain.Task
	for _, t := range f.tasks {
		if t.TenantID == tenantID && (projectID == "" || t.ProjectID == projectID) {
			out = append(out, t)
		}
	}
	return out, "", nil
}
func (f *fakeTaskRepository) Update(ctx context.Context, tenantID string, task domain.Task) error {
	existing, ok := f.tasks[task.ID]
	if !ok || existing.TenantID != tenantID {
		return errors.New("not found")
	}
	f.tasks[task.ID] = task
	return nil
}
func (f *fakeTaskRepository) Delete(ctx context.Context, tenantID, id string) error {
	existing, ok := f.tasks[id]
	if !ok || existing.TenantID != tenantID {
		return errors.New("not found")
	}
	delete(f.tasks, id)
	return nil
}
func (f *fakeTaskRepository) UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error {
	t, ok := f.tasks[id]
	if !ok {
		return errors.New("not found")
	}
	t.WorktreeID = worktreeID
	f.tasks[id] = t
	return nil
}
func (f *fakeTaskRepository) UpdatePromptTemplate(ctx context.Context, tenantID, id, promptTemplate string) error {
	t, ok := f.tasks[id]
	if !ok {
		return errors.New("not found")
	}
	t.PromptTemplate = promptTemplate
	f.tasks[id] = t
	return nil
}
func (f *fakeTaskRepository) UpdateAIPlanJSON(ctx context.Context, tenantID, id, aiPlanJSON string) error {
	t, ok := f.tasks[id]
	if !ok {
		return errors.New("not found")
	}
	t.AIPlanJSON = aiPlanJSON
	f.tasks[id] = t
	return nil
}
func (f *fakeTaskRepository) GetSubtree(ctx context.Context, tenantID, rootID string, maxDepth int) ([]domain.Task, []domain.TaskEdge, error) {
	return nil, nil, errors.New("not implemented")
}
func (f *fakeTaskRepository) GetSubtreeWithChildPercents(ctx context.Context, tenantID, rootID string) ([]usecase.SubtreeProgressNode, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTaskRepository) BatchUpdateProgress(ctx context.Context, tenantID string, updates map[string]int) error {
	return errors.New("not implemented")
}

// fakeEdgeRepository is a minimal usecase.EdgeRepository for this file's
// contract tests.
type fakeEdgeRepository struct {
	edges []domain.TaskEdge
}

func (f *fakeEdgeRepository) Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error {
	f.edges = append(f.edges, edge)
	return nil
}
func (f *fakeEdgeRepository) ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	var out []domain.TaskEdge
	for _, e := range f.edges {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeEdgeRepository) ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	var out []domain.TaskEdge
	for _, e := range f.edges {
		if e.Kind == kind && e.FromTaskID == fromTaskID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeEdgeRepository) ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	return f.ListByKind(ctx, tenantID, kind)
}
func (f *fakeEdgeRepository) ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	var out []domain.TaskEdge
	for _, e := range f.edges {
		if e.Kind == kind && e.ToTaskID == toTaskID {
			out = append(out, e)
		}
	}
	return out, nil
}

// fakeAIProviderContextResolver/fakeProjectExecutionResolver/fakeAICompleter
// back this file's AIDecompose contract tests.
type fakeAIProviderContextResolver struct{}

func (fakeAIProviderContextResolver) ResolveContext(ctx context.Context, tenantID, userID string) (string, error) {
	return "", nil
}

type fakeProjectExecutionResolver struct {
	connectionID string
	connected    bool
}

func (f fakeProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, string, string, bool, error) {
	return f.connectionID, "", "", f.connected, nil
}

type fakeProjectContextResolver struct{}

func (fakeProjectContextResolver) Resolve(ctx context.Context, tenantID, projectID string) (string, string, error) {
	return "", "", nil
}

type fakeTechStackDetector struct{}

func (fakeTechStackDetector) Detect(ctx context.Context, tenantID, projectID string) (domain.TechStack, error) {
	return domain.TechStack{}, nil
}

type fakeVelocityResolver struct{}

func (fakeVelocityResolver) RecentCompletedTasks(ctx context.Context, tenantID, projectID string, n int) ([]domain.Task, error) {
	return nil, nil
}

type fakeAICompleter struct {
	content string
}

func (f fakeAICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	return f.content, nil
}

func wrapperString(v string) *wrapperspb.StringValue {
	return wrapperspb.String(v)
}

func ctxWithTenant(t *testing.T) context.Context {
	t.Helper()
	return tenant.WithTenantID(context.Background(), "tenant-1")
}

// fakeTxRunner is a pass-through usecase.TxRunner for this file's wiring/
// contract tests — no rollback simulation needed here (that's
// internal/usecase/ai_apply_test.go's job); this just proves AIApply is
// wired to a TxRunner reaching the same tasks/edges fakes the rest of
// newTestServer's usecases share.
type fakeTxRunner struct {
	tasks *fakeTaskRepository
	edges *fakeEdgeRepository
}

func (f fakeTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error) error {
	return fn(ctx, f.tasks, f.edges)
}

func newTestServer(tasks *fakeTaskRepository, edges *fakeEdgeRepository) *Server {
	createTaskUC := usecase.NewCreateTask(tasks)
	addEdgeUC := usecase.NewAddEdge(fakeTxRunner{tasks: tasks, edges: edges})
	// resolvePermissionUC is shared: Grant now requires it internally
	// (TASK-TG-03-01's manage-access check) in addition to the standalone
	// ResolvePermission RPC wiring below.
	resolvePermissionUC := usecase.NewResolvePermission(tasks, tasks, stubTeams{}, stubOPA{})
	return New(
		createTaskUC,
		usecase.NewGetTask(tasks),
		addEdgeUC,
		usecase.NewGrant(tasks, resolvePermissionUC, stubEvents{}),
		resolvePermissionUC,
		usecase.NewExecuteTask(tasks, edges, stubExecutor{}, stubExecutor{}),
		usecase.NewHasActiveExecutions(tasks),
		usecase.NewListTasks(tasks),
		usecase.NewUpdateTask(tasks, edges),
		usecase.NewDeleteTask(tasks),
		usecase.NewGetDependencies(tasks, edges),
		usecase.NewAIDecompose(
			tasks, edges, fakeAIProviderContextResolver{}, fakeProjectExecutionResolver{connectionID: "conn-1", connected: true},
			fakeProjectContextResolver{}, fakeTechStackDetector{}, fakeVelocityResolver{},
			fakeAICompleter{content: `{"subtasks":[{"title":"Do X"}]}`},
		),
		usecase.NewAIApply(fakeTxRunner{tasks: tasks, edges: edges}),
		usecase.NewGenerateAgentPrompt(tasks, fakeAIProviderContextResolver{}, fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, fakeAICompleter{content: "generated prompt text"}),
		usecase.NewRevokeGrant(tasks, resolvePermissionUC, stubEvents{}),
		usecase.NewListGrants(tasks, resolvePermissionUC),
	)
}

type stubEvents struct{}

func (stubEvents) Publish(ctx context.Context, tenantID, eventType string, payload map[string]any) {}

type stubTeams struct{}

func (stubTeams) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}

type stubOPA struct{}

func (stubOPA) Decision(ctx context.Context, level domain.GrantLevel, action, tenantID string) (bool, error) {
	return true, nil
}

type stubExecutor struct{}

func (stubExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	return "ref", nil
}

func TestServer_ListTasks(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Title: "a"}
	tasks.tasks["t2"] = domain.Task{ID: "t2", TenantID: "tenant-1", Title: "b"}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	resp, err := s.ListTasks(ctxWithTenant(t), &taskv1.ListTasksRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetTasks()) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(resp.GetTasks()))
	}
}

func TestServer_UpdateTask(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Title: "old", Status: domain.StatusOpen}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	resp, err := s.UpdateTask(ctxWithTenant(t), &taskv1.UpdateTaskRequest{
		Id:    "t1",
		Title: wrapperString("new"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetTask().GetTitle() != "new" {
		t.Errorf("expected title=new, got %q", resp.GetTask().GetTitle())
	}
}

func TestServer_UpdateTask_RejectsInProgressTransition(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Status: domain.StatusOpen}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	_, err := s.UpdateTask(ctxWithTenant(t), &taskv1.UpdateTaskRequest{
		Id:     "t1",
		Status: wrapperString(domain.StatusInProgress),
	})
	if err == nil {
		t.Fatal("expected an error transitioning into in_progress via UpdateTask")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestServer_DeleteTask(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1"}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	if _, err := s.DeleteTask(ctxWithTenant(t), &taskv1.DeleteTaskRequest{Id: "t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tasks.tasks["t1"]; ok {
		t.Error("expected task to be deleted")
	}
}

func TestServer_GetDependencies(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["a"] = domain.Task{ID: "a", TenantID: "tenant-1", Title: "A"}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{{FromTaskID: "root", ToTaskID: "a", Kind: domain.EdgeKindDependsOn}}}
	s := newTestServer(tasks, edges)

	resp, err := s.GetDependencies(ctxWithTenant(t), &taskv1.GetDependenciesRequest{TaskId: "root"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetDependencies()) != 1 || resp.GetDependencies()[0].GetId() != "a" {
		t.Errorf("unexpected dependencies: %+v", resp.GetDependencies())
	}
}

func TestServer_AIDecompose(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	resp, err := s.AIDecompose(ctxWithTenant(t), &taskv1.AIDecomposeRequest{TaskId: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetProposals()) == 0 {
		t.Error("expected at least one proposal")
	}
}

func TestServer_AIApply(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{}
	s := newTestServer(tasks, edges)

	resp, err := s.AIApply(ctxWithTenant(t), &taskv1.AIApplyRequest{
		TaskId:    "parent",
		Proposals: []*taskv1.SubtaskProposal{{Title: "Design API"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetCreatedSubtasks()) != 1 || resp.GetCreatedSubtasks()[0].GetTitle() != "Design API" {
		t.Errorf("unexpected created subtasks: %+v", resp.GetCreatedSubtasks())
	}
	if len(edges.edges) != 1 || edges.edges[0].Kind != domain.EdgeKindParentChild {
		t.Errorf("expected one parent_child edge, got %+v", edges.edges)
	}
}

// TestServer_AIDecompose_ReturnsRawResponse confirms the widened
// AIDecomposeResponse carries raw_response through, not just proposals.
func TestServer_AIDecompose_ReturnsRawResponse(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	resp, err := s.AIDecompose(ctxWithTenant(t), &taskv1.AIDecomposeRequest{TaskId: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetRawResponse() == "" {
		t.Error("expected a non-empty raw_response")
	}
}

// TestServer_AIApplyAIDecompose_ProposalFieldsSurviveProtoRoundTrip locks
// in TASK-TG-02-06's widened proposal shape: type/estimated_hours/
// depends_on_indices/prompt_template all survive proto <-> domain
// conversion, not just title/description.
func TestServer_AIApplyAIDecompose_ProposalFieldsSurviveProtoRoundTrip(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{}
	s := newTestServer(tasks, edges)

	resp, err := s.AIApply(ctxWithTenant(t), &taskv1.AIApplyRequest{
		TaskId: "parent",
		Proposals: []*taskv1.SubtaskProposal{
			{Title: "Design API", Description: "the design", Type: "task", EstimatedHours: wrapperspb.Double(2.5), PromptTemplate: "do the design"},
		},
		RawAiResponse: `{"subtasks":[]}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetCreatedSubtasks()) != 1 {
		t.Fatalf("expected 1 created subtask, got %+v", resp.GetCreatedSubtasks())
	}
	created := tasks.tasks[resp.GetCreatedSubtasks()[0].GetId()]
	if created.Description != "the design" || created.Type != "task" || created.PromptTemplate != "do the design" {
		t.Errorf("expected widened proposal fields to survive round-trip, got %+v", created)
	}
	if created.EstimatedHours == nil || *created.EstimatedHours != 2.5 {
		t.Errorf("expected EstimatedHours=2.5, got %+v", created.EstimatedHours)
	}
	if tasks.tasks["parent"].AIPlanJSON != `{"subtasks":[]}` {
		t.Errorf("expected raw_ai_response to persist to ai_plan_json, got %q", tasks.tasks["parent"].AIPlanJSON)
	}
}

// ctxWithTenantAndUser is ctxWithTenant plus a caller user ID — needed for
// RPCs that route through Grant's manage-access check (TASK-TG-03-01),
// which resolves the owner-intrinsic short-circuit off the caller's user
// ID, not just the tenant.
func ctxWithTenantAndUser(t *testing.T, userID string) context.Context {
	t.Helper()
	return tenant.WithUserID(ctxWithTenant(t), userID)
}

func TestServer_Grant_ReturnsPersistedID(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	resp, err := s.Grant(ctxWithTenantAndUser(t, "user-1"), &taskv1.GrantRequest{TaskId: "t1", SubjectId: "u2", Level: taskv1.GrantLevel_GRANT_LEVEL_ADMIN})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetId() == "" {
		t.Error("expected a non-empty grant id")
	}
}

func TestServer_RevokeGrant_And_ListGrants(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	s := newTestServer(tasks, &fakeEdgeRepository{})
	ctx := ctxWithTenantAndUser(t, "user-1")

	granted, err := s.Grant(ctx, &taskv1.GrantRequest{TaskId: "t1", SubjectId: "u2", Level: taskv1.GrantLevel_GRANT_LEVEL_USER})
	if err != nil {
		t.Fatalf("unexpected error granting: %v", err)
	}

	listResp, err := s.ListGrants(ctx, &taskv1.ListGrantsRequest{TaskId: "t1"})
	if err != nil {
		t.Fatalf("unexpected error listing: %v", err)
	}
	if len(listResp.GetGrants()) != 1 || listResp.GetGrants()[0].GetId() != granted.GetId() {
		t.Fatalf("unexpected grants: %+v", listResp.GetGrants())
	}

	if _, err := s.RevokeGrant(ctx, &taskv1.RevokeGrantRequest{TaskId: "t1", GrantId: granted.GetId()}); err != nil {
		t.Fatalf("unexpected error revoking: %v", err)
	}

	listResp2, err := s.ListGrants(ctx, &taskv1.ListGrantsRequest{TaskId: "t1"})
	if err != nil {
		t.Fatalf("unexpected error re-listing: %v", err)
	}
	if len(listResp2.GetGrants()) != 0 {
		t.Errorf("expected the grant to be gone after revoke, got %+v", listResp2.GetGrants())
	}
}

func TestServer_GenerateAgentPrompt(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	s := newTestServer(tasks, &fakeEdgeRepository{})

	resp, err := s.GenerateAgentPrompt(ctxWithTenant(t), &taskv1.GenerateAgentPromptRequest{TaskId: "t1", Save: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetPrompt() != "generated prompt text" {
		t.Errorf("expected the generated prompt text to be returned, got %q", resp.GetPrompt())
	}
	if tasks.tasks["t1"].PromptTemplate != "generated prompt text" {
		t.Errorf("expected Save=true to persist PromptTemplate, got %q", tasks.tasks["t1"].PromptTemplate)
	}
}

// TestTaskCreateGetChannels_StillRegistered-equivalent guard for this
// layer: TASK-222's CreateTask/GetTask translation methods must keep
// working after this task's additions.
func TestServer_CreateTaskAndGetTask_StillWork(t *testing.T) {
	s := newTestServer(newFakeTaskRepository(), &fakeEdgeRepository{})

	created, err := s.CreateTask(ctxWithTenant(t), &taskv1.CreateTaskRequest{Title: "x"})
	if err != nil {
		t.Fatalf("unexpected error creating: %v", err)
	}
	got, err := s.GetTask(ctxWithTenant(t), &taskv1.GetTaskRequest{Id: created.GetTask().GetId()})
	if err != nil {
		t.Fatalf("unexpected error getting: %v", err)
	}
	if got.GetTask().GetTitle() != "x" {
		t.Errorf("unexpected task: %+v", got.GetTask())
	}
}

// recordingOPA records the action it was asked to decide on — used to
// prove a real (non-"read") action on the wire actually reaches OPA,
// regression guard against README.md's previously-undocumented
// hardcoded-"read" gap (TASK-TG-03-06).
type recordingOPA struct{ gotAction string }

func (r *recordingOPA) Decision(ctx context.Context, level domain.GrantLevel, action, tenantID string) (bool, error) {
	r.gotAction = action
	return true, nil
}

func TestServer_ResolvePermission_ThreadsRealActionToOPA(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	opa := &recordingOPA{}
	// Constructed directly (not via newTestServer/New) so this test can
	// inject a recording OPA fake without widening every other usecase's
	// wiring just for this one assertion.
	s := &Server{resolvePermission: usecase.NewResolvePermission(tasks, tasks, stubTeams{}, opa)}

	_, err := s.ResolvePermission(ctxWithTenant(t), &taskv1.ResolvePermissionRequest{TaskId: "t1", UserId: "user-1", Action: "manage"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opa.gotAction != "manage" {
		t.Errorf("expected the real wire action %q to reach OPA, got %q", "manage", opa.gotAction)
	}
}
