package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/tddagent"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// TddAgentMCP drives tdd-agent's test-spec tools on a project's dev server: infra-fleet relay -> Dev Server Agent "mcp.callTool" ->
// tdd-agent MCP server. The wire side (the mcp.callTool message, how a reply is classified, how run_get is read) is
// backend-go/common/tddagent, shared with workflow-service; this file only finds the dev server and shapes the tools' arguments.
type TddAgentMCP struct {
	infra infrafleetv1.InfraFleetServiceClient
}

var _ usecase.TestSpecAgent = (*TddAgentMCP)(nil)

func NewTddAgentMCP(infra infrafleetv1.InfraFleetServiceClient) *TddAgentMCP {
	return &TddAgentMCP{infra: infra}
}

const (
	tddCallTimeout  = 45 * time.Second
	tddStartTimeout = 90 * time.Second
)

// relayFor sends to the dev server a call is about: by dev server id when the connection has one, else by connection id.
func (a *TddAgentMCP) relayFor(conn usecase.AnalysisConnection, timeout time.Duration) tddagent.Relay {
	return func(ctx context.Context, method, paramsJSON string) (string, error) {
		tctx, err := withTenantMetadata(ctx)
		if err != nil {
			return "", err
		}
		tctx, cancel := context.WithTimeout(tctx, timeout)
		defer cancel()
		if conn.DevServerID != "" {
			resp, err := a.infra.RelayByDevServer(tctx, &infrafleetv1.RelayByDevServerRequest{DevServerId: conn.DevServerID, Method: method, ParamsJson: paramsJSON})
			return resp.GetResultJson(), err
		}
		resp, err := a.infra.Relay(tctx, &infrafleetv1.RelayRequest{ConnectionId: conn.ConnectionID, Method: method, ParamsJson: paramsJSON})
		return resp.GetResultJson(), err
	}
}

func (a *TddAgentMCP) call(ctx context.Context, conn usecase.AnalysisConnection, tool string, args map[string]any, out any, opts ...tddagent.Option) error {
	timeout := tddCallTimeout
	if tool == "testspec_start" {
		timeout = tddStartTimeout
	}
	return tddagent.Call(ctx, a.relayFor(conn, timeout), conn.RepoPath, tool, args, out, opts...)
}

func (a *TddAgentMCP) Config(ctx context.Context, conn usecase.AnalysisConnection) (usecase.TestSpecConfig, error) {
	var out struct {
		Config usecase.TestSpecConfig `json:"config"`
	}
	if err := a.call(ctx, conn, "testspec_get", nil, &out); err != nil {
		return usecase.TestSpecConfig{}, err
	}
	return out.Config, nil
}

func (a *TddAgentMCP) Start(ctx context.Context, conn usecase.AnalysisConnection, in usecase.TestSpecStartInput) (usecase.TestSpecStarted, error) {
	body := map[string]any{"kind": in.Kind, "ref": in.Ref}
	switch in.Kind {
	case "solution":
		body["document"] = in.Document
	case "plan":
		body["files"] = in.Files
	default:
		return usecase.TestSpecStarted{}, fmt.Errorf("grpcclient: test-spec context kind %q", in.Kind)
	}
	args := map[string]any{
		"context":  body,
		"external": map[string]string{"system": "orca", "executionId": in.ExecutionID, "stepId": in.StepID, "userId": in.UserID},
	}
	var out struct {
		RunID   string `json:"run_id"`
		Reused  bool   `json:"reused"`
		Context struct {
			Mode   string   `json:"mode"`
			Ref    string   `json:"ref"`
			Path   string   `json:"path"`
			SHA256 string   `json:"sha256"`
			Notes  []string `json:"notes"`
		} `json:"context"`
	}
	if err := a.call(ctx, conn, "testspec_start", args, &out, tddagent.WithTimeoutMs(int(tddStartTimeout/time.Millisecond)-5000)); err != nil {
		return usecase.TestSpecStarted{}, err
	}
	return usecase.TestSpecStarted{RunID: out.RunID, Reused: out.Reused, Mode: out.Context.Mode, Ref: out.Context.Ref, Path: out.Context.Path,
		SHA256: out.Context.SHA256, Notes: out.Context.Notes}, nil
}

func (a *TddAgentMCP) Dispatch(ctx context.Context, conn usecase.AnalysisConnection, runID string) error {
	return a.call(ctx, conn, "run_dispatch", map[string]any{"run_id": runID}, nil)
}

func (a *TddAgentMCP) Run(ctx context.Context, conn usecase.AnalysisConnection, runID string) (usecase.TestSpecRunState, error) {
	var v tddagent.RunView
	if err := a.call(ctx, conn, "run_get", map[string]any{"run_id": runID}, &v); err != nil {
		return usecase.TestSpecRunState{}, err
	}
	return usecase.TestSpecRunState{Status: v.Status(), Blockers: v.Run.Result.Blockers, ProcessAlive: v.ProcessAlive()}, nil
}

func (a *TddAgentMCP) Features(ctx context.Context, conn usecase.AnalysisConnection) ([]string, error) {
	var out struct {
		Features []struct {
			Feature string `json:"feature"`
		} `json:"features"`
	}
	if err := a.call(ctx, conn, "testspec_get", nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Features))
	for _, f := range out.Features {
		names = append(names, f.Feature)
	}
	return names, nil
}

func (a *TddAgentMCP) Summary(ctx context.Context, conn usecase.AnalysisConnection, feature string) (json.RawMessage, error) {
	var out struct {
		Feature json.RawMessage `json:"feature"`
	}
	if err := a.call(ctx, conn, "testspec_get", map[string]any{"feature": feature}, &out); err != nil {
		return nil, err
	}
	return out.Feature, nil
}

func (a *TddAgentMCP) Document(ctx context.Context, conn usecase.AnalysisConnection, feature, document string, offset int) (usecase.TestSpecText, error) {
	var out struct {
		Document string `json:"document"`
		Version  int    `json:"version"`
		Status   string `json:"status"`
		Length   int    `json:"length"`
		Text     string `json:"text"`
		Next     *int   `json:"next"`
	}
	if err := a.call(ctx, conn, "testspec_get", map[string]any{"feature": feature, "document": document, "offset": offset}, &out); err != nil {
		return usecase.TestSpecText{}, err
	}
	return usecase.TestSpecText{Document: out.Document, Version: out.Version, Status: out.Status, Length: out.Length, Text: out.Text, Next: out.Next}, nil
}

func (a *TddAgentMCP) Cancel(ctx context.Context, conn usecase.AnalysisConnection, runID, reason string) error {
	return a.call(ctx, conn, "run_cancel", map[string]any{"run_id": runID, "reason": reason}, nil)
}
