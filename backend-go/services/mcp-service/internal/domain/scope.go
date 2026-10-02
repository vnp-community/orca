package domain

// Risk is the closed set of tool risk classes (CONTRACT §1 McpRisk).
type Risk string

const (
	RiskRead            Risk = "read"
	RiskWriteReversible Risk = "write_reversible"
	RiskExec            Risk = "exec"
	RiskDestructive     Risk = "destructive"
	RiskAdmin           Risk = "admin"
)

// ScopeDescriptor describes one OAuth/PAT scope a client may request.
type ScopeDescriptor struct {
	ID          string
	Label       string
	Description string
	Risk        Risk
}

// ScopeCatalog is the single source of McpServerInfo.scopesSupported.
// Later solutions extend it additively (BE-MCP-SOL-006).
func ScopeCatalog() []ScopeDescriptor {
	return []ScopeDescriptor{
		{ID: "orca:read", Label: "Read", Description: "Read worktrees, tasks and other workspace data.", Risk: RiskRead},
		{ID: "orca:write", Label: "Write", Description: "Make reversible changes such as creating tasks or notes.", Risk: RiskWriteReversible},
		{ID: "orca:exec", Label: "Execute", Description: "Run commands and start agents in a workspace.", Risk: RiskExec},
		{ID: "orca:admin", Label: "Admin", Description: "Manage tenant-level settings and integrations.", Risk: RiskAdmin},
	}
}
