package domain

import (
	"strings"
	"time"
)

// maxApprovalExtendSeconds caps one extension so a deadline cannot be pushed out indefinitely in a single call.
const maxApprovalExtendSeconds = 30 * 24 * 3600

// maxApprovalCommentLen is in bytes, matching the column-side limits of both dialects.
const maxApprovalCommentLen = 2000

type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "pending"
	ApprovalStatusApproved  ApprovalStatus = "approved"
	ApprovalStatusRejected  ApprovalStatus = "rejected"
	ApprovalStatusCancelled ApprovalStatus = "cancelled"
	ApprovalStatusExpired   ApprovalStatus = "expired"
)

func (s ApprovalStatus) Terminal() bool {
	return s != ApprovalStatusPending
}

type Approval struct {
	ID                  string
	TenantID            string
	RequestID           string
	SubjectType         SubjectType
	SubjectID           string
	Stage               string
	Status              ApprovalStatus
	RequestedBy         string
	DecidedBy           *string
	DecidedAt           *time.Time
	Comment             string
	DueAt               *time.Time
	Version             int64
	SubjectDigest       string
	SelfApprovalAllowed bool
	IdempotencyKey      *string
	RemindedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func ValidateApprovalComment(s string) (string, error) {
	c := strings.TrimSpace(s)
	if c == "" {
		return "", ErrApprovalCommentRequired
	}
	if len(c) > maxApprovalCommentLen {
		return "", ErrApprovalCommentTooLong
	}
	return c, nil
}

// validateOptionalApprovalComment lets approvals carry no comment but still bounds its size.
func validateOptionalApprovalComment(s string) (string, error) {
	c := strings.TrimSpace(s)
	if len(c) > maxApprovalCommentLen {
		return "", ErrApprovalCommentTooLong
	}
	return c, nil
}

func (a *Approval) Approve(by, comment string, now time.Time) error {
	if a.Status != ApprovalStatusPending {
		return ErrApprovalNotPending
	}
	c, err := validateOptionalApprovalComment(comment)
	if err != nil {
		return err
	}
	a.Status = ApprovalStatusApproved
	a.DecidedBy = &by
	a.DecidedAt = &now
	a.Comment = RedactSecrets(c)
	a.Version++
	a.UpdatedAt = now
	return nil
}

func (a *Approval) Reject(by, comment string, now time.Time) error {
	if a.Status != ApprovalStatusPending {
		return ErrApprovalNotPending
	}
	c, err := ValidateApprovalComment(comment)
	if err != nil {
		return err
	}
	a.Status = ApprovalStatusRejected
	a.DecidedBy = &by
	a.DecidedAt = &now
	a.Comment = RedactSecrets(c)
	a.Version++
	a.UpdatedAt = now
	return nil
}

func (a *Approval) Cancel(by, reason string, now time.Time) error {
	if a.Status != ApprovalStatusPending {
		return ErrApprovalNotPending
	}
	c, err := ValidateApprovalComment(reason)
	if err != nil {
		return err
	}
	a.Status = ApprovalStatusCancelled
	a.DecidedBy = &by
	a.DecidedAt = &now
	a.Comment = RedactSecrets(c)
	a.Version++
	a.UpdatedAt = now
	return nil
}

func (a *Approval) Expire(now time.Time) error {
	if a.Status != ApprovalStatusPending {
		return ErrApprovalNotPending
	}
	a.Status = ApprovalStatusExpired
	a.DecidedAt = &now
	a.Version++
	a.UpdatedAt = now
	return nil
}

func (a Approval) EffectiveStatus(now time.Time) ApprovalStatus {
	if a.Status == ApprovalStatusPending && a.DueAt != nil && !now.Before(*a.DueAt) {
		return ApprovalStatusExpired
	}
	return a.Status
}

// Extend moves the deadline of a pending approval out by seconds (from the old deadline, or from now when none)
// and re-arms the reminder, so a long review is not forced through expire-then-reopen.
func (a *Approval) Extend(seconds int, now time.Time) error {
	if a.Status != ApprovalStatusPending {
		return ErrApprovalNotPending
	}
	if seconds <= 0 || seconds > maxApprovalExtendSeconds {
		return ErrApprovalExtendInvalid
	}
	base := now
	if a.DueAt != nil && a.DueAt.After(now) {
		base = *a.DueAt
	}
	due := base.Add(time.Duration(seconds) * time.Second)
	a.DueAt = &due
	a.RemindedAt = nil
	a.Version++
	a.UpdatedAt = now
	return nil
}
