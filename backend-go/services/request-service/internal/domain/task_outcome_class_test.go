package domain

import "testing"

func TestClassifyTaskOutcome_Table(t *testing.T) {
	cases := []struct {
		name                    string
		taskType, cause, status string
		want                    Outcome
		ok                      bool
	}{
		{"leaf claimed", "task", "execute_claim", "in_progress", OutcomeStarted, true},
		{"leaf reached review", "bug", "execution_completed", "review", OutcomeSucceeded, true},
		{"leaf marked done by a user", "feature", "user_update", "done", OutcomeSucceeded, true},
		{"leaf failed and went back to open", "task", "execution_failed", "open", OutcomeFailed, true},
		{"leaf failed back to blocked", "task", "execution_failed", "blocked", OutcomeFailed, true},
		{"leaf recovered after a crash", "task", "recovery", "open", OutcomeFailed, true},
		{"leaf cancelled by a user", "task", "user_update", "cancelled", OutcomeCancelled, true},
		{"phase derived done", "phase", "derived", "done", OutcomePhaseDone, true},
		{"plan derived done", "plan", "derived", "done", OutcomePlanDone, true},
		{"phase derived but not done", "phase", "derived", "in_progress", "", false},
		{"phase done by hand is outside the table", "phase", "user_update", "done", "", false},
		{"plan derived cancelled", "plan", "derived", "cancelled", "", false},
		{"leaf review by a user is outside the table", "task", "user_update", "review", "", false},
		{"leaf completion landing on done is outside the table", "task", "execution_completed", "done", "", false},
		{"claim that did not reach in_progress", "task", "execute_claim", "open", "", false},
		{"unknown cause", "task", "mystery", "open", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ClassifyTaskOutcome(tc.taskType, tc.cause, tc.status)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("got (%q,%v), want (%q,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestOutcome_CompletesContainer(t *testing.T) {
	for _, o := range AllOutcomes() {
		want := o == OutcomePhaseDone || o == OutcomePlanDone
		if o.CompletesContainer() != want {
			t.Errorf("%s: CompletesContainer=%v", o, o.CompletesContainer())
		}
	}
}
