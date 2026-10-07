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

type fakeGraphCoreClient struct {
	codeintelv1.CodeIntelServiceClient
	structReq   *codeintelv1.GetStructureRequest
	structResp  *codeintelv1.GetStructureResponse
	structErr   error
	routeReq    *codeintelv1.GetRouteMapRequest
	routeResp   *codeintelv1.GetRouteMapResponse
	routeErr    error
}

func (f *fakeGraphCoreClient) GetStructure(ctx context.Context, in *codeintelv1.GetStructureRequest, opts ...grpc.CallOption) (*codeintelv1.GetStructureResponse, error) {
	f.structReq = in
	if f.structErr != nil {
		return nil, f.structErr
	}
	return f.structResp, nil
}

func (f *fakeGraphCoreClient) GetRouteMap(ctx context.Context, in *codeintelv1.GetRouteMapRequest, opts ...grpc.CallOption) (*codeintelv1.GetRouteMapResponse, error) {
	f.routeReq = in
	if f.routeErr != nil {
		return nil, f.routeErr
	}
	return f.routeResp, nil
}

func TestCodeIntelViewStructure_SuccessEnvelope(t *testing.T) {
	r := NewRegistry()
	fake := &fakeGraphCoreClient{
		structResp: &codeintelv1.GetStructureResponse{
			Meta: &codeintelv1.ResultMeta{
				Repo:       "orca",
				WorktreeId: "wt-1",
				View:       codeintelv1.ViewKind_VIEW_KIND_STRUCTURE,
				TotalCount: 5,
			},
			Data: &codeintelv1.ModuleGraph{
				Nodes: []*codeintelv1.ModuleNode{
					{Id: "src/main.go", Language: "go"},
				},
			},
			NextPageToken: "next-token-123",
		},
	}
	deps := codeIntelDeps{
		core:   fake,
		limits: CodeIntelLimits{MaxResponseBytes: 2 << 20},
	}
	registerCodeIntelStructure(r, deps)

	rawArgs := []json.RawMessage{json.RawMessage(`{"projectId":"proj-1","worktreeId":"wt-1","path":"src","depth":2,"limit":10,"pageToken":"token-1","ifNoneMatch":"etag-1"}`)}
	res, err := r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.structure", rawArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.structReq == nil {
		t.Fatalf("core client not invoked")
	}
	if fake.structReq.GetSelector().GetProjectId() != "proj-1" || fake.structReq.GetSelector().GetWorktreeRef() != "wt-1" {
		t.Errorf("unexpected selector in request: %+v", fake.structReq.GetSelector())
	}
	if fake.structReq.GetPath() != "src" || fake.structReq.GetDepth() != 2 || fake.structReq.GetLimit() != 10 {
		t.Errorf("unexpected structure params: %+v", fake.structReq)
	}
	if fake.structReq.GetPageToken() != "token-1" || fake.structReq.GetIfNoneMatch() != "etag-1" {
		t.Errorf("unexpected pageToken/ifNoneMatch: %+v", fake.structReq)
	}

	bytes, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	str := string(bytes)

	if !strings.Contains(str, `"view":"structure"`) || !strings.Contains(str, `"data":`) {
		t.Errorf("expected envelope with view and data, got: %s", str)
	}
	if !strings.Contains(str, `"nextPageToken":"next-token-123"`) {
		t.Errorf("expected nextPageToken verbatim in envelope, got: %s", str)
	}
	if !strings.Contains(str, `"src/main.go"`) {
		t.Errorf("expected moduleNode path in data, got: %s", str)
	}
}

func TestCodeIntelViewStructure_ValidationTable(t *testing.T) {
	r := NewRegistry()
	fake := &fakeGraphCoreClient{}
	deps := codeIntelDeps{
		core:   fake,
		limits: CodeIntelLimits{MaxResponseBytes: 2 << 20},
	}
	registerCodeIntelStructure(r, deps)

	tests := []struct {
		name    string
		payload string
		errSub  string
	}{
		{"depth below 1", `{"projectId":"p1","worktreeId":"w1","depth":0}`, "depth"},
		{"depth above 3", `{"projectId":"p1","worktreeId":"w1","depth":4}`, "depth"},
		{"path traversal", `{"projectId":"p1","worktreeId":"w1","path":"../secret"}`, "CODEINTEL_PATH_NOT_ALLOWED"},
		{"limit below 1", `{"projectId":"p1","worktreeId":"w1","limit":0}`, "limit"},
		{"ifNoneMatch too long", `{"projectId":"p1","worktreeId":"w1","ifNoneMatch":"` + strings.Repeat("a", 81) + `"}`, "ifNoneMatch"},
		{"forbidden key", `{"projectId":"p1","worktreeId":"w1","command":"ls"}`, "command"},
		{"unknown field", `{"projectId":"p1","worktreeId":"w1","unknown":123}`, "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.structure", []json.RawMessage{json.RawMessage(tc.payload)})
			if err == nil || !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("expected error containing %q, got: %v", tc.errSub, err)
			}
		})
	}
}

func TestCodeIntelViewRoutes_SuccessEnvelopeAndValidation(t *testing.T) {
	r := NewRegistry()
	fake := &fakeGraphCoreClient{
		routeResp: &codeintelv1.GetRouteMapResponse{
			Meta: &codeintelv1.ResultMeta{
				Repo:       "orca",
				WorktreeId: "wt-1",
				View:       codeintelv1.ViewKind_VIEW_KIND_ROUTES,
			},
			Data: &codeintelv1.RouteMap{
				Routes: []*codeintelv1.RouteNode{
					{Path: "/api/v1/health", Method: "GET"},
				},
			},
		},
	}
	deps := codeIntelDeps{
		core:   fake,
		limits: CodeIntelLimits{MaxResponseBytes: 2 << 20},
	}
	registerCodeIntelRoutes(r, deps)

	// 1. Success
	rawArgs := []json.RawMessage{json.RawMessage(`{"projectId":"proj-1","worktreeId":"wt-1","limit":100}`)}
	res, err := r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.routes", rawArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.routeReq == nil || fake.routeReq.GetLimit() != 100 {
		t.Errorf("unexpected routeReq: %+v", fake.routeReq)
	}

	bytes, _ := json.Marshal(res)
	str := string(bytes)
	if !strings.Contains(str, `"/api/v1/health"`) {
		t.Errorf("expected route path in output, got: %s", str)
	}

	// 2. Validation bounds: limit 0 or 501
	_, err = r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.routes", []json.RawMessage{json.RawMessage(`{"projectId":"p1","worktreeId":"w1","limit":0}`)})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected limit out of range error for 0, got: %v", err)
	}

	_, err = r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.routes", []json.RawMessage{json.RawMessage(`{"projectId":"p1","worktreeId":"w1","limit":501}`)})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected limit out of range error for 501, got: %v", err)
	}

	// 3. Unimplemented core error -> CODEINTEL_UNAVAILABLE
	fake.routeErr = status.Error(codes.Unimplemented, "unimplemented route map")
	_, err = r.Dispatch(context.Background(), Identity{TenantID: "t-1", UserID: "u-1"}, "codeIntel.routes", []json.RawMessage{json.RawMessage(`{"projectId":"p1","worktreeId":"w1"}`)})
	if err == nil || !strings.Contains(err.Error(), "CODEINTEL_UNAVAILABLE") {
		t.Fatalf("expected CODEINTEL_UNAVAILABLE, got: %v", err)
	}
}
