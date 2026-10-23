package grpcclient

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func lastTSATool(t *testing.T, infra *fakeAgentInfra) (string, map[string]any) {
	t.Helper()
	if len(infra.dev) == 0 {
		t.Fatal("nothing was sent")
	}
	call := infra.dev[len(infra.dev)-1]
	p := paramsOf(t, call.GetParamsJson())
	if call.GetMethod() != "mcp.callTool" || p["server"] != "test-script-agent" || p["projectDir"] != "/home/dev/notes" {
		t.Fatalf("call %+v params %v", call, p)
	}
	args, _ := p["arguments"].(map[string]any)
	return p["tool"].(string), args
}

func TestTestScriptAgentMCP_StartSendsOnlyWhatTheAgentNeeds(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"ok":true,"result":{"job_id":"tsa-1","reused":false,"state":"queued"}}`}
	got, err := NewTestScriptAgentMCP(infra).Start(ctxWithIdentity(), tddConn(), usecase.TestScriptStartInput{
		Feature: "TDD-NOTES", SpecVersion: 2, Branch: "tsa/REQ-12-TDD-NOTES-v2", BaseRef: "main", TestsPaths: []string{"tests"},
		Generator: "claude", ExecutionID: "req-1", StepID: "testscript", Attempt: 2,
	})
	if err != nil || got.JobID != "tsa-1" || got.State != "queued" {
		t.Fatalf("got %+v err %v", got, err)
	}
	tool, args := lastTSATool(t, infra)
	ext := args["external"].(map[string]any)
	if tool != "tsa_start" || args["feature"] != "TDD-NOTES" || args["spec_version"] != float64(2) || args["branch"] != "tsa/REQ-12-TDD-NOTES-v2" ||
		args["base"] != "main" || args["generator"] != "claude" || args["model"] != nil || args["test_repo"] != nil {
		t.Fatalf("args %v", args)
	}
	if paths := args["tests_paths"].([]any); len(paths) != 1 || paths[0] != "tests" {
		t.Fatalf("tests_paths %v", args["tests_paths"])
	}
	if ext["system"] != "orca" || ext["executionId"] != "req-1" || ext["stepId"] != "testscript" || ext["attempt"] != "2" {
		t.Fatalf("external %v", ext)
	}
}

func TestTestScriptAgentMCP_JobSaysWhetherEveryFilePassed(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"ok":true,"result":{"job_id":"tsa-1","state":"done","branch":"b","worktree":"/w","commit":"c1",
		"files":[{"path":"a.py","status":"generated"},{"path":"b.py","status":"needs-review"}]}}`}
	a := NewTestScriptAgentMCP(infra)
	job, err := a.Job(ctxWithIdentity(), tddConn(), "tsa-1")
	if err != nil || job.State != "done" || job.Commit != "c1" || job.Worktree != "/w" || job.AllGenerated || len(job.Raw) == 0 {
		t.Fatalf("job %+v err %v", job, err)
	}
	if tool, args := lastTSATool(t, infra); tool != "tsa_get" || args["job_id"] != "tsa-1" {
		t.Fatalf("tool %s args %v", tool, args)
	}
	infra.result = `{"ok":true,"result":{"state":"done","files":[{"status":"generated"}]}}`
	if job, _ := a.Job(ctxWithIdentity(), tddConn(), "tsa-1"); !job.AllGenerated {
		t.Fatal("all files generated")
	}
	infra.result = `{"ok":true,"result":{"state":"error","error":{"code":"FRAMEWORK_UNSUPPORTED","message":"no pytest"}}}`
	if job, _ := a.Job(ctxWithIdentity(), tddConn(), "tsa-1"); job.ErrorCode != "FRAMEWORK_UNSUPPORTED" || job.AllGenerated {
		t.Fatalf("job %+v", job)
	}
}

func TestTestScriptAgentMCP_ReportCancelAndErrors(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"ok":true,"result":{"path":"tests/tsa/x/tsa-report.md","total":30,"text":"# TSA","next":20}}`}
	a := NewTestScriptAgentMCP(infra)
	txt, err := a.Report(ctxWithIdentity(), tddConn(), "tsa-1", 0)
	if err != nil || txt.Text != "# TSA" || txt.Next == nil || *txt.Next != 20 || txt.Total != 30 {
		t.Fatalf("text %+v err %v", txt, err)
	}
	infra.result = `{"ok":true,"result":{"path":"tests/tsa/x/test_a.py","total":12,"text":"import pytest","next":null}}`
	f, err := a.File(ctxWithIdentity(), tddConn(), "tsa-1", "tests/tsa/x/test_a.py", 0)
	if err != nil || f.Text != "import pytest" || f.Next != nil || f.Path != "tests/tsa/x/test_a.py" {
		t.Fatalf("file %+v err %v", f, err)
	}
	if tool, args := lastTSATool(t, infra); tool != "tsa_file" || args["path"] != "tests/tsa/x/test_a.py" || args["job_id"] != "tsa-1" {
		t.Fatalf("tool %s args %v", tool, args)
	}
	infra.result = `{"ok":true,"result":{"state":"cancelled"}}`
	if err := a.Cancel(ctxWithIdentity(), tddConn(), "tsa-1", "stop"); err != nil {
		t.Fatal(err)
	}
	if tool, args := lastTSATool(t, infra); tool != "tsa_cancel" || args["reason"] != "stop" {
		t.Fatalf("tool %s args %v", tool, args)
	}
	infra.result = `{"ok":false,"error":{"code":"MCP_SERVER_NOT_ALLOWED","message":"not configured"}}`
	var be *tddagent.BridgeError
	if _, err := a.Job(ctxWithIdentity(), tddConn(), "tsa-1"); !errors.As(err, &be) || be.Code != "MCP_SERVER_NOT_ALLOWED" {
		t.Fatalf("err %v", err)
	}
	infra.result = `{"ok":true,"isError":true,"error":{"code":"TSA_JOB_NOT_FOUND","message":"no job"}}`
	var te *tddagent.ToolError
	if _, err := a.Job(ctxWithIdentity(), tddConn(), "tsa-1"); !errors.As(err, &te) || te.Code != "TSA_JOB_NOT_FOUND" {
		t.Fatalf("err %v", err)
	}
}
