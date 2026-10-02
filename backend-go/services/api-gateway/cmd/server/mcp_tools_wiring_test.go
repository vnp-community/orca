package main

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

func TestBuildMCPToolStackDefaultsToPack1AndFailClosed(t *testing.T) {
	t.Setenv("MCP_TOOL_PACKS_ENABLED", "")
	reg := wscompat.NewRegistry()
	wscompat.RegisterProductionChannels(reg, wscompat.ChannelDeps{TaskActivityEnabled: true})
	s, err := buildMCPToolStack(reg, nil, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	tools, _ := s.Catalog.ListTools(context.Background(), mcpserver.Principal{TenantID: "t", Scopes: []string{"orca:read", "orca:write", "orca:admin"}})
	if len(tools) == 0 {
		t.Fatal("pack 1 must be listed")
	}
	for _, tl := range tools {
		if sp, ok := s.Catalog.Lookup(tl.Name); !ok || sp.Pack != 1 {
			t.Errorf("%s is not a pack 1 tool", tl.Name)
		}
	}
}

func TestBuildMCPToolStackRejectsBadEnv(t *testing.T) {
	t.Setenv("MCP_TOOL_PACKS_ENABLED", "1,7")
	if _, err := buildMCPToolStack(wscompat.NewRegistry(), nil, quietLogger); err == nil {
		t.Error("pack 7 must be rejected")
	}
	t.Setenv("MCP_TOOL_PACKS_ENABLED", "1,2")
	t.Setenv("MCP_TOOL_TIMEOUT", "soon")
	if _, err := buildMCPToolStack(wscompat.NewRegistry(), nil, quietLogger); err == nil {
		t.Error("bad timeout must be rejected")
	}
}

func TestBuildMCPToolStackRejectsBadPtyEnv(t *testing.T) {
	t.Setenv("MCP_TERMINAL_MAX_PER_SESSION", "0")
	if _, err := buildMCPToolStack(wscompat.NewRegistry(), nil, quietLogger); err == nil {
		t.Error("a zero terminal cap must be rejected")
	}
	t.Setenv("MCP_TERMINAL_MAX_PER_SESSION", "3")
	t.Setenv("MCP_TERMINAL_IDLE_TIMEOUT", "soon")
	if _, err := buildMCPToolStack(wscompat.NewRegistry(), nil, quietLogger); err == nil {
		t.Error("a bad idle timeout must be rejected")
	}
}

func TestSessionClosedOptionIsWiredToTheExecutor(t *testing.T) {
	reg := wscompat.NewRegistry()
	wscompat.RegisterProductionChannels(reg, wscompat.ChannelDeps{TaskActivityEnabled: true})
	s, err := buildMCPToolStack(reg, nil, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Executor.Close()
	var d mcpserver.Deps
	withMCPSessionClosed(s)(&d)
	if d.OnSessionClosed == nil {
		t.Fatal("OnSessionClosed not set")
	}
	d.OnSessionClosed("unknown-row", "user") // must not panic or block
}
