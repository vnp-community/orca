package usecase

import "context"

// ExecutionContext carries a running execution's ProjectID and triggering
// user id down to whichever step executor needs them (ProviderResolver's
// user > project > server priority chain, ServerResolver's
// TargetKindProject resolution) — TASK-WF-002-03's resolution of the
// "Execution context plumbing" gap BE-SOL-002's sketch left implicit.
//
// Design decision (documented per that task's explicit instruction to pick
// one option and note it): neither of the two options that task's file
// sketched — widening domain.StepExecutor.Execute's signature, or having
// wave_dispatcher.go resolve provider/server itself before calling
// Execute — was used. Both would touch every step type (Condition/Webhook
// included, neither of which has any provider/server-resolution concept)
// or couple wave_dispatcher.go to agent-specific resolution logic it
// otherwise knows nothing about. Instead, ExecutionContext is threaded via
// context.Context — the same mechanism common/tenant already uses for
// per-request identity (tenant.RequireTenantID/tenant.UserID) — set once
// where an execution's dispatch begins (Execute.runToCompletion,
// ExecuteAdHocStep.Execute, RecoverExecutions.resumeToCompletion) and read
// by whichever adapter-layer executor cares (AgentExecutor); every other
// step type's Execute signature, and wave_dispatcher.go itself, is
// unchanged.
type ExecutionContext struct {
	ProjectID   string
	TriggeredBy string
}

type executionContextKey struct{}

// WithExecutionContext attaches ec to ctx.
func WithExecutionContext(ctx context.Context, ec ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey{}, ec)
}

// ExecutionContextFrom reads back the ExecutionContext WithExecutionContext
// attached — the zero value (both fields empty) if none was ever set. A
// zero-value ExecutionContext is not an error: ProviderResolver's chain and
// ServerResolver's TargetKindProject branch both treat an empty
// ProjectID/TriggeredBy as "no user/project scope," falling through to a
// narrower/server-scope answer, exactly like ExecuteAdHocStepInput's
// already-missing ProjectID does today.
func ExecutionContextFrom(ctx context.Context) ExecutionContext {
	ec, _ := ctx.Value(executionContextKey{}).(ExecutionContext)
	return ec
}
