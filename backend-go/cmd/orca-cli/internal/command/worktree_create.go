package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
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

// ExecuteWorktreeCreate is the testable core behind `orca worktree create`:
// it validates required flags BEFORE ever calling clientFactory (so an
// invalid invocation never opens an HTTP connection, let alone issues a
// request), then delegates to RunWorktreeCreate and reports the outcome via
// output.Report/output.ReportError. Kept separate from root.go's cobra
// wiring so tests can drive it directly without a real command-line parse.
func ExecuteWorktreeCreate(ctx context.Context, clientFactory func() (*apiclient.Client, error), opts WorktreeCreateOptions, jsonMode bool) int {
	if opts.ProjectID == "" || opts.RepoID == "" || opts.Name == "" {
		return output.ReportError(&apiclient.CLIError{
			StatusCode: 400,
			Code:       "INVALID_ARGUMENT",
			Message:    "--project-id, --repo-id, and --name are required",
		}, jsonMode)
	}

	cli, err := clientFactory()
	if err != nil {
		return output.ReportError(err, jsonMode)
	}

	result, err := RunWorktreeCreate(ctx, cli, opts)
	if err != nil {
		return output.ReportError(err, jsonMode)
	}
	return output.Report(result, result.Warnings, jsonMode)
}
