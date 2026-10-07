package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeStatusCoreClient struct {
	codeintelv1.CodeIntelServiceClient
	req  *codeintelv1.GetIndexStatusRequest
	resp *codeintelv1.GetIndexStatusResponse
	err  error
}

func (f *fakeStatusCoreClient) GetIndexStatus(ctx context.Context, in *codeintelv1.GetIndexStatusRequest, opts ...grpc.CallOption) (*codeintelv1.GetIndexStatusResponse, error) {
	f.req = in
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestCodeIntelViewStatus_SuccessFlatShape(t *testing.T) {
	r := NewRegistry()
	fake := &fakeStatusCoreClient{
		resp: &codeintelv1.GetIndexStatusResponse{
			Status: &codeintelv1.IndexStatus{
				Overall: "OVERLAY",
				Tools:   []*codeintelv1.ToolIndexStatus{},
				Binding: &codeintelv1.RepoBinding{
					Id:         "rb-1",
					ProjectId:  "proj-1",
					IndexScope: codeintelv1.IndexScope_INDEX_SCOPE_EXACT,
				},
			},
		},
	}
	deps := codeIntelDeps{
		core:   fake,
		limits: CodeIntelLimits{MaxResponseBytes: 2 << 20},
	}
	registerCodeIntelStatus(r, deps)

	rawArgs := []json.RawMessage{json.RawMessage(`{"projectId":"proj-1","worktreeId":"wt-1","refresh":true}`)}
	res, err := r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.status", rawArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.req == nil || fake.req.GetSelector().GetProjectId() != "proj-1" || fake.req.GetSelector().GetWorktreeRef() != "wt-1" || !fake.req.GetRefresh() {
		t.Fatalf("unexpected request to core: %+v", fake.req)
	}

	bytes, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	str := string(bytes)

	// Invariant: Flat IndexStatus, no "data" or "meta" envelope
	if strings.Contains(str, `"data":`) || strings.Contains(str, `"meta":`) {
		t.Errorf("expected flat IndexStatus without data/meta envelope, got: %s", str)
	}
	// Invariant: overall=OVERLAY retains uppercase
	if !strings.Contains(str, `"overall":"OVERLAY"`) {
		t.Errorf("expected overall: OVERLAY in uppercase, got: %s", str)
	}
	// Invariant: tools: [] empty array
	if !strings.Contains(str, `"tools":[]`) {
		t.Errorf("expected empty array for tools, got: %s", str)
	}
	// Invariant: activeJob nil => key omitted
	if strings.Contains(str, `"activeJob"`) {
		t.Errorf("expected activeJob to be omitted when nil, got: %s", str)
	}
	// Invariant: indexScope wire format
	if !strings.Contains(str, `"indexScope":"exact"`) {
		t.Errorf("expected exact indexScope, got: %s", str)
	}
}

func TestCodeIntelViewStatus_ActiveJobWithNullPercent(t *testing.T) {
	r := NewRegistry()
	fake := &fakeStatusCoreClient{
		resp: &codeintelv1.GetIndexStatusResponse{
			Status: &codeintelv1.IndexStatus{
				Overall: "BUILDING",
				ActiveJob: &codeintelv1.ActiveReindexJob{
					Id:    "job-1",
					Stage: "parsing",
					// Percent is nil
				},
				Binding: &codeintelv1.RepoBinding{
					Id:         "rb-1",
					IndexScope: codeintelv1.IndexScope_INDEX_SCOPE_UNSPECIFIED,
				},
			},
		},
	}
	deps := codeIntelDeps{
		core:   fake,
		limits: CodeIntelLimits{MaxResponseBytes: 2 << 20},
	}
	registerCodeIntelStatus(r, deps)

	rawArgs := []json.RawMessage{json.RawMessage(`{"projectId":"proj-1","worktreeId":"wt-1"}`)}
	res, err := r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.status", rawArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bytes, _ := json.Marshal(res)
	str := string(bytes)

	// Invariant: activeJob.percent absent => null
	if !strings.Contains(str, `"percent":null`) {
		t.Errorf("expected percent: null in activeJob, got: %s", str)
	}
	// Invariant: unspecified indexScope => unresolved
	if !strings.Contains(str, `"indexScope":"unresolved"`) {
		t.Errorf("expected unresolved for unspecified indexScope, got: %s", str)
	}
}

func TestCodeIntelViewStatus_GuardsAndErrors(t *testing.T) {
	r := NewRegistry()
	fake := &fakeStatusCoreClient{
		err: status.Error(codes.Unimplemented, "not implemented yet"),
	}
	deps := codeIntelDeps{
		core:   fake,
		limits: CodeIntelLimits{MaxResponseBytes: 2 << 20},
	}
	registerCodeIntelStatus(r, deps)

	// 1. DeviceID != "" -> NOT_AUTHORIZED
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1", DeviceID: "dev-1"}, "codeIntel.status", []json.RawMessage{json.RawMessage(`{"projectId":"p1","worktreeId":"w1"}`)})
	if err == nil || !strings.Contains(err.Error(), "NOT_AUTHORIZED") {
		t.Fatalf("expected NOT_AUTHORIZED for device session, got: %v", err)
	}

	// 2. Missing params / selector -> INVALID_PARAMS mentioning projectId
	_, err = r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.status", []json.RawMessage{})
	if err == nil || !strings.Contains(err.Error(), "INVALID_PARAMS") || !strings.Contains(err.Error(), "projectId") {
		t.Fatalf("expected INVALID_PARAMS with projectId, got: %v", err)
	}

	// 3. Forbidden keys -> INVALID_PARAMS
	_, err = r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.status", []json.RawMessage{json.RawMessage(`{"projectId":"p1","worktreeId":"w1","tenantId":"hack"}`)})
	if err == nil || !strings.Contains(err.Error(), "INVALID_PARAMS") {
		t.Fatalf("expected INVALID_PARAMS for forbidden key, got: %v", err)
	}

	// 4. Unimplemented error from core -> CODEINTEL_UNAVAILABLE
	_, err = r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.status", []json.RawMessage{json.RawMessage(`{"projectId":"p1","worktreeId":"w1"}`)})
	if err == nil || !strings.Contains(err.Error(), "CODEINTEL_UNAVAILABLE") {
		t.Fatalf("expected CODEINTEL_UNAVAILABLE for unimplemented core, got: %v", err)
	}
}
