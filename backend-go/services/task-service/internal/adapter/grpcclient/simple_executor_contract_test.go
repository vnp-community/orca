package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const contractNonce = "abcdef0123456789XYZ"

type memoryRecords struct {
	recs      []domain.ExecutionRecord
	insertErr error
}

func (m *memoryRecords) InsertExecutionRecord(ctx context.Context, r domain.ExecutionRecord) (domain.ExecutionRecord, error) {
	if m.insertErr != nil {
		return domain.ExecutionRecord{}, m.insertErr
	}
	r.ID = "rec-1"
	m.recs = append(m.recs, r)
	return r, nil
}

func (m *memoryRecords) ListExecutionRecords(context.Context, string, []string, bool, int) ([]domain.ExecutionRecord, error) {
	return nil, nil
}

func contractExec(t *testing.T, relay *fakeInfraFleetServiceClient) (*SimpleExecutor, *memoryRecords) {
	t.Helper()
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "T"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/wt", connected: true}
	recs := &memoryRecords{}
	return newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay).WithExecutionRecords(recs), recs
}

func contractIn() usecase.ContractExecuteInput {
	return usecase.ContractExecuteInput{TenantID: "tenant-1", TaskID: "t1", RequestID: "req:r1:t1:2", WorktreePath: "/srv/wt",
		Prompt: "implement it", ResultNonce: contractNonce, ExecutionLinkID: "link-1", Attempt: 2}
}

func relayWith(result map[string]any) *fakeInfraFleetServiceClient {
	b, _ := json.Marshal(result)
	return &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: string(b)}}
}

const doneBody = `{"schema_version":1,"status":"done","summary":"ok"}`

func stdoutWith(body string) string {
	return "working...\nORCA_RESULT_BEGIN " + contractNonce + "\n" + body + "\nORCA_RESULT_END " + contractNonce + "\n"
}

func TestExecuteWithContract_SendsResultBlockAndReportChanges(t *testing.T) {
	relay := relayWith(map[string]any{"stdout": stdoutWith(doneBody), "exitCode": 0})
	exec, _ := contractExec(t, relay)
	if _, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn()); err != nil {
		t.Fatal(err)
	}
	var sent agentExecPromptParams
	if err := json.Unmarshal([]byte(relay.gotRelay.GetParamsJson()), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.ResultBlock == nil || sent.ResultBlock.Nonce != contractNonce || !sent.ReportChanges || sent.MaxOutputBytes != 1<<20 {
		t.Fatalf("params: %+v", sent)
	}
	if sent.Prompt != "implement it" || sent.TrustPreset != "full" {
		t.Fatalf("prompt/trust: %+v", sent)
	}
}

func TestExecuteWithContract_RequiresRequestPrefixAndPrompt(t *testing.T) {
	relay := relayWith(map[string]any{"stdout": "", "exitCode": 0})
	exec, _ := contractExec(t, relay)
	in := contractIn()
	in.RequestID = "plain-1"
	_, err := exec.ExecuteWithContract(ctxWithTenant(t), in)
	if err == nil || !strings.Contains(err.Error(), "TASK_EXECUTE_CONTRACT_REQUIRES_REQUEST") {
		t.Fatalf("got %v", err)
	}
	in = contractIn()
	in.Prompt = "  "
	if _, err = exec.ExecuteWithContract(ctxWithTenant(t), in); err == nil || !strings.Contains(err.Error(), "TASK_EXECUTE_PROMPT_REQUIRED") {
		t.Fatalf("got %v", err)
	}
	if relay.gotRelay != nil {
		t.Fatal("relay must not be called when the preconditions fail (no full trust without them)")
	}
}

func TestExecuteWithContract_PersistsRecordWithTail16KB(t *testing.T) {
	noise := strings.Repeat("Nguyễn ", 5000)
	relay := relayWith(map[string]any{"stdout": noise + stdoutWith(doneBody), "exitCode": 0, "changes": map[string]any{"files": []string{"a.go"}}})
	exec, recs := contractExec(t, relay)
	out, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if err != nil || out.Record.ID != "rec-1" || len(recs.recs) != 1 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	r := recs.recs[0]
	if r.ParseStatus != domain.ParseStatusOK || r.FailureClass != "" || r.Attempt != 2 || r.ExecutionLinkID != "link-1" {
		t.Fatalf("record: %+v", r)
	}
	if len(r.StdoutTail) > domain.MaxStdoutTailBytes || !strings.Contains(r.StdoutTail, "ORCA_RESULT_END") {
		t.Fatalf("tail len=%d", len(r.StdoutTail))
	}
	if string(r.Changes) != `{"files":["a.go"]}` || len(r.Result) == 0 {
		t.Fatalf("changes=%s result=%s", r.Changes, r.Result)
	}
}

func TestExecuteWithContract_ParsedFromAgent(t *testing.T) {
	relay := relayWith(map[string]any{"stdout": "no markers here", "exitCode": 0,
		"parsed": map[string]any{"ok": true, "value": json.RawMessage(doneBody)}})
	exec, recs := contractExec(t, relay)
	if _, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn()); err != nil {
		t.Fatal(err)
	}
	if recs.recs[0].ParseStatus != domain.ParseStatusOK {
		t.Fatalf("%+v", recs.recs[0])
	}
}

func TestExecuteWithContract_FallbackScansStdoutForOldAgent(t *testing.T) {
	relay := relayWith(map[string]any{"stdout": stdoutWith(doneBody), "exitCode": 0}) // no parsed field
	exec, recs := contractExec(t, relay)
	if _, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn()); err != nil || recs.recs[0].ParseStatus != domain.ParseStatusOK {
		t.Fatalf("err=%v rec=%+v", err, recs.recs)
	}
}

func failureOf(t *testing.T, err error) *usecase.ExecutionFailure {
	t.Helper()
	var f *usecase.ExecutionFailure
	if !errors.As(err, &f) {
		t.Fatalf("want *ExecutionFailure, got %v", err)
	}
	return f
}

func TestExecuteWithContract_MissingBlockIsAgentDefect(t *testing.T) {
	exec, recs := contractExec(t, relayWith(map[string]any{"stdout": "I did it, trust me", "exitCode": 0}))
	_, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	f := failureOf(t, err)
	if f.Class != domain.FailureAgentDefect || f.RecordID != "rec-1" || recs.recs[0].ParseStatus != domain.ParseStatusMissing || recs.recs[0].FailureClass != domain.FailureAgentDefect {
		t.Fatalf("failure=%+v rec=%+v", f, recs.recs[0])
	}
}

func TestExecuteWithContract_WrongNonceIsMissing(t *testing.T) {
	body := strings.ReplaceAll(stdoutWith(doneBody), contractNonce, "ffffffffffffffffff")
	exec, recs := contractExec(t, relayWith(map[string]any{"stdout": body, "exitCode": 0}))
	_, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if failureOf(t, err).Class != domain.FailureAgentDefect || recs.recs[0].ParseStatus != domain.ParseStatusMissing {
		t.Fatalf("a forged block must not be accepted: %+v", recs.recs[0])
	}
}

func TestExecuteWithContract_NeedsInfoNotReview(t *testing.T) {
	body := `{"schema_version":1,"status":"needs_info","questions":["which table?"]}`
	exec, recs := contractExec(t, relayWith(map[string]any{"stdout": stdoutWith(body), "exitCode": 0}))
	_, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if f := failureOf(t, err); f.Class != domain.FailureNeedsInfo || recs.recs[0].ParseStatus != domain.ParseStatusOK {
		t.Fatalf("failure=%+v", f)
	}
}

func TestExecuteWithContract_TimeoutAndTransientRelayAreRetryable(t *testing.T) {
	exec, _ := contractExec(t, relayWith(map[string]any{"stdout": "", "exitCode": nil, "timedOut": true}))
	_, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if failureOf(t, err).Class != domain.FailureRetryable {
		t.Fatal("timeout must be retryable")
	}
	relay := &fakeInfraFleetServiceClient{relayErr: status.Error(codes.Unavailable, "gone")}
	exec, recs := contractExec(t, relay)
	_, err = exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if failureOf(t, err).Class != domain.FailureRetryable || len(recs.recs) != 1 {
		t.Fatalf("transient relay failure: err=%v recs=%d", err, len(recs.recs))
	}
	relay = &fakeInfraFleetServiceClient{relayErr: status.Error(codes.NotFound, "no connection")}
	exec, _ = contractExec(t, relay)
	_, err = exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if failureOf(t, err).Class != domain.FailureEnvDefect {
		t.Fatal("non-transient relay failure is an env defect")
	}
}

func TestExecuteWithContract_InsertFailureDoesNotMaskRunFailure(t *testing.T) {
	exec, recs := contractExec(t, relayWith(map[string]any{"stdout": "nothing", "exitCode": 0}))
	recs.insertErr = errors.New("db down")
	_, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if f := failureOf(t, err); f.Class != domain.FailureAgentDefect || f.RecordID != "" {
		t.Fatalf("failure=%+v", f)
	}
	// And a clean run still succeeds when the record cannot be stored.
	exec, recs = contractExec(t, relayWith(map[string]any{"stdout": stdoutWith(doneBody), "exitCode": 0}))
	recs.insertErr = errors.New("db down")
	if _, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn()); err != nil {
		t.Fatalf("clean run must survive a bookkeeping failure: %v", err)
	}
}

func TestExecuteWithContract_ExitNonZeroWithDone(t *testing.T) {
	exec, _ := contractExec(t, relayWith(map[string]any{"stdout": stdoutWith(doneBody), "exitCode": 1}))
	_, err := exec.ExecuteWithContract(ctxWithTenant(t), contractIn())
	if f := failureOf(t, err); f.Code != "EXIT_NONZERO_WITH_DONE" {
		t.Fatalf("%+v", f)
	}
}

func TestExecute_PlainRunUnchangedByContractRefactor(t *testing.T) {
	relay := relayWith(map[string]any{"stdout": "out", "exitCode": 0})
	exec, _ := contractExec(t, relay)
	if _, err := exec.Execute(ctxWithTenant(t), "tenant-1", "t1", "req-1", "/srv/wt", ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(relay.gotRelay.GetParamsJson(), "resultBlock") || strings.Contains(relay.gotRelay.GetParamsJson(), "reportChanges") {
		t.Fatalf("plain params must not carry contract fields: %s", relay.gotRelay.GetParamsJson())
	}
}
