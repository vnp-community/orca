// Package tools is the MCP tool catalog and executor: it maps hand-declared
// ToolSpecs onto wscompat.Registry channels (BE-MCP-SOL-007/008). It is a
// protocol adapter only: allow/deny/approval decisions come from the
// mcpserver.PolicyGate port, never from here.
package tools

import (
	"encoding/json"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/stablyai/orca-go/common/mcpscope"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// Risk values are the CONTRACT McpRisk strings (same constants as mcpserver).
type Risk = string

// SpecKind distinguishes a 1:1 channel tool from an adapter-side composite.
type SpecKind int

const (
	KindChannel SpecKind = iota
	// KindComposite tools run adapter code over several channels (BE-009);
	// the parity test honours UsesChannels.
	KindComposite
)

// Annotations are the MCP behaviour hints; parity test keeps them consistent
// with Risk.
type Annotations struct {
	ReadOnly, Destructive, Idempotent, OpenWorld bool
}

// ArgsFunc maps validated tool input to the channel's positional args. It is
// the ONLY place identity-like fields could enter, and none may.
type ArgsFunc func(in json.RawMessage, id wscompat.Identity) ([]json.RawMessage, error)

// ToolSpec describes one MCP tool.
type ToolSpec struct {
	Name string
	// NameReason is required when Name differs from ChannelToToolName(Channel).
	NameReason   string
	Channel      string
	UsesChannels []string
	Kind         SpecKind
	Namespace    string
	Pack         int // 1..4 (BE-008)
	Title        string
	Description  string
	Risk         Risk
	Annotations  Annotations
	// Fields drive Input schema AND the default Args mapping, so they cannot
	// drift apart.
	Fields []Field
	// Consts are wire keys always set by the adapter (never from input).
	Consts map[string]any
	// ArgsOverride replaces the default field mapping (rare).
	ArgsOverride ArgsFunc
	// Declared marks a spec whose Args are not implemented yet: it is
	// catalogued (parity, admin view) but never listed or executed.
	Declared bool
	// List marks results that are an array; the tool wraps them as {items:[]}.
	List bool
	// Untrusted marks outputs carrying third-party text (tickets, comments).
	Untrusted bool
	// CacheTTL > 0 caches the normalized result per (tenant,user,tool,args)
	// to spare provider rate limits (AGENTS.md: gh rate limit).
	CacheTTL       int // seconds
	MaxResultBytes int
	// Post may reshape the decoded result before redaction (e.g. files_read).
	Post func(v any) any
	// Compose implements a KindComposite tool over one or more channels.
	Compose ComposeFunc
	// KeepKeys leaves result keys exactly as the tool produced them (snake_case
	// documented in the tool descriptions) instead of camelizing them.
	KeepKeys bool
	// PathArg names the input holding a worktree-relative path that must pass
	// the sensitive-path rules (files tools); PathOptional allows it empty.
	PathArg      string
	PathOptional bool
	// PathFilter drops sensitive paths from a list result.
	PathFilter pathFilterKind
	// PII marks results with personal data (emails, phones) masked under
	// MCP_PII_MASK=directory (default) and all.
	PII bool

	input    *jsonschema.Schema
	resolved *jsonschema.Resolved
	output   *jsonschema.Schema
}

// RequiredScope derives the OAuth scope from Risk; it can never be narrower.
func (s *ToolSpec) RequiredScope() string { return RiskToScope(s.Risk) }

// RiskToScope maps CONTRACT McpRisk -> McpScopeId.
func RiskToScope(r Risk) string {
	switch r {
	case mcpserver.RiskRead:
		return mcpscope.Read
	case mcpserver.RiskWriteReversible:
		return mcpscope.Write
	case mcpserver.RiskExec:
		return mcpscope.Exec
	default:
		return mcpscope.Admin
	}
}

// Meta is what the PolicyGate sees.
func (s *ToolSpec) Meta() mcpserver.ToolMeta {
	return mcpserver.ToolMeta{Name: s.Name, Channel: s.Channel, Namespace: s.Namespace, Risk: s.Risk,
		RequiredScope: s.RequiredScope(), OpenWorld: s.Annotations.OpenWorld, UntrustedOutput: s.Untrusted}
}

// ChannelToToolName implements D7: "." -> "_", camelCase kept.
func ChannelToToolName(channel string) string { return strings.ReplaceAll(channel, ".", "_") }

func namespaceOf(channel string) string {
	if i := strings.IndexByte(channel, '.'); i > 0 {
		return channel[:i]
	}
	return channel
}
