package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

// The reviewer's feedback is user content: the activity statement may filter on it but never select it (C9).
func TestActivityUnion_RevisionBranchNeverSelectsFeedback(t *testing.T) {
	contracttest.AssertActivityUnionOmitsFeedback(t, activityUnion)
}
