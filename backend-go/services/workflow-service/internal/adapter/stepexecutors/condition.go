package stepexecutors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// conditionStepConfig is the Condition step type's config shape: a boolean
// expression (domain.EvaluateCondition's grammar) plus the flat key-value
// context to evaluate it against — normally the accumulated outputs of
// earlier steps in a real DAG execution (workflow-service.md §4); for
// ExecuteAdHocStep it's whatever the caller supplies directly.
type conditionStepConfig struct {
	Expression string            `json:"expression"`
	Context    map[string]string `json:"context"`
}

type conditionResultOutput struct {
	Result bool   `json:"result"`
	Error  string `json:"error,omitempty"`
}

// ConditionExecutor is the real, in-process implementation of the
// Condition step type — wraps domain.EvaluateCondition, no I/O
// (workflow-service.md §4: "pure function over accumulated step outputs").
type ConditionExecutor struct{}

func NewConditionExecutor() *ConditionExecutor {
	return &ConditionExecutor{}
}

// Execute parses stepConfigJSON and evaluates its expression. Per §9's
// "fail-safe-false on unparseable input" and domain.EvaluateCondition's
// doc, a malformed expression is reported as a *failed* StepResult (status
// + the parse error's message in output_json), NOT as a returned Go error
// — the executor ran successfully, the condition it was asked to evaluate
// was simply invalid, which is a business-level outcome, not an executor
// malfunction. Malformed *config JSON* (not even valid JSON), by contrast,
// is a genuine executor-level error.
func (e *ConditionExecutor) Execute(_ context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg conditionStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: condition: invalid step config JSON: %w", err)
	}

	result, err := domain.EvaluateCondition(cfg.Expression, cfg.Context)
	if err != nil {
		output, _ := json.Marshal(conditionResultOutput{Result: false, Error: err.Error()})
		return domain.StepResult{Status: domain.ResultStatusFailed, OutputJSON: string(output)}, nil
	}

	output, err := json.Marshal(conditionResultOutput{Result: result})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: condition: marshal output: %w", err)
	}
	return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: string(output)}, nil
}
