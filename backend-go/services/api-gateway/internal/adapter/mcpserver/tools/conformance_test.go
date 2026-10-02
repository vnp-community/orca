package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

type bearer struct{ tok string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.tok)
	return http.DefaultTransport.RoundTrip(r)
}

// TestMCPConformanceInitializeListCall drives the real /mcp handler with the
// official SDK client: initialize -> tools/list -> tools/call on a read tool,
// then a write tool the fail-closed default must refuse.
func TestMCPConformanceInitializeListCall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Packs = map[int]bool{1: true, 2: true}
	reg := fakeRegistry(nil, nil)
	cat, err := NewCatalog(AllSpecs(), cfg, nil, reg.Channels())
	if err != nil {
		t.Fatal(err)
	}
	ex := NewExecutor(cat, reg, nil, nil, cfg, quiet)

	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	r := chi.NewRouter()
	mcpserver.NewHandler(mcpserver.Deps{
		Config: mcpserver.Config{ResourceURL: base + "/mcp", IssuerURL: "https://auth.example.com", PageSize: 500,
			ScopesSupported: []string{"orca:read", "orca:write"}, ServerVersion: "test"},
		Logger:     quiet,
		Verifier:   mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{"tok": {TenantID: "t1", UserID: "alice", Scopes: []string{"orca:read", "orca:write"}}}},
		Catalog:    cat,
		Executor:   ex,
		CursorKeys: [][]byte{[]byte("k1-0123456789abcdef0123456789abcd")},
	}).Mount(r)
	srv.Config.Handler = r
	srv.Start()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "conf", Version: "1"}, nil).Connect(ctx,
		&mcp.StreamableClientTransport{Endpoint: base + "/mcp", HTTPClient: &http.Client{Transport: bearer{"tok"}}}, nil)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer cs.Close()

	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]*mcp.Tool{}
	for _, tl := range list.Tools {
		have[tl.Name] = tl
	}
	pl := have["project_list"]
	if pl == nil || pl.Annotations == nil || !pl.Annotations.ReadOnlyHint || pl.InputSchema == nil || pl.OutputSchema == nil {
		t.Fatalf("project_list descriptor: %+v", pl)
	}
	if have["task_create"] != nil {
		t.Error("write tools must be hidden by the fail-closed default")
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "project_list", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("call: %v %+v", err, res)
	}
	sc, _ := res.StructuredContent.(map[string]any)
	items, _ := sc["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["defaultBranch"] != "main" {
		t.Errorf("structured content %v", sc)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "project_list", Arguments: map[string]any{"bogus": 1}})
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "INVALID_ARGUMENTS") {
		t.Errorf("bad args should be an isError result: %v %+v", err, res)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "task_create", Arguments: map[string]any{"title": "x"}})
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "MCP_POLICY_DENIED") {
		t.Errorf("fail-closed deny expected: %v %+v", err, res)
	}
	if _, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "no_such_tool"}); err == nil {
		t.Error("unknown tool must be a protocol error")
	}
}
