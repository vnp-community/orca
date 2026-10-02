package mcppolicy

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

func TestRequestContextCarriesVerifiedFacts(t *testing.T) {
	ctx := RequestContext(context.Background(), mcpserver.Principal{}, mcpserver.RequestInfo{
		SessionID: "row-1", ClientName: "cursor", Depth: 2, Root: "root-1"})
	if d, r := DepthFromContext(ctx); d != 2 || r != "root-1" {
		t.Errorf("depth/root: %d %q", d, r)
	}
	if SessionIDFromContext(ctx) != "row-1" || ClientNameFromContext(ctx) != "cursor" {
		t.Errorf("session/client: %q %q", SessionIDFromContext(ctx), ClientNameFromContext(ctx))
	}
	// And they reach mcp-service in the CallContext the gate builds.
	cc := callContext(ctx, principal)
	if cc.GetDepth() != 2 || cc.GetMcpRoot() != "root-1" || cc.GetMcpSessionId() != "row-1" || cc.GetClientName() != "cursor" {
		t.Errorf("call context: %+v", cc)
	}
}

type decideClient struct {
	*fakeClient
	decided *mcpv1.DecideApprovalRequest
}

func (d *decideClient) DecideApproval(_ context.Context, in *mcpv1.DecideApprovalRequest, _ ...grpc.CallOption) (*mcpv1.Approval, error) {
	d.decided = in
	return &mcpv1.Approval{}, nil
}

func TestElicitationFieldsAndDecisionVia(t *testing.T) {
	f := &fakeClient{authorize: func(*mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
		return &mcpv1.AuthorizeToolCallResponse{Outcome: "require_approval", ApprovalId: "ap1", ParamsHash: "h1",
			ElicitationEligible: true, ApprovalPrompt: "Create task?"}, nil
	}}
	c := &decideClient{fakeClient: f}
	g := NewGate(c, Config{}, nil)
	d, err := g.Decide(context.Background(), principal, execMeta, args)
	if err != nil || d.Outcome != mcpserver.OutcomeRequireApproval || !d.ElicitationEligible || d.ApprovalPrompt != "Create task?" {
		t.Fatalf("decision: %+v %v", d, err)
	}
	if err := g.DecideByElicitation(context.Background(), principal, "ap1", true); err != nil {
		t.Fatal(err)
	}
	if c.decided.GetVia() != "elicitation" || c.decided.GetDecision() != "approve" || c.decided.GetParamsHash() != "h1" || c.decided.GetApprovalId() != "ap1" {
		t.Errorf("decide request: %+v", c.decided)
	}
	if err := g.DecideByElicitation(context.Background(), principal, "ap1", false); err != nil || c.decided.GetDecision() != "deny" {
		t.Errorf("decline must deny: %+v %v", c.decided, err)
	}
	// Only the principal that was asked may answer.
	other := principal
	other.UserID = "someone-else"
	if err := g.DecideByElicitation(context.Background(), other, "ap1", true); err == nil {
		t.Error("another user must not decide the approval")
	}
	if err := g.DecideByElicitation(context.Background(), principal, "unknown", true); err == nil {
		t.Error("unknown approval must be refused")
	}
}

func TestKillGuardMapsToSharedSentinel(t *testing.T) {
	if ErrKillSwitchActive != mcpserver.ErrKillSwitchActive {
		t.Fatal("the verifier error must be the sentinel /mcp maps to 403")
	}
}

func TestWatchKillCancelsRunningWork(t *testing.T) {
	var blocked atomic.Bool
	f := &fakeClient{killState: func(*mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error) {
		return &mcpv1.GetKillStateResponse{Active: blocked.Load()}, nil
	}}
	g := NewGate(f, Config{}, nil)
	guard := NewKillGuard(f, 0)
	var ticks atomic.Int64
	guard.now = func() time.Time { return time.Unix(ticks.Add(60), 0) } // every poll is past the cache TTL
	ctx, cancel := g.WatchKill(context.Background(), guard, principal, 5*time.Millisecond)
	defer cancel()
	time.Sleep(30 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatal("cancelled although not kill-switched")
	}
	blocked.Store(true)
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("kill switch did not cancel the watched context")
	}
}
