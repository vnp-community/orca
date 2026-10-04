package eventbus

import "testing"

func TestDurableNameHasNoSubjectDelimiters(t *testing.T) {
	got := durableName("orca.scm.pull_request.merged")
	if got != "issue-status-sync-orca-scm-pull_request-merged" {
		t.Fatalf("durableName = %q", got)
	}
}
