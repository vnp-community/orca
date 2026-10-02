package agentconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

var entries = []domain.AgentServerEntry{
	{Name: "orca", Transport: domain.TransportHTTP, URL: "https://orca.example.com/mcp", BearerVar: "ORCA_MCP_TOKEN"},
	{Name: "api", Transport: domain.TransportHTTP, URL: "https://api.example.com/mcp", Headers: []domain.EnvBinding{{Name: "X-Api-Key", Var: "ORCA_MCP_1"}}},
	{Name: "local", Transport: domain.TransportStdio, Command: "orca-mcp-launch", Args: []string{"--", "npx", "-y", "pkg@1.2.3"}, Env: []domain.EnvBinding{{Name: "API_TOKEN", Var: "ORCA_MCP_2"}}},
}

func TestRenderers_AllFourAreMarkedUnverifiedAndHoldOnlyPlaceholders(t *testing.T) {
	r := Renderer{}
	for _, kind := range []string{"claude", "codex", "gemini", "opencode"} {
		if !r.Supports(kind) {
			t.Fatalf("%s unsupported", kind)
		}
		out, err := r.Render(kind, "posix", entries)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Unverified {
			t.Errorf("%s must be flagged unverified", kind)
		}
		all := strings.Join(out.ExtraArgs, " ")
		for _, f := range out.Files {
			all += f.Content
			if f.Mode != "0600" {
				t.Errorf("%s: file mode %s", kind, f.Mode)
			}
		}
		for _, e := range out.Env {
			all += string(e.Value.Reveal())
		}
		if !strings.Contains(all, "ORCA_MCP_1") || !strings.Contains(all, "ORCA_MCP_2") || !strings.Contains(all, "ORCA_MCP_TOKEN") {
			t.Errorf("%s: placeholders missing: %s", kind, all)
		}
		if strings.Contains(all, "omp_") {
			t.Errorf("%s: a token value must never be rendered", kind)
		}
	}
	if r.Supports("ollama") || r.Supports("") {
		t.Fatal("only the four CLIs are supported")
	}
}

func TestRenderClaude_ShapeAndStableOrder(t *testing.T) {
	out, _ := Renderer{}.Render("claude", "posix", entries)
	var doc struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(out.Files[0].Content), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.MCPServers["api"]["type"] != "http" || doc.MCPServers["api"]["headers"].(map[string]any)["X-Api-Key"] != "${ORCA_MCP_1}" {
		t.Fatalf("%v", doc.MCPServers["api"])
	}
	if doc.MCPServers["orca"]["headers"].(map[string]any)["Authorization"] != "Bearer ${ORCA_MCP_TOKEN}" {
		t.Fatalf("%v", doc.MCPServers["orca"])
	}
	if doc.MCPServers["local"]["command"] != "orca-mcp-launch" {
		t.Fatalf("%v", doc.MCPServers["local"])
	}
	if out.ExtraArgs[0] != "--mcp-config" || !strings.HasPrefix(out.ExtraArgs[1], ConfigDirToken) {
		t.Fatalf("%v", out.ExtraArgs)
	}
	again, _ := Renderer{}.Render("claude", "posix", []domain.AgentServerEntry{entries[2], entries[0], entries[1]})
	if again.Files[0].Content != out.Files[0].Content {
		t.Fatal("output must not depend on input order")
	}
}

func TestRenderOpencodeAndGeminiEnvDelivery(t *testing.T) {
	oc, _ := Renderer{}.Render("opencode", "posix", entries)
	if oc.Env[0].Name != "OPENCODE_CONFIG_CONTENT" || !strings.Contains(string(oc.Env[0].Value.Reveal()), "{env:ORCA_MCP_1}") {
		t.Fatalf("%v", oc.Env)
	}
	gm, _ := Renderer{}.Render("gemini", "posix", entries)
	if gm.Env[0].Name != "GEMINI_CLI_SYSTEM_SETTINGS_PATH" {
		t.Fatalf("%v", gm.Env)
	}
	cx, _ := Renderer{}.Render("codex", "posix", entries)
	if len(cx.Files) != 0 || !strings.Contains(strings.Join(cx.ExtraArgs, " "), "mcp_servers.orca.bearer_token_env_var") {
		t.Fatalf("%v", cx.ExtraArgs)
	}
}
