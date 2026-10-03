package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

func filesExec(t *testing.T, hits *atomic.Int32, cfgFn func(*Config)) *Executor {
	t.Helper()
	r := wscompat.NewRegistry()
	reg := func(ch string, out any) {
		r.Register(ch, func(context.Context, wscompat.Identity, []json.RawMessage) (any, error) { hits.Add(1); return out, nil })
	}
	reg("files.readPreview", map[string]any{"content": []byte("-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----")})
	reg("files.readDir", []map[string]any{{"name": ".env", "is_directory": false}, {"name": "main.go"}, {"name": ".git", "is_directory": true}})
	reg("files.search", []map[string]any{{"path": "config/.env.production", "line": 1}, {"path": "src/a.go", "line": 2}})
	reg("files.listAll", []string{"a.go", ".ssh/id_rsa", "deploy/server.pem", "docs/x.md"})
	reg("files.readChunk", map[string]any{"content": []byte("-----BEGIN PRIVATE KEY-----\nx")})
	cfg := DefaultConfig()
	cfg.Packs = allPacks
	if cfgFn != nil {
		cfgFn(&cfg)
	}
	cat, err := NewCatalog(AllSpecs(), cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewExecutor(cat, r, &mcpservertest.FakeGate{}, nil, cfg, quiet)
}

func TestFilesToolsRejectSensitiveInputPaths(t *testing.T) {
	var hits atomic.Int32
	ex := filesExec(t, &hits, func(c *Config) { c.SensitivePathExtra = []string{"internal/*"} })
	for _, p := range []string{".env", "app/.env.local", ".git/config", "keys/server.pem", "../x", "/etc/passwd", "internal/secret.go", ".ssh/id_rsa", "a%2e%2e/b"} {
		in, _ := json.Marshal(map[string]any{"worktree_id": "w1", "path": p})
		for _, tool := range []string{"files_read", "files_stat", "files_readChunk"} {
			_, err := ex.CallTool(context.Background(), alice, tool, in)
			if err == nil || errText(err) != "MCP_NOT_FOUND: not found or not permitted" {
				t.Errorf("%s %q: %v", tool, p, err)
			}
		}
	}
	if hits.Load() != 0 {
		t.Errorf("downstream reached %d times", hits.Load())
	}
	// .env.example is allowed by the rules; empty readDir path is the root.
	if _, err := ex.CallTool(context.Background(), alice, "files_readDir", json.RawMessage(`{"worktree_id":"w1"}`)); err != nil {
		t.Errorf("root readDir: %v", err)
	}
}

func TestFilesResultsAreFiltered(t *testing.T) {
	var hits atomic.Int32
	ex := filesExec(t, &hits, nil)
	items := func(tool, in string) []any {
		res, err := ex.CallTool(context.Background(), alice, tool, json.RawMessage(in))
		if err != nil {
			t.Fatal(tool, err)
		}
		return res.StructuredContent.(map[string]any)["items"].([]any)
	}
	if got := items("files_readDir", `{"worktree_id":"w1"}`); len(got) != 1 {
		t.Errorf("readDir: %v", got)
	}
	if got := items("files_search", `{"worktree_id":"w1","pattern":"x"}`); len(got) != 1 {
		t.Errorf("search: %v", got)
	}
	if got := items("files_listAll", `{"worktree_id":"w1"}`); len(got) != 2 {
		t.Errorf("listAll: %v", got)
	}
}

func TestFilesPrivateKeyContentIsWithheld(t *testing.T) {
	var hits atomic.Int32
	ex := filesExec(t, &hits, nil)
	for tool, in := range map[string]string{"files_read": `{"worktree_id":"w1","path":"notes.txt"}`, "files_readChunk": `{"worktree_id":"w1","path":"notes.txt"}`} {
		res, err := ex.CallTool(context.Background(), alice, tool, json.RawMessage(in))
		if err != nil {
			t.Fatal(err)
		}
		m := res.StructuredContent.(map[string]any)
		if m["withheld"] == nil || m["text"] != nil || m["content"] != nil {
			t.Errorf("%s: %v", tool, m)
		}
	}
	_ = base64.StdEncoding
}
