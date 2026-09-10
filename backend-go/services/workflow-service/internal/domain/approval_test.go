package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewApproval_RequiresTenantTemplateRequestedBy(t *testing.T) {
	if _, err := NewApproval("a1", "", "tmpl-1", "user-1"); !errors.Is(err, ErrApprovalEmptyTenant) {
		t.Errorf("expected ErrApprovalEmptyTenant, got %v", err)
	}
	if _, err := NewApproval("a1", "tenant-1", "", "user-1"); !errors.Is(err, ErrApprovalEmptyTemplate) {
		t.Errorf("expected ErrApprovalEmptyTemplate, got %v", err)
	}
	if _, err := NewApproval("a1", "tenant-1", "tmpl-1", ""); !errors.Is(err, ErrApprovalEmptyRequestedBy) {
		t.Errorf("expected ErrApprovalEmptyRequestedBy, got %v", err)
	}
}

func TestNewApproval_StartsPending(t *testing.T) {
	a, err := NewApproval("a1", "tenant-1", "tmpl-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Status != ApprovalPending {
		t.Errorf("expected status=pending, got %v", a.Status)
	}
}

func TestApproval_Approve(t *testing.T) {
	a, _ := NewApproval("a1", "tenant-1", "tmpl-1", "user-1")
	now := time.Now()
	if err := a.Approve("admin-1", now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Status != ApprovalApproved || a.ResolvedBy != "admin-1" || a.ResolvedAt == nil {
		t.Errorf("expected approved by admin-1 with ResolvedAt set, got %+v", a)
	}
}

func TestApproval_Reject(t *testing.T) {
	a, _ := NewApproval("a1", "tenant-1", "tmpl-1", "user-1")
	now := time.Now()
	if err := a.Reject("admin-1", now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Status != ApprovalRejected || a.ResolvedBy != "admin-1" || a.ResolvedAt == nil {
		t.Errorf("expected rejected by admin-1 with ResolvedAt set, got %+v", a)
	}
}

func TestApproval_ApproveNotPending_Rejected(t *testing.T) {
	a, _ := NewApproval("a1", "tenant-1", "tmpl-1", "user-1")
	now := time.Now()
	if err := a.Approve("admin-1", now); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	if err := a.Approve("admin-2", now); !errors.Is(err, ErrApprovalNotPending) {
		t.Errorf("expected ErrApprovalNotPending on a second approve, got %v", err)
	}
	if err := a.Reject("admin-2", now); !errors.Is(err, ErrApprovalNotPending) {
		t.Errorf("expected ErrApprovalNotPending rejecting an already-approved approval, got %v", err)
	}
}
