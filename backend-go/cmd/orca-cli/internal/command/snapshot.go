package command

import (
	"context"
	"fmt"
	"os"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
)

// ExecuteSnapshot fetches --worktree's agent scrollback (BR-CLI-06's "flat
// scrollback file" contract) and either writes it byte-for-byte to
// outputPath (when non-empty) or prints it to stdout — never JSON-wrapped,
// even in --json mode, since the payload is inherently a flat text file,
// not a structured result.
func ExecuteSnapshot(ctx context.Context, clientFactory func() (*apiclient.Client, error), worktreeID, outputPath string, jsonMode bool) int {
	if worktreeID == "" {
		return output.ReportError(&apiclient.CLIError{
			StatusCode: 400, Code: "INVALID_ARGUMENT", Message: "--worktree is required",
		}, jsonMode)
	}

	cli, err := clientFactory()
	if err != nil {
		return output.ReportError(err, jsonMode)
	}

	text, truncated, err := cli.GetAgentSnapshot(ctx, worktreeID)
	if err != nil {
		return output.ReportError(err, jsonMode)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
			return output.ReportError(err, jsonMode)
		}
	} else {
		fmt.Print(text)
	}
	if truncated {
		fmt.Fprintln(os.Stderr, "warning: snapshot truncated (retention bound exceeded)")
	}
	return output.ExitOK
}
