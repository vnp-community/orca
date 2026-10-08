package grpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

type memSpecs struct{ specs map[string]domain.TaskSpec }

func (m *memSpecs) Upsert(_ context.Context, s domain.TaskSpec, expected int64) (domain.TaskSpec, error) {
	cur, ok := m.specs[s.TaskID]
	if ok && cur.IsLocked() {
		return domain.TaskSpec{}, domain.ErrTaskSpecLocked
	}
	if (ok && cur.Version != expected) || (!ok && expected != 0) {
		return domain.TaskSpec{}, domain.ErrTaskSpecVersionConflict
	}
	s.Version = expected + 1
	m.specs[s.TaskID] = s
	return s, nil
}

func (m *memSpecs) GetMany(_ context.Context, _ string, ids []string) ([]domain.TaskSpec, error) {
	var out []domain.TaskSpec
	for _, id := range ids {
		if s, ok := m.specs[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memSpecs) LockSubtree(_ context.Context, _ string, ids []string, at time.Time) (int, error) {
	n := 0
	for _, id := range ids {
		if s, ok := m.specs[id]; ok && !s.IsLocked() {
			s.LockedAt = &at
			m.specs[id] = s
			n++
		}
	}
	return n, nil
}
func (m *memSpecs) IsLocked(context.Context, string, string) (bool, error) { return false, nil }
func (m *memSpecs) HasSpec(context.Context, string, string) (bool, error)  { return false, nil }

type memRecords struct{ recs []domain.ExecutionRecord }

func (m *memRecords) InsertExecutionRecord(_ context.Context, r domain.ExecutionRecord) (domain.ExecutionRecord, error) {
	return r, nil
}

func (m *memRecords) ListExecutionRecords(context.Context, string, []string, bool, int) ([]domain.ExecutionRecord, error) {
	return m.recs, nil
}

func newSpecServer(t *testing.T) (*Server, *memRecords) {
	t.Helper()
	tasks := newFakeTaskRepository()
	for _, id := range []string{"plan", "t1"} {
		parent := ""
		if id == "t1" {
			parent = "plan"
		}
		task, err := domain.NewTask(id, "tenant-1", id, domain.StatusOpen, parent, "p")
		if err != nil {
			t.Fatal(err)
		}
		tasks.tasks[id] = task
	}
	specs := &memSpecs{specs: map[string]domain.TaskSpec{}}
	recs := &memRecords{}
	srv := newTestServer(tasks, &fakeEdgeRepository{}).
		WithTaskSpecs(usecase.NewSetTaskSpec(tasks, specs, nil), usecase.NewGetTaskSpecs(specs), usecase.NewLockTaskSpecs(tasks, specs, nil)).
		WithExecutionRecords(usecase.NewListExecutionRecords(recs, nil))
	return srv, recs
}

func TestServer_TaskSpec_SetGetLockRoundTrip(t *testing.T) {
	srv, _ := newSpecServer(t)
	ctx := ctxWithTenant(t)
	set, err := srv.SetTaskSpec(ctx, &taskv1.SetTaskSpecRequest{TaskId: "t1", SchemaVersion: 1, SpecJson: `{"b":1,"a":2}`})
	if err != nil || set.GetSpec().GetSpecJson() != `{"a":2,"b":1}` || set.GetSpec().GetVersion() != 1 || set.GetSpec().GetLocked() {
		t.Fatalf("set: %+v %v", set, err)
	}
	got, err := srv.GetTaskSpecs(ctx, &taskv1.GetTaskSpecsRequest{TaskIds: []string{"t1", "plan"}})
	if err != nil || len(got.GetSpecs()) != 1 || got.GetSpecs()[0].GetDigest() != set.GetSpec().GetDigest() {
		t.Fatalf("get: %+v %v", got, err)
	}
	lock, err := srv.LockTaskSpecs(ctx, &taskv1.LockTaskSpecsRequest{PlanTaskId: "plan"})
	if err != nil || lock.GetLocked() != 1 {
		t.Fatalf("lock: %+v %v", lock, err)
	}
	_, err = srv.SetTaskSpec(ctx, &taskv1.SetTaskSpecRequest{TaskId: "t1", SchemaVersion: 1, SpecJson: `{"a":9}`, ExpectedVersion: 1})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("locked write must be FailedPrecondition, got %v", err)
	}
}

func TestServer_TaskSpec_ErrorCodes(t *testing.T) {
	srv, _ := newSpecServer(t)
	ctx := ctxWithTenant(t)
	if _, err := srv.SetTaskSpec(ctx, &taskv1.SetTaskSpecRequest{TaskId: "t1", SchemaVersion: 1, SpecJson: `[]`}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid spec: %v", err)
	}
	_, _ = srv.SetTaskSpec(ctx, &taskv1.SetTaskSpecRequest{TaskId: "t1", SchemaVersion: 1, SpecJson: `{}`})
	if _, err := srv.SetTaskSpec(ctx, &taskv1.SetTaskSpecRequest{TaskId: "t1", SchemaVersion: 1, SpecJson: `{"x":1}`, ExpectedVersion: 5}); status.Code(err) != codes.Aborted {
		t.Fatalf("version conflict must be Aborted: %v", err)
	}
}

func TestServer_TaskSpec_UnimplementedWhenNotWired(t *testing.T) {
	srv := newTestServer(newFakeTaskRepository(), &fakeEdgeRepository{})
	if _, err := srv.GetTaskSpecs(ctxWithTenant(t), &taskv1.GetTaskSpecsRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v", err)
	}
	if _, err := srv.ListExecutionRecords(ctxWithTenant(t), &taskv1.ListExecutionRecordsRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v", err)
	}
}

func TestServer_ListExecutionRecords_MapsFields(t *testing.T) {
	srv, recs := newSpecServer(t)
	now := time.Now().UTC()
	recs.recs = []domain.ExecutionRecord{{ID: "r1", TaskID: "t1", ExecutionLinkID: "l1", Attempt: 2, SpecDigest: "sd", PacketDigest: "pd",
		TemplateVersion: "v1", ParseStatus: domain.ParseStatusInvalid, FailureClass: domain.FailureAgentDefect,
		Result: []byte(`{"a":1}`), Changes: []byte(`{"f":[]}`), StdoutTail: "tail", CreatedAt: now}}
	got, err := srv.ListExecutionRecords(ctxWithTenant(t), &taskv1.ListExecutionRecordsRequest{TaskIds: []string{"t1"}})
	if err != nil || len(got.GetRecords()) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	r := got.GetRecords()[0]
	if r.GetId() != "r1" || r.GetAttempt() != 2 || r.GetParseStatus() != "invalid" || r.GetFailureClass() != "agent_defect" ||
		r.GetResultJson() != `{"a":1}` || r.GetChangesJson() != `{"f":[]}` || r.GetStdoutTail() != "tail" || !r.GetCreatedAt().AsTime().Equal(now) {
		t.Fatalf("mapping: %+v", r)
	}
	if _, err := srv.ListExecutionRecords(ctxWithTenant(t), &taskv1.ListExecutionRecordsRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty ids must be InvalidArgument, got %v", err)
	}
}
