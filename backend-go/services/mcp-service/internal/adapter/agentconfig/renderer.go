// Package agentconfig renders neutral MCP server entries into each agent CLI's
// configuration. All four formats are UNVERIFIED against the CLI versions Orca
// ships (chưa xác minh): they need a spike and golden tests before the
// MCP_AGENT_CONFIG_ENABLED flag is turned on. Files and args carry only
// ${ORCA_MCP_*} placeholders, never a secret value.
package agentconfig

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// ConfigDirToken is replaced by the agent (D2) with its private per-task
// temp directory; the backend never writes into a worktree.
const ConfigDirToken = "{{ORCA_MCP_CONFIG_DIR}}"

type Renderer struct{}

var _ usecase.AgentConfigRenderer = Renderer{}

func (Renderer) Supports(kind string) bool {
	switch kind {
	case "claude", "codex", "gemini", "opencode":
		return true
	}
	return false
}

func (r Renderer) Render(kind, hostOS string, entries []domain.AgentServerEntry) (usecase.RenderedAgentConfig, error) {
	sorted := append([]domain.AgentServerEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	var out usecase.RenderedAgentConfig
	var err error
	switch kind {
	case "claude":
		out, err = renderClaude(sorted)
	case "codex":
		out, err = renderCodex(sorted)
	case "gemini":
		out, err = renderGemini(sorted)
	case "opencode":
		out, err = renderOpencode(sorted)
	default:
		return usecase.RenderedAgentConfig{}, fmt.Errorf("agentconfig: unsupported agent %q", kind)
	}
	out.Unverified = true
	return out, err
}

func bearer(v string, wrap func(string) string) string { return "Bearer " + wrap(v) }

func dollar(v string) string      { return "${" + v + "}" }
func opencodeEnv(v string) string { return "{env:" + v + "}" }

func headerMap(e domain.AgentServerEntry, wrap func(string) string) map[string]string {
	h := map[string]string{}
	for _, b := range e.Headers {
		h[b.Name] = wrap(b.Var)
	}
	if e.BearerVar != "" {
		h["Authorization"] = bearer(e.BearerVar, wrap)
	}
	return h
}

func envMap(e domain.AgentServerEntry, wrap func(string) string) map[string]string {
	m := map[string]string{}
	for _, b := range e.Env {
		m[b.Name] = wrap(b.Var)
	}
	return m
}

func renderClaude(entries []domain.AgentServerEntry) (usecase.RenderedAgentConfig, error) {
	servers := map[string]any{}
	for _, e := range entries {
		if e.Transport == domain.TransportHTTP {
			servers[e.Name] = map[string]any{"type": "http", "url": e.URL, "headers": headerMap(e, dollar)}
		} else {
			servers[e.Name] = map[string]any{"command": e.Command, "args": e.Args, "env": envMap(e, dollar)}
		}
	}
	b, err := json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	if err != nil {
		return usecase.RenderedAgentConfig{}, err
	}
	return usecase.RenderedAgentConfig{
		Files:     []usecase.AgentConfigFile{{Path: "claude-mcp.json", Content: string(b), Mode: "0600"}},
		ExtraArgs: []string{"--mcp-config", ConfigDirToken + "/claude-mcp.json"},
	}, nil
}

func tomlString(s string) string { return strconv.Quote(s) }

func tomlArray(a []string) string {
	q := make([]string, len(a))
	for i, s := range a {
		q[i] = tomlString(s)
	}
	return "[" + strings.Join(q, ",") + "]"
}

// renderCodex uses -c overrides (no file). Header/env keys follow Codex's
// documented env-indirection names, unverified for the shipped version.
func renderCodex(entries []domain.AgentServerEntry) (usecase.RenderedAgentConfig, error) {
	var args []string
	add := func(k, v string) { args = append(args, "-c", k+"="+v) }
	for _, e := range entries {
		p := "mcp_servers." + e.Name + "."
		if e.Transport == domain.TransportHTTP {
			add(p+"url", tomlString(e.URL))
			if e.BearerVar != "" {
				add(p+"bearer_token_env_var", tomlString(e.BearerVar))
			}
			for _, b := range e.Headers {
				add(p+"env_http_headers."+tomlString(b.Name), tomlString(b.Var))
			}
			continue
		}
		add(p+"command", tomlString(e.Command))
		add(p+"args", tomlArray(e.Args))
		for _, b := range e.Env {
			add(p+"env."+tomlString(b.Name), tomlString("${"+b.Var+"}"))
		}
	}
	return usecase.RenderedAgentConfig{ExtraArgs: args}, nil
}

func renderGemini(entries []domain.AgentServerEntry) (usecase.RenderedAgentConfig, error) {
	servers := map[string]any{}
	for _, e := range entries {
		if e.Transport == domain.TransportHTTP {
			servers[e.Name] = map[string]any{"httpUrl": e.URL, "headers": headerMap(e, dollar)}
		} else {
			servers[e.Name] = map[string]any{"command": e.Command, "args": e.Args, "env": envMap(e, dollar)}
		}
	}
	b, err := json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	if err != nil {
		return usecase.RenderedAgentConfig{}, err
	}
	return usecase.RenderedAgentConfig{
		Files: []usecase.AgentConfigFile{{Path: "gemini-settings.json", Content: string(b), Mode: "0600"}},
		Env:   []usecase.AgentEnv{{Name: "GEMINI_CLI_SYSTEM_SETTINGS_PATH", Value: domain.NewSecretValue(ConfigDirToken + "/gemini-settings.json")}},
	}, nil
}

func renderOpencode(entries []domain.AgentServerEntry) (usecase.RenderedAgentConfig, error) {
	servers := map[string]any{}
	for _, e := range entries {
		if e.Transport == domain.TransportHTTP {
			servers[e.Name] = map[string]any{"type": "remote", "url": e.URL, "headers": headerMap(e, opencodeEnv)}
		} else {
			servers[e.Name] = map[string]any{"type": "local", "command": append([]string{e.Command}, e.Args...), "environment": envMap(e, opencodeEnv)}
		}
	}
	b, err := json.Marshal(map[string]any{"mcp": servers})
	if err != nil {
		return usecase.RenderedAgentConfig{}, err
	}
	return usecase.RenderedAgentConfig{Env: []usecase.AgentEnv{{Name: "OPENCODE_CONFIG_CONTENT", Value: domain.NewSecretValue(string(b))}}}, nil
}
