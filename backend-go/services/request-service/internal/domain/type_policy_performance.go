package domain

import (
	"context"
	"fmt"
	"math"
	"strconv"
)

// improvementTolerance absorbs float rounding when comparing against target_change_percent.
const improvementTolerance = 1e-9

type performancePolicy struct {
	checks CheckReader
}

func (p performancePolicy) PlanPreconditions(ctx context.Context, req Request, plan PlanProposal) error {
	if p.checks == nil {
		return ErrPerfBaselineMissing()
	}
	baseline, ok, err := p.checks.LatestCheck(ctx, req.ID, CheckPerfBaseline)
	if err != nil {
		return err
	}
	if !ok || baseline.Status != CheckStatusPassed {
		return ErrPerfBaselineMissing()
	}
	tasks := flattenProposalTasks(plan)
	if len(tasks) < 2 || !hasEffectiveLabel(tasks[0], PolicyLabelCheckBaseline) || !hasEffectiveLabel(tasks[len(tasks)-1], PolicyLabelCheckAfter) {
		return ErrPlanPerfCheckTasksMissing()
	}
	return nil
}

func (performancePolicy) PreExecutionGate(context.Context, Request, TaskRef) (*GateRequirement, error) {
	return nil, nil
}

// CompletionChecks recomputes every verdict from the numbers; the status an agent attached to the record is ignored.
func (performancePolicy) CompletionChecks(_ Request, checks []RequestCheck) ([]CheckVerdict, error) {
	baselineRow, hasBaseline := LatestCheck(checks, CheckPerfBaseline)
	if !hasBaseline || baselineRow.Status != CheckStatusPassed {
		return []CheckVerdict{{Kind: CheckPerfBaseline, Stage: "task", Status: CheckVerdictMissing, Summary: "Thiếu perf_baseline"}}, nil
	}
	baseline, err := DecodePerfBaseline(baselineRow.Metrics)
	if err != nil {
		return nil, fmt.Errorf("decode perf_baseline: %w", err)
	}
	afterRow, hasAfter := LatestCheck(checks, CheckPerfAfter)
	if !hasAfter {
		return []CheckVerdict{{Kind: CheckPerfAfter, Stage: "task", Status: CheckVerdictMissing, Summary: "Chưa đo lại (perf_after)"}}, nil
	}
	after, err := DecodePerfAfter(afterRow.Metrics)
	if err != nil {
		return nil, fmt.Errorf("decode perf_after: %w", err)
	}
	measured := make(map[string]float64, len(after.Metrics))
	for _, m := range after.Metrics {
		measured[m.Name] = m.Value
	}
	var out []CheckVerdict
	for _, m := range baseline.Metrics {
		v := CheckVerdict{Kind: CheckPerfAfter, Stage: "task"}
		value, ok := measured[m.Name]
		if !ok {
			v.Status, v.Summary = CheckVerdictMissing, "Thiếu số đo lại của "+m.Name
			out = append(out, v)
			continue
		}
		change, err := ImprovementPercent(m.Baseline, value, m.Direction)
		switch {
		case err != nil:
			v.Status, v.Summary = CheckVerdictFailed, "baseline bằng 0 hoặc hướng không hợp lệ: "+m.Name
		case change+improvementTolerance >= m.TargetChangePercent:
			v.Status, v.Summary = CheckVerdictPassed, fmt.Sprintf("%s cải thiện %s%%", m.Name, percent(change))
		default:
			v.Status, v.Summary = CheckVerdictFailed, fmt.Sprintf("Hiệu năng chưa đạt: %s %s%% < %s%%", m.Name, percent(change), percent(m.TargetChangePercent))
		}
		out = append(out, v)
	}
	return out, nil
}

func (performancePolicy) OnCompleted(Request) ([]FollowUp, error) { return nil, nil }

func percent(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}
