package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

type fakeStepApprovalClient struct {
	workflowv1.WorkflowServiceClient
	listReq    *workflowv1.ListStepApprovalsRequest
	resolveReq *workflowv1.ResolveStepApprovalRequest
	ctx        context.Context
}

var sampleStepApproval = &workflowv1.StepApproval{
	Id: "a1", ExecutionId: "e1", StepId: "approve_g1", ProjectId: "p1", Kind: "tdd_agent",
	SubjectJson: `{"runId":"run-1","gate":"G1","role":"BA"}`, DecisionJson: `{"srs":"ready"}`, DecisionHash: "h1",
	Status: "pending", CreatedAt: timestamppb.Now(),
}

func (f *fakeStepApprovalClient) ListStepApprovals(ctx context.Context, in *workflowv1.ListStepApprovalsRequest, _ ...grpc.CallOption) (*workflowv1.ListStepApprovalsResponse, error) {
	f.listReq, f.ctx = in, ctx
	return &workflowv1.ListStepApprovalsResponse{Approvals: []*workflowv1.StepApproval{sampleStepApproval}}, nil
}

func (f *fakeStepApprovalClient) GetStepApproval(ctx context.Context, in *workflowv1.GetStepApprovalRequest, _ ...grpc.CallOption) (*workflowv1.GetStepApprovalResponse, error) {
	return &workflowv1.GetStepApprovalResponse{Approval: sampleStepApproval, CanDecide: true}, nil
}

func (f *fakeStepApprovalClient) ResolveStepApproval(ctx context.Context, in *workflowv1.ResolveStepApprovalRequest, _ ...grpc.CallOption) (*workflowv1.ResolveStepApprovalResponse, error) {
	f.resolveReq, f.ctx = in, ctx
	resolved := proto.Clone(sampleStepApproval).(*workflowv1.StepApproval)
	resolved.Status, resolved.ResolvedBy = "approved", "user-1"
	return &workflowv1.ResolveStepApprovalResponse{Approval: resolved}, nil
}

func TestStepApprovalChannelsSpeakCamelCaseAndForwardIdentity(t *testing.T) {
	fake := &fakeStepApprovalClient{}
	r := NewRegistry()
	registerWorkflowStepApprovalChannels(r, fake)
	id := Identity{TenantID: "tenant-1", UserID: "user-1"}

	res, err := r.Dispatch(context.Background(), id, "workflow.stepApproval.list", argsJSON(t, map[string]any{"projectId": "p1"}))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res)
	var list []map[string]any
	_ = json.Unmarshal(raw, &list)
	if len(list) != 1 || list[0]["decisionHash"] != "h1" || list[0]["subject"].(map[string]any)["gate"] != "G1" ||
		list[0]["decision"].(map[string]any)["srs"] != "ready" || fake.listReq.GetProjectId() != "p1" {
		t.Fatalf("list = %s", raw)
	}
	if tenant, user := outgoingTenantUser(fake.ctx); tenant != "tenant-1" || user != "user-1" {
		t.Fatalf("identity %q %q", tenant, user)
	}

	res, err = r.Dispatch(context.Background(), id, "workflow.stepApproval.get", argsJSON(t, map[string]any{"id": "a1"}))
	if err != nil {
		t.Fatal(err)
	}
	if v := res.(stepApprovalView); v.CanDecide == nil || !*v.CanDecide {
		t.Fatalf("get = %+v", v)
	}

	res, err = r.Dispatch(context.Background(), id, "workflow.stepApproval.resolve",
		argsJSON(t, map[string]any{"id": "a1", "decision": "approved", "decisionHash": "h1", "rationale": "ok"}))
	if err != nil {
		t.Fatal(err)
	}
	if v := res.(stepApprovalView); v.Status != "approved" || fake.resolveReq.GetDecisionHash() != "h1" || fake.resolveReq.GetRationale() != "ok" {
		t.Fatalf("resolve = %+v req %+v", v, fake.resolveReq)
	}
}
