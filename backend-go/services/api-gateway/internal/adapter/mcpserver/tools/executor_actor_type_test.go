package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

func TestCallTool_MarksActorAgent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Packs = allPacks
	reg := wscompat.NewRegistry()
	var seen string
	reg.Register("project.list", func(ctx context.Context, _ wscompat.Identity, _ []json.RawMessage) (any, error) {
		seen = tenant.ActorType(ctx)
		return []map[string]any{{"id": "p1"}}, nil
	})
	gate := &mcpservertest.FakeGate{}
	cat, err := NewCatalog(AllSpecs(), cfg, gate, reg.Channels())
	if err != nil {
		t.Fatal(err)
	}
	ex := NewExecutor(cat, reg, gate, nil, cfg, quiet)

	// A caller-supplied "system" actor must not survive: MCP is always an agent.
	ctx := tenant.WithActorType(context.Background(), tenant.ActorSystem)
	if _, err := ex.CallTool(ctx, alice, "project_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if seen != tenant.ActorAgent {
		t.Fatalf("MCP tool call actor = %q, want agent", seen)
	}

	// A plain WS-style dispatch that bypasses CallTool stays "user".
	seen = ""
	if _, err := reg.Dispatch(context.Background(), wscompat.Identity{TenantID: "t1", UserID: "alice"}, "project.list", nil); err != nil {
		t.Fatal(err)
	}
	if seen != tenant.ActorUser {
		t.Fatalf("non-MCP dispatch actor = %q, want user", seen)
	}
}
