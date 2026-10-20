package mysql

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

// The reviewer's feedback is user content: the activity statement may filter on it but never select it (C9).
func TestActivityUnion_RevisionBranchNeverSelectsFeedback(t *testing.T) {
	contracttest.AssertActivityUnionOmitsFeedback(t, activityUnion)
}

// Every branch of the union binds its own tenant and Request; a branch added without raising activityBranches would shift the
// arguments of all the branches after it.
func TestActivityUnion_BranchCountMatchesTheBoundArguments(t *testing.T) {
	if n := strings.Count(activityUnion, "tenant_id = ? AND request_id = ?"); n != activityBranches {
		t.Fatalf("the union has %d branches, activityBranches is %d", n, activityBranches)
	}
	if n := strings.Count(activityUnion, "?"); n != 2*activityBranches {
		t.Fatalf("the union binds %d arguments, want %d", n, 2*activityBranches)
	}
}
