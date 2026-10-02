package domain

import (
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Decision values of the tool gate (CONTRACT McpDecision).
const (
	DecisionAllow           = "allow"
	DecisionRequireApproval = "require_approval"
)

const (
	CodePolicyHardDeny        = "MCP_POLICY_HARD_DENY"
	CodePolicyVersionConflict = "MCP_POLICY_VERSION_CONFLICT"
	MaxPolicyNoteLength       = 500
)

const (
	SubjectPolicyChanged     = "orca.mcp.policy.changed"
	SubjectSettingsChanged   = "orca.mcp.settings.changed"
	SubjectAuditAppended     = "orca.mcp.audit.appended"
	SubjectApprovalRequested = "orca.mcp.approval.requested"
	SubjectApprovalResolved  = "orca.mcp.approval.resolved"
	SubjectKillSwitchChanged = "orca.mcp.killswitch.changed"
)

func ErrPolicyHardDeny(tools []string) error {
	if len(tools) > 5 {
		tools = tools[:5]
	}
	return apperrors.New(apperrors.KindFailedPrecondition, CodePolicyHardDeny,
		"policy would loosen hard-denied tools: "+strings.Join(tools, ", "), nil)
}

func ErrPolicyVersionConflict(current int) error {
	return apperrors.New(apperrors.KindAlreadyExists, CodePolicyVersionConflict,
		"policy was changed by someone else; reload and retry (current="+itoa(current)+")", nil)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ValidDecision reports whether d is one of the three gate decisions.
func ValidDecision(d string) bool {
	return d == DecisionAllow || d == DecisionRequireApproval || d == DecisionDeny
}

// ValidRisk reports whether r is a known tool risk class.
func ValidRisk(r string) bool {
	switch Risk(r) {
	case RiskRead, RiskWriteReversible, RiskExec, RiskDestructive, RiskAdmin:
		return true
	}
	return false
}

// ToolPolicyMatch lists only the dimensions that are set (CHECK >= 1).
type ToolPolicyMatch struct {
	Tool, Namespace, Risk, ClientID string
	Roles                           []string
}

func (m ToolPolicyMatch) dimensions() int {
	n := 0
	for _, s := range []string{m.Tool, m.Namespace, m.Risk, m.ClientID} {
		if s != "" {
			n++
		}
	}
	if len(m.Roles) > 0 {
		n++
	}
	return n
}

// AsMap omits unset dimensions: a JSON null would make Rego's `not m.tool`
// treat the dimension as set.
func (m ToolPolicyMatch) AsMap() map[string]any {
	out := map[string]any{}
	if m.Tool != "" {
		out["tool"] = m.Tool
	}
	if m.Namespace != "" {
		out["namespace"] = m.Namespace
	}
	if m.Risk != "" {
		out["risk"] = m.Risk
	}
	if m.ClientID != "" {
		out["clientId"] = m.ClientID
	}
	if len(m.Roles) > 0 {
		roles := make([]any, len(m.Roles))
		for i, r := range m.Roles {
			roles[i] = r
		}
		out["roles"] = roles
	}
	return out
}

// ToolPolicy is one versioned tenant rule (mcp.tool_policies).
type ToolPolicy struct {
	ID        string
	Version   int
	Match     ToolPolicyMatch
	Decision  string
	Note      string
	CreatedBy string
	UpdatedBy string
	UpdatedAt time.Time
}

// Validate mirrors the CHECK constraints so callers get typed errors.
func (p ToolPolicy) Validate() error {
	if !ValidDecision(p.Decision) {
		return ErrInvalidArgument("decision must be allow, require_approval or deny")
	}
	if p.Match.dimensions() < 1 {
		return ErrInvalidArgument("policy must match at least one of tool, namespace, risk, clientId, roles")
	}
	if p.Match.Risk != "" && !ValidRisk(p.Match.Risk) {
		return ErrInvalidArgument("unknown risk in match")
	}
	for _, r := range p.Match.Roles {
		if r != RoleAdmin && r != RoleUser {
			return ErrInvalidArgument("roles must be admin or user")
		}
	}
	if len([]rune(p.Note)) > MaxPolicyNoteLength {
		return ErrInvalidArgument("note is limited to 500 characters")
	}
	return nil
}

// ToolRef describes a tool; the gateway builds it from its catalog, never from the client.
type ToolRef struct {
	Name, Channel, Namespace, Risk, RequiredScope, Title string
	OpenWorld, SpawnsProcess, ReadUntrusted              bool
}

func (t ToolRef) AsMap() map[string]any {
	return map[string]any{
		"name": t.Name, "channel": t.Channel, "namespace": t.Namespace, "risk": t.Risk,
		"required_scope": t.RequiredScope, "open_world": t.OpenWorld, "spawns_process": t.SpawnsProcess,
	}
}

// CallContext is the verified caller context of one tool call.
type CallContext struct {
	ClientID, ClientName, TokenKind, MCPSessionID, MCPRoot, TokenID, GrantID string
	Scopes                                                                   []string
	Depth                                                                    int
}

// PolicyInput is the Rego input (see policy/orca-authz/mcp.rego).
type PolicyInput struct {
	UserID, UserRole string
	ClientID         string
	ClientStatus     string
	Scopes           []string
	Enabled          bool
	MaxDepth         int
	KillActive       bool
	Depth            int
	UntrustedRead    bool
	Policies         []ToolPolicy
	Tool             ToolRef
}

func (in PolicyInput) AsMap() map[string]any {
	pols := make([]any, 0, len(in.Policies))
	for _, p := range in.Policies {
		pols = append(pols, map[string]any{"id": p.ID, "decision": p.Decision, "match": p.Match.AsMap()})
	}
	scopes := make([]any, len(in.Scopes))
	for i, s := range in.Scopes {
		scopes[i] = s
	}
	return map[string]any{
		"user":            map[string]any{"id": in.UserID, "role": in.UserRole},
		"client":          map[string]any{"id": in.ClientID, "status": in.ClientStatus},
		"token":           map[string]any{"scopes": scopes},
		"settings":        map[string]any{"enabled": in.Enabled, "max_depth": in.MaxDepth},
		"killswitch":      map[string]any{"active": in.KillActive},
		"session":         map[string]any{"depth": in.Depth, "untrusted_read": in.UntrustedRead},
		"tenant_policies": pols,
		"tool":            in.Tool.AsMap(),
	}
}

// PolicyDecision is the gate's verdict.
type PolicyDecision struct {
	Decision string
	Source   string
	Reasons  []string
	Epoch    int64
}

// DenyUnavailable is the fail-closed verdict used on any engine failure.
func DenyUnavailable() PolicyDecision {
	return PolicyDecision{Decision: DecisionDeny, Source: "policy_error", Reasons: []string{"policy_unavailable"}}
}

// ParseDecision converts the engine output; anything malformed is a deny.
func ParseDecision(m map[string]any) PolicyDecision {
	d, _ := m["decision"].(string)
	if !ValidDecision(d) {
		return DenyUnavailable()
	}
	src, _ := m["source"].(string)
	var reasons []string
	if rs, ok := m["reasons"].([]any); ok {
		for _, r := range rs {
			if s, ok := r.(string); ok {
				reasons = append(reasons, s)
			}
		}
	}
	return PolicyDecision{Decision: d, Source: src, Reasons: reasons}
}
