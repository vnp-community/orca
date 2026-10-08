package domain

import "context"

// securityPolicy: the Plan is approved through pre_deploy (StartGate); completion needs a passed re-check.
type securityPolicy struct{}

func (securityPolicy) PlanPreconditions(context.Context, Request, PlanProposal) error { return nil }

func (securityPolicy) PreExecutionGate(context.Context, Request, TaskRef) (*GateRequirement, error) {
	return nil, nil
}

func (securityPolicy) CompletionChecks(_ Request, checks []RequestCheck) ([]CheckVerdict, error) {
	return []CheckVerdict{humanConfirmedVerdict(checks, CheckSecurityRecheck, "security re-check")}, nil
}

func (securityPolicy) OnCompleted(Request) ([]FollowUp, error) { return nil, nil }

// humanConfirmedVerdict judges a pass/fail check that carries no numbers Orca could recompute. An agent's own
// "passed" is not accepted: a person (or an Orca-verified result) must stand behind it. A "failed" is always honoured.
func humanConfirmedVerdict(checks []RequestCheck, kind CheckKind, label string) CheckVerdict {
	latest, ok := LatestCheck(checks, kind)
	v := CheckVerdict{Kind: kind, Stage: "task"}
	switch {
	case !ok:
		v.Status, v.Summary = CheckVerdictMissing, "Chưa có kết quả "+label+" (kind "+string(kind)+")"
	case latest.Status == CheckStatusFailed:
		v.Status, v.Summary = CheckVerdictFailed, "Kết quả "+label+" không đạt: "+latest.Summary
	case latest.Source == CheckSourceAgent:
		v.Status, v.Summary = CheckVerdictMissing, "Kết quả "+label+" do agent khai, cần người xác nhận bằng tài khoản người dùng"
	default:
		v.Status, v.Summary = CheckVerdictPassed, label+" đạt"
	}
	return v
}
