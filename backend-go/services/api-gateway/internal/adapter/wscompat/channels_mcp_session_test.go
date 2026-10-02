package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type sessionClient struct {
	mcpv1.McpServiceClient
	closeReq *mcpv1.CloseSessionRequest
	closeErr error
	md       metadata.MD
	adminHit bool
}

var sessionFixture = []*mcpv1.McpSession{{
	Id: "11111111-1111-4111-8111-111111111111", UserId: "u1", ClientName: "cursor", GrantId: "g1", TokenId: "t1", ProtocolVersion: "2025-06-18",
	CreatedAt: timestamppb.New(time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)), LastSeenAt: timestamppb.New(time.Date(2026, 10, 2, 1, 5, 0, 0, time.UTC)),
	ActiveStreams: 2, ToolCalls: 7,
}}

func (f *sessionClient) ListSessions(ctx context.Context, _ *mcpv1.ListSessionsRequest, _ ...grpc.CallOption) (*mcpv1.ListSessionsResponse, error) {
	f.md, _ = metadata.FromOutgoingContext(ctx)
	return &mcpv1.ListSessionsResponse{Sessions: sessionFixture}, nil
}
func (f *sessionClient) ListSessionsAdmin(ctx context.Context, _ *mcpv1.ListSessionsRequest, _ ...grpc.CallOption) (*mcpv1.ListSessionsResponse, error) {
	f.adminHit = true
	return &mcpv1.ListSessionsResponse{Sessions: sessionFixture}, nil
}
func (f *sessionClient) CloseSession(_ context.Context, in *mcpv1.CloseSessionRequest, _ ...grpc.CallOption) (*mcpv1.CloseSessionResponse, error) {
	f.closeReq = in
	if f.closeErr != nil {
		return nil, f.closeErr
	}
	return &mcpv1.CloseSessionResponse{SessionId: in.GetSessionId()}, nil
}

type fakeCloser struct{ ids, reasons []string }

func (f *fakeCloser) CloseSession(id, reason string) {
	f.ids, f.reasons = append(f.ids, id), append(f.reasons, reason)
}

func TestMcpSessionChannels(t *testing.T) {
	c := &sessionClient{}
	closer := &fakeCloser{}
	d := McpChannelDeps{Enabled: true, Client: c, SessionCloser: closer}

	// Golden JSON: CONTRACT McpSessionView, camelCase; no userId on the user channel.
	got, err := callMcp(t, d, "user", "mcp.session.list", nil)
	want := `[{"id":"11111111-1111-4111-8111-111111111111","clientName":"cursor","grantId":"g1","tokenId":"t1","createdAt":"2026-10-02T01:00:00Z","lastSeenAt":"2026-10-02T01:05:00Z","protocolVersion":"2025-06-18","activeStreams":2,"toolCalls":7}]`
	if err != nil || got != want {
		t.Fatalf("user list:\n got %s (%v)\nwant %s", got, err, want)
	}
	if c.md.Get("x-orca-user-id")[0] != "u1" {
		t.Errorf("the list is scoped by session identity metadata, got %v", c.md)
	}

	// Admin list: mcp.admin.session.list is admin-only and adds userId.
	if _, err := callMcp(t, d, "user", "mcp.admin.session.list", nil); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_ADMIN: ") {
		t.Errorf("non-admin: %v", err)
	}
	got, err = callMcp(t, d, "admin", "mcp.admin.session.list", nil)
	if err != nil || !c.adminHit || !strings.Contains(got, `"userId":"u1"`) {
		t.Fatalf("admin list: %s %v", got, err)
	}

	// Close: single object arg, ok:true, fans out to the replicas.
	got, err = callMcp(t, d, "user", "mcp.session.close", map[string]any{"sessionId": "11111111-1111-4111-8111-111111111111"})
	if err != nil || got != `{"ok":true}` {
		t.Fatalf("close: %s %v", got, err)
	}
	if c.closeReq.GetReason() != "user" || len(closer.ids) != 1 || closer.reasons[0] != "user" {
		t.Errorf("close request %+v / fan-out %+v", c.closeReq, closer)
	}
	if _, err := callMcp(t, d, "admin", "mcp.session.close", map[string]any{"sessionId": "x"}); err != nil || closer.reasons[1] != "admin" {
		t.Errorf("admin close reason: %v %v", err, closer.reasons)
	}

	// Somebody else's / unknown session: MCP_NOT_FOUND, and nothing is fanned out.
	c.closeErr = grpcErr(codes.NotFound, "MCP_NOT_FOUND: not found")
	n := len(closer.ids)
	if _, err := callMcp(t, d, "user", "mcp.session.close", map[string]any{"sessionId": "someone-elses"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_FOUND: ") {
		t.Errorf("foreign session: %v", err)
	}
	if len(closer.ids) != n {
		t.Error("a failed close must not signal other replicas")
	}
	if _, err := callMcp(t, d, "user", "mcp.session.close", map[string]any{}); err == nil || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT: ") {
		t.Errorf("missing sessionId: %v", err)
	}
	if _, err := callMcp(t, McpChannelDeps{Enabled: false, Client: c}, "user", "mcp.session.list", nil); err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED: ") {
		t.Errorf("disabled: %v", err)
	}
	// The channel is registered under the contract names.
	r := NewRegistry()
	RegisterMcpChannels(r, d)
	arg, _ := json.Marshal(map[string]any{"sessionId": "s"})
	c.closeErr = nil
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", Role: "user"}, "mcp.session.close", []json.RawMessage{arg}); err != nil {
		t.Fatal(err)
	}
}
