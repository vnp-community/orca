package domain

// OnFailurePolicy controls whether RunNow's action loop continues to the
// next action or stops the run when one action fails.
type OnFailurePolicy string

const (
	OnFailureStop     OnFailurePolicy = "stop"
	OnFailureContinue OnFailurePolicy = "continue"
)

// Valid reports whether p is one of the two known policies.
func (p OnFailurePolicy) Valid() bool {
	switch p {
	case OnFailureStop, OnFailureContinue:
		return true
	default:
		return false
	}
}

// AutomationAction is one step in an automation's ordered action chain.
// Mirrors BL-AT-01's `actions: [{type, params}, ...]` schema field-for-field.
type AutomationAction struct {
	StepType       StepType
	StepConfigJSON string
	// OnFailure defaults to OnFailureStop when empty, mirroring the proto
	// default — a chain whose later steps usually depend on an earlier one
	// having actually happened.
	OnFailure OnFailurePolicy
}
