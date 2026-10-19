package agentrelay

const (
	// AnalysisMaxOutputBytes is what read-only and write-profile runs ask of the agent: slightly more than the 256 KiB a
	// caller reads, so a JSON block at the end of the output survives.
	AnalysisMaxOutputBytes = 256*1024 + 64*1024
	// ContractMaxOutputBytes and ContractTimeoutMS bound a task run that writes code and runs the task's own checks; the
	// agent's five minute default cut those off twice in a row.
	ContractMaxOutputBytes = 1 << 20
	ContractTimeoutMS      = 25 * 60 * 1000
)

// Run holds what every agent.execPrompt call has in common. The builders add the mode-specific options on top.
type Run struct {
	StepID       string
	Prompt       string
	WorktreePath string
	// TrustPreset applies to Plain and Contract runs only; ReadOnly drops it and WriteProfile fixes it to TrustDefault.
	TrustPreset string
	Model       string
	AccountID   string
	InitFile    string
	// Env must never hold a token or credential.
	Env       map[string]string
	TimeoutMS int
	// MaxOutputBytes overrides the builder's default when positive.
	MaxOutputBytes int
	// KnowledgeRoot is the project's repository root, where the knowledge graph lives.
	KnowledgeRoot     string
	ProposeChangesets bool
	// ResultNonce asks the agent to parse a result block; Contract sends it even when empty so the agent can refuse.
	ResultNonce string
}

// Plain builds a run with the caller's trust preset and no access-mode options: a workflow step, an orchestration
// dispatch or a task without a result contract.
func Plain(r Run) ExecParams {
	p := ExecParams{
		StepID: r.StepID, Prompt: r.Prompt, WorktreePath: r.WorktreePath, TrustPreset: r.TrustPreset,
		Model: r.Model, AccountID: r.AccountID, InitFile: r.InitFile, Env: r.Env, TimeoutMS: r.TimeoutMS,
		MaxOutputBytes: r.MaxOutputBytes,
	}
	// proposeChangesets without a root would be refused by the agent, so it is only sent together with one.
	if r.KnowledgeRoot != "" {
		p.KnowledgeRoot, p.ProposeChangesets = r.KnowledgeRoot, r.ProposeChangesets
	}
	if r.ResultNonce != "" {
		p.ResultBlock = &ResultBlock{Nonce: r.ResultNonce}
	}
	return p
}

// ReadOnly builds an analysis run that must not change the repository. With accessMode=readonly the agent drops any
// trust preset, so none is sent.
func ReadOnly(r Run) ExecParams {
	r.TrustPreset = ""
	p := Plain(r)
	p.AccessMode, p.WorkspaceKind, p.ReportChanges = AccessReadOnly, WorkspaceRepoRoot, true
	p.MaxOutputBytes = outputBytesOr(r.MaxOutputBytes, AnalysisMaxOutputBytes)
	return p
}

// WriteProfile builds a run that writes in a worktree under a named permission profile. The profile is the whole
// permission story; the default trust preset is sent only so the agent never sees an empty one, and it ignores it.
func WriteProfile(r Run, profile string) (ExecParams, error) {
	if profile == "" {
		return ExecParams{}, ErrProfileRequired
	}
	r.TrustPreset = TrustDefault
	p := Plain(r)
	p.AccessMode, p.WorkspaceKind, p.ReportChanges = AccessWrite, WorkspaceWorktree, true
	p.PermissionProfile = profile
	p.MaxOutputBytes = outputBytesOr(r.MaxOutputBytes, AnalysisMaxOutputBytes)
	return p, nil
}

// Contract builds a task run that reports changes and a result block, with the long timeout such a run needs. The
// nonce is always sent: an empty one is the agent's to refuse, not ours to silently drop.
func Contract(r Run) ExecParams {
	p := Plain(r)
	p.ResultBlock = &ResultBlock{Nonce: r.ResultNonce}
	p.ReportChanges = true
	p.MaxOutputBytes = outputBytesOr(r.MaxOutputBytes, ContractMaxOutputBytes)
	if p.TimeoutMS <= 0 {
		p.TimeoutMS = ContractTimeoutMS
	}
	return p
}

func outputBytesOr(requested, fallback int) int {
	if requested > 0 {
		return requested
	}
	return fallback
}
