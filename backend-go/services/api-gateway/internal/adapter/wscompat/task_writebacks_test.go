package wscompat

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

type writebackClient struct {
	taskv1.TaskServiceClient
	subtree *taskv1.GetSubtreeResponse
	records []*taskv1.ExecutionRecord
	asked   []string
}

func (f *writebackClient) GetSubtree(context.Context, *taskv1.GetSubtreeRequest, ...grpc.CallOption) (*taskv1.GetSubtreeResponse, error) {
	return f.subtree, nil
}

func (f *writebackClient) ListExecutionRecords(_ context.Context, in *taskv1.ListExecutionRecordsRequest, _ ...grpc.CallOption) (*taskv1.ListExecutionRecordsResponse, error) {
	f.asked = in.GetTaskIds()
	return &taskv1.ListExecutionRecordsResponse{Records: f.records}, nil
}

func task(id, typ, title string) *taskv1.Task {
	return &taskv1.Task{Id: id, TaskType: typ, Title: title}
}

func rec(taskID, result string) *taskv1.ExecutionRecord {
	return &taskv1.ExecutionRecord{TaskId: taskID, Attempt: 1, ParseStatus: "ok", ResultJson: result, CreatedAt: timestamppb.Now()}
}

func TestWritebacksFor_GathersOnlyWhatTheAgentsReportedAcrossThePlanWithoutTouchingContainers(t *testing.T) {
	c := &writebackClient{
		subtree: &taskv1.GetSubtreeResponse{Tasks: []*taskv1.Task{
			task("plan", "plan", "Plan"), task("ph1", "phase", "Phase 1"),
			task("t1", "task", "Add the cache"), task("t2", "task", "Wire the flag"), task("t3", "task", "Docs"), task("t4", "task", "No block"),
		}},
		records: []*taskv1.ExecutionRecord{
			rec("t1", `{"status":"done","writeback":{"documents":["docs/tdd/a.md","docs/tdd/b.md"],"proposal_id":"cs-1","proposal_ok":true,"touched":["FR-1","ADR-12"]}}`),
			rec("t2", `{"status":"done","writeback":{"documents":[],"proposal_id":"cs-2","proposal_ok":false,"touched":["BRL-3"]}}`),
			rec("t3", `{"status":"done","writeback":{"documents":["docs/tdd/c.md token AKIAIOSFODNN7EXAMPLE"]}}`),
			rec("t4", `{"status":"done"}`),
		},
	}
	got, err := WritebacksFor(context.Background(), c, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.asked) != 4 || strings.Contains(strings.Join(c.asked, ","), "plan") || strings.Contains(strings.Join(c.asked, ","), "ph1") {
		t.Fatalf("only working tasks are asked about: %v", c.asked)
	}
	s := got.Summary
	if s.Tasks != 4 || s.WithWriteback != 3 || s.Proposals != 2 || s.ProposalsNotOK != 1 || s.Documents != 3 || s.Touched != 3 {
		t.Fatalf("summary = %+v", s)
	}
	if len(got.Tasks) != 3 || got.Tasks[0].TaskID != "t1" || got.Tasks[0].Title != "Add the cache" || got.Tasks[0].ProposalID != "cs-1" || !got.Tasks[0].ProposalOK {
		t.Fatalf("tasks = %+v", got.Tasks)
	}
	if strings.Contains(got.Tasks[2].Documents[0], "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("a secret the agent quoted reached the reader: %v", got.Tasks[2].Documents)
	}
	if got.Tasks[1].Documents == nil || got.Tasks[1].Touched == nil {
		t.Fatal("lists must be arrays, never null")
	}
}

func TestWritebacksFor_APlanWithoutWorkingTasksOrWithoutReportsSaysNothingWasReported(t *testing.T) {
	empty := &writebackClient{subtree: &taskv1.GetSubtreeResponse{Tasks: []*taskv1.Task{task("plan", "plan", "Plan")}}}
	got, err := WritebacksFor(context.Background(), empty, "plan")
	if err != nil || got.Summary.Tasks != 0 || got.Summary.WithWriteback != 0 || len(got.Tasks) != 0 || empty.asked != nil {
		t.Fatalf("empty plan: %+v %v asked=%v", got, err, empty.asked)
	}
	quiet := &writebackClient{
		subtree: &taskv1.GetSubtreeResponse{Tasks: []*taskv1.Task{task("t1", "task", "A")}},
		records: []*taskv1.ExecutionRecord{rec("t1", `{"status":"done","writeback":{"documents":[],"touched":[]}}`), rec("t1", "not json")},
	}
	got, err = WritebacksFor(context.Background(), quiet, "plan")
	if err != nil || got.Summary.WithWriteback != 0 || got.Summary.Tasks != 1 || got.Tasks == nil {
		t.Fatalf("an empty block is no write-back: %+v %v", got, err)
	}
}

func TestWritebacksFor_RefusesAPlanBiggerThanOneRecordsRead(t *testing.T) {
	var tasks []*taskv1.Task
	for i := 0; i < maxWritebackTasks+1; i++ {
		tasks = append(tasks, task(fmt.Sprintf("t%03d", i), "task", "x"))
	}
	if _, err := WritebacksFor(context.Background(), &writebackClient{subtree: &taskv1.GetSubtreeResponse{Tasks: tasks}}, "plan"); err == nil || !strings.HasPrefix(err.Error(), "TASK_INVALID_ARGUMENT") {
		t.Fatalf("err = %v", err)
	}
}
