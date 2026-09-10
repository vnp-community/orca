package stepexecutors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// ActionHandler runs one named action's business logic against its
// paramsJSON, returning the step's terminal StepResult. Registered by name
// via ActionExecutor.Register — TASK-WF-02-07 wires the `action` StepType
// itself but registers no concrete handlers (no catalog exists yet, see
// domain.ActionStepConfig's doc comment).
type ActionHandler func(ctx context.Context, paramsJSON string) (domain.StepResult, error)

// ActionExecutor is the real Action step executor: dispatches
// domain.ActionStepConfig.ActionName to a registered ActionHandler.
// Unregistered names fail with the typed usecase.ErrNoActionHandlerRegistered
// sentinel — a clear, typed error rather than a silent no-op or a panic.
type ActionExecutor struct {
	handlers map[string]ActionHandler
}

func NewActionExecutor() *ActionExecutor {
	return &ActionExecutor{handlers: make(map[string]ActionHandler)}
}

// Register wires actionName to the ActionHandler that runs it. Called from
// cmd/server/main.go during composition, before the gRPC server starts
// accepting requests — not safe for concurrent use with Execute.
func (e *ActionExecutor) Register(actionName string, handler ActionHandler) {
	e.handlers[actionName] = handler
}

var _ domain.StepExecutor = (*ActionExecutor)(nil)

func (e *ActionExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.ActionStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: action: invalid step config JSON: %w", err)
	}

	handler, ok := e.handlers[cfg.ActionName]
	if !ok {
		return domain.StepResult{}, usecase.ErrNoActionHandlerRegistered
	}
	return handler(ctx, string(cfg.Params))
}
