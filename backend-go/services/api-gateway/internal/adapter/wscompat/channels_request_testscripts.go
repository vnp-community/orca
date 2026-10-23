package wscompat

import (
	"context"
	"encoding/json"
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// TestScriptRunView is one attempt of the test-script lane of a Request (docs/integrate/TDD-REQ-TESTSCRIPT-001). Summary is the job as
// the Test Script Agent reported it when it ended; it is passed through as JSON ({} until then).
type TestScriptRunView struct {
	ID            string          `json:"id"`
	Attempt       int32           `json:"attempt"`
	Status        string          `json:"status"`
	ReasonCode    string          `json:"reasonCode,omitempty"`
	ReasonMessage string          `json:"reasonMessage,omitempty"`
	TestSpecRunID string          `json:"testSpecRunId,omitempty"`
	Feature       string          `json:"feature,omitempty"`
	SpecVersion   int32           `json:"specVersion,omitempty"`
	TsaJobID      string          `json:"tsaJobId,omitempty"`
	Branch        string          `json:"branch,omitempty"`
	BaseRef       string          `json:"baseRef,omitempty"`
	WorktreePath  string          `json:"worktreePath,omitempty"`
	CommitSHA     string          `json:"commitSha,omitempty"`
	Summary       json.RawMessage `json:"summary"`
	StartedBy     string          `json:"startedBy,omitempty"`
	StartedAt     string          `json:"startedAt,omitempty"`
	FinishedAt    string          `json:"finishedAt,omitempty"`
	UpdatedAt     string          `json:"updatedAt,omitempty"`
}

func testScriptRunViewOf(r *requestv1.TestScriptRun) TestScriptRunView {
	summary := json.RawMessage(r.GetSummaryJson())
	if len(strings.TrimSpace(string(summary))) == 0 || !json.Valid(summary) {
		summary = json.RawMessage("{}")
	}
	return TestScriptRunView{
		ID: r.GetId(), Attempt: r.GetAttempt(), Status: r.GetStatus(), ReasonCode: r.GetReasonCode(), ReasonMessage: r.GetReasonMessage(),
		TestSpecRunID: r.GetTestSpecRunId(), Feature: r.GetFeature(), SpecVersion: r.GetSpecVersion(), TsaJobID: r.GetTsaJobId(),
		Branch: r.GetBranch(), BaseRef: r.GetBaseRef(), WorktreePath: r.GetWorktreePath(), CommitSHA: r.GetCommitSha(), Summary: summary,
		StartedBy: r.GetStartedBy(), StartedAt: rfc3339(r.GetStartedAt()), FinishedAt: rfc3339(r.GetFinishedAt()), UpdatedAt: rfc3339(r.GetUpdatedAt()),
	}
}

// registerTestScripts adds the channels behind the test-script panel of the Execution tab: one read of the lane, a Re-run, a paged
// read of the agent's report and a paged read of one output file (a generated script or the manifest).
func (c requestChannels) registerTestScripts(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "request.testScripts", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		resp, err := c.req.GetTestScripts(ctx, &requestv1.GetTestScriptsRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		history := make([]TestScriptRunView, 0, len(resp.GetHistory()))
		for _, h := range resp.GetHistory() {
			history = append(history, testScriptRunViewOf(h))
		}
		out := struct {
			RequestID string              `json:"requestId"`
			Enabled   bool                `json:"enabled"`
			Current   *TestScriptRunView  `json:"current"`
			History   []TestScriptRunView `json:"history"`
		}{RequestID: resp.GetRequestId(), Enabled: resp.GetEnabled(), History: history}
		if resp.GetCurrent() != nil {
			v := testScriptRunViewOf(resp.GetCurrent())
			out.Current = &v
		}
		return out, nil
	})
	c.handle(r, "request.testScriptsStart", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		resp, err := c.req.StartTestScripts(ctx, &requestv1.StartTestScriptsRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		return struct {
			Run TestScriptRunView `json:"run"`
		}{testScriptRunViewOf(resp.GetRun())}, nil
	})
	c.handle(r, "request.testScriptReport", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID     string `json:"id"`
			Offset int32  `json:"offset"`
		}](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		if in.Offset < 0 {
			return nil, invalidRequestArg("offset must not be negative")
		}
		resp, err := c.req.GetTestScriptReport(ctx, &requestv1.GetTestScriptReportRequest{RequestId: in.ID, Offset: in.Offset})
		if err != nil {
			return nil, err
		}
		return struct {
			Path    string `json:"path"`
			Length  int32  `json:"length"`
			Text    string `json:"text"`
			HasNext bool   `json:"hasNext"`
			Next    int32  `json:"next"`
		}{resp.GetPath(), resp.GetLength(), resp.GetText(), resp.GetHasNext(), resp.GetNext()}, nil
	})
	c.handle(r, "request.testScriptFile", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			Offset int32  `json:"offset"`
		}](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		if strings.TrimSpace(in.Path) == "" {
			return nil, invalidRequestArg("path is required")
		}
		if in.Offset < 0 {
			return nil, invalidRequestArg("offset must not be negative")
		}
		resp, err := c.req.GetTestScriptFile(ctx, &requestv1.GetTestScriptFileRequest{RequestId: in.ID, Path: in.Path, Offset: in.Offset})
		if err != nil {
			return nil, err
		}
		return struct {
			Path    string `json:"path"`
			Length  int32  `json:"length"`
			Text    string `json:"text"`
			HasNext bool   `json:"hasNext"`
			Next    int32  `json:"next"`
		}{resp.GetPath(), resp.GetLength(), resp.GetText(), resp.GetHasNext(), resp.GetNext()}, nil
	})
}
