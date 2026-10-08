package contracttest

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// SubjectHandlerFixture is one subject handler plus the data to exercise it.
// Every CR that owns a SubjectHandler (005, 007, 008, 012, 013, 014) runs RunSubjectHandlerContract on its handler.
type SubjectHandlerFixture struct {
	Handler usecase.SubjectHandler
	Subject domain.SubjectType
	// Request is a Request the handler accepts for Subject (right status, type, size).
	Request domain.Request
	// Approval builds the approval row the hooks receive, as DecideApproval would pass it.
	Approval func(subjectID, digest string) domain.Approval
	// Reset restores fixture state between sub-tests (optional).
	Reset func()
	// Ctx scopes the calls for handlers that read the database; defaults to a fixed fake tenant for stub handlers.
	Ctx context.Context
}

// RunSubjectHandlerContract checks the invariants the approval engine relies on:
// ValidateForRequest is stable and non-empty, refuses a foreign subject type, the hooks never panic and
// OnClosedWithoutDecision is idempotent. "Writes only this service's DB and never calls the network" cannot be
// proven generically; handlers whose hooks reach out must be reviewed against that rule by hand.
func RunSubjectHandlerContract(t *testing.T, factory func(t *testing.T) SubjectHandlerFixture) {
	t.Helper()
	stubCtx := tenant.WithTenantID(context.Background(), "t1")

	t.Run("ValidateForRequest is stable", func(t *testing.T) {
		f := factory(t)
		ctx := stubCtx
		if f.Ctx != nil {
			ctx = f.Ctx
		}
		id1, d1, err := f.Handler.ValidateForRequest(ctx, ctx, f.Request, f.Subject)
		if err != nil {
			t.Fatal(err)
		}
		id2, d2, err := f.Handler.ValidateForRequest(ctx, ctx, f.Request, f.Subject)
		if err != nil || id1 != id2 || d1 != d2 {
			t.Fatalf("not stable: (%q,%q) vs (%q,%q) err %v", id1, d1, id2, d2, err)
		}
		if id1 == "" || d1 == "" {
			t.Fatalf("subject id and digest must be non-empty: %q %q", id1, d1)
		}
	})

	t.Run("ValidateForRequest refuses a request outside the gate", func(t *testing.T) {
		f := factory(t)
		ctx := stubCtx
		if f.Ctx != nil {
			ctx = f.Ctx
		}
		wrong := f.Request
		wrong.Status = domain.RequestStatusNew
		if _, _, err := f.Handler.ValidateForRequest(ctx, ctx, wrong, f.Subject); err == nil {
			t.Fatal("a Request that is not at the gate must be refused")
		}
	})

	t.Run("OnClosedWithoutDecision is idempotent", func(t *testing.T) {
		f := factory(t)
		ctx := stubCtx
		if f.Ctx != nil {
			ctx = f.Ctx
		}
		id, digest, err := f.Handler.ValidateForRequest(ctx, ctx, f.Request, f.Subject)
		if err != nil {
			t.Fatal(err)
		}
		a := f.Approval(id, digest)
		for i := 0; i < 2; i++ {
			if err := f.Handler.OnClosedWithoutDecision(ctx, ctx, a, "expired"); err != nil {
				t.Fatalf("call %d: %v", i+1, err)
			}
		}
	})
}
