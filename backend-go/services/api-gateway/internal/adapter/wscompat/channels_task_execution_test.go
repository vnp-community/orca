package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// executionTaskClient answers the two task-service reads execution.get makes.
type executionTaskClient struct {
	taskv1.TaskServiceClient
	records    []*taskv1.ExecutionRecord
	recordsErr error
	output     *taskv1.GetTaskExecutionOutputResponse
	outputErr  error
	outputAsk  int
}

func (f *executionTaskClient) ListExecutionRecords(context.Context, *taskv1.ListExecutionRecordsRequest, ...grpc.CallOption) (*taskv1.ListExecutionRecordsResponse, error) {
	return &taskv1.ListExecutionRecordsResponse{Records: f.records}, f.recordsErr
}

func (f *executionTaskClient) GetTaskExecutionOutput(context.Context, *taskv1.GetTaskExecutionOutputRequest, ...grpc.CallOption) (*taskv1.GetTaskExecutionOutputResponse, error) {
	f.outputAsk++
	return f.output, f.outputErr
}

func executionGet(t *testing.T, c *executionTaskClient) (map[string]any, error) {
	t.Helper()
	r := NewRegistry()
	registerTaskExecutionChannels(r, c)
	res, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "u1"}, "execution.get", argsJSON(t, map[string]any{"taskId": "t1", "latestOnly": true}))
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(res)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out, nil
}

func TestExecutionGet_ContractRecordMapsToTheResultTabShape(t *testing.T) {
	c := &executionTaskClient{records: []*taskv1.ExecutionRecord{
		{TaskId: "t1", Attempt: 1, ParseStatus: "failed-old", CreatedAt: timestamppb.New(timestamppb.Now().AsTime().Add(-3600e9))},
		{
			TaskId: "t1", Attempt: 2, ParseStatus: "ok", FailureClass: "",
			ResultJson:  `{"schema_version":1,"status":"done","summary":"added x; token AKIAIOSFODNN7EXAMPLE","files_changed":["a.go","b.go"],"checks_run":[{"id":"unit","exit":0}],"outputs":{"url":"https://x","k":["AKIAIOSFODNN7EXAMPLE"]}}`,
			ChangesJson: `{"available":true,"changedFiles":[{"path":"b.go","change":"modified"},{"path":"c.go","change":"added"}]}`,
			StdoutTail:  "ran ok",
			CreatedAt:   timestamppb.Now(),
		},
	}}
	got, err := executionGet(t, c)
	if err != nil {
		t.Fatal(err)
	}
	if got["taskId"] != "t1" || got["attempt"] != float64(2) || got["parseStatus"] != "ok" || got["status"] != "done" || got["source"] != "execution_record" {
		t.Fatalf("shape: %v", got)
	}
	if files := got["filesChanged"].([]any); len(files) != 3 || files[0] != "b.go" || files[1] != "c.go" || files[2] != "a.go" {
		t.Fatalf("observed changes first, then what the agent reported, no duplicates: %v", files)
	}
	if checks := got["checksRun"].([]any); len(checks) != 1 || checks[0].(map[string]any)["id"] != "unit" {
		t.Fatalf("checks: %v", checks)
	}
	if blob, _ := json.Marshal(got); strings.Contains(string(blob), "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("secrets must be masked in the summary and outputs: %s", blob)
	}
	if c.outputAsk != 0 {
		t.Fatal("a record answers the call; the raw output is not read")
	}
}

func TestExecutionGet_RunWithoutAResultBlockShowsTheStoredOutput(t *testing.T) {
	c := &executionTaskClient{output: &taskv1.GetTaskExecutionOutputResponse{Found: true, Output: "analysis finished", Redacted: true}}
	got, err := executionGet(t, c)
	if err != nil {
		t.Fatal(err)
	}
	if got["parseStatus"] != "missing" || got["stdoutTail"] != "analysis finished" || got["source"] != "last_execution_output" || got["redacted"] != true {
		t.Fatalf("got %v", got)
	}
	if got["filesChanged"] == nil || got["checksRun"] == nil {
		t.Fatalf("lists are present so the parser needs no special case: %v", got)
	}
}

func TestExecutionGet_NothingToShowIsNotFound(t *testing.T) {
	c := &executionTaskClient{output: &taskv1.GetTaskExecutionOutputResponse{}}
	if _, err := executionGet(t, c); err == nil || !strings.HasPrefix(err.Error(), "TASK_NOT_FOUND:") {
		t.Fatalf("the UI maps TASK_NOT_FOUND to its legacy state: %v", err)
	}
}

func TestExecutionGet_OlderTaskServiceStillAnswers(t *testing.T) {
	c := &executionTaskClient{recordsErr: status.Error(codes.Unimplemented, "x"), outputErr: status.Error(codes.Unimplemented, "x")}
	if _, err := executionGet(t, c); err == nil || !strings.HasPrefix(err.Error(), "TASK_NOT_FOUND:") {
		t.Fatalf("got %v", err)
	}
}

func TestExecutionGet_PermissionAndInternalDetailsStayOut(t *testing.T) {
	c := &executionTaskClient{recordsErr: status.Error(codes.PermissionDenied, "TASK_NO_GRANT: secret internal detail")}
	_, err := executionGet(t, c)
	if err == nil || !strings.HasPrefix(err.Error(), "TASK_FORBIDDEN:") || strings.Contains(err.Error(), "internal detail") {
		t.Fatalf("got %v", err)
	}
	c = &executionTaskClient{output: nil, outputErr: status.Error(codes.Internal, "dsn=postgres://user:pw@db")}
	_, err = executionGet(t, c)
	if err == nil || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("got %v", err)
	}
}

func TestExecutionGet_TaskIDIsRequired(t *testing.T) {
	r := NewRegistry()
	registerTaskExecutionChannels(r, &executionTaskClient{})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t"}, "execution.get", argsJSON(t, map[string]any{})); err == nil {
		t.Fatal("expected an error")
	}
}

func TestExecutionGet_ShowsWhatTheAgentReadAndTouchedInTheKnowledgeGraph_RedactedAndNeverNull(t *testing.T) {
	c := &executionTaskClient{records: []*taskv1.ExecutionRecord{{
		TaskId: "t1", Attempt: 1, ParseStatus: "ok", CreatedAt: timestamppb.Now(),
		ResultJson: `{"schema_version":1,"status":"done","summary":"s","checks_run":[{"id":"go test","exit":0}],` +
			`"knowledge":{"available":true,"snapshot":"snap-1","read":["FR-1"],"gaps":["token AKIAIOSFODNN7EXAMPLE missing"]},` +
			`"writeback":{"documents":["docs/tdd/a.md"],"proposal_id":"cs-1","proposal_ok":true}}`,
	}}}
	out, err := executionGet(t, c)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := out["knowledge"].(map[string]any)
	w, _ := out["writeback"].(map[string]any)
	if k["available"] != true || k["snapshot"] != "snap-1" || len(k["read"].([]any)) != 1 || k["conflicts"] == nil {
		t.Fatalf("knowledge = %v", k)
	}
	if gaps := k["gaps"].([]any); len(gaps) != 1 || strings.Contains(gaps[0].(string), "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("a secret the agent quoted must not reach the reader: %v", gaps)
	}
	if w["proposalId"] != "cs-1" || w["proposalOk"] != true || w["touched"] == nil || len(w["documents"].([]any)) != 1 {
		t.Fatalf("writeback = %v", w)
	}
	// A run that reported neither block shows neither.
	c.records[0].ResultJson = `{"schema_version":1,"status":"done","summary":"s"}`
	out, _ = executionGet(t, c)
	if _, has := out["knowledge"]; has {
		t.Fatalf("knowledge appeared without a report: %v", out)
	}
}
