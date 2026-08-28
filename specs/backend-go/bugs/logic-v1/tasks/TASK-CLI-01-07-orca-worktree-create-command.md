# TASK-CLI-01-07: `orca worktree create` — apiclient methods + command wiring + exit codes

**From Solution:** SOL-CLI-01
**Priority:** P1
**Service:** `orca-cli`
**File:** `backend-go/cmd/orca-cli/internal/command/worktree_create.go`
**Depends on:** TASK-CLI-01-04 (REST route), TASK-CLI-01-05 (auth route), TASK-CLI-01-06 (scaffold)
**Status:** [x] DONE — implemented apiclient/{auth,worktree}.go, output/output.go, command/{worktree_create,root}.go + main.go; added worktree_create_test.go (idempotency stability/override, agent-spawn-failure exit-0, missing-flags exit-2-no-HTTP, --json shape); `go build ./cmd/orca-cli/...` and `go test ./cmd/orca-cli/... -v` pass from both the module and workspace root.

---

## Context

This is the command BUG-CLI-01 exists to close: `orca worktree create --name <branch> --base <ref>` calling the real `POST /v1/worktrees` route, with BR-CLI-01's idempotency key auto-derived and BR-CLI-02/03's `--json`/exit-code contract implemented. `--agent`/`--prompt` degrade to a warning (exit 0), not a hard failure — the real spawn RPC doesn't exist until BUG-AG-01 lands, so `SpawnAgent` is stubbed to always return `AGENT_SPAWN_NOT_SUPPORTED` for this task; wiring it to a real RPC is explicitly out of this solution's scope.

## Changes to make

**1. `backend-go/cmd/orca-cli/internal/apiclient/auth.go`:**

```go
package apiclient

import "context"

type LoginResult struct {
	JWT       string
	ExpiresAt string
}

// Login calls POST /auth/cli-token.
func Login(ctx context.Context, c *Client, email, password string) (LoginResult, error) {
	var resp struct {
		JWT       string `json:"jwt"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := c.do(ctx, "POST", "/auth/cli-token", map[string]string{
		"email": email, "password": password,
	}, &resp); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{JWT: resp.JWT, ExpiresAt: resp.ExpiresAt}, nil
}
```

**2. `backend-go/cmd/orca-cli/internal/apiclient/worktree.go`:**

```go
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
```

**3. `backend-go/cmd/orca-cli/internal/output/output.go`:**

```go
// Package output implements orca-cli's dual human/JSON reporting and
// BR-CLI-02/03's exit-code mapping.
package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

const (
	ExitOK          = 0
	ExitServerError = 1
	ExitUsageError  = 2
)

// Report prints result as JSON (jsonMode) or a human-readable summary, and
// returns the process exit code for main to use.
func Report(result any, warnings []string, jsonMode bool) int {
	if jsonMode {
		_ = json.NewEncoder(os.Stdout).Encode(result)
	} else {
		fmt.Printf("%+v\n", result)
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
	}
	return ExitOK
}

// ReportError prints err (JSON or human, matching Report's dual-mode
// contract) and returns the exit code BR-CLI-02/03's table specifies:
// a *apiclient.CLIError is a client-side usage error (exit 2) when its
// StatusCode is 400/422-shaped, else a server error (exit 1); any other
// error (network failure, etc.) is exit 1.
func ReportError(err error, jsonMode bool) int {
	code, message, exitCode := "UNKNOWN", err.Error(), ExitServerError
	if cliErr, ok := err.(*apiclient.CLIError); ok {
		code, message = cliErr.Code, cliErr.Message
		if cliErr.StatusCode == 400 {
			exitCode = ExitUsageError
		}
	}
	if jsonMode {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"error": map[string]string{"code": code, "message": message},
		})
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s\n", code, message)
	}
	return exitCode
}
```

**4. `backend-go/cmd/orca-cli/internal/command/worktree_create.go`:**

```go
package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

type WorktreeCreateOptions struct {
	ProjectID, RepoID, Name, Base, Agent, Prompt, IdempotencyKeyOverride string
}

// IdempotencyKey returns the user-supplied key, or
// sha256(project_id|repo_id|branch) per BR-CLI-01 when none was given.
func (o WorktreeCreateOptions) IdempotencyKey() string {
	if o.IdempotencyKeyOverride != "" {
		return o.IdempotencyKeyOverride
	}
	sum := sha256.Sum256([]byte(o.ProjectID + "|" + o.RepoID + "|" + o.Name))
	return hex.EncodeToString(sum[:])
}

type Result struct {
	WorktreeID string   `json:"worktreeId"`
	Path       string   `json:"path"`
	HeadSHA    string   `json:"headSha"`
	PtyID      string   `json:"ptyId,omitempty"`
	Warnings   []string `json:"warnings"`
}

// RunWorktreeCreate composes CreateWorktree -> (best-effort) SpawnAgent ->
// SendPrompt, per SOL-CLI-01's "CLI composes worktree-create -> agent-spawn
// -> prompt-inject itself, not api-gateway" rationale — worktree-create and
// agent-spawn are two different services with no shared transaction.
func RunWorktreeCreate(ctx context.Context, cli *apiclient.Client, opts WorktreeCreateOptions) (Result, error) {
	wt, err := cli.CreateWorktree(ctx, apiclient.CreateWorktreeInput{
		ProjectID: opts.ProjectID, RepoID: opts.RepoID, Branch: opts.Name, BaseRef: opts.Base,
		IdempotencyKey: opts.IdempotencyKey(),
	})
	if err != nil {
		return Result{}, err
	}
	result := Result{WorktreeID: wt.WorktreeID, Path: wt.Path, HeadSHA: wt.HeadSHA, Warnings: []string{}}
	if opts.Agent == "" {
		return result, nil
	}
	spawn, err := cli.SpawnAgent(ctx, wt.WorktreeID, opts.Agent)
	if err != nil {
		result.Warnings = append(result.Warnings, "AGENT_SPAWN_NOT_SUPPORTED: "+err.Error())
		return result, nil // worktree succeeded; exit 0 with a warning, not exit 1
	}
	result.PtyID = spawn.PtyID
	return result, nil
}
```

**5. `backend-go/cmd/orca-cli/internal/command/root.go`** — cobra command tree wiring `worktree create` to `RunWorktreeCreate`, reading `--project-id`/`--repo-id`/`--name`/`--base`/`--agent`/`--json` flags, resolving `apiclient.Client` from `config.Load()`, and calling `output.Report`/`output.ReportError` to set `os.Exit`'s code in `main.go`.

**6. `backend-go/cmd/orca-cli/main.go`** — composition root: `command.Execute()`, `os.Exit(exitCode)`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
go test ./cmd/orca-cli/... -v
```

Expected new tests: `worktree_create_test.go` — a fake `apiclient.Client` (interface-seamed or an httptest server) confirms: `--agent` failure produces exit `0` + populated `warnings`, never exit `1`; missing required flags produce exit `2` without any HTTP call; `--json` output is valid JSON matching `Result`'s shape byte-for-byte on a fixed fake response; `IdempotencyKey()` returns a stable sha256 hex string for the same three inputs across calls.
