package domain

import (
	"strings"
	"testing"
	"time"
)

func TestApproval_Approve(t *testing.T) {
	now := time.Now()
	a := &Approval{Status: ApprovalStatusPending, Version: 1}
	if err := a.Approve("user1", "ok", now); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if a.Status != ApprovalStatusApproved {
		t.Errorf("got %v", a.Status)
	}
	if a.Version != 2 {
		t.Errorf("got %v", a.Version)
	}

	// Should fail from non-pending
	if err := a.Approve("user1", "ok again", now); err != ErrApprovalNotPending {
		t.Errorf("got %v", err)
	}
}

func TestApproval_Reject(t *testing.T) {
	now := time.Now()
	a := &Approval{Status: ApprovalStatusPending, Version: 1}
	if err := a.Reject("user1", "", now); err != ErrApprovalCommentRequired {
		t.Errorf("got %v", err)
	}
	if err := a.Reject("user1", strings.Repeat("a", 2001), now); err != ErrApprovalCommentTooLong {
		t.Errorf("got %v", err)
	}
	if err := a.Reject("user1", "bad", now); err != nil {
		t.Errorf("got %v", err)
	}
	if a.Status != ApprovalStatusRejected {
		t.Errorf("got %v", a.Status)
	}

	if err := a.Reject("user1", "bad", now); err != ErrApprovalNotPending {
		t.Errorf("got %v", err)
	}
}

func TestApproval_Cancel(t *testing.T) {
	now := time.Now()
	a := &Approval{Status: ApprovalStatusPending, Version: 1}
	if err := a.Cancel("user1", "changed mind", now); err != nil {
		t.Errorf("got %v", err)
	}
	if a.Status != ApprovalStatusCancelled {
		t.Errorf("got %v", a.Status)
	}
	if err := a.Cancel("user1", "changed mind", now); err != ErrApprovalNotPending {
		t.Errorf("got %v", err)
	}
}

func TestApproval_Expire(t *testing.T) {
	now := time.Now()
	a := &Approval{Status: ApprovalStatusPending, Version: 1}
	if err := a.Expire(now); err != nil {
		t.Errorf("got %v", err)
	}
	if a.Status != ApprovalStatusExpired {
		t.Errorf("got %v", a.Status)
	}
	if err := a.Expire(now); err != ErrApprovalNotPending {
		t.Errorf("got %v", err)
	}
}

func TestApproval_EffectiveStatus(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	a := Approval{Status: ApprovalStatusPending, DueAt: &future}
	if s := a.EffectiveStatus(now); s != ApprovalStatusPending {
		t.Errorf("got %v", s)
	}

	a = Approval{Status: ApprovalStatusPending, DueAt: &past}
	if s := a.EffectiveStatus(now); s != ApprovalStatusExpired {
		t.Errorf("got %v", s)
	}

	a = Approval{Status: ApprovalStatusPending, DueAt: &now}
	if s := a.EffectiveStatus(now); s != ApprovalStatusExpired {
		t.Errorf("got %v", s)
	}
}
