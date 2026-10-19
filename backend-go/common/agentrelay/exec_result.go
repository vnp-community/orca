package agentrelay

// ExecResult is what agent.execPrompt returned. Every field the agent adds over time is optional, so a result from an
// older agent decodes with those fields zero.
type ExecResult struct {
	Stdout string
	Stderr string
	// ExitCode is nil when the process never completed (killed on timeout, failed to start).
	ExitCode *int
	TimedOut bool
	// StepID echoes the request's StepID.
	StepID   string
	Warnings []string
	Applied  *Applied
	Changes  *Changes
	Parsed   *Parsed
	// Truncated says which streams the agent cut to fit its output budget.
	Truncated *Truncated
	Stats     Stats
}

// ExitCodeOr returns the exit code, or fallback when the process never completed.
func (r ExecResult) ExitCodeOr(fallback int) int {
	if r.ExitCode == nil {
		return fallback
	}
	return *r.ExitCode
}

// Applied echoes the access options the agent actually used; present only when the caller sent explicit ones.
type Applied struct {
	AccessMode    string `json:"accessMode"`
	WorkspaceKind string `json:"workspaceKind"`
}

// Changes is the agent's before/after diff of the workspace: what the run touched, never file contents.
type Changes struct {
	Available    bool          `json:"available"`
	Reason       string        `json:"reason,omitempty"`
	HeadBefore   *string       `json:"headBefore,omitempty"`
	HeadAfter    *string       `json:"headAfter,omitempty"`
	HeadMoved    bool          `json:"headMoved"`
	ChangedFiles []ChangedFile `json:"changedFiles,omitempty"`
	Truncated    bool          `json:"truncated,omitempty"`
}

// ChangedFile is one path the run touched; Change is added, modified, deleted, untracked or renamed.
type ChangedFile struct {
	Path   string `json:"path"`
	Change string `json:"change"`
}

// Parsed is the agent's reading of the result block. When OK, Value is the block's JSON object; otherwise Code says why
// there is none (for example RESULT_BLOCK_MISSING) and Detail may add a short reason.
type Parsed struct {
	OK     bool
	Value  []byte
	Code   string
	Detail string
}

// Truncated tells which output streams hit the budget.
type Truncated struct {
	Stdout bool `json:"stdout"`
	Stderr bool `json:"stderr"`
}

// Stats is what a stream-json run says about itself: tool names with counts (never arguments or results, which can be
// document text) and claude's own usage figures. An older agent sends neither.
type Stats struct {
	ToolCalls map[string]int
	Usage     *Usage
}

// Usage holds claude's figures for a run. A missing cost is "not reported", never zero.
type Usage struct {
	Turns               int
	DurationMS          int64
	CostUSD             *float64
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
}
