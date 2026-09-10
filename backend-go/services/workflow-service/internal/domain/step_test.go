package domain

import "testing"

func TestStepType_ValidAcceptsAllSevenKnownTypes(t *testing.T) {
	for _, st := range []StepType{
		StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook,
		StepTypeCondition, StepTypeAction, StepTypeParallel,
	} {
		if !st.Valid() {
			t.Errorf("expected %q to be a valid step type", st)
		}
	}
}

func TestStepType_ValidRejectsUnknownAndUnspecified(t *testing.T) {
	for _, st := range []StepType{StepTypeUnspecified, StepType("bogus")} {
		if st.Valid() {
			t.Errorf("expected %q to be invalid", st)
		}
	}
}
