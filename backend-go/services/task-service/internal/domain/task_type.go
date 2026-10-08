package domain

const (
	TypeTask    = "task"
	TypeBug     = "bug"
	TypeFeature = "feature"
	TypeEpic    = "epic"
	TypePlan    = "plan"
	TypePhase   = "phase"
)

func ParseTaskType(s string) (string, error) {
	if s == "" {
		return TypeTask, nil
	}
	switch s {
	case TypeTask, TypeBug, TypeFeature, TypeEpic, TypePlan, TypePhase:
		return s, nil
	default:
		return "", ErrInvalidTaskType
	}
}

// IsContainerType: only plan/phase are containers; epic stays a numbered work task.
func IsContainerType(s string) bool {
	return s == TypePlan || s == TypePhase
}
