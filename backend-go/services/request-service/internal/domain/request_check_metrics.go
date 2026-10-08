package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MaxCheckMetricsBytes bounds metrics_json at the RPC, not the database.
const MaxCheckMetricsBytes = 64 * 1024

// ValidateCheckSubmission checks a RecordRequestCheck payload against the schema of its kind.
func ValidateCheckSubmission(kind CheckKind, status CheckStatus, metrics json.RawMessage, summary string) error {
	if !kind.Valid() {
		return ErrCheckInvalidMetrics(fmt.Sprintf("unknown check kind %q", kind))
	}
	if status != CheckStatusPassed && status != CheckStatusFailed {
		return ErrCheckInvalidMetrics(fmt.Sprintf("unknown check status %q", status))
	}
	if len(metrics) > MaxCheckMetricsBytes {
		return ErrCheckInvalidMetrics(fmt.Sprintf("metrics_json exceeds %d bytes", MaxCheckMetricsBytes))
	}
	if len(metrics) == 0 {
		metrics = json.RawMessage(`{}`)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(metrics, &obj); err != nil || obj == nil {
		return ErrCheckInvalidMetrics("metrics_json must be a JSON object")
	}
	switch kind {
	case CheckPerfBaseline:
		return validatePerfBaseline(metrics)
	case CheckPerfAfter:
		return validatePerfAfter(metrics)
	case CheckTestsBefore, CheckTestsAfter:
		return validateTestCounts(kind, metrics)
	default: // security_recheck, ops_result: free-form object, the summary is what a human reads
		if strings.TrimSpace(summary) == "" {
			return ErrCheckInvalidMetrics(string(kind) + " needs a non-empty summary")
		}
	}
	return nil
}

func validatePerfBaseline(raw json.RawMessage) error {
	var p struct {
		Metrics []struct {
			Name                *string  `json:"name"`
			Unit                *string  `json:"unit"`
			Direction           *string  `json:"direction"`
			Baseline            *float64 `json:"baseline"`
			TargetChangePercent *float64 `json:"target_change_percent"`
		} `json:"metrics"`
		Method *string `json:"method"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ErrCheckInvalidMetrics("perf_baseline: " + err.Error())
	}
	if len(p.Metrics) == 0 {
		return ErrCheckInvalidMetrics("perf_baseline: metrics must list at least one entry")
	}
	seen := map[string]bool{}
	for i, m := range p.Metrics {
		switch {
		case m.Name == nil || strings.TrimSpace(*m.Name) == "":
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_baseline: metrics[%d].name is required", i))
		case seen[*m.Name]:
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_baseline: duplicate metric %q", *m.Name))
		case m.Unit == nil:
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_baseline: metrics[%d].unit is required", i))
		case m.Direction == nil || !Direction(*m.Direction).Valid():
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_baseline: metrics[%d].direction must be lower_is_better or higher_is_better", i))
		case m.Baseline == nil:
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_baseline: metrics[%d].baseline is required", i))
		case m.TargetChangePercent == nil:
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_baseline: metrics[%d].target_change_percent is required", i))
		}
		seen[*m.Name] = true
	}
	if p.Method == nil {
		return ErrCheckInvalidMetrics("perf_baseline: method is required")
	}
	return nil
}

func validatePerfAfter(raw json.RawMessage) error {
	var p struct {
		Metrics []struct {
			Name  *string  `json:"name"`
			Value *float64 `json:"value"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ErrCheckInvalidMetrics("perf_after: " + err.Error())
	}
	if len(p.Metrics) == 0 {
		return ErrCheckInvalidMetrics("perf_after: metrics must list at least one entry")
	}
	for i, m := range p.Metrics {
		if m.Name == nil || strings.TrimSpace(*m.Name) == "" {
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_after: metrics[%d].name is required", i))
		}
		if m.Value == nil {
			return ErrCheckInvalidMetrics(fmt.Sprintf("perf_after: metrics[%d].value is required", i))
		}
	}
	return nil
}

func validateTestCounts(kind CheckKind, raw json.RawMessage) error {
	var p struct {
		Total         *int    `json:"total"`
		Passed        *int    `json:"passed"`
		Failed        *int    `json:"failed"`
		Command       *string `json:"command"`
		TestsModified *bool   `json:"tests_modified"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ErrCheckInvalidMetrics(string(kind) + ": " + err.Error())
	}
	for name, v := range map[string]*int{"total": p.Total, "passed": p.Passed, "failed": p.Failed} {
		if v == nil || *v < 0 {
			return ErrCheckInvalidMetrics(fmt.Sprintf("%s: %s must be a non-negative integer", kind, name))
		}
	}
	if p.Command == nil {
		return ErrCheckInvalidMetrics(string(kind) + ": command is required")
	}
	if kind == CheckTestsAfter && p.TestsModified == nil {
		return ErrCheckInvalidMetrics("tests_after: tests_modified is required")
	}
	return nil
}
