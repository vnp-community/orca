package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// maxSolutionFeedbackRunes mirrors the 2000-character regenerate hint of CR-REQ-007.
const maxSolutionFeedbackRunes = 2000

func (c requestChannels) registerSolution(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "solution.list", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			RequestID string `json:"requestId"`
			Kind      string `json:"kind"`
			Status    string `json:"status"`
			PageSize  int32  `json:"pageSize"`
			PageToken string `json:"pageToken"`
		}](args)
		if err != nil {
			return nil, err
		}
		kind, err := parseProtoEnum("kind", "SOLUTION_KIND_", map[string]int32(requestv1.SolutionKind_value), in.Kind)
		if err != nil {
			return nil, err
		}
		st, err := parseProtoEnum("status", "SOLUTION_STATUS_", map[string]int32(requestv1.SolutionStatus_value), in.Status)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ListSolutions(ctx, &requestv1.ListSolutionsRequest{
			RequestId: in.RequestID, Kind: requestv1.SolutionKind(kind), Status: requestv1.SolutionStatus(st),
			PageSize: clampRequestPageSize(in.PageSize), PageToken: in.PageToken,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Solutions     []SolutionView    `json:"solutions"`
			Runs          []AnalysisRunView `json:"runs"`
			NextPageToken string            `json:"nextPageToken"`
		}{solutionViews(resp.GetSolutions()), analysisRunViews(resp.GetRuns()), resp.GetNextPageToken()}, nil
	})

	// Asynchronous: returns the run id at once; the result arrives as solution.proposed.
	c.handle(r, "solution.generate", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			RequestID      string `json:"requestId"`
			IdempotencyKey string `json:"idempotencyKey"`
			Feedback       string `json:"feedback"`
			AnalysisMode   string `json:"analysisMode"`
		}](args)
		if err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(in.Feedback) > maxSolutionFeedbackRunes {
			return nil, invalidRequestArg("feedback is longer than %d characters", maxSolutionFeedbackRunes)
		}
		mode, err := parseAnalysisMode(in.AnalysisMode)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.GenerateSolution(ctx, &requestv1.GenerateSolutionRequest{
			RequestId: in.RequestID, IdempotencyKey: in.IdempotencyKey, Feedback: in.Feedback, AnalysisMode: mode,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			SolutionID string `json:"solutionId"`
			RunID      string `json:"runId"`
		}{resp.GetSolutionId(), resp.GetRunId()}, nil
	})

	c.handle(r, "solution.choose", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			RequestID  string `json:"requestId"`
			SolutionID string `json:"solutionId"`
			OptionID   string `json:"optionId"`
			Comment    string `json:"comment"`
			Rationale  string `json:"rationale"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ChooseSolutionOption(ctx, &requestv1.ChooseSolutionOptionRequest{
			RequestId: in.RequestID, SolutionId: in.SolutionID, OptionId: in.OptionID, Comment: in.Comment, Rationale: in.Rationale,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Solution             SolutionView `json:"solution"`
			ApprovalDigest       string       `json:"approvalDigest"`
			DecisionStatus       string       `json:"decisionStatus,omitempty"`
			RequiresConfirmation bool         `json:"requiresConfirmation,omitempty"`
		}{SolutionViewOf(resp.GetSolution()), resp.GetApprovalDigest(), resp.GetDecisionStatus(), resp.GetRequiresConfirmation()}, nil
	})
}

func parseAnalysisMode(s string) (requestv1.AnalysisMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return requestv1.AnalysisMode_ANALYSIS_MODE_UNSPECIFIED, nil
	case "complete":
		return requestv1.AnalysisMode_ANALYSIS_MODE_COMPLETE, nil
	case "agent_readonly":
		return requestv1.AnalysisMode_ANALYSIS_MODE_AGENT_READONLY, nil
	}
	return 0, invalidRequestArg("analysisMode must be complete or agent_readonly")
}
