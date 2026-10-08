package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeTasks struct {
	tasks []domain.TaskView
	err   error
	state map[string]usecase.ExecutionStateView
}

func (f *fakeTasks) ListTasks(context.Context, usecase.ListTasksQuery) ([]domain.TaskView, error) {
	return f.tasks, f.err
}
func (f *fakeTasks) GetSubtree(context.Context, string) (usecase.SubtreeView, error) {
	return usecase.SubtreeView{}, f.err
}
func (f *fakeTasks) Execute(context.Context, string, string) error     { return f.err }
func (f *fakeTasks) SetWorktree(context.Context, string, string) error { return f.err }
func (f *fakeTasks) SetStatus(context.Context, string, string) error   { return f.err }
func (f *fakeTasks) ListExecutionStates(context.Context, []string) (map[string]usecase.ExecutionStateView, error) {
	return f.state, f.err
}

type fakeBacklogReader struct{ reqs []domain.Request }

func (f *fakeBacklogReader) ListReturnedRequests(_ context.Context, _ string, flt usecase.BacklogRequestFilter) ([]domain.Request, error) {
	return f.reqs, nil
}
func (f *fakeBacklogReader) ListByStatus(context.Context, string, []domain.RequestStatus, usecase.BacklogRequestFilter) ([]domain.Request, error) {
	return f.reqs, nil
}
func (f *fakeBacklogReader) ParentRequestIDs(context.Context, string, []string) (map[string][]string, error) {
	return map[string][]string{"req-1": {"parent-9"}}, nil
}
func (f *fakeBacklogReader) LatestReturns(context.Context, string, []string) (map[string]domain.ReturnEvent, error) {
	return map[string]domain.ReturnEvent{"req-1": {ActorID: "u-3", At: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}}, nil
}

type fakeGates struct{ approvals []domain.Approval }

func (f *fakeGates) ListGateApprovals(context.Context, string, []string) ([]domain.Approval, error) {
	return f.approvals, nil
}

type fakeOutcomes struct {
	usecase.TaskRunOutcomeRepository
}

func (fakeOutcomes) LatestFailed(context.Context, []string) (map[string]domain.TaskRunOutcome, error) {
	return map[string]domain.TaskRunOutcome{"task-1": {ErrorMessage: "agent crashed"}}, nil
}

type everyoneSees struct{}

func (everyoneSees) Filter(_ context.Context, _ domain.DecisionActor, reqs []domain.Request) ([]domain.Request, error) {
	return reqs, nil
}

func backlogServer(tasks *fakeTasks, reqs []domain.Request, approvals []domain.Approval) *Server {
	reader := &fakeBacklogReader{reqs: reqs}
	return newTestServer(&stubRequestRepository{}).WithExecution(ExecutionUseCases{Backlog: &usecase.ListBacklog{
		Requests: &usecase.ListBacklogRequests{Reader: reader, Visibility: everyoneSees{}},
		Tasks:    &usecase.ListBacklogTasks{Requests: reader, Approvals: &fakeGates{approvals: approvals}, Tasks: tasks, Outcomes: fakeOutcomes{}, Visibility: everyoneSees{}},
	}})
}

func TestServer_ListBacklog_MapsRowsAndGroups(t *testing.T) {
	updated := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	backlogged := domain.Request{
		ID: "req-1", ProjectID: "proj-1", Number: 42, Title: "Fix login", Type: domain.RequestTypeBug, ReporterID: "rep-1", Status: domain.RequestStatusRequestBacklog,
		ReturnedFromStage: domain.ReturnStageTask, ReturnedCategory: domain.ReturnCategoryRejected, ReturnReason: "needs spec", Urgency: domain.UrgencyNormal,
		SourceProvider: domain.SourceProviderJira, SourceRef: "ENG-1", SourceURL: "https://x/ENG-1", UpdatedAt: updated,
	}
	srv := backlogServer(&fakeTasks{}, []domain.Request{backlogged}, nil)
	out, err := srv.ListBacklog(tenantCtx(), &requestv1.ListBacklogRequest{View: requestv1.BacklogView_BACKLOG_VIEW_REQUEST, TenantId: "ignored", UserId: "ignored"})
	if err != nil || len(out.GetRequestRows()) != 1 {
		t.Fatalf("got %v %v", out, err)
	}
	row := out.GetRequestRows()[0]
	if row.GetId() != "req-1" || row.GetNumber() != 42 || row.GetTitle() != "Fix login" || row.GetStage() != "task" || row.GetReturnedCategory() != "rejected" ||
		row.GetReturnedBy() != "u-3" || row.GetReturnedAt() == nil || row.GetParentRequestIds()[0] != "parent-9" || row.GetReturnReason() != "needs spec" ||
		row.GetSourceProvider() != "jira" || row.GetSourceRef() != "ENG-1" || row.GetPriority() != "normal" || !row.GetUpdatedAt().AsTime().Equal(updated) {
		t.Fatalf("row not mapped: %v", row)
	}

	running := domain.Request{ID: "req-2", Type: domain.RequestTypeTask, Status: domain.RequestStatusExecuting}
	hours := 2.5
	tasks := &fakeTasks{
		tasks: []domain.TaskView{
			{ID: "plan-1", Type: domain.TaskTypePlan, RequestID: "req-2", Title: "The plan", Status: "open"},
			{ID: "task-1", Type: "task", RequestID: "req-2", ParentID: "plan-1", Title: "Do it", Status: "open", AssigneeID: "al", EstimatedHours: &hours},
		},
		state: map[string]usecase.ExecutionStateView{"task-1": {LastEngine: "direct_agent", LastLinkStatus: "failed", FailedAttempts: 2, BlockedByTaskIDs: []string{"task-0"}}},
	}
	approvals := []domain.Approval{{ID: "a1", RequestID: "req-2", SubjectType: domain.SubjectTaskList, SubjectID: "plan-1", Status: domain.ApprovalStatusApproved}}
	srv = backlogServer(tasks, []domain.Request{running}, approvals)
	exec, err := srv.ListBacklog(tenantCtx(), &requestv1.ListBacklogRequest{View: requestv1.BacklogView_BACKLOG_VIEW_EXECUTE})
	if err != nil || len(exec.GetExecuteGroups()) != 1 || len(exec.GetTaskGroups()) != 0 {
		t.Fatalf("got %v %v", exec, err)
	}
	g := exec.GetExecuteGroups()[0]
	tr := g.GetTasks()[0]
	if g.GetRequestId() != "req-2" || g.GetPlanId() != "plan-1" || g.GetPlanTitle() != "The plan" || g.GetGateStatus() != "approved" || g.GetTotalTasks() != 1 ||
		tr.GetId() != "task-1" || tr.GetEstimatedHours().GetValue() != 2.5 || tr.GetLastEngine() != "direct_agent" || tr.GetFailedAttempts() != 2 ||
		tr.GetLastError() != "agent crashed" || tr.GetBlockedByTaskIds()[0] != "task-0" || tr.GetAssigneeId() != "al" {
		t.Fatalf("group not mapped: %v", g)
	}

	// Nothing approved: the same task shows in TASK, not EXECUTE.
	srv = backlogServer(tasks, []domain.Request{running}, nil)
	view, _ := srv.ListBacklog(tenantCtx(), &requestv1.ListBacklogRequest{View: requestv1.BacklogView_BACKLOG_VIEW_TASK})
	if len(view.GetTaskGroups()) != 1 || len(view.GetExecuteGroups()) != 0 || view.GetTaskGroups()[0].GetGateStatus() != "none" {
		t.Fatalf("got %v", view)
	}
}

func TestServer_ListBacklog_ErrorCodes(t *testing.T) {
	running := domain.Request{ID: "req-2", Type: domain.RequestTypeTask, Status: domain.RequestStatusExecuting}
	cases := []struct {
		name string
		srv  *Server
		req  *requestv1.ListBacklogRequest
		want codes.Code
		code string
	}{
		{"unspecified view", backlogServer(&fakeTasks{}, nil, nil), &requestv1.ListBacklogRequest{}, codes.InvalidArgument, "REQUEST_BACKLOG_INVALID_VIEW"},
		{"bad token", backlogServer(&fakeTasks{}, nil, nil), &requestv1.ListBacklogRequest{View: requestv1.BacklogView_BACKLOG_VIEW_REQUEST, PageToken: "###"}, codes.InvalidArgument, "REQUEST_BACKLOG_BAD_PAGE_TOKEN"},
		{"task service down", backlogServer(&fakeTasks{err: domain.ErrTaskServiceDown}, []domain.Request{running}, nil), &requestv1.ListBacklogRequest{View: requestv1.BacklogView_BACKLOG_VIEW_TASK}, codes.Unavailable, "REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.srv.ListBacklog(tenantCtx(), tc.req)
			if status.Code(err) != tc.want || !executionContains(status.Convert(err).Message(), tc.code) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func executionContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type memChecks struct{ rows []domain.RequestCheck }

func (m *memChecks) Append(_ context.Context, c domain.RequestCheck) (domain.RequestCheck, error) {
	c.CreatedAt = time.Date(2026, 10, 4, 0, 0, len(m.rows), 0, time.UTC)
	m.rows = append(m.rows, c)
	return c, nil
}
func (m *memChecks) ListByRequest(context.Context, string) ([]domain.RequestCheck, error) {
	return m.rows, nil
}
func (m *memChecks) Latest(context.Context, string, domain.CheckKind) (domain.RequestCheck, bool, error) {
	return domain.RequestCheck{}, false, nil
}

type allowWrite struct{}

func (allowWrite) CanWrite(context.Context, domain.Request) error { return nil }

func TestServer_RecordAndListRequestChecks(t *testing.T) {
	req := domain.Request{ID: "req-1", Type: domain.RequestTypePerformance, Status: domain.RequestStatusExecuting, ReporterID: "u1"}
	repo := &stubRequestRepository{byID: map[string]domain.Request{"req-1": req}}
	checks := &memChecks{}
	srv := NewServer(usecase.NewGetRequest(repo), usecase.NewListRequests(repo)).WithExecution(ExecutionUseCases{
		RecordCheck: &usecase.RecordRequestCheck{Requests: repo, Checks: checks, Authorizer: allowWrite{}},
		ListChecks:  &usecase.ListRequestChecks{Requests: repo, Checks: checks, Visibility: everyoneSees{}},
	})
	ctx := tenant.WithUserID(tenantCtx(), "u1")
	rec, err := srv.RecordRequestCheck(ctx, &requestv1.RecordRequestCheckRequest{
		RequestId: "req-1", Kind: "perf_after", Status: "passed", MetricsJson: `{"metrics":[{"name":"p95","value":150}]}`, Summary: "wrk", TaskId: "t-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := rec.GetCheck()
	if c.GetKind() != "perf_after" || c.GetSource() != "manual" || c.GetRecordedBy() != "u1" || c.GetTaskId() != "t-1" || c.GetCreatedAt() == nil || c.GetId() == "" {
		t.Fatalf("check not mapped: %v", c)
	}
	list, err := srv.ListRequestChecks(ctx, &requestv1.ListRequestChecksRequest{RequestId: "req-1"})
	if err != nil || len(list.GetChecks()) != 1 || list.GetChecks()[0].GetMetricsJson() == "" {
		t.Fatalf("got %v %v", list, err)
	}

	_, err = srv.RecordRequestCheck(ctx, &requestv1.RecordRequestCheckRequest{RequestId: "req-1", Kind: "perf_after", Status: "passed", MetricsJson: `{"metrics":[]}`})
	if status.Code(err) != codes.InvalidArgument || !executionContains(err.Error(), "REQUEST_CHECK_INVALID_METRICS") {
		t.Fatalf("got %v", err)
	}
	_, err = srv.RecordRequestCheck(ctx, &requestv1.RecordRequestCheckRequest{RequestId: "req-1", Kind: "tests_after", Status: "passed", MetricsJson: `{}`})
	if status.Code(err) != codes.FailedPrecondition || !executionContains(err.Error(), "REQUEST_CHECK_NOT_ALLOWED_NOW") {
		t.Fatalf("got %v", err)
	}
	_, err = srv.RecordRequestCheck(ctx, &requestv1.RecordRequestCheckRequest{RequestId: "nope", Kind: "perf_after", Status: "passed"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("got %v", err)
	}
}

func TestServer_ExecutionRPCsUnimplementedWhenNotWired(t *testing.T) {
	srv := newTestServer(&stubRequestRepository{})
	if _, err := srv.StartPhase(tenantCtx(), &requestv1.StartPhaseRequest{}); status.Code(err) != codes.Unimplemented {
		t.Errorf("StartPhase: %v", err)
	}
	if _, err := srv.ReportTaskOutcome(tenantCtx(), &requestv1.ReportTaskOutcomeRequest{}); status.Code(err) != codes.Unimplemented {
		t.Errorf("ReportTaskOutcome: %v", err)
	}
	if _, err := srv.RecordRequestCheck(tenantCtx(), &requestv1.RecordRequestCheckRequest{}); status.Code(err) != codes.Unimplemented {
		t.Errorf("RecordRequestCheck: %v", err)
	}
	if _, err := srv.ListRequestChecks(tenantCtx(), &requestv1.ListRequestChecksRequest{}); status.Code(err) != codes.Unimplemented {
		t.Errorf("ListRequestChecks: %v", err)
	}
}

type notExecutingReader struct{}

func (notExecutingReader) Get(_ context.Context, id string) (domain.Request, error) {
	return domain.Request{ID: id, Status: domain.RequestStatusPlanning}, nil
}

func TestServer_StartPhase_ErrorCodes(t *testing.T) {
	srv := newTestServer(&stubRequestRepository{}).WithExecution(ExecutionUseCases{
		StartPhase: &usecase.StartPhase{Requests: notExecutingReader{}, Tasks: &fakeTasks{}},
	})
	ctx := tenant.WithUserID(tenantCtx(), "u1")
	_, err := srv.StartPhase(ctx, &requestv1.StartPhaseRequest{RequestId: "r", PhaseTaskId: "p"})
	if status.Code(err) != codes.FailedPrecondition || !executionContains(err.Error(), "REQUEST_NOT_EXECUTING") {
		t.Fatalf("got %v", err)
	}
	if _, err := srv.StartPhase(tenantCtx(), &requestv1.StartPhaseRequest{RequestId: "r"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("no user: %v", err)
	}
}

type orphanReader struct{}

func (orphanReader) Get(_ context.Context, id string) (domain.Request, error) {
	return domain.Request{}, domain.ErrRequestNotFound(id)
}

func TestServer_ReportTaskOutcome(t *testing.T) {
	srv := newTestServer(&stubRequestRepository{}).WithExecution(ExecutionUseCases{
		ReportOutcome: &usecase.ReportTaskOutcome{Requests: orphanReader{}, Tasks: &fakeTasks{}},
	})
	good := &requestv1.ReportTaskOutcomeRequest{EventId: "e1", RequestId: "r1", TaskId: "t1", TaskType: "task", Cause: "execution_failed", NewStatus: "open"}
	if _, err := srv.ReportTaskOutcome(tenantCtx(), good); err != nil {
		t.Fatalf("an orphan request is ignored, not an error: %v", err)
	}
	if _, err := srv.ReportTaskOutcome(tenantCtx(), &requestv1.ReportTaskOutcomeRequest{RequestId: "r1"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing ids: %v", err)
	}
	if _, err := srv.ReportTaskOutcome(context.Background(), good); err == nil {
		t.Fatal("a tenant is required")
	}
}
