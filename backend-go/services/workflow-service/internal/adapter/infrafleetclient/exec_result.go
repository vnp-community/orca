package infrafleetclient

import (
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// execResult is the best-effort shape this package assumes agent.exec/
// shell.exec relay results decode to: exitCode 0 (or absent) = success,
// mirroring TS's StepExecutors.ts own `result.exitCode ?? 0` convention
// (backend/src/main/workflow/StepExecutors.ts). Unverified against a live
// Dev Server Agent — see AgentExecutor's/ShellExecutor's doc comments.
type execResult struct {
	ExitCode *int   `json:"exitCode,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

// execResultOutput is what toStepResult marshals into StepResult.OutputJSON.
type execResultOutput struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

// toStepResult maps a decoded execResult to a domain.StepResult: a non-zero
// exitCode or a non-empty error field is a business-level "failed" outcome
// (the relay call itself succeeded — the remote command just didn't), same
// distinction WebhookExecutor draws for non-2xx responses.
func toStepResult(result execResult) (domain.StepResult, error) {
	status := domain.ResultStatusCompleted
	if result.Error != "" || (result.ExitCode != nil && *result.ExitCode != 0) {
		status = domain.ResultStatusFailed
	}
	exitCode := 0
	if result.ExitCode != nil {
		exitCode = *result.ExitCode
	}

	output, err := json.Marshal(execResultOutput{
		ExitCode: exitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		Error:    result.Error,
	})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: marshal output: %w", err)
	}
	return domain.StepResult{Status: status, OutputJSON: string(output)}, nil
}
