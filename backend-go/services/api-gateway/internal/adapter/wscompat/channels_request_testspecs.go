package wscompat

import (
	"context"
	"encoding/json"
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// TestSpecRunView is one attempt of the test-spec lane of a Request (docs/integrate/TDD-REQ-TESTSPEC-001). Summary is the snapshot
// tdd-agent returned when the run finished; it is passed through as JSON ({} until then).
type TestSpecRunView struct {
	ID            string          `json:"id"`
	Attempt       int32           `json:"attempt"`
	Status        string          `json:"status"`
	ReasonCode    string          `json:"reasonCode,omitempty"`
	ReasonMessage string          `json:"reasonMessage,omitempty"`
	ContextMode   string          `json:"contextMode,omitempty"`
	ContextRef    string          `json:"contextRef,omitempty"`
	ContextSHA256 string          `json:"contextSha256,omitempty"`
	TddRunID      string          `json:"tddRunId,omitempty"`
	Feature       string          `json:"feature,omitempty"`
	Summary       json.RawMessage `json:"summary"`
	StartedBy     string          `json:"startedBy,omitempty"`
	StartedAt     string          `json:"startedAt,omitempty"`
	FinishedAt    string          `json:"finishedAt,omitempty"`
	UpdatedAt     string          `json:"updatedAt,omitempty"`
}

func testSpecRunViewOf(r *requestv1.TestSpecRun) TestSpecRunView {
	summary := json.RawMessage(r.GetSummaryJson())
	if len(strings.TrimSpace(string(summary))) == 0 || !json.Valid(summary) {
		summary = json.RawMessage("{}")
	}
	return TestSpecRunView{
		ID: r.GetId(), Attempt: r.GetAttempt(), Status: r.GetStatus(), ReasonCode: r.GetReasonCode(), ReasonMessage: r.GetReasonMessage(),
		ContextMode: r.GetContextMode(), ContextRef: r.GetContextRef(), ContextSHA256: r.GetContextSha256(), TddRunID: r.GetTddRunId(),
		Feature: r.GetFeature(), Summary: summary, StartedBy: r.GetStartedBy(), StartedAt: rfc3339(r.GetStartedAt()),
		FinishedAt: rfc3339(r.GetFinishedAt()), UpdatedAt: rfc3339(r.GetUpdatedAt()),
	}
}

// registerTestSpecs adds the channels behind the test-spec panel of the Execution tab: one read of the lane, a Re-run and a
// paged read of a generated document.
func (c requestChannels) registerTestSpecs(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "request.testSpecs", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		resp, err := c.req.GetTestSpecs(ctx, &requestv1.GetTestSpecsRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		history := make([]TestSpecRunView, 0, len(resp.GetHistory()))
		for _, h := range resp.GetHistory() {
			history = append(history, testSpecRunViewOf(h))
		}
		out := struct {
			RequestID string            `json:"requestId"`
			Enabled   bool              `json:"enabled"`
			Current   *TestSpecRunView  `json:"current"`
			History   []TestSpecRunView `json:"history"`
			// The join before execution: what execution does about the lane right now.
			Gate      string `json:"gate"`
			Join      string `json:"join"`
			JoinWarn  bool   `json:"joinWarn"`
			JoinCause string `json:"joinCause,omitempty"`
		}{
			RequestID: resp.GetRequestId(), Enabled: resp.GetEnabled(), History: history,
			Gate: resp.GetGate(), Join: resp.GetJoin(), JoinWarn: resp.GetJoinWarn(), JoinCause: resp.GetJoinCause(),
		}
		if resp.GetCurrent() != nil {
			v := testSpecRunViewOf(resp.GetCurrent())
			out.Current = &v
		}
		return out, nil
	})
	c.handle(r, "request.testSpecsStart", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		resp, err := c.req.StartTestSpecs(ctx, &requestv1.StartTestSpecsRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		return struct {
			Run TestSpecRunView `json:"run"`
		}{testSpecRunViewOf(resp.GetRun())}, nil
	})
	c.handle(r, "request.testSpecDocument", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID       string `json:"id"`
			Document string `json:"document"`
			Offset   int32  `json:"offset"`
		}](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		switch in.Document {
		case "requirements", "design", "cases":
		default:
			return nil, invalidRequestArg("document must be requirements, design or cases")
		}
		if in.Offset < 0 {
			return nil, invalidRequestArg("offset must not be negative")
		}
		resp, err := c.req.GetTestSpecDocument(ctx, &requestv1.GetTestSpecDocumentRequest{RequestId: in.ID, Document: in.Document, Offset: in.Offset})
		if err != nil {
			return nil, err
		}
		return struct {
			Document string `json:"document"`
			Version  int32  `json:"version"`
			Status   string `json:"status"`
			Length   int32  `json:"length"`
			Text     string `json:"text"`
			HasNext  bool   `json:"hasNext"`
			Next     int32  `json:"next"`
		}{resp.GetDocument(), resp.GetVersion(), resp.GetStatus(), resp.GetLength(), resp.GetText(), resp.GetHasNext(), resp.GetNext()}, nil
	})
}
