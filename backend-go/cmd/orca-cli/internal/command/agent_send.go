package command

import (
	"context"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
)

// ExecuteAgentSend is `orca agent send`'s testable core — text is already
// resolved (root.go's cobra wiring reads it from --text or falls back to
// stdin before calling this), matching worktree_create.go's
// ExecuteWorktreeCreate shape: validate before ever building a client,
// then call and report.
func ExecuteAgentSend(ctx context.Context, clientFactory func() (*apiclient.Client, error), worktreeID, text string, jsonMode bool) int {
	if worktreeID == "" || text == "" {
		return output.ReportError(&apiclient.CLIError{
			StatusCode: 400, Code: "INVALID_ARGUMENT", Message: "--worktree and --text (or stdin) are required",
		}, jsonMode)
	}

	cli, err := clientFactory()
	if err != nil {
		return output.ReportError(err, jsonMode)
	}

	if err := cli.SendAgentInput(ctx, worktreeID, text); err != nil {
		return output.ReportError(err, jsonMode)
	}
	return output.Report(map[string]bool{"sent": true}, nil, jsonMode)
}
