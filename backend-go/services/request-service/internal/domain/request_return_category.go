package domain

type ReturnCategory string

const (
	ReturnCategoryMissingInfo       ReturnCategory = "missing_info"
	ReturnCategoryInfeasible        ReturnCategory = "infeasible"
	ReturnCategoryBlockedDependency ReturnCategory = "blocked_dependency"
	ReturnCategoryRejected          ReturnCategory = "rejected"
	ReturnCategoryOther             ReturnCategory = "other"
)

func AllReturnCategories() []ReturnCategory {
	return []ReturnCategory{ReturnCategoryMissingInfo, ReturnCategoryInfeasible, ReturnCategoryBlockedDependency, ReturnCategoryRejected, ReturnCategoryOther}
}

func ParseReturnCategory(s string) (ReturnCategory, error) {
	for _, c := range AllReturnCategories() {
		if string(c) == s {
			return c, nil
		}
	}
	return "", ErrReturnCategoryInvalid(s)
}

func AllReturnStages() []ReturnStage {
	return []ReturnStage{ReturnStageClassification, ReturnStageAnalysis, ReturnStagePlan, ReturnStagePhase, ReturnStageTask}
}

func ParseReturnStage(s string) (ReturnStage, error) {
	for _, st := range AllReturnStages() {
		if string(st) == s {
			return st, nil
		}
	}
	return "", ErrReturnStageInvalid(s)
}
