package domain

// AgentServerEntry is one MCP server as handed to an agent CLI renderer. It
// holds env var NAMES only: values travel separately in the child-process
// environment so rendered files never contain a secret.
type AgentServerEntry struct {
	Name      string
	Transport string // http|stdio
	URL       string
	Command   string
	Args      []string
	// Headers maps header name -> child env var holding its value (http).
	Headers []EnvBinding
	// Env maps the server's own env var name -> child env var holding its value (stdio).
	Env []EnvBinding
	// BearerVar, when set, adds "Authorization: Bearer ${BearerVar}" (the Orca server).
	BearerVar string
}

// EnvBinding names the agent-process variable that carries a secret value.
type EnvBinding struct{ Name, Var string }

// AgentConfigWarning explains why a server was left out or a caveat applies.
type AgentConfigWarning struct{ ServerName, Reason string }

// Warning reasons (also the wire vocabulary of ResolveAgentMcpConfig).
const (
	WarnNotInRegistry        = "not_in_registry"
	WarnPendingReview        = "pending_review"
	WarnDisabled             = "disabled"
	WarnToolsChanged         = "tools_changed"
	WarnSecretUnresolvable   = "secret_unresolvable"
	WarnSSRFBlocked          = "ssrf_blocked"
	WarnStdioUnsupported     = "stdio_unsupported"
	WarnAgentUnsupported     = "agent_unsupported"
	WarnDepthExceeded        = "depth_exceeded"
	WarnOrcaTokenUnavailable = "orca_token_unavailable"
	WarnConfigUnverified     = "config_format_unverified"
)
