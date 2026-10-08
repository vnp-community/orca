package usecase

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func goodPolicy() domain.ApprovalPolicy {
	return domain.ApprovalPolicy{SubjectType: domain.SubjectPlan, Enabled: true, Approvers: []domain.Principal{{Kind: domain.PrincipalKindRole, ID: "admin"}}}
}

func TestApprovalPolicyAdmin_OnlyAdminHumansMayManage(t *testing.T) {
	e := newApprEnv()
	m := &ManageApprovalPolicies{Repo: apprPolicies{e.s}}
	_, err := m.Upsert(userCtx(uuid.NewString(), "user"), goodPolicy(), 0)
	wantCode(t, err, "REQUEST_APPROVAL_FORBIDDEN")
	_, _, err = m.List(userCtx(uuid.NewString(), "user"), PolicyListFilter{})
	wantCode(t, err, "REQUEST_APPROVAL_FORBIDDEN")
	wantCode(t, m.Delete(lcCtx(), uuid.NewString(), 0), "REQUEST_APPROVAL_FORBIDDEN")
	if _, err = m.Upsert(userCtx(uuid.NewString(), "admin"), goodPolicy(), 0); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalPolicyAdmin_ValidatesPolicy(t *testing.T) {
	m := &ManageApprovalPolicies{Repo: apprPolicies{newApprEnv().s}}
	ctx := userCtx(uuid.NewString(), "admin")
	bad := map[string]func(p *domain.ApprovalPolicy){
		"no approvers": func(p *domain.ApprovalPolicy) { p.Approvers = nil },
		"too many": func(p *domain.ApprovalPolicy) {
			p.Approvers = make([]domain.Principal, 21)
			for i := range p.Approvers {
				p.Approvers[i] = domain.Principal{Kind: domain.PrincipalKindUser, ID: "u"}
			}
		},
		"empty user id":    func(p *domain.ApprovalPolicy) { p.Approvers = []domain.Principal{{Kind: domain.PrincipalKindUser}} },
		"unknown kind":     func(p *domain.ApprovalPolicy) { p.Approvers = []domain.Principal{{Kind: "group", ID: "x"}} },
		"bad subject":      func(p *domain.ApprovalPolicy) { p.SubjectType = "nope" },
		"bad size":         func(p *domain.ApprovalPolicy) { s := "XL"; p.Size = &s },
		"bad urgency":      func(p *domain.ApprovalPolicy) { s := "asap"; p.Urgency = &s },
		"bad request type": func(p *domain.ApprovalPolicy) { s := "epic"; p.RequestType = &s },
	}
	for name, mod := range bad {
		t.Run(name, func(t *testing.T) {
			p := goodPolicy()
			mod(&p)
			_, err := m.Upsert(ctx, p, 0)
			wantCode(t, err, "REQUEST_APPROVAL_POLICY_INVALID")
		})
	}
}

func TestApprovalPolicyAdmin_UpdateNeedsVersionAndCreatedByComesFromCaller(t *testing.T) {
	e := newApprEnv()
	m := &ManageApprovalPolicies{Repo: apprPolicies{e.s}}
	admin := uuid.NewString()
	saved, err := m.Upsert(userCtx(admin, "admin"), goodPolicy(), 5)
	if err != nil || saved.ID == "" || saved.CreatedBy != admin || saved.TenantID != "t1" {
		t.Fatalf("create ignores any supplied version and sets tenant/creator from ctx: %+v %v", saved, err)
	}
	p := goodPolicy()
	p.ID = saved.ID
	_, err = m.Upsert(userCtx(admin, "admin"), p, 0)
	wantCode(t, err, "REQUEST_APPROVAL_VERSION_CONFLICT")
}
