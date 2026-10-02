package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type fakeMcpClient struct {
	mcpv1.McpServiceClient // other RPCs belong to later solutions
	resp                   *mcpv1.GetServerInfoResponse
	err                    error
	calls                  int
}

func (f *fakeMcpClient) GetServerInfo(context.Context, *mcpv1.GetServerInfoRequest, ...grpc.CallOption) (*mcpv1.GetServerInfoResponse, error) {
	f.calls++
	return f.resp, f.err
}

func dispatchMcp(t *testing.T, d McpChannelDeps, channel string) (string, error) {
	t.Helper()
	r := NewRegistry()
	RegisterMcpChannels(r, d)
	out, err := r.Dispatch(context.Background(), Identity{TenantID: "t", UserID: "u", Role: "user"}, channel, nil)
	if err != nil {
		return "", err
	}
	b, jerr := json.Marshal(out)
	if jerr != nil {
		t.Fatal(jerr)
	}
	return string(b), nil
}

func TestMcpServerInfoDisabledIsNotAnError(t *testing.T) {
	c := &fakeMcpClient{}
	got, err := dispatchMcp(t, McpChannelDeps{Enabled: false, Client: c}, "mcp.server.info")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"enabled":false,"resourceUrl":"","protocolVersions":[],"authorizationServer":"","scopesSupported":[],"dcrEnabled":false,"maxTokenDays":0,"killSwitch":{"active":false}}`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if c.calls != 0 {
		t.Error("disabled gateway must not call mcp-service")
	}
}

func TestMcpServerInfoGolden(t *testing.T) {
	c := &fakeMcpClient{resp: &mcpv1.GetServerInfoResponse{
		Enabled: false, DcrEnabled: true, MaxTokenDays: 90,
		KillSwitch: &mcpv1.KillSwitch{Active: true, Reason: "incident", At: timestamppb.New(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))},
		Scopes:     []*mcpv1.ScopeDescriptor{{Id: "orca:read", Label: "Read", Description: "Read data", Risk: "read"}},
	}}
	got, err := dispatchMcp(t, McpChannelDeps{Enabled: true, Client: c, ResourceURL: "https://o.example/mcp", AuthorizationServer: "https://o.example"}, "mcp.server.info")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"enabled":true,"tenantEnabled":false,"resourceUrl":"https://o.example/mcp","protocolVersions":["2025-06-18"],` +
		`"authorizationServer":"https://o.example","scopesSupported":[{"id":"orca:read","label":"Read","description":"Read data","risk":"read"}],` +
		`"dcrEnabled":true,"maxTokenDays":90,"killSwitch":{"active":true,"reason":"incident","at":"2026-10-01T12:00:00Z"}}`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestMcpServerInfoErrors(t *testing.T) {
	cases := []struct {
		name string
		d    McpChannelDeps
		want string
	}{
		{"unavailable", McpChannelDeps{Enabled: true, Client: &fakeMcpClient{err: status.Error(codes.Unavailable, "dial tcp mcp-service:9090: refused")}}, "MCP_UNAVAILABLE: mcp-service temporarily unavailable"},
		{"timeout", McpChannelDeps{Enabled: true, Client: &fakeMcpClient{err: status.Error(codes.DeadlineExceeded, "ctx")}}, "MCP_TIMEOUT: mcp-service did not respond in time"},
		{"internal", McpChannelDeps{Enabled: true, Client: &fakeMcpClient{err: errors.New("pq: boom")}}, "MCP_INTERNAL: internal error"},
		{"not configured", McpChannelDeps{Enabled: true}, "MCP_UNAVAILABLE: mcp-service is not configured"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := dispatchMcp(t, c.d, "mcp.server.info")
			if err == nil || err.Error() != c.want {
				t.Fatalf("got %v want %q", err, c.want)
			}
		})
	}
}

func TestMcpChannelError(t *testing.T) {
	cases := []struct {
		in   error
		want string
	}{
		{nil, ""},
		{status.Error(codes.NotFound, "MCP_NOT_FOUND: x"), "MCP_NOT_FOUND: x"},
		{errors.New("rpc error: code = NotFound desc = MCP_NOT_FOUND: x"), "MCP_NOT_FOUND: x"},
		{fmt.Errorf("wrap: %w", status.Error(codes.FailedPrecondition, "MCP_KILL_SWITCH_ACTIVE: stopped")), "MCP_KILL_SWITCH_ACTIVE: stopped"},
		{status.Error(codes.NotFound, "task 42 missing"), "MCP_NOT_FOUND: not found"},
		{status.Error(codes.PermissionDenied, "nope"), "MCP_NOT_FOUND: not found"},
		{status.Error(codes.InvalidArgument, "bad"), "MCP_INVALID_ARGUMENT: invalid argument"},
		{status.Error(codes.Unavailable, "x"), "MCP_UNAVAILABLE: mcp-service temporarily unavailable"},
		{fmt.Errorf("call: %w", context.DeadlineExceeded), "MCP_TIMEOUT: mcp-service did not respond in time"},
		{status.Error(codes.Internal, "pq: secret-value-123"), "MCP_INTERNAL: internal error"},
		{errors.New("plain"), "MCP_INTERNAL: internal error"},
		{errors.New("MCP_DISABLED: off"), "MCP_DISABLED: off"},
	}
	for _, c := range cases {
		got := mcpChannelError(c.in)
		switch {
		case c.in == nil && got != nil:
			t.Errorf("nil must stay nil, got %v", got)
		case c.in != nil && (got == nil || got.Error() != c.want):
			t.Errorf("mcpChannelError(%v) = %v, want %q", c.in, got, c.want)
		}
	}
}

func TestMcpHandlerGating(t *testing.T) {
	ok := func(context.Context, Identity, []json.RawMessage) (any, error) { return "ran", nil }
	boom := func(context.Context, Identity, []json.RawMessage) (any, error) {
		return nil, status.Error(codes.NotFound, "MCP_NOT_FOUND: nope")
	}
	call := func(d McpChannelDeps, admin bool, fn ChannelHandler, role string) (any, error) {
		return mcpHandler(d, admin, fn)(context.Background(), Identity{Role: role}, nil)
	}
	if _, err := call(McpChannelDeps{Enabled: false}, false, ok, "admin"); err == nil || err.Error() != "MCP_DISABLED: MCP is disabled on this server" {
		t.Errorf("disabled: %v", err)
	}
	for _, role := range []string{"user", ""} {
		if _, err := call(McpChannelDeps{Enabled: true}, true, ok, role); err == nil || err.Error() != "MCP_NOT_ADMIN: admin role required" {
			t.Errorf("role %q: %v", role, err)
		}
	}
	if out, err := call(McpChannelDeps{Enabled: true}, true, ok, "admin"); err != nil || out != "ran" {
		t.Errorf("admin: %v %v", out, err)
	}
	if _, err := call(McpChannelDeps{Enabled: true}, false, boom, "user"); err == nil || err.Error() != "MCP_NOT_FOUND: nope" {
		t.Errorf("error shaping: %v", err)
	}
}
