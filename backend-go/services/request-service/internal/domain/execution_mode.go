package domain

import (
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ExecutionMode says how a Request runs once its plan is approved.
//
// step_by_step is the original behaviour: a person releases every Phase (and, for change_request, approves it first).
// continuous lets Orca start the next Phase itself, so one approval of the plan covers the whole run.
type ExecutionMode string

const (
	ExecutionStepByStep ExecutionMode = "step_by_step"
	ExecutionContinuous ExecutionMode = "continuous"
)

// ParseExecutionMode reads a stored or submitted value. Empty means the default, step_by_step, so rows written
// before the column existed keep the behaviour they had.
func ParseExecutionMode(s string) (ExecutionMode, error) {
	switch ExecutionMode(strings.ToLower(strings.TrimSpace(s))) {
	case "", ExecutionStepByStep:
		return ExecutionStepByStep, nil
	case ExecutionContinuous:
		return ExecutionContinuous, nil
	}
	return "", ErrExecutionModeInvalid(s)
}

func ErrExecutionModeInvalid(v string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_EXECUTION_MODE_INVALID",
		"execution mode must be step_by_step or continuous, got "+v, nil)
}

// OrDefault turns the empty value (not pinned yet) into the default.
func (m ExecutionMode) OrDefault() ExecutionMode {
	if m == "" {
		return ExecutionStepByStep
	}
	return m
}

// EffectiveExecutionModeOf picks the mode a Request runs with: the one pinned when it entered executing, else the
// project setting, else the deployment default.
func EffectiveExecutionModeOf(pinned ExecutionMode, settings *ProjectEngineSettings, defaults EngineDefaults) ExecutionMode {
	if pinned != "" {
		return pinned
	}
	if settings != nil && settings.ExecutionMode != "" {
		return settings.ExecutionMode
	}
	return defaults.ExecutionMode.OrDefault()
}

// EffectiveExecutionGates is ExecutionGates for a run in mode m. Continuous drops the Phase gate: the single approval
// of the plan covers every Phase. Gates that need a human decision for another reason (pre_deploy) always stay.
func (f FlowDefinition) EffectiveExecutionGates(m ExecutionMode) []GateSubject {
	out := make([]GateSubject, 0, len(f.ExecutionGates))
	for _, g := range f.ExecutionGates {
		if g == GatePhase && m == ExecutionContinuous {
			continue
		}
		out = append(out, g)
	}
	return out
}

// HasExecutionGateFor reports whether g blocks inside executing for a run in mode m.
func (f FlowDefinition) HasExecutionGateFor(g GateSubject, m ExecutionMode) bool {
	for _, x := range f.EffectiveExecutionGates(m) {
		if x == g {
			return true
		}
	}
	return false
}

// StartsPhasesItself is true when Orca, not a person, releases the Phases of this flow.
func (f FlowDefinition) StartsPhasesItself(m ExecutionMode, size RequestSize) bool {
	return m == ExecutionContinuous && f.PhasesFor(size)
}
