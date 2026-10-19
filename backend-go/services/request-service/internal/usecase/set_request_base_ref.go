package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type SetRequestBaseRefInput struct {
	RequestID string
	// BaseRef is the branch or tag to start worktrees from; "" restores the repository default.
	BaseRef string
	// ExpectedVersion 0 skips the version check, like ChangeRequestType.
	ExpectedVersion int64
	ActorID         string
	ActorKind       domain.ActorKind
}

type SetRequestBaseRefResult struct {
	Request domain.Request
	// Unverified is true when git could not confirm the ref exists (dev server unreachable); it is stored all the same.
	Unverified bool
}

// SetRequestBaseRefDeps lists every port; NewSetRequestBaseRef refuses a nil one so a missing wiring stops the server at
// startup instead of silently dropping the check it exists for.
type SetRequestBaseRefDeps struct {
	Requests RequestRepository
	Locker   RequestLocker
	Writer   RequestBaseRefWriter
	Presence WorktreePresence
	Checker  BaseRefChecker
	Tx       TxRunner
	Outbox   OutboxWriter
}

type SetRequestBaseRef struct{ d SetRequestBaseRefDeps }

func NewSetRequestBaseRef(d SetRequestBaseRefDeps) (*SetRequestBaseRef, error) {
	switch {
	case d.Requests == nil, d.Locker == nil, d.Writer == nil, d.Presence == nil, d.Checker == nil, d.Tx == nil, d.Outbox == nil:
		return nil, errors.New("usecase: SetRequestBaseRef needs requests, locker, writer, presence, checker, tx and outbox")
	}
	return &SetRequestBaseRef{d: d}, nil
}

// Execute changes the base ref of a Request that has no worktree yet. The git existence check runs before the transaction so a
// slow dev server never holds the Request row lock; everything that decides whether the write is allowed (version, finished
// Request, worktree presence) runs under that lock, so no worktree recorded by another transaction can slip in between the
// check and the write.
func (uc *SetRequestBaseRef) Execute(ctx context.Context, in SetRequestBaseRefInput) (SetRequestBaseRefResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return SetRequestBaseRefResult{}, domain.ErrRequestTenantRequired()
	}
	if in.ActorKind != domain.ActorKindUser {
		return SetRequestBaseRefResult{}, domain.ErrRequestActorNotAllowed(in.ActorKind)
	}
	ref, err := domain.ParseWorktreeBaseRef(in.BaseRef)
	if err != nil {
		return SetRequestBaseRefResult{}, err
	}
	// Malformed ids look the same as missing ones so callers cannot probe id formats.
	if _, err := uuid.Parse(in.RequestID); err != nil {
		return SetRequestBaseRefResult{}, domain.ErrRequestNotFound(in.RequestID)
	}
	current, err := uc.d.Requests.Get(ctx, in.RequestID)
	if err != nil {
		return SetRequestBaseRefResult{}, err
	}
	unverified, err := verifyBaseRef(ctx, uc.d.Checker, current.ID, current.ProjectID, ref)
	if err != nil {
		return SetRequestBaseRefResult{}, err
	}

	var out domain.Request
	err = uc.d.Tx.InTx(ctx, func(txCtx context.Context) error {
		r, err := uc.d.Locker.LockRequest(txCtx, in.RequestID)
		if err != nil {
			return err
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != r.Version {
			return domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
		}
		if r.Status.IsTerminal() {
			return domain.ErrRequestBaseRefLocked(domain.BaseRefLockedTerminal)
		}
		present, reason, err := uc.d.Presence.Has(txCtx, r.ID)
		if err != nil {
			return err
		}
		if present {
			return domain.ErrRequestBaseRefLocked(reason)
		}
		updated, err := uc.d.Writer.SetBaseRef(txCtx, r.ID, string(ref), r.Version)
		if err != nil {
			return err
		}
		ev, err := NewOutboxEvent(txCtx, domain.SubjectRequestBaseRefChanged, map[string]any{
			"request_id": r.ID, "from": r.BaseRef, "to": updated.BaseRef, "actor_id": in.ActorID, "version": updated.Version,
		})
		if err != nil {
			return err
		}
		if err := uc.d.Outbox.InsertOutboxEvent(txCtx, ev); err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return SetRequestBaseRefResult{}, err
	}
	return SetRequestBaseRefResult{Request: out, Unverified: unverified}, nil
}
