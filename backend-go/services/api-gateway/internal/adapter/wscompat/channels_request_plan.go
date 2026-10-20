package wscompat

import (
	"context"
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// maxPlanProposalBytes bounds a client-edited proposal before it is decoded.
const maxPlanProposalBytes = 256 * 1024

func (c requestChannels) registerPlan(r *Registry) {
	c.handle(r, "request.generatePlan", requestChannelOpts{timeout: requestAIChannelTimeout, ai: true}, c.generatePlan)
	c.handle(r, "request.planProposal", requestChannelOpts{timeout: requestRPCTimeout}, c.planProposal)
	c.handle(r, "request.planBundleFile", requestChannelOpts{timeout: requestRPCTimeout}, c.planBundleFile)
	c.registerPlanRevisions(r)
	c.handle(r, "request.cleanupProposal", requestChannelOpts{timeout: requestRPCTimeout}, c.cleanupProposal)
	c.registerEngine(r)
	c.registerPlanAuto(r)
	c.handle(r, "request.startPhase", requestChannelOpts{timeout: requestStartPhaseTimeout}, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID          string `json:"id"`
			PhaseTaskID string `json:"phaseTaskId"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.StartPhase(ctx, &requestv1.StartPhaseRequest{RequestId: in.ID, PhaseTaskId: in.PhaseTaskID})
		if err != nil {
			return nil, err
		}
		return struct {
			PhaseTaskID       string   `json:"phaseTaskId"`
			AlreadyStarted    bool     `json:"alreadyStarted"`
			DispatchedTaskIDs []string `json:"dispatchedTaskIds"`
		}{resp.GetPhaseTaskId(), resp.GetAlreadyStarted(), nonNilStrings(resp.GetDispatchedTaskIds())}, nil
	})
}

// generatePlan is two-phase (CR-REQ-012): propose starts a background run and
// returns its run id, commit persists a proposal a person may have edited.
func (c requestChannels) generatePlan(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID            string          `json:"id"`
		Mode          string          `json:"mode"`
		Feedback      string          `json:"feedback"`
		Proposal      json.RawMessage `json:"proposal"`
		RawAIResponse string          `json:"rawAiResponse"`
		// IdempotencyKey makes a repeated propose return the run it started (CR-REQ-045); commit ignores it.
		IdempotencyKey string `json:"idempotencyKey"`
	}](args)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(strings.TrimSpace(in.Mode)) {
	case "", "propose":
		resp, err := c.req.GeneratePlan(ctx, &requestv1.GeneratePlanRequest{RequestId: in.ID, Feedback: in.Feedback, IdempotencyKey: in.IdempotencyKey})
		if err != nil {
			return nil, err
		}
		return planProposalView(resp.GetRunId(), "", "", "", resp.GetProposal(), resp.GetRawAiResponse()), nil
	case "commit":
		if len(in.Proposal) == 0 || string(in.Proposal) == "null" {
			return nil, invalidRequestArg("proposal required for commit")
		}
		if len(in.Proposal) > maxPlanProposalBytes {
			return nil, invalidRequestArg("proposal is too large")
		}
		var proposal requestv1.PlanProposal
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(in.Proposal, &proposal); err != nil {
			return nil, invalidRequestArg("proposal is not a valid plan")
		}
		ctx, cancel := context.WithTimeout(ctx, requestCommitPlanTimeout)
		defer cancel()
		resp, err := c.req.CommitPlan(ctx, &requestv1.CommitPlanRequest{RequestId: in.ID, Proposal: &proposal, RawAiResponse: in.RawAIResponse})
		if err != nil {
			return nil, err
		}
		return struct {
			PlanTaskID    string   `json:"planTaskId"`
			PhaseTaskIDs  []string `json:"phaseTaskIds"`
			TaskIDs       []string `json:"taskIds"`
			AlreadyExists bool     `json:"alreadyExists"`
		}{resp.GetPlanTaskId(), nonNilStrings(resp.GetPhaseTaskIds()), nonNilStrings(resp.GetTaskIds()), resp.GetAlreadyExists()}, nil
	default:
		return nil, invalidRequestArg("mode must be propose or commit")
	}
}

// planProposalView is shared by the propose result and request.planProposal so
// the UI reads one shape; proposal is null while the run is still going.
func planProposalView(runID, runStatus, errCode, errMessage string, p *requestv1.PlanProposal, raw string) map[string]any {
	out := map[string]any{"runId": runID, "proposal": json.RawMessage("null"), "rawAiResponse": raw}
	if runStatus != "" {
		out["status"] = runStatus
	}
	if errCode != "" {
		out["errorCode"] = errCode
		out["errorMessage"] = errMessage
	}
	if p != nil {
		if b, err := protojson.Marshal(p); err == nil {
			out["proposal"] = json.RawMessage(b)
		}
	}
	return out
}

// planProposal reads the result of an asynchronous propose run (CONTRACT Q2, option a).
func (c requestChannels) planProposal(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID    string `json:"id"`
		RunID string `json:"runId"`
		// Revision reads that version of the Plan (CR-REQ-045); 0 or absent reads the run or the current version.
		Revision int64 `json:"revision"`
	}](args)
	if err != nil {
		return nil, err
	}
	revision, err := planRevisionArg(in.Revision)
	if err != nil {
		return nil, err
	}
	resp, err := c.req.GetPlanProposal(ctx, &requestv1.GetPlanProposalRequest{RequestId: in.ID, RunId: in.RunID, Revision: revision})
	if err != nil {
		return nil, err
	}
	out := planProposalView(resp.GetRunId(), resp.GetStatus(), resp.GetErrorCode(), resp.GetErrorMessage(), resp.GetProposal(), resp.GetRawAiResponse())
	addPlanEngineView(out, resp)
	addPlanAutoView(out, resp)
	return out, nil
}

// addPlanEngineView adds what an openspec Plan carries beyond the proposal. A native Plan gets only engine, so no field
// appears empty and reads as "nothing found".
func addPlanEngineView(out map[string]any, resp *requestv1.GetPlanProposalResponse) {
	if resp.GetEngine() != "" {
		out["engine"] = resp.GetEngine()
	}
	if resp.GetBundle() != nil {
		out["bundle"] = protoJSON(resp.GetBundle())
	}
	if len(resp.GetChecks()) > 0 {
		checks := make([]json.RawMessage, 0, len(resp.GetChecks()))
		for _, c := range resp.GetChecks() {
			checks = append(checks, protoJSON(c))
		}
		out["checks"] = checks
	}
	if resp.GetKnowledge() != nil {
		out["knowledge"] = protoJSON(resp.GetKnowledge())
	}
	if st := runStatsView(resp.GetStats()); st != nil {
		out["stats"] = st
	}
	if resp.GetElapsedMs() > 0 {
		out["elapsedMs"] = resp.GetElapsedMs()
	}
}

func protoJSON(m proto.Message) json.RawMessage {
	b, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(m)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// planBundleFile reads one file of the authored change. The path is passed through untouched: request-service is the one
// that decides what lies inside the change.
func (c requestChannels) planBundleFile(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		Revision int64  `json:"revision"`
	}](args)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, invalidRequestArg("path is required")
	}
	revision, err := planRevisionArg(in.Revision)
	if err != nil {
		return nil, err
	}
	resp, err := c.req.GetPlanBundleFile(ctx, &requestv1.GetPlanBundleFileRequest{RequestId: in.ID, Path: in.Path, Revision: revision})
	if err != nil {
		return nil, err
	}
	return struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Omitted bool   `json:"omitted"`
	}{resp.GetPath(), resp.GetContent(), resp.GetOmitted()}, nil
}

// cleanupProposal removes the worktree of the Request's OpenSpec proposal; the branch and the stored change stay.
func (c requestChannels) cleanupProposal(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[requestIDArgs](args)
	if err != nil {
		return nil, err
	}
	resp, err := c.req.CleanupRequestProposal(ctx, &requestv1.CleanupRequestProposalRequest{RequestId: in.ID})
	if err != nil {
		return nil, err
	}
	return struct {
		Removed   bool   `json:"removed"`
		Abandoned bool   `json:"abandoned"`
		Branch    string `json:"branch"`
	}{resp.GetRemoved(), resp.GetAbandoned(), resp.GetBranch()}, nil
}
