package agentrelay

// This file freezes the params structs that four services used before they shared this package, copied with their json
// tags unchanged. They exist only so the wire format of every flow can be compared against the shared ExecParams and
// CompleteParams; delete a copy only together with the golden file it produced.

// ---- request-service: internal/adapter/grpcclient/agent_prompt_relay.go ----

type legacyRequestParams struct {
	StepID         string            `json:"stepId"`
	Prompt         string            `json:"prompt"`
	WorktreePath   string            `json:"worktreePath"`
	TrustPreset    string            `json:"trustPreset,omitempty"`
	Env            map[string]string `json:"env"`
	TimeoutMS      int               `json:"timeoutMs,omitempty"`
	AccessMode     string            `json:"accessMode,omitempty"`
	WorkspaceKind  string            `json:"workspaceKind,omitempty"`
	ReportChanges  bool              `json:"reportChanges,omitempty"`
	MaxOutputBytes int               `json:"maxOutputBytes,omitempty"`

	KnowledgeRoot     string                `json:"knowledgeRoot,omitempty"`
	ProposeChangesets bool                  `json:"proposeChangesets,omitempty"`
	PermissionProfile string                `json:"permissionProfile,omitempty"`
	ResultBlock       *legacyRequestResultB `json:"resultBlock,omitempty"`
}

type legacyRequestResultB struct {
	Nonce string `json:"nonce"`
}

// legacyRequestInput is usecase.AgentPromptInput.
type legacyRequestInput struct {
	StepID, Prompt, RepoPath, RequestID, ProjectID string
	TimeoutMS                                      int
	ReadOnlyEnforced                               bool
	KnowledgeRoot                                  string
	ProposeChangesets                              bool
	WriteProfile                                   string
	ResultNonce                                    string
}

// legacyBuildRequestParams is buildAgentParams, statement for statement.
func legacyBuildRequestParams(in legacyRequestInput) legacyRequestParams {
	const maxOutputBytesAsk = 256*1024 + 64*1024
	p := legacyRequestParams{
		StepID: in.StepID, Prompt: in.Prompt, WorktreePath: in.RepoPath, TimeoutMS: in.TimeoutMS,
		Env: map[string]string{"ORCA_REQUEST_ID": in.RequestID, "ORCA_PROJECT_ID": in.ProjectID},
	}
	if in.KnowledgeRoot != "" {
		p.KnowledgeRoot, p.ProposeChangesets = in.KnowledgeRoot, in.ProposeChangesets
	}
	if in.ResultNonce != "" {
		p.ResultBlock = &legacyRequestResultB{Nonce: in.ResultNonce}
	}
	switch {
	case in.ReadOnlyEnforced:
		p.AccessMode, p.WorkspaceKind, p.ReportChanges, p.MaxOutputBytes = "readonly", "repo_root", true, maxOutputBytesAsk
	case in.WriteProfile != "":
		p.AccessMode, p.WorkspaceKind, p.ReportChanges, p.MaxOutputBytes = "write", "worktree", true, maxOutputBytesAsk
		p.PermissionProfile, p.TrustPreset = in.WriteProfile, "default"
	default:
		p.TrustPreset = "default"
	}
	return p
}

// ---- task-service: internal/adapter/grpcclient/simple_executor.go (+ simple_executor_contract.go) ----

type legacyTaskParams struct {
	Prompt         string             `json:"prompt"`
	WorktreePath   string             `json:"worktreePath"`
	StepID         string             `json:"stepId,omitempty"`
	TrustPreset    string             `json:"trustPreset,omitempty"`
	Model          string             `json:"model,omitempty"`
	Env            map[string]string  `json:"env,omitempty"`
	InitFile       string             `json:"initFile,omitempty"`
	ResultBlock    *legacyTaskResultB `json:"resultBlock,omitempty"`
	ReportChanges  bool               `json:"reportChanges,omitempty"`
	MaxOutputBytes int                `json:"maxOutputBytes,omitempty"`
	TimeoutMs      int                `json:"timeoutMs,omitempty"`
	KnowledgeRoot  string             `json:"knowledgeRoot,omitempty"`
}

type legacyTaskResultB struct {
	Nonce string `json:"nonce"`
}

// legacyApplyContract is contractRunOptions.apply.
func legacyApplyContract(p *legacyTaskParams, nonce string) {
	const contractRunTimeoutMs, contractMaxOutputBytes = 25 * 60 * 1000, 1 << 20
	p.ResultBlock = &legacyTaskResultB{Nonce: nonce}
	p.ReportChanges = true
	p.MaxOutputBytes = contractMaxOutputBytes
	p.TimeoutMs = contractRunTimeoutMs
}

// ---- workflow-service: internal/adapter/infrafleetclient/agent_step_executor.go ----

type legacyWorkflowParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	AccountID    string            `json:"accountId,omitempty"`
	Model        string            `json:"model,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	InitFile     string            `json:"initFile,omitempty"`
}

// ---- orchestration-service: internal/adapter/infrafleetclient/worker_dispatcher.go ----

type legacyOrchestrationParams struct {
	Prompt       string `json:"prompt"`
	WorktreePath string `json:"worktreePath"`
	StepID       string `json:"stepId,omitempty"`
}

// ---- ai.complete callers ----

// legacyRequestComplete is request-service relayAIComplete's params map.
func legacyRequestComplete(accountID, prompt string) any {
	params := map[string]any{"prompt": prompt}
	if accountID != "" {
		params["accountId"] = accountID
	}
	return params
}

// legacyGitComplete is git-gateway-service RelayExecutor.Complete's params map; task-service's AICompleter sends the same.
func legacyGitComplete(prompt string) any { return map[string]any{"prompt": prompt} }
