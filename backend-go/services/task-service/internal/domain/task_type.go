package domain

import "errors"

const (
	TypeTask    = "task"
	TypeBug     = "bug"
	TypeFeature = "feature"
	TypeEpic    = "epic"
	TypePlan    = "plan"
	TypePhase   = "phase"
)

var ErrInvalidTaskType = errors.New("invalid task type")

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

func IsContainerType(s string) bool {
	return s == TypePlan || s == TypePhase || s == TypeEpic
}
