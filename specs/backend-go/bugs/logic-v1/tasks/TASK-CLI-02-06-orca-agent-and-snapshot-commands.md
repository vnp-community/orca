# TASK-CLI-02-06: `orca agent status/wait/send` + `orca snapshot` commands

**From Solution:** SOL-CLI-02
**Priority:** P1
**Service:** `orca-cli`
**File:** `backend-go/cmd/orca-cli/internal/apiclient/agent.go`
**Depends on:** TASK-CLI-02-05, TASK-CLI-01-06/07 (scaffold + `apiclient.Client`/`output`/cobra tree from SOL-CLI-01)
**Status:** `[ ]` TODO

---

## Context

The last mile: four thin `apiclient` methods (one REST call each — `resolveAgentPtyID` already moved server-side, so no client-side composition is needed) plus the CLI commands that call them, including BR-CLI-05's `orca agent wait --timeout` exit-code-2-on-timeout rule.

## Changes to make

**1. `backend-go/cmd/orca-cli/internal/apiclient/agent.go`:**

```go
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
```

**2. `backend-go/cmd/orca-cli/internal/command/agent_status.go`, `agent_send.go`, `snapshot.go`** — each a thin cobra subcommand calling the matching `apiclient` method and `output.Report`/`output.ReportError`, matching `worktree_create.go`'s existing shape (resolve `--worktree` flag, call, report).

**3. `backend-go/cmd/orca-cli/internal/command/agent_wait.go`** — BR-CLI-05's exit-code-2 rule:

```go
package command

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
)

type WaitResult struct {
	Exited   bool  `json:"exited"`
	ExitCode int32 `json:"exitCode"`
	TimedOut bool  `json:"timedOut"`
}

// RunAgentWait returns (result, exitCode). timedOut maps to exit code 2 —
// the one CLI-specific exit-code rule beyond output.Report's generic
// success/usage-error/server-error table (BR-CLI-05), since a timeout is
// neither a success nor a usage/server error, it's an inconclusive wait.
func RunAgentWait(ctx context.Context, cli *apiclient.Client, worktreeID string, timeout time.Duration) (WaitResult, int) {
	resp, err := cli.WaitAgent(ctx, worktreeID, int32(timeout.Milliseconds()))
	if err != nil {
		return WaitResult{}, output.ReportError(err, false)
	}
	if resp.TimedOut {
		return WaitResult{TimedOut: true}, 2
	}
	return WaitResult{Exited: resp.Exited, ExitCode: resp.ExitCode}, output.ExitOK
}
```

**4. `backend-go/cmd/orca-cli/internal/command/root.go`** — register `agent status|wait|send` and `snapshot` subcommands under the existing cobra tree, each taking `--worktree <id>` (required) plus their own flags (`--timeout` for `wait`, `--text`/stdin for `send`, `--output <file>` for `snapshot`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
go test ./cmd/orca-cli/internal/command/... -run 'TestAgentWait|TestAgentStatus|TestAgentSend|TestSnapshot' -v
```

Expected: `agent_wait_test.go` — `timed_out=true` maps to exit code `2` exactly, every other outcome maps per the base exit-code table; a fake clock/timeout does not leak into the exit-code decision (decided purely from the response body, not client-side elapsed time). `snapshot_test.go` — `--output result.txt` writes the exact response body byte-for-byte, no JSON unwrapping.
