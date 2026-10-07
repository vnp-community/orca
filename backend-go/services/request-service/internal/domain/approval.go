package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrApprovalNotPending         = errors.New("approval not pending")
	ErrApprovalCommentRequired    = errors.New("approval comment required")
	ErrApprovalCommentTooLong     = errors.New("approval comment too long")
	ErrApprovalSubjectTypeInvalid = errors.New("approval subject type invalid")
)

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
	if len(c) > 2000 {
		return "", ErrApprovalCommentTooLong
	}
	return c, nil
}

func (a *Approval) Approve(by, comment string, now time.Time) error {
	if a.Status != ApprovalStatusPending {
		return ErrApprovalNotPending
	}
	a.Status = ApprovalStatusApproved
	a.DecidedBy = &by
	a.DecidedAt = &now
	a.Comment = strings.TrimSpace(comment)
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
	a.Comment = c
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
	a.Comment = c
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
