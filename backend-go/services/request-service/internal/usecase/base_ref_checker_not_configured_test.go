package usecase

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// A checker that cannot be built (no git-gateway or project-service in this deployment) is a defect to surface, not an
// unreachable dev server to shrug at: the ref must not be stored as if it could ever be verified.
func TestSetRequestBaseRef_CheckerNotConfigured_IsRefusedNotStoredUnverified(t *testing.T) {
	f := newSetBaseRefFixture(t)
	f.checker.err = domain.ErrRequestBaseRefCheckNotConfigured()
	r := f.seed(nil)
	_, err := f.uc.Execute(tctx(), setIn(r, "release/1.4"))
	mustCode(t, err, "REQUEST_BASE_REF_CHECK_NOT_CONFIGURED")
	requireNoWrite(t, f, r)
}

func TestSetRequestBaseRef_AnyOtherCheckerError_StaysFailOpen(t *testing.T) {
	f := newSetBaseRefFixture(t)
	f.checker.err = errors.New("dev server offline")
	r := f.seed(nil)
	res, err := f.uc.Execute(tctx(), setIn(r, "release/1.4"))
	if err != nil || !res.Unverified || res.Request.BaseRef != "release/1.4" {
		t.Fatalf("%+v %v", res, err)
	}
}
