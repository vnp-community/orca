package command

import (
	"context"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
)

// ExecuteAgentStatus is `orca agent status`'s testable core, matching
// worktree_create.go's ExecuteWorktreeCreate shape: validate --worktree
// before ever building a client, then call and report.
func ExecuteAgentStatus(ctx context.Context, clientFactory func() (*apiclient.Client, error), worktreeID string, jsonMode bool) int {
	if worktreeID == "" {
		return output.ReportError(&apiclient.CLIError{
			StatusCode: 400, Code: "INVALID_ARGUMENT", Message: "--worktree is required",
		}, jsonMode)
	}

	cli, err := clientFactory()
	if err != nil {
		return output.ReportError(err, jsonMode)
	}

	result, err := cli.GetAgentStatus(ctx, worktreeID)
	if err != nil {
		return output.ReportError(err, jsonMode)
	}
	return output.Report(result, nil, jsonMode)
}
