package usecase

import "context"

// StepIdentity names the workflow step a StepExecutor is running for. Executors
// otherwise only see their config JSON; a step that drives an external job
// (tdd_agent.*) needs it to find that job again after a restart.
type StepIdentity struct {
	ExecutionID string
	StepID      string
}

type stepIdentityKey struct{}

// WithStepIdentity is set by the wave dispatcher before each step runs.
func WithStepIdentity(ctx context.Context, id StepIdentity) context.Context {
	return context.WithValue(ctx, stepIdentityKey{}, id)
}

// StepIdentityFrom returns the running step's identity; ok is false outside a
// wave dispatch (e.g. ExecuteAdHocStep).
func StepIdentityFrom(ctx context.Context) (StepIdentity, bool) {
	id, ok := ctx.Value(stepIdentityKey{}).(StepIdentity)
	return id, ok && id.ExecutionID != ""
}
