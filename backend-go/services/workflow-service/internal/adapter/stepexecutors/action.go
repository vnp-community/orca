package stepexecutors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ActionHandler executes one named action against its already-wired
// downstream client (e.g. git-gateway-service for git.*, issue-tracking-service
// for github.*/jira.*).
type ActionHandler func(ctx context.Context, params map[string]any) (domain.StepResult, error)

// ActionExecutor dispatches ActionStepConfig.Action to a registered
// handler. The set of supported action names is intentionally small and
// explicit — see NewActionExecutor's callers — NOT a generic "call any RPC
// by string name" mechanism, to keep the attack surface (and the product
// surface) bounded.
type ActionExecutor struct {
	handlers map[string]ActionHandler
}

func NewActionExecutor(handlers map[string]ActionHandler) *ActionExecutor {
	return &ActionExecutor{handlers: handlers}
}

var _ domain.StepExecutor = (*ActionExecutor)(nil)

func (e *ActionExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.ActionStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: action: invalid step config JSON: %w", err)
	}
	handler, ok := e.handlers[cfg.Action]
	if !ok {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: action: unregistered action %q", cfg.Action)
	}
	return handler(ctx, cfg.Params)
}
