package apiclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type AgentStatusResult struct {
	AgentRunning  bool   `json:"agent_running"`
	AgentKind     string `json:"agent_kind"`
	ReadyForInput bool   `json:"ready_for_input"`
}

func (c *Client) GetAgentStatus(ctx context.Context, worktreeID string) (AgentStatusResult, error) {
	var resp AgentStatusResult
	err := c.do(ctx, "GET", "/v1/worktrees/"+worktreeID+"/agent/status", nil, &resp)
	return resp, err
}

type WaitAgentResult struct {
	Exited   bool  `json:"exited"`
	ExitCode int32 `json:"exit_code"`
	TimedOut bool  `json:"timed_out"`
}

func (c *Client) WaitAgent(ctx context.Context, worktreeID string, timeoutMs int32) (WaitAgentResult, error) {
	var resp WaitAgentResult
	err := c.do(ctx, "POST", "/v1/worktrees/"+worktreeID+"/agent/wait", map[string]int32{"timeout_ms": timeoutMs}, &resp)
	return resp, err
}

func (c *Client) SendAgentInput(ctx context.Context, worktreeID, text string) error {
	return c.do(ctx, "POST", "/v1/worktrees/"+worktreeID+"/agent/send", map[string]string{"text": text}, nil)
}

// GetAgentSnapshot returns the raw text/plain scrollback body — a
// dedicated request here (not c.do) since the response is not JSON, per
// BR-CLI-06's "flat scrollback file" contract.
func (c *Client) GetAgentSnapshot(ctx context.Context, worktreeID string) (text string, truncated bool, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1/worktrees/"+worktreeID+"/agent/snapshot", nil)
	if err != nil {
		return "", false, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("apiclient: GetAgentSnapshot: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode >= 300 {
		return "", false, decodeErrorBody(resp.StatusCode, body)
	}
	return string(body), resp.Header.Get("X-Orca-Snapshot-Truncated") == "true", nil
}
