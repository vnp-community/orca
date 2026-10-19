// ── workflow.stepApproval.* (workflow-service) ───────────────────────────
package wscompat

import (
	"context"
	"encoding/json"
	"time"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// stepApprovalView is the camelCase shape the frontend reads (plain encoding/json
// of the proto would ship snake_case, see channels_workflow.go's note).
// subject/decision are passed through as JSON, not strings.
type stepApprovalView struct {
	ID           string          `json:"id"`
	ExecutionID  string          `json:"executionId"`
	StepID       string          `json:"stepId"`
	ProjectID    string          `json:"projectId"`
	Kind         string          `json:"kind"`
	Subject      json.RawMessage `json:"subject"`
	Decision     json.RawMessage `json:"decision"`
	DecisionHash string          `json:"decisionHash"`
	Status       string          `json:"status"`
	ResolvedBy   string          `json:"resolvedBy,omitempty"`
	Rationale    string          `json:"rationale,omitempty"`
	CreatedAt    string          `json:"createdAt"`
	ResolvedAt   string          `json:"resolvedAt,omitempty"`
	CanDecide    *bool           `json:"canDecide,omitempty"`
}

func rawJSON(s string) json.RawMessage {
	if s == "" || !json.Valid([]byte(s)) {
		return json.RawMessage("null")
	}
	return json.RawMessage(s)
}

func toStepApprovalView(a *workflowv1.StepApproval) stepApprovalView {
	v := stepApprovalView{
		ID: a.GetId(), ExecutionID: a.GetExecutionId(), StepID: a.GetStepId(), ProjectID: a.GetProjectId(), Kind: a.GetKind(),
		Subject: rawJSON(a.GetSubjectJson()), Decision: rawJSON(a.GetDecisionJson()), DecisionHash: a.GetDecisionHash(),
		Status: a.GetStatus(), ResolvedBy: a.GetResolvedBy(), Rationale: a.GetRationale(),
	}
	if a.GetCreatedAt() != nil {
		v.CreatedAt = a.GetCreatedAt().AsTime().Format(time.RFC3339)
	}
	if a.GetResolvedAt() != nil {
		v.ResolvedAt = a.GetResolvedAt().AsTime().Format(time.RFC3339)
	}
	return v
}

// registerWorkflowStepApprovalChannels: list/get/resolve the decisions running
// workflow steps wait for (docs/integrate/TDD-WF-AGENT-001 §7.5).
func registerWorkflowStepApprovalChannels(r *Registry, client workflowv1.WorkflowServiceClient) {
	r.Register("workflow.stepApproval.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectID   string `json:"projectId"`
			ExecutionID string `json:"executionId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			in = listArgs{} // no filter object: everything the caller can see
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ListStepApprovals(ctx, &workflowv1.ListStepApprovalsRequest{ProjectId: in.ProjectID, ExecutionId: in.ExecutionID})
		if err != nil {
			return nil, err
		}
		out := make([]stepApprovalView, 0, len(resp.GetApprovals()))
		for _, a := range resp.GetApprovals() {
			out = append(out, toStepApprovalView(a))
		}
		return out, nil
	})

	r.Register("workflow.stepApproval.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetStepApproval(ctx, &workflowv1.GetStepApprovalRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		v := toStepApprovalView(resp.GetApproval())
		can := resp.GetCanDecide()
		v.CanDecide = &can
		return v, nil
	})

	r.Register("workflow.stepApproval.resolve", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			ID           string `json:"id"`
			Decision     string `json:"decision"`
			DecisionHash string `json:"decisionHash"`
			Rationale    string `json:"rationale"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ResolveStepApproval(ctx, &workflowv1.ResolveStepApprovalRequest{
			Id: in.ID, Decision: in.Decision, DecisionHash: in.DecisionHash, Rationale: in.Rationale,
		})
		if err != nil {
			return nil, err
		}
		return toStepApprovalView(resp.GetApproval()), nil
	})
}
