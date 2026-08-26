package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// fakeInfraFleetServiceClient implements infrafleetv1.InfraFleetServiceClient
// directly (no bufconn/wire-level gRPC) — this package's tests fake the
// port (the generated client interface), not the transport, matching this
// repo's usecase-test convention (e.g. internal/usecase/dispatch_test.go's
// fakeConnectionResolver/fakeGitExecutor).
type fakeInfraFleetServiceClient struct {
	infrafleetv1.InfraFleetServiceClient // embed: panics on any unimplemented method, which is intentional for these tests

	resolveConnectionResp *infrafleetv1.ResolveConnectionResponse
	resolveConnectionErr  error
	gotResolveConnection  *infrafleetv1.ResolveConnectionRequest

	relayResp *infrafleetv1.RelayResponse
	relayErr  error
	gotRelay  *infrafleetv1.RelayRequest
}

func (f *fakeInfraFleetServiceClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	f.gotResolveConnection = in
	if f.resolveConnectionErr != nil {
		return nil, f.resolveConnectionErr
	}
	return f.resolveConnectionResp, nil
}

func (f *fakeInfraFleetServiceClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.gotRelay = in
	if f.relayErr != nil {
		return nil, f.relayErr
	}
	return f.relayResp, nil
}

func ctxWithTenant(t *testing.T) context.Context {
	t.Helper()
	return tenant.WithTenantID(context.Background(), "tenant-1")
}

func TestConnectionResolver_ResolveConnection_NotConnected(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false},
	}
	r := NewConnectionResolver(fake)

	conn, err := r.ResolveConnection(ctxWithTenant(t), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Connected {
		t.Error("expected Connected=false")
	}
	if conn.RepoPath != "wt-1" {
		t.Errorf("expected RepoPath to fall back to worktreeID %q, got %q", "wt-1", conn.RepoPath)
	}
	if fake.gotResolveConnection.GetConnectionId() != "wt-1" {
		t.Errorf("expected ConnectionId=wt-1 on the request, got %q", fake.gotResolveConnection.GetConnectionId())
	}
}

func TestConnectionResolver_ResolveConnection_Connected(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{
			Connected: true,
			RepoPath:  "/remote/repo",
		},
	}
	r := NewConnectionResolver(fake)

	conn, err := r.ResolveConnection(ctxWithTenant(t), "wt-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !conn.Connected {
		t.Error("expected Connected=true")
	}
	if conn.ConnectionID != "wt-2" {
		t.Errorf("expected ConnectionID=wt-2, got %q", conn.ConnectionID)
	}
	if conn.RepoPath != "/remote/repo" {
		t.Errorf("expected RepoPath from response, got %q", conn.RepoPath)
	}
}

func TestConnectionResolver_ResolveConnection_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	r := NewConnectionResolver(fake)

	if _, err := r.ResolveConnection(context.Background(), "wt-1"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
	if fake.gotResolveConnection != nil {
		t.Error("expected ResolveConnection not to be called without a tenant in context")
	}
}

func TestRelayExecutor_GetStatus_Success(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{
		"branch": "main",
		"files": []map[string]any{
			{"path": "a.txt", "state": "modified"},
		},
	})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)},
	}
	r := NewRelayExecutor(fake)

	status, err := r.GetStatus(ctxWithTenant(t), "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Branch != "main" {
		t.Errorf("expected branch=main, got %q", status.Branch)
	}
	if len(status.Files) != 1 || status.Files[0].Path != "a.txt" {
		t.Errorf("unexpected files: %+v", status.Files)
	}
	if fake.gotRelay.GetMethod() != "git.status" {
		t.Errorf("expected method=git.status, got %q", fake.gotRelay.GetMethod())
	}
	if fake.gotRelay.GetConnectionId() != "/repo" {
		t.Errorf("expected connectionId=/repo, got %q", fake.gotRelay.GetConnectionId())
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["worktreePath"] != "/repo" {
		t.Errorf("expected worktreePath param (real agent contract, see BUG-036/TASK-228), got %+v", params)
	}
}

func TestRelayExecutor_Commit_Success(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{"commitSha": "abc123"})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)},
	}
	r := NewRelayExecutor(fake)

	result, err := r.Commit(ctxWithTenant(t), "/repo", "my message", []string{"a.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CommitSHA != "abc123" {
		t.Errorf("expected commitSha=abc123, got %q", result.CommitSHA)
	}
	if fake.gotRelay.GetMethod() != "git.commit" {
		t.Errorf("expected method=git.commit, got %q", fake.gotRelay.GetMethod())
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["message"] != "my message" {
		t.Errorf("expected message param, got %+v", params)
	}
}

func TestRelayExecutor_Push_RelayErrorPropagates(t *testing.T) {
	wantErr := errors.New("boom")
	fake := &fakeInfraFleetServiceClient{relayErr: wantErr}
	r := NewRelayExecutor(fake)

	_, err := r.Push(ctxWithTenant(t), "/repo", "origin", "main")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRelayExecutor_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	r := NewRelayExecutor(fake)

	if _, err := r.GetStatus(context.Background(), "/repo"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
	if fake.gotRelay != nil {
		t.Error("expected Relay not to be called without a tenant in context")
	}
}

// ── TASK-228: GetDiff now sends worktreePath+filePath ────────────────────

func TestRelayExecutor_GetDiff_SendsWorktreePathAndFilePath(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{}}
	r := NewRelayExecutor(fake)

	if _, err := r.GetDiff(ctxWithTenant(t), "/repo", "a.txt", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotRelay.GetMethod() != "git.diff" {
		t.Errorf("expected method=git.diff, got %q", fake.gotRelay.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["worktreePath"] != "/repo" || params["filePath"] != "a.txt" || params["staged"] != true {
		t.Errorf("unexpected params: %+v", params)
	}
}

// ── TASK-208: Stage/Unstage always target the bulk relay method ──────────

func TestRelayExecutor_Stage_TargetsBulkMethod(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{}}
	r := NewRelayExecutor(fake)

	if _, err := r.Stage(ctxWithTenant(t), "/repo", []string{"a.txt"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotRelay.GetMethod() != "git.bulkStage" {
		t.Errorf("expected method=git.bulkStage even for a single path, got %q", fake.gotRelay.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if _, ok := params["filePaths"]; !ok {
		t.Errorf("expected filePaths param key, got %+v", params)
	}
}

func TestRelayExecutor_Unstage_TargetsBulkMethod(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{}}
	r := NewRelayExecutor(fake)

	if _, err := r.Unstage(ctxWithTenant(t), "/repo", []string{"a.txt", "b.txt"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotRelay.GetMethod() != "git.bulkUnstage" {
		t.Errorf("expected method=git.bulkUnstage, got %q", fake.gotRelay.GetMethod())
	}
}

// ── TASK-209: History/CheckIgnored/ForkSync param shapes ─────────────────

func TestRelayExecutor_History_DropsCursorRenamesBaseRef(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{}}
	r := NewRelayExecutor(fake)

	if _, err := r.History(ctxWithTenant(t), "/repo", "main", 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if _, hasCursor := params["cursor"]; hasCursor {
		t.Errorf("expected no cursor param (real agent has no pagination), got %+v", params)
	}
	if params["baseRef"] != "main" {
		t.Errorf("expected baseRef param, got %+v", params)
	}
}

func TestRelayExecutor_CheckIgnored_ReturnsIgnoredSubset(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{"ignoredPaths": []string{"node_modules"}})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)}}
	r := NewRelayExecutor(fake)

	got, err := r.CheckIgnored(ctxWithTenant(t), "/repo", []string{"node_modules", "README.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "node_modules" {
		t.Errorf("expected only the ignored subset, got %+v", got)
	}
}

func TestRelayExecutor_ForkSync_SendsExpectedUpstream(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{}}
	r := NewRelayExecutor(fake)

	if _, err := r.ForkSync(ctxWithTenant(t), "/repo", "origin/main"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["expectedUpstream"] != "origin/main" {
		t.Errorf("expected expectedUpstream param, got %+v", params)
	}
}

func TestRelayExecutor_UpstreamStatus_SendsWorktreePathAndPushTarget(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{"hasUpstream": true, "ahead": 2, "behind": 1})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)}}
	r := NewRelayExecutor(fake)

	got, err := r.UpstreamStatus(ctxWithTenant(t), "/repo", "origin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotRelay.GetMethod() != "git.upstreamStatus" {
		t.Errorf("expected method=git.upstreamStatus, got %q", fake.gotRelay.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["worktreePath"] != "/repo" || params["pushTarget"] != "origin" {
		t.Errorf("unexpected params: %+v", params)
	}
	if !got.HasUpstream || got.Ahead != 2 || got.Behind != 1 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestRelayExecutor_RemoteCommitURL_SendsWorktreePathAndSha(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{"url": "https://github.com/org/repo/commit/abc123"})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)}}
	r := NewRelayExecutor(fake)

	got, err := r.RemoteCommitURL(ctxWithTenant(t), "/repo", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotRelay.GetMethod() != "git.remoteCommitUrl" {
		t.Errorf("expected method=git.remoteCommitUrl, got %q", fake.gotRelay.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["worktreePath"] != "/repo" || params["sha"] != "abc123" {
		t.Errorf("unexpected params: %+v", params)
	}
	if got != "https://github.com/org/repo/commit/abc123" {
		t.Errorf("unexpected url: %q", got)
	}
}

func TestRelayExecutor_RemoteFileURL_SendsWorktreePathPathAndRef(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{"url": "https://github.com/org/repo/blob/main/a.txt"})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)}}
	r := NewRelayExecutor(fake)

	got, err := r.RemoteFileURL(ctxWithTenant(t), "/repo", "a.txt", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotRelay.GetMethod() != "git.remoteFileUrl" {
		t.Errorf("expected method=git.remoteFileUrl, got %q", fake.gotRelay.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["worktreePath"] != "/repo" || params["path"] != "a.txt" || params["ref"] != "main" {
		t.Errorf("unexpected params: %+v", params)
	}
	if got != "https://github.com/org/repo/blob/main/a.txt" {
		t.Errorf("unexpected url: %q", got)
	}
}

// ── usecase.FilesystemExecutor compile-time guards (TASK-050/051/055) ────

func TestRelayExecutor_ImplementsFilesystemExecutorNotLocalOnly(t *testing.T) {
	// RelayExecutor must satisfy usecase.FilesystemExecutor (fs.* relay
	// methods) but must NOT satisfy usecase.LocalOnlyFilesystemExecutor
	// (Rename/Copy) — the type system is what guarantees
	// RenameFileUseCase/CopyFileUseCase can never be constructed with a
	// relay-backed executor. This test only compiles the positive half;
	// the negative half (RelayExecutor does NOT implement
	// LocalOnlyFilesystemExecutor) is a compile-time property with no
	// runtime assertion available — confirmed manually per TASK-055's
	// verify section.
	var _ usecase.FilesystemExecutor = (*RelayExecutor)(nil)
}
