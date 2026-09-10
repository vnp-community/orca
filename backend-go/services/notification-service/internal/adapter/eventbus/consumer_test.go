package eventbus

import "testing"

// TestSubjects_IncludesProjectDevServerChanged is a regression guard for
// TASK-PRF-03-08's cross-service wiring: a missing entry here fails
// silently (the event is simply never consumed, no error anywhere), so
// this asserts the static subscription list directly rather than relying
// on an integration test to notice.
func TestSubjects_IncludesProjectDevServerChanged(t *testing.T) {
	want := SubjectBinding{StreamName: "PROJECT", Subject: "orca.project.devserver.changed"}
	for _, b := range Subjects {
		if b == want {
			return
		}
	}
	t.Errorf("expected %+v in Subjects, got %+v", want, Subjects)
}
