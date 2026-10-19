package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReservingProposalWorkspace makes every Ensure of the proposal worktree safe against a concurrent change of the base ref.
//
// The worktree is created in git-gateway, outside any request-service transaction, so the row lock alone cannot stop
// SetRequestBaseRef from slipping in between "read the ref" and "worktree exists". Instead, before git is asked for anything,
// Ensure takes the Request row lock, reads the base ref fresh from the database and records the change as 'preparing'. From
// that commit on WorktreePresence is true, so SetRequestBaseRef is refused (REQUEST_BASE_REF_LOCKED); and if SetRequestBaseRef
// committed first, the fresh read sees its value. The worktree is then created from that value outside the lock.
type ReservingProposalWorkspace struct {
	ProposalWorkspace
	locker  RequestLocker
	changes OpenSpecChangeRepository
	tx      TxRunner
	newID   func() string
}

var _ ProposalWorkspace = (*ReservingProposalWorkspace)(nil)

// NewReservingProposalWorkspace refuses a nil port: a server wired without the reservation would silently reopen the race.
func NewReservingProposalWorkspace(inner ProposalWorkspace, locker RequestLocker, changes OpenSpecChangeRepository, tx TxRunner) (*ReservingProposalWorkspace, error) {
	if inner == nil || locker == nil || changes == nil || tx == nil {
		return nil, errors.New("usecase: ReservingProposalWorkspace needs the workspace, locker, change repository and tx runner")
	}
	return &ReservingProposalWorkspace{ProposalWorkspace: inner, locker: locker, changes: changes, tx: tx, newID: uuid.NewString}, nil
}

// reservation is what Ensure learned under the lock.
type reservation struct {
	baseRef string
	// marker is the change row as reserved; nil for a finished Request, which records nothing.
	marker *domain.OpenSpecChange
}

// Ensure ignores in.BaseRef: the stored value, read under the lock, is the only one that cannot be stale.
func (w *ReservingProposalWorkspace) Ensure(ctx context.Context, in EnsureWorkspaceInput) (Workspace, error) {
	res, err := w.reserve(ctx, in)
	if err != nil {
		return Workspace{}, err
	}
	in.BaseRef = res.baseRef
	ws, err := w.ProposalWorkspace.Ensure(ctx, in)
	if err != nil {
		w.releaseIfNothingWasCreated(ctx, in.RequestID, res, err)
		return Workspace{}, err
	}
	return ws, nil
}

func (w *ReservingProposalWorkspace) reserve(ctx context.Context, in EnsureWorkspaceInput) (reservation, error) {
	var out reservation
	err := w.tx.InTx(ctx, func(txCtx context.Context) error {
		req, err := w.locker.LockRequest(txCtx, in.RequestID)
		if err != nil {
			return err
		}
		out = reservation{baseRef: req.BaseRef}
		if req.Status.IsTerminal() {
			return nil // the ref of a finished Request cannot change, so there is nothing to protect
		}
		c, found, err := w.changes.GetByRequest(txCtx, req.ID)
		if err != nil {
			return err
		}
		switch {
		case !found:
			if !domain.ValidChangeID(in.ChangeID) {
				return fmt.Errorf("%w: the change id is not valid, so the proposal worktree cannot be reserved", domain.ErrOpenSpecInvalidOutput)
			}
			c = domain.OpenSpecChange{ID: w.newID(), RequestID: req.ID, ChangeID: in.ChangeID, Branch: domain.ProposalBranch(req.Number),
				Status: domain.ChangeStatusPreparing, SyncState: domain.SyncStateInSync}
		case c.Status == domain.ChangeStatusAbandoned:
			if err := c.Reopen(); err != nil {
				return err
			}
		default:
			out.marker = &c // a retry or a recreate: the marker is already there
			return nil
		}
		if c, err = w.changes.Upsert(txCtx, c); err != nil {
			return err
		}
		out.marker = &c
		return nil
	})
	return out, err
}

// releaseIfNothingWasCreated abandons the marker when git said the base ref does not exist: that answer proves no worktree was
// made, and a marker left behind would freeze the very ref the person has to fix. Any other failure keeps the marker, because
// the worktree may exist (the gateway can fail after git ran), and "locked" must never be wrong in that direction. A process
// that dies between reserve and create also keeps it; the next run of the Request reuses the marker.
func (w *ReservingProposalWorkspace) releaseIfNothingWasCreated(ctx context.Context, requestID string, res reservation, cause error) {
	if res.marker == nil || !domain.IsBaseRefNotFound(cause) {
		return
	}
	// The caller's context may be what failed; the release must still run.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := w.tx.InTx(ctx, func(txCtx context.Context) error {
		if _, err := w.locker.LockRequest(txCtx, requestID); err != nil {
			return err
		}
		c, found, err := w.changes.GetByRequest(txCtx, requestID)
		if err != nil || !found || c.Status != domain.ChangeStatusPreparing || c.WorkspaceID != "" {
			return err // a worktree was recorded meanwhile, or the row moved on: leave it
		}
		if err := c.Abandon(); err != nil {
			return err
		}
		_, err = w.changes.Upsert(txCtx, c)
		return err
	})
	if err != nil {
		slog.WarnContext(ctx, "could not release the proposal worktree reservation; the base ref stays locked until the next run",
			slog.String("request_id", requestID), slog.Any("error", err))
	}
}
