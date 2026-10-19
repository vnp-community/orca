package agentrelay

// MethodExecPrompt and MethodComplete are the agent RPC names relayed through infra-fleet-service.
const (
	MethodExecPrompt = "agent.execPrompt"
	MethodComplete   = "ai.complete"
)

// Values the agent understands for the string options of agent.execPrompt.
const (
	TrustDefault = "default"
	TrustFull    = "full"

	AccessWrite    = "write"
	AccessReadOnly = "readonly"

	WorkspaceWorktree = "worktree"
	WorkspaceRepoRoot = "repo_root"
	WorkspaceScratch  = "scratch"
)

// ExecParams is the params object of agent.execPrompt: the union of what request, task, workflow and orchestration
// services send. Every optional field is omitempty so a flow that does not use it sends the same JSON as before;
// Prompt and WorktreePath are always sent because the agent rejects a missing one with its own typed error.
//
// Build it with Plain, ReadOnly, WriteProfile or Contract rather than by hand: they hold the combinations the agent
// accepts. The field order keeps request-service's historical key order.
type ExecParams struct {
	StepID       string            `json:"stepId,omitempty"`
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	TimeoutMS    int               `json:"timeoutMs,omitempty"`

	// Only for agents that advertise agent.execPrompt.readonly; older agents ignore unknown keys, so the caller decides
	// before sending.
	AccessMode     string `json:"accessMode,omitempty"`
	WorkspaceKind  string `json:"workspaceKind,omitempty"`
	ReportChanges  bool   `json:"reportChanges,omitempty"`
	MaxOutputBytes int    `json:"maxOutputBytes,omitempty"`

	// Only for agents that advertise agent.execPrompt.knowledgeHub; the agent builds the MCP config itself.
	KnowledgeRoot     string `json:"knowledgeRoot,omitempty"`
	ProposeChangesets bool   `json:"proposeChangesets,omitempty"`

	// A write run under a named profile: the agent chooses every flag from the name, so no tool rule travels.
	PermissionProfile string       `json:"permissionProfile,omitempty"`
	ResultBlock       *ResultBlock `json:"resultBlock,omitempty"`

	// Passthrough of the caller's resolved account and profile; the credential itself never travels.
	Model     string `json:"model,omitempty"`
	AccountID string `json:"accountId,omitempty"`
	InitFile  string `json:"initFile,omitempty"`
}

// ResultBlock asks the agent to find a delimited result block in stdout and parse it; Nonce authenticates the block.
type ResultBlock struct {
	Nonce string `json:"nonce"`
}

// validate rejects combinations the agent would refuse or, worse, run with weaker limits than the caller meant.
func (p ExecParams) validate() error {
	if p.AccessMode == AccessReadOnly && p.PermissionProfile != "" {
		return ErrConflictingAccess
	}
	if p.ProposeChangesets && p.KnowledgeRoot == "" {
		return ErrProposeWithoutRoot
	}
	return nil
}
