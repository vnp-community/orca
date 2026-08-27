package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// fakeGitGatewayServiceClient implements gitgatewayv1.GitGatewayServiceClient
// over per-method hook functions for the 6 methods this REST-route layer's
// tests exercise — tests set only the hooks they exercise; any unset hook
// fails the test loudly instead of nil-panicking. The embedded (nil)
// interface satisfies every OTHER GitGatewayServiceClient method added
// since this fake was written (staging/history/remote/AI-assist/files.*,
// TASK-208..211/049) without this REST-route test file needing a hook for
// each — those RPCs aren't exercised by git_routes.go's REST handlers, so
// a panic-on-call default is correct: it would mean this file started
// calling one of them and needs a real hook added.
// over per-method hook functions — tests set only the hooks they exercise;
// any unset hook fails the test loudly instead of nil-panicking.
//
// The 7 worktree.*-Func hooks below (CreateWorktree/RemoveWorktree/
// ForceDeleteBranch/DetectWorktrees/PrefetchCreateBase/ResolvePrBase/
// ResolveMrBase) were added to keep this fake satisfying
// gitgatewayv1.GitGatewayServiceClient after TASK-192's proto regen grew
// the interface — this file's own git_routes.go usage is unaffected
// (SOL-031's worktree.* channels are wired through wscompat, not
// httpgateway's REST routes); none of this file's existing tests exercise
// them, so their hooks are expected to stay nil/unused.
type fakeGitGatewayServiceClient struct {
	gitgatewayv1.GitGatewayServiceClient

	t *testing.T

	getStatusFunc             func(ctx context.Context, in *gitgatewayv1.GetStatusRequest) (*gitgatewayv1.GetStatusResponse, error)
	getDiffFunc               func(ctx context.Context, in *gitgatewayv1.GetDiffRequest) (*gitgatewayv1.GetDiffResponse, error)
	commitFunc                func(ctx context.Context, in *gitgatewayv1.CommitRequest) (*gitgatewayv1.CommitResponse, error)
	pushFunc                  func(ctx context.Context, in *gitgatewayv1.PushRequest) (*gitgatewayv1.PushResponse, error)
	pullFunc                  func(ctx context.Context, in *gitgatewayv1.PullRequest) (*gitgatewayv1.PullResponse, error)
	generateCommitMessageFunc func(ctx context.Context, in *gitgatewayv1.GenerateCommitMessageRequest) (*gitgatewayv1.GenerateCommitMessageResponse, error)

	cloneFunc                  func(ctx context.Context, in *gitgatewayv1.CloneRequest) (*gitgatewayv1.CloneResponse, error)
	initRepoFunc               func(ctx context.Context, in *gitgatewayv1.InitRepoRequest) (*gitgatewayv1.InitRepoResponse, error)
	baseRefDefaultFunc         func(ctx context.Context, in *gitgatewayv1.BaseRefDefaultRequest) (*gitgatewayv1.BaseRefDefaultResponse, error)
	searchRefsFunc             func(ctx context.Context, in *gitgatewayv1.SearchRefsRequest) (*gitgatewayv1.SearchRefsResponse, error)
	checkHooksFunc             func(ctx context.Context, in *gitgatewayv1.CheckHooksRequest) (*gitgatewayv1.CheckHooksResponse, error)
	readIssueCommandFunc       func(ctx context.Context, in *gitgatewayv1.ReadIssueCommandRequest) (*gitgatewayv1.ReadIssueCommandResponse, error)
	writeIssueCommandFunc      func(ctx context.Context, in *gitgatewayv1.WriteIssueCommandRequest) (*emptypb.Empty, error)
	scanSetupScriptImportsFunc func(ctx context.Context, in *gitgatewayv1.ScanSetupScriptImportsRequest) (*gitgatewayv1.ScanSetupScriptImportsResponse, error)
	createWorktreeFunc         func(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error)
	removeWorktreeFunc         func(ctx context.Context, in *gitgatewayv1.RemoveWorktreeRequest) (*emptypb.Empty, error)
	forceDeleteBranchFunc      func(ctx context.Context, in *gitgatewayv1.ForceDeleteBranchRequest) (*emptypb.Empty, error)
	detectWorktreesFunc        func(ctx context.Context, in *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error)
	prefetchCreateBaseFunc     func(ctx context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest) (*gitgatewayv1.PrefetchCreateBaseResponse, error)
	resolvePrBaseFunc          func(ctx context.Context, in *gitgatewayv1.ResolvePrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error)
	resolveMrBaseFunc          func(ctx context.Context, in *gitgatewayv1.ResolveMrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error)
}

func (f *fakeGitGatewayServiceClient) GetStatus(ctx context.Context, in *gitgatewayv1.GetStatusRequest, _ ...grpc.CallOption) (*gitgatewayv1.GetStatusResponse, error) {
	if f.getStatusFunc == nil {
		f.t.Fatal("unexpected call to GetStatus")
	}
	return f.getStatusFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) GetDiff(ctx context.Context, in *gitgatewayv1.GetDiffRequest, _ ...grpc.CallOption) (*gitgatewayv1.GetDiffResponse, error) {
	if f.getDiffFunc == nil {
		f.t.Fatal("unexpected call to GetDiff")
	}
	return f.getDiffFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) Commit(ctx context.Context, in *gitgatewayv1.CommitRequest, _ ...grpc.CallOption) (*gitgatewayv1.CommitResponse, error) {
	if f.commitFunc == nil {
		f.t.Fatal("unexpected call to Commit")
	}
	return f.commitFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) Push(ctx context.Context, in *gitgatewayv1.PushRequest, _ ...grpc.CallOption) (*gitgatewayv1.PushResponse, error) {
	if f.pushFunc == nil {
		f.t.Fatal("unexpected call to Push")
	}
	return f.pushFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) Pull(ctx context.Context, in *gitgatewayv1.PullRequest, _ ...grpc.CallOption) (*gitgatewayv1.PullResponse, error) {
	if f.pullFunc == nil {
		f.t.Fatal("unexpected call to Pull")
	}
	return f.pullFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) GenerateCommitMessage(ctx context.Context, in *gitgatewayv1.GenerateCommitMessageRequest, _ ...grpc.CallOption) (*gitgatewayv1.GenerateCommitMessageResponse, error) {
	if f.generateCommitMessageFunc == nil {
		f.t.Fatal("unexpected call to GenerateCommitMessage")
	}
	return f.generateCommitMessageFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) Clone(ctx context.Context, in *gitgatewayv1.CloneRequest, _ ...grpc.CallOption) (*gitgatewayv1.CloneResponse, error) {
	if f.cloneFunc == nil {
		f.t.Fatal("unexpected call to Clone")
	}
	return f.cloneFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) InitRepo(ctx context.Context, in *gitgatewayv1.InitRepoRequest, _ ...grpc.CallOption) (*gitgatewayv1.InitRepoResponse, error) {
	if f.initRepoFunc == nil {
		f.t.Fatal("unexpected call to InitRepo")
	}
	return f.initRepoFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) BaseRefDefault(ctx context.Context, in *gitgatewayv1.BaseRefDefaultRequest, _ ...grpc.CallOption) (*gitgatewayv1.BaseRefDefaultResponse, error) {
	if f.baseRefDefaultFunc == nil {
		f.t.Fatal("unexpected call to BaseRefDefault")
	}
	return f.baseRefDefaultFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) SearchRefs(ctx context.Context, in *gitgatewayv1.SearchRefsRequest, _ ...grpc.CallOption) (*gitgatewayv1.SearchRefsResponse, error) {
	if f.searchRefsFunc == nil {
		f.t.Fatal("unexpected call to SearchRefs")
	}
	return f.searchRefsFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) CheckHooks(ctx context.Context, in *gitgatewayv1.CheckHooksRequest, _ ...grpc.CallOption) (*gitgatewayv1.CheckHooksResponse, error) {
	if f.checkHooksFunc == nil {
		f.t.Fatal("unexpected call to CheckHooks")
	}
	return f.checkHooksFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ReadIssueCommand(ctx context.Context, in *gitgatewayv1.ReadIssueCommandRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadIssueCommandResponse, error) {
	if f.readIssueCommandFunc == nil {
		f.t.Fatal("unexpected call to ReadIssueCommand")
	}
	return f.readIssueCommandFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) WriteIssueCommand(ctx context.Context, in *gitgatewayv1.WriteIssueCommandRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	if f.writeIssueCommandFunc == nil {
		f.t.Fatal("unexpected call to WriteIssueCommand")
	}
	return f.writeIssueCommandFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ScanSetupScriptImports(ctx context.Context, in *gitgatewayv1.ScanSetupScriptImportsRequest, _ ...grpc.CallOption) (*gitgatewayv1.ScanSetupScriptImportsResponse, error) {
	if f.scanSetupScriptImportsFunc == nil {
		f.t.Fatal("unexpected call to ScanSetupScriptImports")
	}
	return f.scanSetupScriptImportsFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	if f.createWorktreeFunc == nil {
		f.t.Fatal("unexpected call to CreateWorktree")
	}
	return f.createWorktreeFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) RemoveWorktree(ctx context.Context, in *gitgatewayv1.RemoveWorktreeRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	if f.removeWorktreeFunc == nil {
		f.t.Fatal("unexpected call to RemoveWorktree")
	}
	return f.removeWorktreeFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ForceDeleteBranch(ctx context.Context, in *gitgatewayv1.ForceDeleteBranchRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	if f.forceDeleteBranchFunc == nil {
		f.t.Fatal("unexpected call to ForceDeleteBranch")
	}
	return f.forceDeleteBranchFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) DetectWorktrees(ctx context.Context, in *gitgatewayv1.DetectWorktreesRequest, _ ...grpc.CallOption) (*gitgatewayv1.DetectWorktreesResponse, error) {
	if f.detectWorktreesFunc == nil {
		f.t.Fatal("unexpected call to DetectWorktrees")
	}
	return f.detectWorktreesFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) PrefetchCreateBase(ctx context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.PrefetchCreateBaseResponse, error) {
	if f.prefetchCreateBaseFunc == nil {
		f.t.Fatal("unexpected call to PrefetchCreateBase")
	}
	return f.prefetchCreateBaseFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ResolvePrBase(ctx context.Context, in *gitgatewayv1.ResolvePrBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.ResolveBaseResponse, error) {
	if f.resolvePrBaseFunc == nil {
		f.t.Fatal("unexpected call to ResolvePrBase")
	}
	return f.resolvePrBaseFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ResolveMrBase(ctx context.Context, in *gitgatewayv1.ResolveMrBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.ResolveBaseResponse, error) {
	if f.resolveMrBaseFunc == nil {
		f.t.Fatal("unexpected call to ResolveMrBase")
	}
	return f.resolveMrBaseFunc(ctx, in)
}

var _ gitgatewayv1.GitGatewayServiceClient = (*fakeGitGatewayServiceClient)(nil)

func newTestGitRouter(client gitgatewayv1.GitGatewayServiceClient) chi.Router {
	r := chi.NewRouter()
	mountGitRoutes(r, client)
	return r
}

// withTestIdentity injects id into the request context the same way
// authMiddleware does for real requests (see middleware.go's withIdentity),
// without needing a full JWT/session round-trip in these route tests.
func withTestIdentity(r *http.Request, id usecase.Identity) *http.Request {
	return r.WithContext(withIdentity(r.Context(), id))
}

// outgoingTenantID reads the tenant ID the handler attached via
// gatewaygrpc.AttachIdentity onto the outbound gRPC context — used to prove
// identity, not the JSON request body, is what reaches the downstream call.
func outgoingTenantID(t *testing.T, ctx context.Context) string {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing gRPC metadata on ctx (gatewaygrpc.AttachIdentity not called?)")
	}
	vals := md.Get("x-orca-tenant-id")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func TestMountGitRoutes_GetStatus_SuccessRoundTrip(t *testing.T) {
	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}
	client := &fakeGitGatewayServiceClient{
		t: t,
		getStatusFunc: func(ctx context.Context, in *gitgatewayv1.GetStatusRequest) (*gitgatewayv1.GetStatusResponse, error) {
			if in.GetWorktreeId() != "wt-1" {
				t.Fatalf("worktree_id = %q, want wt-1", in.GetWorktreeId())
			}
			if got := outgoingTenantID(t, ctx); got != identity.TenantID {
				t.Fatalf("outgoing tenant id = %q, want %q", got, identity.TenantID)
			}
			return &gitgatewayv1.GetStatusResponse{
				Branch: "main",
				Files:  []*gitgatewayv1.FileStatus{{Path: "foo.go", State: "modified"}},
			}, nil
		},
	}
	router := newTestGitRouter(client)

	req := withTestIdentity(httptest.NewRequest(http.MethodGet, "/v1/git/status?worktree_id=wt-1", nil), identity)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Branch string `json:"branch"`
		Files  []struct {
			Path  string `json:"path"`
			State string `json:"state"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v; body=%s", err, rec.Body.String())
	}
	if body.Branch != "main" {
		t.Fatalf("branch = %q, want main", body.Branch)
	}
	if len(body.Files) != 1 || body.Files[0].Path != "foo.go" || body.Files[0].State != "modified" {
		t.Fatalf("unexpected files: %+v", body.Files)
	}
}

// TestMountGitRoutes_Commit_IdentitySuppliesTenantIDNotBody proves tenant_id
// comes from the authenticated Identity, never from the JSON request body —
// even a body that tries to smuggle a tenant_id field is ignored (the
// request struct doesn't decode one), and the real tenant only reaches the
// downstream call via gatewaygrpc.AttachIdentity's outbound metadata.
func TestMountGitRoutes_Commit_IdentitySuppliesTenantIDNotBody(t *testing.T) {
	identity := usecase.Identity{TenantID: "real-tenant", UserID: "real-user"}
	var sawTenantID string
	client := &fakeGitGatewayServiceClient{
		t: t,
		commitFunc: func(ctx context.Context, in *gitgatewayv1.CommitRequest) (*gitgatewayv1.CommitResponse, error) {
			sawTenantID = outgoingTenantID(t, ctx)
			if in.GetWorktreeId() != "wt-1" || in.GetMessage() != "fix bug" {
				t.Fatalf("unexpected request: %+v", in)
			}
			return &gitgatewayv1.CommitResponse{CommitSha: "abc123"}, nil
		},
	}
	router := newTestGitRouter(client)

	// Body attempts to smuggle a spoofed tenant_id; the handler's request
	// struct has no such field, and identity must win regardless.
	bodyJSON := `{"worktree_id":"wt-1","message":"fix bug","tenant_id":"attacker-tenant"}`
	req := withTestIdentity(
		httptest.NewRequest(http.MethodPost, "/v1/git/commit", bytes.NewBufferString(bodyJSON)),
		identity,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if sawTenantID != identity.TenantID {
		t.Fatalf("downstream saw tenant id %q, want %q (from identity, not body)", sawTenantID, identity.TenantID)
	}
	if sawTenantID == "attacker-tenant" {
		t.Fatal("downstream call used the body-supplied tenant_id — identity was not enforced")
	}
}

// TestMountGitRoutes_GenerateCommitMessage_MapsUnimplementedTo501 covers
// the gRPC-error->HTTP-status mapping: git-gateway-service currently
// returns codes.Unimplemented for GenerateCommitMessage server-side, and
// writeGRPCError must surface that as HTTP 501.
func TestMountGitRoutes_GenerateCommitMessage_MapsUnimplementedTo501(t *testing.T) {
	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}
	client := &fakeGitGatewayServiceClient{
		t: t,
		generateCommitMessageFunc: func(ctx context.Context, in *gitgatewayv1.GenerateCommitMessageRequest) (*gitgatewayv1.GenerateCommitMessageResponse, error) {
			return nil, status.Error(codes.Unimplemented, "method GenerateCommitMessage not implemented")
		},
	}
	router := newTestGitRouter(client)

	bodyJSON := `{"worktree_id":"wt-1"}`
	req := withTestIdentity(
		httptest.NewRequest(http.MethodPost, "/v1/git/commit-message", bytes.NewBufferString(bodyJSON)),
		identity,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding error body: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != codes.Unimplemented.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.Unimplemented.String())
	}
}

func TestMountGitRoutes_CreateWorktree_SuccessRoundTrip(t *testing.T) {
	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}
	client := &fakeGitGatewayServiceClient{
		t: t,
		createWorktreeFunc: func(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error) {
			if in.GetProjectId() != "proj-1" || in.GetRepoId() != "repo-1" || in.GetBranch() != "feature-x" {
				t.Fatalf("unexpected request: %+v", in)
			}
			if got := outgoingTenantID(t, ctx); got != identity.TenantID {
				t.Fatalf("outgoing tenant id = %q, want %q", got, identity.TenantID)
			}
			return &gitgatewayv1.CreateWorktreeResponse{
				WorktreeId: "wt-1",
				Path:       "/worktrees/wt-1",
				HeadSha:    "abc123",
			}, nil
		},
	}
	router := newTestGitRouter(client)

	bodyJSON := `{"project_id":"proj-1","repo_id":"repo-1","branch":"feature-x","base_ref":"main"}`
	req := withTestIdentity(
		httptest.NewRequest(http.MethodPost, "/v1/worktrees", bytes.NewBufferString(bodyJSON)),
		identity,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		WorktreeID string `json:"worktree_id"`
		Path       string `json:"path"`
		HeadSha    string `json:"head_sha"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v; body=%s", err, rec.Body.String())
	}
	if body.WorktreeID != "wt-1" || body.Path != "/worktrees/wt-1" || body.HeadSha != "abc123" {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

// TestMountGitRoutes_CreateWorktree_MissingBranch_Returns400WithoutCallingClient
// proves validation happens before the downstream call — the fake's
// createWorktreeFunc is left nil, so any call to it fails the test via
// f.t.Fatal.
func TestMountGitRoutes_CreateWorktree_MissingBranch_Returns400WithoutCallingClient(t *testing.T) {
	client := &fakeGitGatewayServiceClient{t: t}
	router := newTestGitRouter(client)

	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}
	bodyJSON := `{"project_id":"proj-1","repo_id":"repo-1"}`
	req := withTestIdentity(
		httptest.NewRequest(http.MethodPost, "/v1/worktrees", bytes.NewBufferString(bodyJSON)),
		identity,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// TestMountGitRoutes_CreateWorktree_IdentitySuppliesTenantIDNotBody proves
// tenant_id comes from the authenticated Identity, never from the JSON
// request body — same guarantee as commit/push/pull's REST routes.
func TestMountGitRoutes_CreateWorktree_IdentitySuppliesTenantIDNotBody(t *testing.T) {
	identity := usecase.Identity{TenantID: "real-tenant", UserID: "real-user"}
	var sawTenantID string
	client := &fakeGitGatewayServiceClient{
		t: t,
		createWorktreeFunc: func(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error) {
			sawTenantID = outgoingTenantID(t, ctx)
			return &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-1"}, nil
		},
	}
	router := newTestGitRouter(client)

	bodyJSON := `{"project_id":"proj-1","repo_id":"repo-1","branch":"feature-x","tenant_id":"attacker-tenant"}`
	req := withTestIdentity(
		httptest.NewRequest(http.MethodPost, "/v1/worktrees", bytes.NewBufferString(bodyJSON)),
		identity,
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if sawTenantID != identity.TenantID {
		t.Fatalf("downstream saw tenant id %q, want %q (from identity, not body)", sawTenantID, identity.TenantID)
	}
}

func TestMountGitRoutes_MissingWorktreeID_Returns400(t *testing.T) {
	client := &fakeGitGatewayServiceClient{t: t}
	router := newTestGitRouter(client)

	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}
	req := withTestIdentity(httptest.NewRequest(http.MethodGet, "/v1/git/status", nil), identity)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
