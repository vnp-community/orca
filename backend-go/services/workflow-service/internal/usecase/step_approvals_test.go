package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type memStepApprovals struct {
	rows map[string]domain.StepApproval
}

func (m *memStepApprovals) UpsertPendingStepApproval(_ context.Context, a domain.StepApproval) (domain.StepApproval, error) {
	m.rows[a.ID] = a
	return a, nil
}
func (m *memStepApprovals) GetStepApproval(_ context.Context, _, id string) (domain.StepApproval, error) {
	a, ok := m.rows[id]
	if !ok {
		return domain.StepApproval{}, domain.ErrStepApprovalNotFound
	}
	return a, nil
}
func (m *memStepApprovals) LatestStepApproval(context.Context, string, string, string) (domain.StepApproval, error) {
	return domain.StepApproval{}, domain.ErrStepApprovalNotFound
}
func (m *memStepApprovals) ListPendingStepApprovals(_ context.Context, _, projectID, _ string, _ int) ([]domain.StepApproval, error) {
	var out []domain.StepApproval
	for _, a := range m.rows {
		if a.Status == domain.StepApprovalPending && (projectID == "" || a.ProjectID == projectID) {
			out = append(out, a)
		}
	}
	return out, nil
}
func (m *memStepApprovals) ResolveStepApproval(_ context.Context, a domain.StepApproval) (bool, error) {
	if m.rows[a.ID].Status != domain.StepApprovalPending {
		return false, nil
	}
	m.rows[a.ID] = a
	return true, nil
}
func (m *memStepApprovals) MarkStepApprovalSubmitted(context.Context, string, string, time.Time) error {
	return nil
}

type roles map[string]string // "project/user" -> role

func (r roles) MemberRole(_ context.Context, projectID, userID string) (string, error) {
	return r[projectID+"/"+userID], nil
}

func approvalFixture() (*StepApprovals, *memStepApprovals) {
	a, _ := domain.NewStepApproval("a1", "t1", "e1", "approve_g1", "p1", "tdd_agent", nil, []byte(`{"srs":"ready"}`), "h1", time.Now())
	other, _ := domain.NewStepApproval("a2", "t1", "e2", "approve_g1", "p2", "tdd_agent", nil, nil, "h9", time.Now())
	store := &memStepApprovals{rows: map[string]domain.StepApproval{"a1": a, "a2": other}}
	uc := NewStepApprovals(store, roles{"p1/owner": "owner", "p1/dev": "member"}, nil)
	return uc, store
}

func as(user string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), user)
}

func code(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestStepApprovalsAreVisibleToProjectMembersOnly(t *testing.T) {
	uc, _ := approvalFixture()
	list, err := uc.List(as("dev"), "", "")
	if err != nil || len(list) != 1 || list[0].ID != "a1" {
		t.Fatalf("member sees %v %v", list, err)
	}
	if list, _ := uc.List(as("stranger"), "", ""); len(list) != 0 {
		t.Fatalf("stranger sees %v", list)
	}
	if _, _, err := uc.Get(as("stranger"), "a1"); code(err) != "STEP_APPROVAL_NOT_FOUND" {
		t.Fatalf("stranger get: %v", err)
	}
	if _, can, err := uc.Get(as("dev"), "a1"); err != nil || can {
		t.Fatalf("member can decide=%v err=%v", can, err)
	}
	if _, can, _ := uc.Get(as("owner"), "a1"); !can {
		t.Fatal("owner cannot decide")
	}
}

func TestOnlyApproverRolesDecideAndOnlyTheShownDecision(t *testing.T) {
	uc, store := approvalFixture()
	if _, err := uc.Resolve(as("dev"), "a1", true, "h1", "ok"); code(err) != "STEP_APPROVAL_FORBIDDEN" {
		t.Fatalf("member resolve: %v", err)
	}
	if _, err := uc.Resolve(as("owner"), "a1", true, "h0", "ok"); code(err) != "STEP_APPROVAL_STALE" {
		t.Fatalf("stale hash: %v", err)
	}
	if _, err := uc.Resolve(as("owner"), "a1", true, "h1", ""); code(err) != "STEP_APPROVAL_INVALID" {
		t.Fatalf("no rationale: %v", err)
	}
	a, err := uc.Resolve(as("owner"), "a1", true, "h1", "SRS is ready")
	if err != nil || a.Status != domain.StepApprovalApproved || store.rows["a1"].ResolvedBy != "owner" {
		t.Fatalf("resolve %v %v", a, err)
	}
	if _, err := uc.Resolve(as("owner"), "a1", false, "h1", "again"); code(err) != "STEP_APPROVAL_NOT_PENDING" {
		t.Fatalf("second resolve: %v", err)
	}
}

func TestConfiguredApproverRolesWiden(t *testing.T) {
	_, store := approvalFixture()
	uc := NewStepApprovals(store, roles{"p1/dev": "member"}, []string{"owner", "member"})
	if _, err := uc.Resolve(as("dev"), "a1", false, "h1", "not yet"); err != nil {
		t.Fatalf("member with member role allowed: %v", err)
	}
}
