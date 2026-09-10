package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ExecutionContext carries everything a step's Config's {{...}} tokens can
// reference — built once per Execute call (from ExecuteRequest.inputs_json
// + the execution's ProjectID/triggering user), threaded through every
// wave. BUG-WF-02 found no variable interpolation at all: step configs
// could not reference ExecuteRequest.inputs_json values or earlier steps'
// outputs.
type ExecutionContext struct {
	Inputs    map[string]any            // from ExecuteRequest.inputs_json
	Outputs   map[string]map[string]any // stepId -> parsed OutputJSON, accumulated wave-by-wave
	ProjectID string
	UserID    string // triggering user
}

// interpolationToken matches a {{...}} token, capturing its trimmed inner
// expression.
var interpolationToken = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// Interpolate replaces every {{...}} token in configJSON with its resolved
// value, leaving unresolvable tokens untouched (visible, not silently
// dropped) rather than failing the whole step — a step referencing a typo'd
// or not-yet-available token should surface that in its own output/error,
// not abort the wave.
func Interpolate(configJSON string, execCtx ExecutionContext) (string, error) {
	result := interpolationToken.ReplaceAllStringFunc(configJSON, func(match string) string {
		sub := interpolationToken.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		expr := strings.TrimSpace(sub[1])
		val, ok := resolveToken(expr, execCtx)
		if !ok {
			return match
		}
		return jsonEscapeAndQuoteIfNeeded(val)
	})
	return result, nil
}

// resolveToken resolves one {{...}} expression against execCtx — see
// Interpolate's doc comment for the four token kinds this understands.
func resolveToken(expr string, execCtx ExecutionContext) (any, bool) {
	switch {
	case expr == "now()":
		return time.Now().UTC().Format(time.RFC3339), true
	case expr == "project.id":
		return execCtx.ProjectID, true
	case expr == "user.id":
		return execCtx.UserID, true
	case strings.HasPrefix(expr, "outputs."):
		parts := strings.SplitN(strings.TrimPrefix(expr, "outputs."), ".", 2)
		if len(parts) != 2 {
			return nil, false
		}
		stepOut, ok := execCtx.Outputs[parts[0]]
		if !ok {
			return nil, false
		}
		return digPath(stepOut, parts[1])
	default:
		val, ok := execCtx.Inputs[expr]
		return val, ok
	}
}

// digPath walks a dot-separated path (e.g. "result.count") through nested
// map[string]any values — the shape encoding/json.Unmarshal produces for an
// arbitrary JSON object decoded into map[string]any, which is how a step's
// OutputJSON is parsed before being stored in ExecutionContext.Outputs.
func digPath(m map[string]any, path string) (any, bool) {
	var cur any = m
	for _, part := range strings.Split(path, ".") {
		asMap, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = asMap[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// jsonEscapeAndQuoteIfNeeded renders val for substitution into configJSON
// at a {{...}} token's exact position. The overwhelmingly common case is a
// token embedded inside an already-quoted JSON string (e.g.
// "prompt": "do {{feature_description}} now") — for that shape, a string
// value must be escaped (quotes/backslashes/control characters) but NOT
// re-wrapped in its own quotes, since the surrounding quotes are already
// there in configJSON. A non-string value (from outputs.*, typically a
// number/bool/object) is rendered as its own raw JSON — correct both when
// it fills a whole field's value standalone (e.g. "count": {{outputs.a.n}})
// and, for scalar numbers/bools, when embedded mid-string.
func jsonEscapeAndQuoteIfNeeded(val any) string {
	if s, ok := val.(string); ok {
		b, err := json.Marshal(s)
		if err != nil {
			return s
		}
		// json.Marshal of a string always produces a leading/trailing
		// '"' — strip them, since the token's own surrounding quotes (if
		// any) are already present in configJSON.
		return string(b[1 : len(b)-1])
	}
	b, err := json.Marshal(val)
	if err != nil {
		return fmt.Sprintf("%v", val)
	}
	return string(b)
}
