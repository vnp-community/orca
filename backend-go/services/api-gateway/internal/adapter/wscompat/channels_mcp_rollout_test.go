package wscompat

import (
	"context"
	"strings"
	"testing"
)

// Rollback (BE-MCP-SOL-015 section E): with the process flag off EVERY mcp.*
// channel answers MCP_DISABLED, even for an admin, without touching
// mcp-service; mcp.server.info alone keeps answering enabled=false so the UI
// can hide the section without an error.
func TestEveryMcpChannelIsDisabledWhenProcessFlagIsOff(t *testing.T) {
	client := &fakeMcpClient{}
	r := NewRegistry()
	RegisterMcpChannels(r, McpChannelDeps{Enabled: false, Client: client})

	admin := Identity{TenantID: "t", UserID: "u", Role: "admin"}
	checked := 0
	for _, ch := range r.Channels() {
		if !strings.HasPrefix(ch.Name, "mcp.") || ch.Name == "mcp.server.info" {
			continue
		}
		checked++
		var err error
		switch ch.Kind {
		case ChannelUnary:
			_, err = r.Dispatch(context.Background(), admin, ch.Name, nil)
		case ChannelStreamChannel:
			_, _, _, err = r.DispatchStreamChannel(context.Background(), admin, ch.Name, nil)
		default:
			t.Logf("skipping %s (kind %v is not an RPC-style mcp channel)", ch.Name, ch.Kind)
			checked--
			continue
		}
		if err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED:") {
			t.Errorf("%s with MCP_ENABLED=false: %v, want MCP_DISABLED", ch.Name, err)
		}
	}
	if checked < 20 {
		t.Fatalf("only %d mcp.* channels exercised; the registry changed shape?", checked)
	}
	if client.calls != 0 {
		t.Errorf("a disabled gateway made %d call(s) to mcp-service", client.calls)
	}
	// Rolling forward again needs no data change: the same registration with
	// the flag on is no longer MCP_DISABLED (it fails later, for other reasons).
	r2 := NewRegistry()
	RegisterMcpChannels(r2, McpChannelDeps{Enabled: true, Client: client})
	_, err := r2.Dispatch(context.Background(), Identity{TenantID: "t", UserID: "u", Role: "user"}, "mcp.admin.settings.get", nil)
	if err == nil || strings.HasPrefix(err.Error(), "MCP_DISABLED:") {
		t.Errorf("enabled gateway still reports disabled: %v", err)
	}
}
