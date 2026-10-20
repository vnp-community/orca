package grpcclient

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func tddConn() usecase.AnalysisConnection {
	return usecase.AnalysisConnection{DevServerID: "ds-1", RepoPath: "/home/dev/notes"}
}

func lastTool(t *testing.T, infra *fakeAgentInfra) (string, map[string]any, map[string]any) {
	t.Helper()
	if len(infra.dev) == 0 {
		t.Fatal("nothing was sent")
	}
	call := infra.dev[len(infra.dev)-1]
	if call.GetMethod() != "mcp.callTool" || call.GetDevServerId() != "ds-1" {
		t.Fatalf("call %+v", call)
	}
	p := paramsOf(t, call.GetParamsJson())
	if p["server"] != "tdd-agent" || p["projectDir"] != "/home/dev/notes" {
		t.Fatalf("params %v", p)
	}
	args, _ := p["arguments"].(map[string]any)
	return p["tool"].(string), args, p
}

func TestTddAgentMCP_ConfigReadsTheDevServersChoiceOfMode(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"ok":true,"result":{"config":{"enabled":true,"llm":"claude-cli","context_mode":"plan","context_path":"","specs_dir":"docs/testspec"},"features":[]}}`}
	cfg, err := NewTddAgentMCP(infra).Config(ctxWithIdentity(), tddConn())
	if err != nil || !cfg.Enabled || cfg.ContextMode != "plan" || cfg.LLM != "claude-cli" {
		t.Fatalf("cfg %+v err %v", cfg, err)
	}
	if tool, args, _ := lastTool(t, infra); tool != "testspec_get" || len(args) != 0 {
		t.Fatalf("tool %s args %v", tool, args)
	}
}

func TestTddAgentMCP_StartSendsASolutionAsADocumentAndAPlanAsFiles(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"ok":true,"result":{"run_id":"run-1","reused":true,"context":{"mode":"solution","ref":"REQ-2","path":".tdd-agent/context/solution-REQ-2","sha256":"abc","notes":["n"]}}}`}
	a := NewTddAgentMCP(infra)
	got, err := a.Start(ctxWithIdentity(), tddConn(), usecase.TestSpecStartInput{
		Kind: "solution", Ref: "REQ-2", Document: json.RawMessage(`{"id":"s1","options":{"options":[]}}`), ExecutionID: "req-1", StepID: "testspec", UserID: "u-1",
	})
	if err != nil || got.RunID != "run-1" || !got.Reused || got.SHA256 != "abc" || got.Mode != "solution" || len(got.Notes) != 1 {
		t.Fatalf("got %+v err %v", got, err)
	}
	tool, args, p := lastTool(t, infra)
	ctxArg := args["context"].(map[string]any)
	doc := ctxArg["document"].(map[string]any)
	ext := args["external"].(map[string]any)
	if tool != "testspec_start" || ctxArg["kind"] != "solution" || ctxArg["ref"] != "REQ-2" || doc["id"] != "s1" || ctxArg["files"] != nil {
		t.Fatalf("args %v", args)
	}
	if ext["system"] != "orca" || ext["executionId"] != "req-1" || ext["stepId"] != "testspec" || ext["userId"] != "u-1" || p["timeoutMs"] == nil {
		t.Fatalf("external %v params %v", ext, p)
	}
	if _, err := a.Start(ctxWithIdentity(), tddConn(), usecase.TestSpecStartInput{Kind: "plan", Ref: "REQ-2", Files: map[string]string{"tasks.md": "t"}}); err != nil {
		t.Fatal(err)
	}
	_, args, _ = lastTool(t, infra)
	ctxArg = args["context"].(map[string]any)
	if files := ctxArg["files"].(map[string]any); files["tasks.md"] != "t" || ctxArg["document"] != nil {
		t.Fatalf("plan args %v", ctxArg)
	}
	if _, err := a.Start(ctxWithIdentity(), tddConn(), usecase.TestSpecStartInput{Kind: "other"}); err == nil {
		t.Fatal("an unknown kind is refused before anything is sent")
	}
}

func TestTddAgentMCP_RunReadsTheStateOfTheRun(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"ok":true,"result":{"run":{"run_id":"run-1","status":"blocked","result":{"blockers":["LLM_NOT_CONFIGURED"]}},"process":{"alive":true}}}`}
	st, err := NewTddAgentMCP(infra).Run(ctxWithIdentity(), tddConn(), "run-1")
	if err != nil || st.Status != "blocked" || len(st.Blockers) != 1 || !st.ProcessAlive {
		t.Fatalf("state %+v err %v", st, err)
	}
	if tool, args, _ := lastTool(t, infra); tool != "run_get" || args["run_id"] != "run-1" {
		t.Fatalf("tool %s args %v", tool, args)
	}
}

func TestTddAgentMCP_FeaturesSummaryDocumentAndCancel(t *testing.T) {
	a := func(result string) (*TddAgentMCP, *fakeAgentInfra) {
		infra := &fakeAgentInfra{result: result}
		return NewTddAgentMCP(infra), infra
	}
	ag, _ := a(`{"ok":true,"result":{"config":{},"features":[{"feature":"TDD-NOTES"},{"feature":"TDD-B"}]}}`)
	if names, err := ag.Features(ctxWithIdentity(), tddConn()); err != nil || len(names) != 2 || names[0] != "TDD-NOTES" {
		t.Fatalf("features %v %v", names, err)
	}
	ag, infra := a(`{"ok":true,"result":{"config":{},"feature":{"feature":"TDD-NOTES","counts":{"TestCase":16}}}}`)
	if raw, err := ag.Summary(ctxWithIdentity(), tddConn(), "TDD-NOTES"); err != nil || string(raw) != `{"feature":"TDD-NOTES","counts":{"TestCase":16}}` {
		t.Fatalf("summary %s %v", raw, err)
	}
	if _, args, _ := lastTool(t, infra); args["feature"] != "TDD-NOTES" {
		t.Fatalf("args %v", args)
	}
	ag, infra = a(`{"ok":true,"result":{"document":"cases","version":1,"status":"Draft","length":900,"text":"abc","next":3}}`)
	if text, err := ag.Document(ctxWithIdentity(), tddConn(), "TDD-NOTES", "cases", 0); err != nil || text.Text != "abc" || text.Next == nil || *text.Next != 3 || text.Length != 900 {
		t.Fatalf("document %+v %v", text, err)
	}
	if _, args, _ := lastTool(t, infra); args["document"] != "cases" || args["offset"] != float64(0) {
		t.Fatalf("args %v", args)
	}
	ag, infra = a(`{"ok":true,"result":{"run_id":"r","cancelled":[]}}`)
	if err := ag.Cancel(ctxWithIdentity(), tddConn(), "run-1", "re-run"); err != nil {
		t.Fatal(err)
	}
	if tool, args, _ := lastTool(t, infra); tool != "run_cancel" || args["run_id"] != "run-1" || args["reason"] != "re-run" {
		t.Fatalf("tool %s args %v", tool, args)
	}
}

func TestTddAgentMCP_ErrorsKeepTheirKind(t *testing.T) {
	a := func(result string, err error) error {
		_, e := NewTddAgentMCP(&fakeAgentInfra{result: result, err: err}).Config(ctxWithIdentity(), tddConn())
		return e
	}
	var te *tddagent.ToolError
	var be *tddagent.BridgeError
	if err := a(`{"ok":true,"isError":true,"error":{"code":"LLM_NOT_CONFIGURED","message":"set it"}}`, nil); !errors.As(err, &te) || te.Code != "LLM_NOT_CONFIGURED" {
		t.Fatalf("tool error %v", err)
	}
	if err := a(`{"ok":false,"error":{"code":"MCP_TOOL_NOT_ALLOWED","message":"no"}}`, nil); !errors.As(err, &be) || be.Code != "MCP_TOOL_NOT_ALLOWED" {
		t.Fatalf("bridge error %v", err)
	}
	if err := a("", errors.New("agent offline")); !errors.Is(err, tddagent.ErrTransport) {
		t.Fatalf("transport %v", err)
	}
}

func TestTddAgentMCP_ReachesAConnectionWithoutADevServerId(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"ok":true,"result":{"config":{}}}`}
	if _, err := NewTddAgentMCP(infra).Config(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c1", RepoPath: "/r"}); err != nil {
		t.Fatal(err)
	}
	if len(infra.relay) != 1 || infra.relay[0].GetConnectionId() != "c1" || len(infra.dev) != 0 {
		t.Fatalf("relay %+v dev %+v", infra.relay, infra.dev)
	}
}
