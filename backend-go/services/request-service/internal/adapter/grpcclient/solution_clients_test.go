package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAICompletionRelay_CompletesThroughRelayAndForwardsTenant(t *testing.T) {
	infra := &fakeInfra{replies: []string{"xin chào"}}
	c := NewAICompletionRelay(infra, stubResolver{conn: AIConnection{ConnectionID: "proj"}}, nil, time.Second)
	got, err := c.Complete(ctxWithIdentity(), "p1", "PROMPT")
	if err != nil || got != "xin chào" {
		t.Fatalf("got %q err %v", got, err)
	}
	call := infra.relayCalls[0]
	if call.GetMethod() != "ai.complete" || call.GetConnectionId() != "proj" {
		t.Fatalf("call = %+v", call)
	}
	var params map[string]any
	_ = json.Unmarshal([]byte(call.GetParamsJson()), &params)
	if params["prompt"] != "PROMPT" || len(params) != 1 {
		t.Fatalf("params = %v", params)
	}
	if len(infra.lastMetadata.Get("x-orca-tenant-id")) != 1 {
		t.Fatalf("tenant not forwarded: %v", infra.lastMetadata)
	}
}

func TestAICompletionRelay_FallsBackToTheDevServerAndMapsErrors(t *testing.T) {
	infra := &fakeInfra{replies: []string{"ok"}}
	c := NewAICompletionRelay(infra, stubResolver{conn: AIConnection{DevServerID: "ds-1"}}, nil, time.Second)
	if _, err := c.Complete(ctxWithIdentity(), "p", "x"); err != nil || len(infra.devCalls) != 1 || infra.devCalls[0].GetDevServerId() != "ds-1" {
		t.Fatalf("dev server route: %v %+v", err, infra.devCalls)
	}

	noServer := NewAICompletionRelay(infra, stubResolver{err: usecase.ErrNoDevServer}, nil, time.Second)
	if _, err := noServer.Complete(ctxWithIdentity(), "p", "x"); !errors.Is(err, usecase.ErrNoDevServer) {
		t.Fatalf("err = %v", err)
	}

	slow := &fakeInfra{replies: []string{"late"}, relayDelay: 300 * time.Millisecond}
	timed := NewAICompletionRelay(slow, stubResolver{conn: AIConnection{ConnectionID: "c"}}, nil, 20*time.Millisecond)
	if _, err := timed.Complete(ctxWithIdentity(), "p", "x"); !errors.Is(err, usecase.ErrClassifierTimeout) {
		t.Fatalf("timeout err = %v", err)
	}
	if _, err := NewAICompletionRelay(infra, stubResolver{conn: AIConnection{ConnectionID: "c"}}, nil, time.Second).Complete(tenantlessCtx(), "p", "x"); err == nil {
		t.Fatal("tenant is required")
	}
}

func tenantlessCtx() context.Context { return context.Background() }

func TestAnalysisConnections_KeepsRepoPathAndWorktree(t *testing.T) {
	infra := &fakeInfra{connected: true, repoPath: "/srv/repo", worktreeID: "wt-9"}
	resolver := NewAIConnectionResolver(infra, &fakeProjects{})
	got, err := NewAnalysisConnections(resolver).ResolveForProject(ctxWithIdentity(), "proj")
	if err != nil || got.ConnectionID != "proj" || got.RepoPath != "/srv/repo" || got.WorktreeID != "wt-9" {
		t.Fatalf("got %+v err %v", got, err)
	}
	// The dev-server fallback has no repo path or worktree to offer.
	fallback := &fakeInfra{connected: false, health: []*infrafleetv1.DevServerHealth{{DevServerId: "ds-1", Reachable: true}}}
	projects := &fakeProjects{repos: []*projectv1.Repo{{DevServerId: "ds-1"}}}
	got, err = NewAnalysisConnections(NewAIConnectionResolver(fallback, projects)).ResolveForProject(ctxWithIdentity(), "proj")
	if err != nil || got.DevServerID != "ds-1" || got.RepoPath != "" || got.WorktreeID != "" {
		t.Fatalf("fallback got %+v err %v", got, err)
	}
	_, err = NewAnalysisConnections(NewAIConnectionResolver(&fakeInfra{}, &fakeProjects{})).ResolveForProject(ctxWithIdentity(), "proj")
	if !errors.Is(err, usecase.ErrNoDevServer) {
		t.Fatalf("err = %v", err)
	}
}

// --- agent.execPrompt ---

type fakeAgentInfra struct {
	infrafleetv1.InfraFleetServiceClient
	mu     sync.Mutex
	result string
	err    error
	relay  []*infrafleetv1.RelayRequest
	dev    []*infrafleetv1.RelayByDevServerRequest
	md     metadata.MD
}

func (f *fakeAgentInfra) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.relay = append(f.relay, in)
	f.md, _ = metadata.FromOutgoingContext(ctx)
	return &infrafleetv1.RelayResponse{ResultJson: f.result}, f.err
}

func (f *fakeAgentInfra) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dev = append(f.dev, in)
	return &infrafleetv1.RelayResponse{ResultJson: f.result}, f.err
}

func paramsOf(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAgentPromptRelay_PromptOnlyParamsAreDefaultTrustWithTwoEnvKeys(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"stdout":"ok","exitCode":0}`}
	r := NewAgentPromptRelay(infra)
	res, err := r.ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c1"}, usecase.AgentPromptInput{
		StepID: "run-1", Prompt: "P", RepoPath: "/srv/repo", RequestID: "req-1", ProjectID: "proj-1", TimeoutMS: 1000,
	})
	if err != nil || res.Stdout != "ok" || res.ExitCode != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	call := infra.relay[0]
	if call.GetMethod() != "agent.execPrompt" || call.GetConnectionId() != "c1" {
		t.Fatalf("call = %+v", call)
	}
	p := paramsOf(t, call.GetParamsJson())
	if p["worktreePath"] != "/srv/repo" || p["stepId"] != "run-1" || p["trustPreset"] != "default" || p["prompt"] != "P" {
		t.Fatalf("params = %v", p)
	}
	env := p["env"].(map[string]any)
	if len(env) != 2 || env["ORCA_REQUEST_ID"] != "req-1" || env["ORCA_PROJECT_ID"] != "proj-1" {
		t.Fatalf("env = %v", env)
	}
	for _, k := range []string{"accessMode", "workspaceKind", "reportChanges", "maxOutputBytes"} {
		if _, has := p[k]; has {
			t.Errorf("prompt-only run sent %s, which old agents would silently ignore", k)
		}
	}
	if len(infra.md.Get("x-orca-tenant-id")) != 1 {
		t.Fatal("tenant not forwarded")
	}
}

func TestAgentPromptRelay_EnforcedReadonlyParams(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"stdout":"{}","exitCode":0,"applied":{"accessMode":"readonly","workspaceKind":"repo_root"},"changes":{"available":true,"headMoved":true,"changedFiles":[{"path":"a"},{"path":"b"}]},"warnings":["READONLY_VIOLATION"]}`}
	res, err := NewAgentPromptRelay(infra).ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{DevServerID: "ds-1"}, usecase.AgentPromptInput{
		StepID: "s", Prompt: "P", RepoPath: "/r", ReadOnlyEnforced: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(infra.dev) != 1 || len(infra.relay) != 0 || infra.dev[0].GetDevServerId() != "ds-1" {
		t.Fatalf("route wrong: %+v", infra)
	}
	p := paramsOf(t, infra.dev[0].GetParamsJson())
	if p["accessMode"] != "readonly" || p["workspaceKind"] != "repo_root" || p["reportChanges"] != true || p["maxOutputBytes"] == nil {
		t.Fatalf("params = %v", p)
	}
	if _, has := p["trustPreset"]; has {
		t.Fatal("trustPreset must not be sent with accessMode=readonly")
	}
	if res.AppliedAccessMode != "readonly" || !res.HeadMoved || res.ChangedFiles != 2 || !res.ChangesAvailable || len(res.Warnings) != 1 || res.Warnings[0] != "READONLY_VIOLATION" {
		t.Fatalf("res = %+v", res)
	}
}

// buildAgentParams has no code path that yields full trust: check every combination of the inputs that vary.
func TestBuildAgentParams_NeverFullTrust(t *testing.T) {
	for _, ro := range []bool{false, true} {
		for _, timeout := range []int{0, 5000} {
			raw, _ := json.Marshal(buildAgentParams(usecase.AgentPromptInput{Prompt: "p", RepoPath: "/r", ReadOnlyEnforced: ro, TimeoutMS: timeout}))
			if strings.Contains(string(raw), `"full"`) {
				t.Fatalf("ro=%v timeout=%d params carry full trust: %s", ro, timeout, raw)
			}
		}
	}
}

func TestAgentPromptRelay_ResultDetails(t *testing.T) {
	r := NewAgentPromptRelay(&fakeAgentInfra{result: `{"stdout":"x","stderr":"e","timedOut":true}`})
	res, err := r.ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, usecase.AgentPromptInput{Prompt: "p"})
	if err != nil || res.ExitCode != -1 || !res.TimedOut || res.Stderr != "e" {
		t.Fatalf("no exit code must read as -1: %+v %v", res, err)
	}
	big := strings.Repeat("ế", 100*1024) // 300 KB of three-byte runes
	b, _ := json.Marshal(map[string]any{"stdout": big, "exitCode": 0})
	res, err = NewAgentPromptRelay(&fakeAgentInfra{result: string(b)}).ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, usecase.AgentPromptInput{Prompt: "p"})
	if err != nil || len(res.Stdout) > maxStdoutBytes || !strings.HasSuffix(res.Stdout, "ế") {
		t.Fatalf("stdout %d bytes, err %v", len(res.Stdout), err)
	}
	if _, err := NewAgentPromptRelay(&fakeAgentInfra{result: "not json"}).ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, usecase.AgentPromptInput{}); err == nil {
		t.Fatal("garbage result must error")
	}
}

func TestAgentPromptRelay_ErrorMapping(t *testing.T) {
	cases := map[string]struct {
		err  error
		want func(error) bool
	}{
		"readonly unsupported": {status.Error(codes.InvalidArgument, "agent error: READONLY_MODE_UNSUPPORTED"), func(e error) bool { return errors.Is(e, usecase.ErrAgentReadonlyUnsupported) }},
		"deadline":             {status.Error(codes.DeadlineExceeded, "slow"), func(e error) bool { return errors.Is(e, context.DeadlineExceeded) }},
		"other":                {errors.New("boom"), func(e error) bool { return e != nil && !errors.Is(e, usecase.ErrAgentReadonlyUnsupported) }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewAgentPromptRelay(&fakeAgentInfra{err: tc.err}).ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, usecase.AgentPromptInput{})
			if !tc.want(err) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if _, err := NewAgentPromptRelay(&fakeAgentInfra{}).ExecPrompt(context.Background(), usecase.AnalysisConnection{}, usecase.AgentPromptInput{}); err == nil {
		t.Fatal("tenant is required")
	}
}

// --- git-gateway probe ---

type fakeGit struct {
	gitgatewayv1.GitGatewayServiceClient
	resp  *gitgatewayv1.GetStatusResponse
	err   error
	calls []string
}

func (f *fakeGit) GetStatus(_ context.Context, in *gitgatewayv1.GetStatusRequest, _ ...grpc.CallOption) (*gitgatewayv1.GetStatusResponse, error) {
	f.calls = append(f.calls, in.GetWorktreeId())
	return f.resp, f.err
}

func TestRepoStateProbe(t *testing.T) {
	git := &fakeGit{resp: &gitgatewayv1.GetStatusResponse{Branch: "main", Files: []*gitgatewayv1.FileStatus{{Path: "b.go", State: "added"}, {Path: "a.go", State: "modified"}}}}
	p := NewRepoStateProbe(git)
	snap, err := p.Snapshot(ctxWithIdentity(), "wt-1")
	if err != nil || snap.Branch != "main" || len(snap.Files) != 2 || snap.Files[0].Path != "a.go" || git.calls[0] != "wt-1" {
		t.Fatalf("snap=%+v err=%v calls=%v", snap, err, git.calls)
	}
	other := usecase.RepoSnapshot{Branch: "main", Files: []usecase.RepoFileState{{Path: "b.go", State: "added"}, {Path: "a.go", State: "modified"}}}
	if !snap.Equal(other) {
		t.Fatal("order of files must not matter")
	}
	if _, err := p.Snapshot(ctxWithIdentity(), ""); !errors.Is(err, usecase.ErrProbeUnavailable) || len(git.calls) != 1 {
		t.Fatalf("empty worktree id: err=%v calls=%v", err, git.calls)
	}
	if _, err := NewRepoStateProbe(&fakeGit{err: errors.New("down")}).Snapshot(ctxWithIdentity(), "wt"); err == nil {
		t.Fatal("RPC error must surface")
	}
}

type fakeProjectContext struct {
	projectv1.ProjectServiceClient
	resp *projectv1.ProjectContext
	err  error
}

func (f fakeProjectContext) GetProjectContext(context.Context, *projectv1.GetProjectContextRequest, ...grpc.CallOption) (*projectv1.ProjectContext, error) {
	return f.resp, f.err
}

func TestProjectContextResolver(t *testing.T) {
	r := NewProjectContextResolver(fakeProjectContext{resp: &projectv1.ProjectContext{ProjectName: "orca", RepoUrl: "https://x/y.git"}})
	got, err := r.Read(ctxWithIdentity(), "p")
	if err != nil || got.Name != "orca" || got.RepoURL != "https://x/y.git" {
		t.Fatalf("got %+v err %v", got, err)
	}
	if _, err := NewProjectContextResolver(fakeProjectContext{err: errors.New("down")}).Read(ctxWithIdentity(), "p"); err == nil {
		t.Fatal("error must surface so the caller can continue without context")
	}
	if _, err := r.Read(tenantOnly(), "p"); err == nil {
		t.Fatal("project-service needs the acting user")
	}
}
