package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type DecideApprovalInput struct {
	ApprovalID, Decision, ParamsHash, Note, Via string
}

// DecideApproval records the owner's decision. Only the approval's owner can
// decide: the owner is the metadata user, matched inside the single UPDATE, so
// another user (admin included) simply gets MCP_NOT_FOUND.
type DecideApproval struct {
	core      *GovernanceCore
	approvals ApprovalRepository
	red       domain.Redactor
	clock     Clock
}

func NewDecideApproval(core *GovernanceCore, approvals ApprovalRepository, red domain.Redactor, clock Clock) *DecideApproval {
	return &DecideApproval{core: core, approvals: approvals, red: red, clock: clock}
}

func (uc *DecideApproval) Execute(ctx context.Context, in DecideApprovalInput) (domain.Approval, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.Approval{}, err
	}
	if _, err := uuid.Parse(in.ApprovalID); err != nil {
		return domain.Approval{}, domain.ErrNotFound()
	}
	if in.Decision != domain.DecisionApprove && in.Decision != domain.DecisionDeny {
		return domain.Approval{}, domain.ErrInvalidArgument("decision must be approve or deny")
	}
	approve := in.Decision == domain.DecisionApprove
	if approve && in.ParamsHash == "" {
		return domain.Approval{}, domain.ErrInvalidArgument("paramsHash is required to approve")
	}
	via := in.Via
	if via != domain.ViaWeb && via != domain.ViaMobile && via != domain.ViaElicitation {
		return domain.Approval{}, domain.ErrInvalidArgument("unknown decision channel")
	}
	if len([]rune(in.Note)) > 500 {
		return domain.Approval{}, domain.ErrInvalidArgument("note is limited to 500 characters")
	}
	if ks, err := uc.core.KillState(ctx, id.TenantID); err != nil {
		return domain.Approval{}, domain.ErrInternal("failed to read kill state", err)
	} else if _, blocked := ks.Blocked("", "", ""); blocked {
		return domain.Approval{}, domain.ErrKillSwitchActive()
	}
	if via == domain.ViaElicitation && approve {
		// An elicitation answer comes through the (possibly manipulated)
		// client: only the lowest risk class may be approved this way.
		a, err := uc.approvals.GetApproval(ctx, id.TenantID, in.ApprovalID)
		if err != nil || a.UserID != id.UserID {
			return domain.Approval{}, domain.ErrNotFound()
		}
		if !domain.ElicitationEligibleRisks[domain.Risk(a.Risk)] {
			return domain.Approval{}, domain.ErrInvalidArgument("this approval must be decided in Orca")
		}
	}
	note := in.Note
	if uc.red != nil {
		note, _ = uc.red.Redact(note)
	}
	a, err := uc.approvals.DecideApproval(ctx, DecideApprovalRepoInput{
		TenantID: id.TenantID, UserID: id.UserID, ApprovalID: in.ApprovalID, Approve: approve,
		ParamsHash: in.ParamsHash, Note: note, Via: via, Now: uc.clock.Now(),
	})
	if err != nil {
		return domain.Approval{}, wrapRepoErr(err, "failed to decide approval")
	}
	return a, nil
}

// ListApprovals returns the caller's own approvals, newest first (keyset).
type ListApprovals struct {
	approvals ApprovalRepository
	clock     Clock
}

func NewListApprovals(approvals ApprovalRepository, clock Clock) *ListApprovals {
	return &ListApprovals{approvals: approvals, clock: clock}
}

type ListApprovalsInput struct {
	PendingOnly bool
	Cursor      string
	Limit       int
}

type ListApprovalsOutput struct {
	Approvals  []domain.Approval
	NextCursor string
}

func (uc *ListApprovals) Execute(ctx context.Context, in ListApprovalsInput) (ListApprovalsOutput, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return ListApprovalsOutput{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	q := ApprovalQuery{TenantID: id.TenantID, UserID: id.UserID, PendingOnly: in.PendingOnly, Limit: limit + 1, Now: uc.clock.Now()}
	if in.Cursor != "" {
		parts := strings.SplitN(in.Cursor, "|", 2)
		if len(parts) != 2 {
			return ListApprovalsOutput{}, domain.ErrInvalidArgument("invalid cursor")
		}
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return ListApprovalsOutput{}, domain.ErrInvalidArgument("invalid cursor")
		}
		if _, err := uuid.Parse(parts[1]); err != nil {
			return ListApprovalsOutput{}, domain.ErrInvalidArgument("invalid cursor")
		}
		q.CursorAt, q.CursorID = t, parts[1]
	}
	rows, err := uc.approvals.ListApprovals(ctx, q)
	if err != nil {
		return ListApprovalsOutput{}, wrapRepoErr(err, "failed to list approvals")
	}
	out := ListApprovalsOutput{Approvals: rows}
	if len(rows) > limit {
		out.Approvals = rows[:limit]
		last := rows[limit-1]
		out.NextCursor = last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID
	}
	return out, nil
}

// WaitApproval blocks until the approval is no longer pending (or max wait).
// It only reports the status; consuming happens on the next AuthorizeToolCall.
type WaitApproval struct {
	approvals ApprovalRepository
	clock     Clock
	poll      time.Duration
}

func NewWaitApproval(approvals ApprovalRepository, clock Clock, poll time.Duration) *WaitApproval {
	if poll <= 0 {
		poll = 400 * time.Millisecond
	}
	return &WaitApproval{approvals: approvals, clock: clock, poll: poll}
}

const maxApprovalWait = 60 * time.Second

func (uc *WaitApproval) Execute(ctx context.Context, approvalID string, maxWait time.Duration) (string, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return "", err
	}
	if _, err := uuid.Parse(approvalID); err != nil {
		return "", domain.ErrNotFound()
	}
	if maxWait <= 0 || maxWait > maxApprovalWait {
		maxWait = maxApprovalWait
	}
	deadline := time.NewTimer(maxWait)
	defer deadline.Stop()
	tick := time.NewTicker(uc.poll)
	defer tick.Stop()
	for {
		a, err := uc.approvals.GetApproval(ctx, id.TenantID, approvalID)
		if err != nil || a.UserID != id.UserID {
			return "", domain.ErrNotFound()
		}
		if st := a.EffectiveStatus(uc.clock.Now()); st != domain.ApprovalPending {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return domain.ApprovalPending, nil
		case <-deadline.C:
			return domain.ApprovalPending, nil
		case <-tick.C:
		}
	}
}

// ExpireApprovals is the worker that moves overdue pending approvals to expired.
type ExpireApprovals struct {
	approvals ApprovalRepository
	clock     Clock
}

func NewExpireApprovals(approvals ApprovalRepository, clock Clock) *ExpireApprovals {
	return &ExpireApprovals{approvals: approvals, clock: clock}
}

func (uc *ExpireApprovals) Execute(ctx context.Context, batch int) (int, error) {
	now := uc.clock.Now()
	refs, err := uc.approvals.ListExpiredApprovalRefs(ctx, now, batch)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range refs {
		ok, err := uc.approvals.ExpireApproval(ctx, r.TenantID, r.ID, now)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}
