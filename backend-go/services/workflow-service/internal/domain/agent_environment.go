package domain

import (
	"fmt"
	"path"
	"strings"
)

// AgentBinaryMap mirrors BL-PRF-04's resolveAgentBinary AGENT_MAP verbatim.
// NOT wired into any live RPC call today — agent.execPrompt (this service's
// only agent-spawn RPC) resolves its own binary server-side from the model
// name; kept here for spec fidelity, see this file's package doc comment.
var AgentBinaryMap = map[string]string{
	"claude-opus-4-5":   "claude",
	"claude-sonnet-4-5": "claude",
	"codex":             "codex",
	"gemini":            "gemini",
	"opencode":          "opencode",
}

// ResolveAgentBinary implements BL-PRF-04's resolveAgentBinary — unknown/
// empty model falls back to "claude".
func ResolveAgentBinary(model string) string {
	if bin, ok := AgentBinaryMap[model]; ok {
		return bin
	}
	return "claude"
}

// TrustPresetArgs mirrors BL-PRF-04's buildAgentArgs TRUST_ARGS verbatim.
// NOT sent over agent.execPrompt (bare trustPreset string, interpreted
// server-side) — see this file's package doc comment.
var TrustPresetArgs = map[string][]string{
	"minimal":    {"--trust", "minimal"},
	"standard":   {"--trust", "standard"},
	"permissive": {"--trust", "full", "--dangerously-skip-permissions"},
}

// BuildAgentArgs implements BL-PRF-04's buildAgentArgs — unknown/empty
// preset falls back to "standard".
func BuildAgentArgs(trustPreset string) []string {
	if args, ok := TrustPresetArgs[trustPreset]; ok {
		return args
	}
	return TrustPresetArgs["standard"]
}

// AgentEnv is the profile-derived environment BuildAgentEnv produces —
// serialized into agent.execPrompt's `env` field.
type AgentEnv map[string]string

// BuildAgentEnv implements BL-PRF-04 step 5's agentEnv construction.
// resolved is tenant-service's ResolvedProfile.Settings, already-decoded
// generic JSON (map[string]any), reads shell.envVars/shell.pathAdditions/
// agent.preferredModel the same way tenant-service's own ResolveProfile
// merge helpers do.
//
// PATH join uses ":" unconditionally — the target is the Dev Server Agent's
// host shell environment (Linux/macOS only, per
// 02-microservices-decomposition.md's "Dev Server Agent" framing), not this
// Go service's own OS, so this repo's cross-platform path-separator rule
// (AGENTS.md) doesn't apply to this opaque remote-shell string.
func BuildAgentEnv(resolved map[string]any, userID, projectID, projectName, existingPath string) AgentEnv {
	env := AgentEnv{}

	if shell, ok := resolved["shell"].(map[string]any); ok {
		if vars, ok := shell["envVars"].(map[string]any); ok {
			for k, v := range vars {
				if s, ok := v.(string); ok {
					env[k] = s
				}
			}
		}
		if adds, ok := shell["pathAdditions"].([]any); ok && len(adds) > 0 {
			parts := make([]string, 0, len(adds)+1)
			for _, a := range adds {
				if s, ok := a.(string); ok {
					parts = append(parts, s)
				}
			}
			parts = append(parts, existingPath)
			env["PATH"] = strings.Join(parts, ":")
		}
	}

	// Per-user credential isolation — BL-PRF-04 step 5's GH_CONFIG_DIR/
	// GLAB_CONFIG_DIR, keyed by userID so two users' agent sessions on the
	// same dev server never share gh/glab auth state.
	env["GH_CONFIG_DIR"] = path.Join("/home/dev/.config/gh", userID)
	env["GLAB_CONFIG_DIR"] = path.Join("/home/dev/.config/glab-cli", userID)

	if agent, ok := resolved["agent"].(map[string]any); ok {
		if model, ok := agent["preferredModel"].(string); ok && model != "" {
			env["ANTHROPIC_MODEL"] = model
		}
	}
	env["ORCA_PROJECT_ID"] = projectID
	env["ORCA_PROJECT_NAME"] = projectName
	return env
}

// PreambleInput is BuildProjectContext's input — a subset of
// project-service's ProjectContext plus worktree/user fields it doesn't
// carry (worktree.path/branch come from git-gateway-service's
// CreateWorktree result, user.name/email/departmentName from the caller's
// own context or a lightweight lookup — sourcing these is this task's own
// open item, see this task's Verify section).
type PreambleInput struct {
	ProjectName, Description, RepoURL, DevServerHostname string
	WorktreePath, Branch                                 string
	UserName, UserEmail, DepartmentName                  string
}

// BuildProjectContext implements BL-PRF-04's buildProjectContext verbatim,
// including its exact field order and blank trailing line.
func BuildProjectContext(in PreambleInput) string {
	team := in.DepartmentName
	if team == "" {
		team = "No team"
	}
	lines := []string{
		"# Orca Project Context",
		"Project: " + in.ProjectName,
		"Description: " + in.Description,
		"Repository: " + in.RepoURL,
		"Working directory: " + in.WorktreePath,
		"Branch: " + in.Branch,
		"Dev Server: " + in.DevServerHostname,
		fmt.Sprintf("Developer: %s (%s)", in.UserName, in.UserEmail),
		"Team: " + team,
		"",
	}
	return strings.Join(lines, "\n")
}
