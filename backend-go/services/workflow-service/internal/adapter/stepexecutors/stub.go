package stepexecutors

import (
	"context"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ErrNotImplemented is the sentinel every stub StepExecutor wraps. See
// workflow-service.md §2: agent/shell/notification step *execution* is
// dispatched here but actually executed on the Dev Server Agent's execution
// plane, reached only through infra-fleet-service's relay client — a real
// dependency this scaffold cannot stand up (infra-fleet-service is itself a
// stub-based dependency in this scaffolding effort, per this build's
// instructions).
var ErrNotImplemented = errors.New("adapter/stepexecutors: not implemented — relays to infra-fleet-service execution plane, see workflow-service.md")

// stubExecutor is the shared shape of the three execution-plane-backed step
// types this scaffold cannot implement for real yet.
type stubExecutor struct {
	stepType domain.StepType
}

func (s stubExecutor) Execute(_ context.Context, _ string) (domain.StepResult, error) {
	return domain.StepResult{}, fmt.Errorf("%s step: %w", s.stepType, ErrNotImplemented)
}

// NewAgentStub returns the AgentStepExecutor stub. A real implementation
// must be built from infra-fleet-service's actual relay contract at
// implementation time — explicitly NOT a port of TS's
// StepExecutors.executeAgent(), which sends a request shape the execution
// plane doesn't accept (workflow-service.md §3.2, "TS Gap 4" — a live P0 bug
// in TS, not a design to inherit).
func NewAgentStub() domain.StepExecutor {
	return stubExecutor{stepType: domain.StepTypeAgent}
}

// NewShellStub returns the ShellStepExecutor stub — same infra-fleet-service
// relay dependency as Agent, different RPC/params once implemented.
func NewShellStub() domain.StepExecutor {
	return stubExecutor{stepType: domain.StepTypeShell}
}

// NewNotificationStub returns the NotificationStepExecutor stub — matches
// TS's notification.send, relayed through infra-fleet-service once
// implemented.
func NewNotificationStub() domain.StepExecutor {
	return stubExecutor{stepType: domain.StepTypeNotification}
}
