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

// ExecuteAgentWait is `orca agent wait`'s testable core: it validates
// --worktree before ever building a client (no HTTP call on a bad
// invocation), then delegates to RunAgentWait and prints the result via
// output.Report — but only for the success/timeout outcomes RunAgentWait
// itself doesn't already print. A genuine RPC/transport error is
// distinguished from "timed out with exit 2" by result.TimedOut rather
// than by the numeric exit code alone, since RunAgentWait's own
// ReportError path can also return 2 (a 400-shaped CLIError) — reusing
// that same value for an unrelated reason.
func ExecuteAgentWait(ctx context.Context, clientFactory func() (*apiclient.Client, error), worktreeID string, timeout time.Duration, jsonMode bool) int {
	if worktreeID == "" {
		return output.ReportError(&apiclient.CLIError{
			StatusCode: 400, Code: "INVALID_ARGUMENT", Message: "--worktree is required",
		}, jsonMode)
	}

	cli, err := clientFactory()
	if err != nil {
		return output.ReportError(err, jsonMode)
	}

	result, exitCode := RunAgentWait(ctx, cli, worktreeID, timeout)
	if exitCode == output.ExitOK || result.TimedOut {
		output.Report(result, nil, jsonMode)
	}
	return exitCode
}
