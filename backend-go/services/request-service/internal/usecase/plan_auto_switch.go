package usecase

import "context"

// PlanAutoSwitch answers whether Orca may start a Plan by itself: the deployment switch, a wired CommitPlan (a Plan
// nobody can approve is worthless) and the project's own switch must all be on.
type PlanAutoSwitch struct {
	EnvOn       bool
	CommitWired bool
	Settings    EngineSettingsRepository
}

// GloballyAvailable ignores the project; RetryPlanGeneration and the Plan read use it before looking at a project.
func (s PlanAutoSwitch) GloballyAvailable() bool { return s.EnvOn && s.CommitWired }

// Enabled never turns a feature that spends money on because a read failed: the caller decides whether to retry.
func (s PlanAutoSwitch) Enabled(ctx context.Context, projectID string) (bool, error) {
	if !s.GloballyAvailable() {
		return false, nil
	}
	if s.Settings == nil {
		return true, nil
	}
	cur, found, err := s.Settings.Get(ctx, projectID)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return cur.PlanAutoGenerate, nil
}
