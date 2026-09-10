package usecase

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

var templatePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// Interpolate substitutes {{path}} references in raw against inputs (an
// execution's ExecuteRequest.inputs) and outputs (already-completed steps'
// results, keyed by step id). Runs once per step, immediately before that
// step's executor is invoked — NOT once for the whole execution up front —
// since {{outputs.<stepId>.*}} is only resolvable once stepId has actually
// completed; interpolating early would either require a second pass per
// step anyway or force strict topological pre-validation that duplicates
// work BuildWaves already does.
func Interpolate(raw string, inputs map[string]any, outputs map[string]domain.StepResult) (string, error) {
	var firstErr error
	result := templatePattern.ReplaceAllStringFunc(raw, func(match string) string {
		if firstErr != nil {
			return match
		}
		path := templatePattern.FindStringSubmatch(match)[1]
		if rest, ok := strings.CutPrefix(path, "outputs."); ok {
			val, err := resolveStepOutput(outputs, rest)
			if err != nil {
				firstErr = err
				return match
			}
			return val
		}
		val, ok := inputs[path]
		if !ok {
			firstErr = fmt.Errorf("interpolate: unknown reference %q", path)
			return match
		}
		return fmt.Sprintf("%v", val)
	})
	if firstErr != nil {
		return "", firstErr
	}
	return result, nil
}

// resolveStepOutput errors if stepId isn't in outputs yet — a DAG-author
// error (referencing a step that hasn't run, or doesn't exist, in this
// DAG), not a silent empty string. path is "<stepId>.<field>[.<nested>...]"
// (everything after the "outputs." prefix Interpolate already cut) —
// field may itself contain further dots to walk into nested JSON, e.g.
// "outputs.stepX.data.branch" reaches OutputJSON's {"data":{"branch":...}}.
func resolveStepOutput(outputs map[string]domain.StepResult, path string) (string, error) {
	stepID, field, ok := strings.Cut(path, ".")
	if !ok {
		return "", fmt.Errorf("interpolate: outputs reference %q missing a field after the step id", path)
	}
	res, ok := outputs[stepID]
	if !ok {
		return "", fmt.Errorf("interpolate: step %q has not completed (or does not exist in this DAG)", stepID)
	}

	var parsed any
	if err := json.Unmarshal([]byte(res.OutputJSON), &parsed); err != nil {
		return "", fmt.Errorf("interpolate: step %q output is not valid JSON: %w", stepID, err)
	}

	current := parsed
	walked := stepID
	for _, segment := range strings.Split(field, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("interpolate: %q is not an object, cannot read field %q", walked, segment)
		}
		val, ok := obj[segment]
		if !ok {
			return "", fmt.Errorf("interpolate: step %q output has no field %q", stepID, field)
		}
		current = val
		walked += "." + segment
	}
	return fmt.Sprintf("%v", current), nil
}

// interpolateStepConfig walks rawConfig's JSON tree (an object, array, or
// scalar), interpolating every string-typed leaf via Interpolate and
// re-marshaling. A non-JSON or non-string-leaf-bearing config is returned
// unchanged rather than erroring — most step config fields are strings
// (Prompt, Script, Message, ConnectionID), but this stays generic rather
// than hardcoding each StepType's shape, matching domain.Step.Config's own
// "only the matching StepExecutor needs to parse it" convention.
func interpolateStepConfig(rawConfig string, inputs map[string]any, outputs map[string]domain.StepResult) (string, error) {
	if strings.TrimSpace(rawConfig) == "" {
		return rawConfig, nil
	}
	var tree any
	if err := json.Unmarshal([]byte(rawConfig), &tree); err != nil {
		return "", fmt.Errorf("interpolate: step config is not valid JSON: %w", err)
	}

	interpolated, err := interpolateValue(tree, inputs, outputs)
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(interpolated)
	if err != nil {
		return "", fmt.Errorf("interpolate: re-marshal step config: %w", err)
	}
	return string(out), nil
}

// interpolateValue recurses through v (as decoded by encoding/json: string,
// float64, bool, nil, []any, map[string]any) interpolating every string
// leaf.
func interpolateValue(v any, inputs map[string]any, outputs map[string]domain.StepResult) (any, error) {
	switch val := v.(type) {
	case string:
		return Interpolate(val, inputs, outputs)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			interpolatedChild, err := interpolateValue(child, inputs, outputs)
			if err != nil {
				return nil, err
			}
			out[k] = interpolatedChild
		}
		return out, nil
	case []any:
		out := make([]any, len(val))
		for i, child := range val {
			interpolatedChild, err := interpolateValue(child, inputs, outputs)
			if err != nil {
				return nil, err
			}
			out[i] = interpolatedChild
		}
		return out, nil
	default:
		return v, nil
	}
}
