package stepexecutors

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// ParallelExecutor is the real Parallel step executor: fans
// domain.ParallelStepConfig.SubSteps out concurrently and aggregates their
// results (Promise.allSettled semantics — every sub-step runs to
// completion regardless of a sibling's failure, honoring
// AllowPartialFailure for the AGGREGATE outcome only).
//
// ParallelExecutor needs the SAME StepExecutorRegistry the wave dispatcher
// uses, to recursively resolve each sub-step's own executor — injected at
// construction via SetRegistry (main.go wires it after the registry itself
// is built, a two-phase init since ParallelExecutor IS one of the
// registry's own entries: Registry -> registers ParallelExecutor ->
// ParallelExecutor needs a reference back to that same Registry).
type ParallelExecutor struct {
	registry usecase.StepExecutorRegistry
}

func NewParallelExecutor() *ParallelExecutor {
	return &ParallelExecutor{}
}

// SetRegistry wires the StepExecutorRegistry ParallelExecutor recursively
// resolves each sub-step's executor against — see this type's doc comment
// for the two-phase init this requires. Called once from cmd/server/main.go,
// before the gRPC server starts accepting requests.
func (e *ParallelExecutor) SetRegistry(registry usecase.StepExecutorRegistry) {
	e.registry = registry
}

var _ domain.StepExecutor = (*ParallelExecutor)(nil)

func (e *ParallelExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.ParallelStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: parallel: invalid step config JSON: %w", err)
	}

	results := make([]domain.StepResult, len(cfg.SubSteps))
	errs := make([]error, len(cfg.SubSteps))
	var wg sync.WaitGroup
	for i, sub := range cfg.SubSteps {
		wg.Add(1)
		go func(i int, sub domain.Step) {
			defer wg.Done()
			executor, err := e.registry.Resolve(sub.Type)
			if err != nil {
				errs[i] = err
				return
			}
			results[i], errs[i] = executor.Execute(ctx, string(sub.Config)) // allSettled — every sub-step runs regardless of siblings
		}(i, sub)
	}
	wg.Wait()

	anyFailed := false
	agg := make(map[string]any, len(cfg.SubSteps))
	for i, sub := range cfg.SubSteps {
		if errs[i] != nil || results[i].Status == domain.ResultStatusFailed {
			anyFailed = true
		}
		agg[sub.ID] = subStepOutcome{Status: results[i].Status, OutputJSON: results[i].OutputJSON, Error: errString(errs[i])}
	}

	outputJSON, err := json.Marshal(agg)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: parallel: marshal aggregate output: %w", err)
	}
	if anyFailed && !cfg.AllowPartialFailure {
		return domain.StepResult{Status: domain.ResultStatusFailed, OutputJSON: string(outputJSON)}, fmt.Errorf("stepexecutors: parallel: one or more sub-steps failed")
	}
	return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: string(outputJSON)}, nil
}

// subStepOutcome is one sub-step's entry in the aggregate OutputJSON —
// carries enough of its StepResult (plus a hard executor error, if any) for
// a caller/{{outputs.parallelStepId.subStepId...}} interpolation to inspect
// what happened, not just whether the aggregate as a whole succeeded.
type subStepOutcome struct {
	Status     domain.ResultStatus `json:"status"`
	OutputJSON string              `json:"outputJson,omitempty"`
	Error      string              `json:"error,omitempty"`
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
