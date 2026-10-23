package grpcclient

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/stablyai/orca-go/common/tddagent"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// TestScriptAgentServer is the name of the Test Script Agent in the Dev Server Agent's mcp.callTool allowlist.
const TestScriptAgentServer = "test-script-agent"

// TestScriptAgentMCP drives the Test Script Agent on a project's dev server through the same bridge as tdd-agent (infra-fleet relay ->
// Dev Server Agent "mcp.callTool" -> tsa MCP server); only the server name and the tools differ (docs/integrate/TDD-REQ-TESTSCRIPT-001 5.4).
type TestScriptAgentMCP struct {
	relay *TddAgentMCP
}

var _ usecase.TestScriptAgent = (*TestScriptAgentMCP)(nil)

func NewTestScriptAgentMCP(infra infrafleetv1.InfraFleetServiceClient) *TestScriptAgentMCP {
	return &TestScriptAgentMCP{relay: NewTddAgentMCP(infra)}
}

const tsaCallTimeout = 45 * time.Second

func (a *TestScriptAgentMCP) call(ctx context.Context, conn usecase.AnalysisConnection, tool string, args map[string]any, out any) error {
	return tddagent.Call(ctx, a.relay.relayFor(conn, tsaCallTimeout), conn.RepoPath, tool, args, out, tddagent.WithServer(TestScriptAgentServer))
}

func (a *TestScriptAgentMCP) Start(ctx context.Context, conn usecase.AnalysisConnection, in usecase.TestScriptStartInput) (usecase.TestScriptStarted, error) {
	external := map[string]string{"system": "orca", "executionId": in.ExecutionID, "stepId": in.StepID}
	if in.Attempt > 0 {
		external["attempt"] = strconv.Itoa(in.Attempt)
	}
	args := map[string]any{"feature": in.Feature, "external": external}
	if in.SpecVersion > 0 {
		args["spec_version"] = in.SpecVersion
	}
	for k, v := range map[string]string{"branch": in.Branch, "base": in.BaseRef, "test_repo": in.TestRepo, "generator": in.Generator, "model": in.Model} {
		if v != "" {
			args[k] = v
		}
	}
	if len(in.TestsPaths) > 0 {
		args["tests_paths"] = in.TestsPaths
	}
	var out struct {
		JobID  string `json:"job_id"`
		Reused bool   `json:"reused"`
		State  string `json:"state"`
	}
	if err := a.call(ctx, conn, "tsa_start", args, &out); err != nil {
		return usecase.TestScriptStarted{}, err
	}
	return usecase.TestScriptStarted{JobID: out.JobID, Reused: out.Reused, State: out.State}, nil
}

func (a *TestScriptAgentMCP) Job(ctx context.Context, conn usecase.AnalysisConnection, jobID string) (usecase.TestScriptJob, error) {
	var raw json.RawMessage
	if err := a.call(ctx, conn, "tsa_get", map[string]any{"job_id": jobID}, &raw); err != nil {
		return usecase.TestScriptJob{}, err
	}
	return decodeTestScriptJob(raw)
}

// decodeTestScriptJob reads a tsa_get answer; AllGenerated is true when there are files and every one passed the spec check.
func decodeTestScriptJob(raw json.RawMessage) (usecase.TestScriptJob, error) {
	var v struct {
		State string `json:"state"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Branch   string `json:"branch"`
		Worktree string `json:"worktree"`
		Commit   string `json:"commit"`
		Files    []struct {
			Status string `json:"status"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return usecase.TestScriptJob{}, err
	}
	job := usecase.TestScriptJob{State: v.State, Branch: v.Branch, Worktree: v.Worktree, Commit: v.Commit, Raw: raw}
	if v.Error != nil {
		job.ErrorCode, job.ErrorText = v.Error.Code, v.Error.Message
	}
	job.AllGenerated = len(v.Files) > 0
	for _, f := range v.Files {
		if f.Status != "generated" {
			job.AllGenerated = false
		}
	}
	return job, nil
}

func (a *TestScriptAgentMCP) Report(ctx context.Context, conn usecase.AnalysisConnection, jobID string, offset int) (usecase.TestScriptText, error) {
	var out struct {
		Path  string `json:"path"`
		Total int    `json:"total"`
		Text  string `json:"text"`
		Next  *int   `json:"next"`
	}
	if err := a.call(ctx, conn, "tsa_report", map[string]any{"job_id": jobID, "offset": offset}, &out); err != nil {
		return usecase.TestScriptText{}, err
	}
	return usecase.TestScriptText{Path: out.Path, Total: out.Total, Text: out.Text, Next: out.Next}, nil
}

func (a *TestScriptAgentMCP) File(ctx context.Context, conn usecase.AnalysisConnection, jobID, path string, offset int) (usecase.TestScriptText, error) {
	var out struct {
		Path  string `json:"path"`
		Total int    `json:"total"`
		Text  string `json:"text"`
		Next  *int   `json:"next"`
	}
	if err := a.call(ctx, conn, "tsa_file", map[string]any{"job_id": jobID, "path": path, "offset": offset}, &out); err != nil {
		return usecase.TestScriptText{}, err
	}
	return usecase.TestScriptText{Path: out.Path, Total: out.Total, Text: out.Text, Next: out.Next}, nil
}

func (a *TestScriptAgentMCP) Cancel(ctx context.Context, conn usecase.AnalysisConnection, jobID, reason string) error {
	return a.call(ctx, conn, "tsa_cancel", map[string]any{"job_id": jobID, "reason": reason}, nil)
}
