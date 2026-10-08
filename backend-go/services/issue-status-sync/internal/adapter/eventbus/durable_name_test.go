package eventbus

import (
	"strings"
	"testing"
)

func TestDurableNameHasNoSubjectDelimiters(t *testing.T) {
	got := durableName("orca.scm.pull_request.merged")
	if got != "issue-status-sync-orca-scm-pull_request-merged" {
		t.Fatalf("durableName = %q", got)
	}
}

func TestRequestDurableNamesAreLegalAndStable(t *testing.T) {
	cases := map[string]string{
		requestStatusChangedSubject: "issue-status-sync-orca-request-request-status_changed",
		requestCompletedSubject:     "issue-status-sync-orca-request-request-completed",
	}
	for subject, want := range cases {
		got := durableName(subject)
		if got != want {
			t.Errorf("durableName(%q) = %q, want %q", subject, got, want)
		}
		if strings.ContainsAny(got, ".*> \t") {
			t.Errorf("durable name %q has a character NATS rejects", got)
		}
	}
}
