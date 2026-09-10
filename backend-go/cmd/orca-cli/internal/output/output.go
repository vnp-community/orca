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
