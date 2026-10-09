package domain

// PlanLabelTestRegression marks the task that adds the regression test a bug or security fix needs. The other
// labels the execution policies read (gate:pre_deploy, rollback, check:*) are PolicyLabel* in type_policy.go.
const PlanLabelTestRegression = "test:regression"

const (
	maxLabelsPerTask = 10
	maxLabelRunes    = 64
)
