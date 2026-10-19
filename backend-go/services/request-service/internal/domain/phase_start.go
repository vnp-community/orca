package domain

import "time"

// PhaseStart is the idempotency record of "start this Phase".
type PhaseStart struct {
	TenantID    string
	PhaseTaskID string
	RequestID   string
	StartedBy   string
	// Trigger is "auto" for a Phase Orca released itself (continuous mode), else "manual".
	Trigger   PhaseStartTrigger
	StartedAt time.Time
}

type PhaseStartTrigger string

const (
	PhaseStartManual PhaseStartTrigger = "manual"
	PhaseStartAuto   PhaseStartTrigger = "auto"
)

// OrDefault reads an unset trigger as manual, the only kind that existed before continuous mode.
func (t PhaseStartTrigger) OrDefault() PhaseStartTrigger {
	if t == "" {
		return PhaseStartManual
	}
	return t
}
