package apiclient

import "context"

type CreateWorktreeInput struct {
	ProjectID, RepoID, Branch, BaseRef, IdempotencyKey string
}

type WorktreeResult struct {
	WorktreeID string `json:"worktree_id"`
	Path       string `json:"path"`
	HeadSHA    string `json:"head_sha"`
}

// CreateWorktree calls POST /v1/worktrees.
func (c *Client) CreateWorktree(ctx context.Context, in CreateWorktreeInput) (WorktreeResult, error) {
	var resp WorktreeResult
	err := c.do(ctx, "POST", "/v1/worktrees", map[string]string{
		"project_id": in.ProjectID, "repo_id": in.RepoID, "branch": in.Branch,
		"base_ref": in.BaseRef, "idempotency_key": in.IdempotencyKey,
	}, &resp)
	return resp, err
}

// SpawnAgent is a placeholder seam — BUG-AG-01 has no real agent-spawn RPC
// yet (today's only spawn RPC, SpawnTerminalSession, launches a bare
// shell). Always returns AGENT_SPAWN_NOT_SUPPORTED so RunWorktreeCreate's
// caller can degrade gracefully (exit 0 + warning) instead of erroring.
// Replace this body, and only this body, once BUG-AG-01 lands a real RPC.
func (c *Client) SpawnAgent(ctx context.Context, worktreeID, agentType string) (SpawnAgentResult, error) {
	return SpawnAgentResult{}, &CLIError{Code: "AGENT_SPAWN_NOT_SUPPORTED", Message: "agent spawn is not yet implemented (see BUG-AG-01)"}
}

type SpawnAgentResult struct {
	PtyID string
}
