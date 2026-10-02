package mcpserver

import "context"

// Gate outcomes and risk levels shared by the tool catalog and every gate.
const (
	OutcomeAllow           = "allow"
	OutcomeRequireApproval = "require_approval"
	OutcomeDeny            = "deny"

	RiskRead            = "read"
	RiskWriteReversible = "write_reversible"
	RiskExec            = "exec"
	RiskDestructive     = "destructive"
	RiskAdmin           = "admin"
)

// EffectiveDecision is the tenant-default decision for one tool, as shown in
// tools/list filtering and the admin Tools tab (McpToolView.effective*).
type EffectiveDecision struct {
	Decision string // allow | require_approval | deny
	// Source is default | tenant_policy | hard_deny | kill_switch (CONTRACT).
	Source string
}

// ToolPolicyView is an OPTIONAL capability of a PolicyGate: it answers "what
// would the tenant default be for this tool" without a concrete call. The
// catalog type-asserts the gate to it; gates without it make every
// scope-permitted tool visible and decide at call time.
type ToolPolicyView interface {
	EffectiveDecision(ctx context.Context, tenantID string, meta ToolMeta) (EffectiveDecision, error)
}
