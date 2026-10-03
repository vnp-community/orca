package mcppolicy

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type fakeClient struct {
	mcpv1.McpServiceClient // nil: any RPC the test does not stub panics loudly

	mu         sync.Mutex
	authorize  func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error)
	wait       func(*mcpv1.WaitApprovalRequest) (*mcpv1.WaitApprovalResponse, error)
	killState  func(*mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error)
	completed  []*mcpv1.CompleteToolCallRequest
	authorized []*mcpv1.AuthorizeToolCallRequest
	md         []metadata.MD
	deadlines  []bool
	killCalls  int
}

func (f *fakeClient) record(ctx context.Context) {
	md, _ := metadata.FromOutgoingContext(ctx)
	_, hasDeadline := ctx.Deadline()
	f.mu.Lock()
	f.md = append(f.md, md)
	f.deadlines = append(f.deadlines, hasDeadline)
	f.mu.Unlock()
}

func (f *fakeClient) AuthorizeToolCall(ctx context.Context, in *mcpv1.AuthorizeToolCallRequest, _ ...grpc.CallOption) (*mcpv1.AuthorizeToolCallResponse, error) {
	f.record(ctx)
	f.mu.Lock()
	f.authorized = append(f.authorized, in)
	f.mu.Unlock()
	return f.authorize(in)
}
func (f *fakeClient) WaitApproval(ctx context.Context, in *mcpv1.WaitApprovalRequest, _ ...grpc.CallOption) (*mcpv1.WaitApprovalResponse, error) {
	f.record(ctx)
	return f.wait(in)
}
func (f *fakeClient) CompleteToolCall(ctx context.Context, in *mcpv1.CompleteToolCallRequest, _ ...grpc.CallOption) (*mcpv1.CompleteToolCallResponse, error) {
	f.record(ctx)
	f.mu.Lock()
	f.completed = append(f.completed, in)
	f.mu.Unlock()
	return &mcpv1.CompleteToolCallResponse{}, nil
}
func (f *fakeClient) GetKillState(ctx context.Context, in *mcpv1.GetKillStateRequest, _ ...grpc.CallOption) (*mcpv1.GetKillStateResponse, error) {
	f.record(ctx)
	f.mu.Lock()
	f.killCalls++
	f.mu.Unlock()
	return f.killState(in)
}
func (f *fakeClient) EvaluateToolCall(ctx context.Context, in *mcpv1.EvaluateToolCallRequest, _ ...grpc.CallOption) (*mcpv1.PolicyDecision, error) {
	f.record(ctx)
	return &mcpv1.PolicyDecision{Decision: "require_approval", Source: "weird"}, nil
}

var (
	principal = mcpserver.Principal{TenantID: "t1", UserID: "u1", Role: "user", ClientID: "c1", GrantID: "g1", Scopes: []string{"orca:exec"}}
	execMeta  = mcpserver.ToolMeta{Name: "terminal_send", Channel: "terminal.send", Namespace: "terminal", Risk: "exec", RequiredScope: "orca:exec"}
	args      = json.RawMessage(`{"data":"ls"}`)
)

func allowResp(id string) (*mcpv1.AuthorizeToolCallResponse, error) {
	return &mcpv1.AuthorizeToolCallResponse{Outcome: "allow", CallId: id}, nil
}

func TestDecideOutcomes(t *testing.T) {
	cases := []struct {
		name    string
		resp    *mcpv1.AuthorizeToolCallResponse
		err     error
		want    string
		wantErr bool
	}{
		{"allow", &mcpv1.AuthorizeToolCallResponse{Outcome: "allow", CallId: "call-1"}, nil, "allow", false},
		{"allow without call id is refused", &mcpv1.AuthorizeToolCallResponse{Outcome: "allow"}, nil, "deny", true},
		{"deny keeps the neutral message", &mcpv1.AuthorizeToolCallResponse{Outcome: "deny", Message: "nope"}, nil, "deny", false},
		{"approval", &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "a1"}, nil, "require_approval", false},
		{"approval without id is refused", &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval"}, nil, "deny", true},
		{"unknown outcome denies", &mcpv1.AuthorizeToolCallResponse{Outcome: "sure"}, nil, "deny", false},
		{"empty outcome denies", &mcpv1.AuthorizeToolCallResponse{}, nil, "deny", false},
		{"rpc error denies", nil, errors.New("unavailable"), "deny", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeClient{authorize: func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) { return c.resp, c.err }}
			d, err := NewGate(f, Config{}, nil).Decide(context.Background(), principal, execMeta, args)
			if d.Outcome != c.want || (err != nil) != c.wantErr {
				t.Fatalf("outcome=%s err=%v, want %s err=%v", d.Outcome, err, c.want, c.wantErr)
			}
			if c.want == "deny" && d.Message == "" && c.name != "unknown outcome denies" && c.name != "empty outcome denies" {
				t.Fatal("a deny must carry a message for the agent")
			}
		})
	}
}

func TestNoClientMeansDeny(t *testing.T) {
	var g *Gate
	if d, err := g.Decide(context.Background(), principal, execMeta, args); d.Outcome != "deny" || err == nil {
		t.Fatal("nil gate must deny")
	}
	if d, err := NewGate(nil, Config{}, nil).Decide(context.Background(), principal, execMeta, args); d.Outcome != "deny" || err == nil {
		t.Fatal("no client must deny")
	}
}

func TestDecideSendsIdentityMetadataDeadlineAndCatalogFacts(t *testing.T) {
	f := &fakeClient{authorize: func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) { return allowResp("c") }}
	meta := execMeta
	meta.Channel, meta.Name, meta.UntrustedOutput, meta.OpenWorld = "agent.start", "agent_start", true, true
	ctx := WithDepth(WithSessionID(context.Background(), "sess-9"), 1, "root-1")
	if _, err := NewGate(f, Config{}, nil).Decide(ctx, principal, meta, args); err != nil {
		t.Fatal(err)
	}
	md := f.md[0]
	if md.Get("x-orca-tenant-id")[0] != "t1" || md.Get("x-orca-user-id")[0] != "u1" || md.Get("x-orca-role")[0] != "user" {
		t.Fatalf("identity must travel as metadata: %v", md)
	}
	if !f.deadlines[0] {
		t.Fatal("every call to mcp-service needs a deadline")
	}
	in := f.authorized[0]
	if string(in.Arguments) != string(args) || in.Tool.Channel != "agent.start" || !in.Tool.SpawnsProcess || !in.Tool.ReadUntrusted || !in.Tool.OpenWorld {
		t.Fatalf("tool facts: %+v", in.Tool)
	}
	if in.Ctx.ClientId != "c1" || in.Ctx.McpSessionId != "sess-9" || in.Ctx.Depth != 1 || in.Ctx.McpRoot != "root-1" || in.Ctx.TokenKind != "mcp_oauth" || in.Ctx.GrantId != "g1" {
		t.Fatalf("call context: %+v", in.Ctx)
	}
	pat := principal
	pat.ClientID, pat.GrantID = "", ""
	_, _ = NewGate(f, Config{}, nil).Decide(context.Background(), pat, execMeta, args)
	if f.authorized[1].Ctx.TokenKind != "mcp_pat" {
		t.Fatal("PAT detection")
	}
}

func TestApprovalFlowConsumesOnlyViaSecondAuthorization(t *testing.T) {
	calls := 0
	f := &fakeClient{}
	f.authorize = func(in *mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
		calls++
		if calls == 1 {
			return &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "a1"}, nil
		}
		return allowResp("call-77")
	}
	f.wait = func(*mcpv1.WaitApprovalRequest) (*mcpv1.WaitApprovalResponse, error) {
		return &mcpv1.WaitApprovalResponse{Status: "approved"}, nil
	}
	g := NewGate(f, Config{}, nil)
	d, _ := g.Decide(context.Background(), principal, execMeta, args)
	if d.Outcome != "require_approval" || d.ApprovalID != "a1" {
		t.Fatalf("%+v", d)
	}
	ok, err := g.AwaitApproval(context.Background(), principal, "a1")
	if err != nil || !ok {
		t.Fatalf("approved => run: %v %v", ok, err)
	}
	if string(f.authorized[1].Arguments) != string(args) {
		t.Fatal("the consume call must carry the original arguments")
	}
	g.Complete(context.Background(), principal, execMeta, "ok", 1500*time.Millisecond)
	if len(f.completed) != 1 || f.completed[0].CallId != "call-77" || f.completed[0].Result != "ok" || f.completed[0].DurationMs != 1500 {
		t.Fatalf("%+v", f.completed)
	}
	// A second wait for the same approval finds nothing to consume.
	if ok, _ := g.AwaitApproval(context.Background(), principal, "a1"); ok {
		t.Fatal("approval id must be single-use at the gateway too")
	}
}

func TestAwaitApprovalNegativeOutcomes(t *testing.T) {
	for _, status := range []string{"pending", "denied", "expired", "cancelled", "weird"} {
		t.Run(status, func(t *testing.T) {
			f := &fakeClient{}
			f.authorize = func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
				return &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "a1"}, nil
			}
			f.wait = func(*mcpv1.WaitApprovalRequest) (*mcpv1.WaitApprovalResponse, error) {
				return &mcpv1.WaitApprovalResponse{Status: status}, nil
			}
			g := NewGate(f, Config{}, nil)
			_, _ = g.Decide(context.Background(), principal, execMeta, args)
			ok, err := g.AwaitApproval(context.Background(), principal, "a1")
			if ok || err != nil {
				t.Fatalf("status %s must not run the tool (ok=%v err=%v)", status, ok, err)
			}
			if len(f.authorized) != 1 {
				t.Fatal("no consume attempt unless approved")
			}
		})
	}
}

func TestAwaitApprovalRejectsForeignAndUnknownIDsAndLostRace(t *testing.T) {
	f := &fakeClient{}
	n := 0
	f.authorize = func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
		n++
		if n == 1 {
			return &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "a1"}, nil
		}
		// Another concurrent call consumed the approval first.
		return &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "a2"}, nil
	}
	f.wait = func(*mcpv1.WaitApprovalRequest) (*mcpv1.WaitApprovalResponse, error) {
		return &mcpv1.WaitApprovalResponse{Status: "approved"}, nil
	}
	g := NewGate(f, Config{}, nil)
	_, _ = g.Decide(context.Background(), principal, execMeta, args)
	other := principal
	other.UserID = "u2"
	if ok, _ := g.AwaitApproval(context.Background(), other, "a1"); ok {
		t.Fatal("another user must not redeem the approval")
	}
	if ok, _ := g.AwaitApproval(context.Background(), principal, "ghost"); ok {
		t.Fatal("unknown approval id")
	}
	if ok, _ := g.AwaitApproval(context.Background(), principal, "a1"); ok {
		t.Fatal("a lost consume race must not run the tool")
	}
}

func TestAwaitApprovalRPCErrorIsAnError(t *testing.T) {
	f := &fakeClient{}
	f.authorize = func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
		return &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "a1"}, nil
	}
	f.wait = func(*mcpv1.WaitApprovalRequest) (*mcpv1.WaitApprovalResponse, error) { return nil, errors.New("boom") }
	g := NewGate(f, Config{}, nil)
	_, _ = g.Decide(context.Background(), principal, execMeta, args)
	if ok, err := g.AwaitApproval(context.Background(), principal, "a1"); ok || err == nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestCompleteMatchesCallsFIFOAndSkipsUnknown(t *testing.T) {
	n := 0
	f := &fakeClient{authorize: func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
		n++
		return allowResp([]string{"", "c1", "c2"}[n])
	}}
	g := NewGate(f, Config{}, nil)
	_, _ = g.Decide(context.Background(), principal, execMeta, args)
	_, _ = g.Decide(context.Background(), principal, execMeta, args)
	g.Complete(context.Background(), principal, execMeta, "error", time.Second)
	g.Complete(context.Background(), principal, execMeta, "weird", time.Second)
	g.Complete(context.Background(), principal, execMeta, "ok", time.Second) // nothing left: no RPC
	if len(f.completed) != 2 || f.completed[0].CallId != "c1" || f.completed[0].Result != "error" || f.completed[1].CallId != "c2" || f.completed[1].Result != "error" {
		t.Fatalf("%+v", f.completed)
	}
}

func TestEffectiveDecisionMapsSourcesAndFailsClosed(t *testing.T) {
	f := &fakeClient{}
	d, err := NewGate(f, Config{}, nil).EffectiveDecision(context.Background(), "t1", execMeta)
	if err != nil || d.Decision != "require_approval" || d.Source != "default" {
		t.Fatalf("unknown sources collapse to default: %+v %v", d, err)
	}
	if _, err := NewGate(nil, Config{}, nil).EffectiveDecision(context.Background(), "t1", execMeta); err == nil {
		t.Fatal("no client")
	}
}

func TestKillGuardCachesAndFallsBackBriefly(t *testing.T) {
	f := &fakeClient{killState: func(*mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error) {
		return &mcpv1.GetKillStateResponse{Active: true, Reason: "incident"}, nil
	}}
	now := time.Unix(1000, 0)
	kg := NewKillGuard(f, 0)
	kg.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if b, reason, err := kg.Blocked(context.Background(), principal, ""); !b || reason != "incident" || err != nil {
			t.Fatalf("%v %v %v", b, reason, err)
		}
	}
	if f.killCalls != 1 {
		t.Fatalf("cache must absorb repeats, got %d RPCs", f.killCalls)
	}
	now = now.Add(6 * time.Second)
	f.killState = func(*mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error) { return nil, errors.New("down") }
	if b, _, err := kg.Blocked(context.Background(), principal, ""); !b || err != nil {
		t.Fatal("a recent answer covers a brief outage")
	}
	now = now.Add(40 * time.Second)
	if _, _, err := kg.Blocked(context.Background(), principal, ""); err == nil {
		t.Fatal("an old answer must not hide an outage: fail closed")
	}
}

func TestAwaitApprovalStatusReportsTerminalStatus(t *testing.T) {
	cases := map[string]string{"pending": "pending", "denied": "denied", "expired": "expired", "cancelled": "cancelled"}
	for status, want := range cases {
		f := &fakeClient{}
		f.authorize = func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
			return &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "a1"}, nil
		}
		f.wait = func(*mcpv1.WaitApprovalRequest) (*mcpv1.WaitApprovalResponse, error) {
			return &mcpv1.WaitApprovalResponse{Status: status}, nil
		}
		g := NewGate(f, Config{}, nil)
		_, _ = g.Decide(context.Background(), principal, execMeta, args)
		ok, got, err := g.AwaitApprovalStatus(context.Background(), principal, "a1")
		if ok || err != nil || got != want {
			t.Errorf("%s: ok=%v status=%q err=%v", status, ok, got, err)
		}
	}
	// An unknown approval never reached mcp-service: no status.
	if ok, st, _ := NewGate(&fakeClient{}, Config{}, nil).AwaitApprovalStatus(context.Background(), principal, "ghost"); ok || st != "" {
		t.Fatalf("unknown id: ok=%v status=%q", ok, st)
	}
}
