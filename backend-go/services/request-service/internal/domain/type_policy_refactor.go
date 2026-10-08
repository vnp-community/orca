package domain

import (
	"context"
	"fmt"
	"strings"
)

type refactorPolicy struct{}

func (refactorPolicy) PlanPreconditions(_ context.Context, _ Request, plan PlanProposal) error {
	tasks := flattenProposalTasks(plan)
	if len(tasks) < 2 || !hasEffectiveLabel(tasks[0], PolicyLabelCheckTestsPre) || !hasEffectiveLabel(tasks[len(tasks)-1], PolicyLabelCheckTestsAfter) {
		return ErrPlanTestCheckTasksMissing()
	}
	return nil
}

func (refactorPolicy) PreExecutionGate(context.Context, Request, TaskRef) (*GateRequirement, error) {
	return nil, nil
}

// CompletionChecks needs the test run before and after. tests_modified is the agent's own word: Orca runs no git
// command to check it, so the summary says so.
func (refactorPolicy) CompletionChecks(_ Request, checks []RequestCheck) ([]CheckVerdict, error) {
	beforeRow, hasBefore := LatestCheck(checks, CheckTestsBefore)
	afterRow, hasAfter := LatestCheck(checks, CheckTestsAfter)
	var out []CheckVerdict
	if !hasBefore {
		out = append(out, CheckVerdict{Kind: CheckTestsBefore, Stage: "task", Status: CheckVerdictMissing, Summary: "Thiếu kết quả test trước refactor (tests_before)"})
	}
	if !hasAfter {
		out = append(out, CheckVerdict{Kind: CheckTestsAfter, Stage: "task", Status: CheckVerdictMissing, Summary: "Thiếu kết quả test sau refactor (tests_after)"})
	}
	if len(out) > 0 {
		return out, nil
	}
	before, err := DecodeTestCounts(beforeRow.Metrics)
	if err != nil {
		return nil, fmt.Errorf("decode tests_before: %w", err)
	}
	after, err := DecodeTestCounts(afterRow.Metrics)
	if err != nil {
		return nil, fmt.Errorf("decode tests_after: %w", err)
	}
	var violations []string
	if after.Failed > 0 {
		violations = append(violations, fmt.Sprintf("%d test còn đỏ", after.Failed))
	}
	if after.Total < before.Total {
		violations = append(violations, fmt.Sprintf("số test giảm từ %d xuống %d", before.Total, after.Total))
	}
	if after.TestsModified {
		violations = append(violations, "test bị sửa trong lúc refactor")
	}
	v := CheckVerdict{Kind: CheckTestsAfter, Stage: "task"}
	if len(violations) == 0 {
		v.Status, v.Summary = CheckVerdictPassed, "Test cũ vẫn xanh (tests_modified do agent khai, chưa được xác minh)"
	} else {
		v.Status = CheckVerdictFailed
		v.Summary = "Refactor chưa đạt: " + strings.Join(violations, "; ") + ". tests_modified do agent khai, chưa được xác minh"
	}
	return []CheckVerdict{v}, nil
}

func (refactorPolicy) OnCompleted(Request) ([]FollowUp, error) { return nil, nil }
