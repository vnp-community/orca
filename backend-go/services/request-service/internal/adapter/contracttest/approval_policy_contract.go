package contracttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func policyOf(tenantID string, mod func(p *domain.ApprovalPolicy)) domain.ApprovalPolicy {
	p := domain.ApprovalPolicy{
		ID: uuid.NewString(), TenantID: tenantID, SubjectType: domain.SubjectPlan, Enabled: true, CreatedBy: uuid.NewString(), AllowRequesterApprove: true,
		Approvers: []domain.Principal{{Kind: domain.PrincipalKindReporter}, {Kind: domain.PrincipalKindTeam, ID: "t1"}, {Kind: domain.PrincipalKindRole, ID: "admin"}},
	}
	if mod != nil {
		mod(&p)
	}
	return p
}

func strPtr(s string) *string { return &s }

// RunApprovalPolicyContract covers TASK-REQ-010-03 (repository) and the storage half of 010-07 (admin CRUD, isolation).
func RunApprovalPolicyContract(t *testing.T, newEnv func(t *testing.T) ApprovalEnv) {
	env := newEnv(t)
	t.Run("UpsertGetRoundTripKeepsApproversAndDue", func(t *testing.T) {
		tenantID := newTenant()
		ctx := tenOf(tenantID)
		due := 90 * time.Minute
		p := policyOf(tenantID, func(p *domain.ApprovalPolicy) {
			p.ProjectID, p.RequestType, p.Size, p.Urgency = strPtr(uuid.NewString()), strPtr("bug"), strPtr("L"), strPtr("urgent")
			p.DueAfter, p.Priority, p.AllowRequesterApprove = &due, 7, true
		})
		saved, err := env.Policies.Upsert(ctx, p, 0)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Version != 1 || saved.ID != p.ID || saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
			t.Fatalf("saved = %+v", saved)
		}
		got, err := env.Policies.Get(ctx, tenantID, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if *got.ProjectID != *p.ProjectID || *got.RequestType != "bug" || *got.Size != "L" || *got.Urgency != "urgent" || got.DueAfter == nil || *got.DueAfter != due ||
			got.Priority != 7 || !got.AllowRequesterApprove || !got.Enabled || got.CreatedBy != p.CreatedBy ||
			len(got.Approvers) != 3 || got.Approvers[0].Kind != domain.PrincipalKindReporter || got.Approvers[1].String() != "team:t1" || got.Approvers[2].String() != "role:admin" {
			t.Fatalf("round trip = %+v", got)
		}
	})

	t.Run("UpsertVersionRulesAndDelete", func(t *testing.T) {
		tenantID := newTenant()
		ctx := tenOf(tenantID)
		p := policyOf(tenantID, nil)
		if _, err := env.Policies.Upsert(ctx, p, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := env.Policies.Upsert(ctx, p, 0); !errors.Is(err, domain.ErrApprovalVersionConflict) {
			t.Fatalf("creating an existing id = %v", err)
		}
		p.Priority = 9
		p.Approvers = []domain.Principal{{Kind: domain.PrincipalKindUser, ID: "boss"}}
		upd, err := env.Policies.Upsert(ctx, p, 1)
		if err != nil || upd.Version != 2 || upd.Priority != 9 || len(upd.Approvers) != 1 || upd.Approvers[0].ID != "boss" {
			t.Fatalf("update = %+v %v", upd, err)
		}
		if _, err := env.Policies.Upsert(ctx, p, 1); !errors.Is(err, domain.ErrApprovalVersionConflict) {
			t.Fatalf("stale version = %v", err)
		}
		missing := policyOf(tenantID, nil)
		if _, err := env.Policies.Upsert(ctx, missing, 3); !errors.Is(err, domain.ErrApprovalPolicyNotFound) {
			t.Fatalf("update of a missing policy = %v", err)
		}
		if err := env.Policies.Delete(ctx, tenantID, p.ID, 1); !errors.Is(err, domain.ErrApprovalVersionConflict) {
			t.Fatalf("delete with stale version = %v", err)
		}
		if err := env.Policies.Delete(ctx, tenantID, p.ID, 2); err != nil {
			t.Fatal(err)
		}
		if err := env.Policies.Delete(ctx, tenantID, p.ID, 0); !errors.Is(err, domain.ErrApprovalPolicyNotFound) {
			t.Fatalf("second delete = %v", err)
		}
		if _, err := env.Policies.Get(ctx, tenantID, p.ID); !errors.Is(err, domain.ErrApprovalPolicyNotFound) {
			t.Fatalf("get after delete = %v", err)
		}
	})

	t.Run("CandidatesFilterByContextAndEnabled", func(t *testing.T) {
		tenantID := newTenant()
		ctx := tenOf(tenantID)
		project := uuid.NewString()
		add := func(mod func(p *domain.ApprovalPolicy)) domain.ApprovalPolicy {
			p := policyOf(tenantID, mod)
			if _, err := env.Policies.Upsert(ctx, p, 0); err != nil {
				t.Fatal(err)
			}
			return p
		}
		generic := add(nil)
		large := add(func(p *domain.ApprovalPolicy) { p.Size = strPtr("L") })
		small := add(func(p *domain.ApprovalPolicy) { p.Size = strPtr("S") })
		scoped := add(func(p *domain.ApprovalPolicy) { p.ProjectID, p.RequestType = &project, strPtr("bug") })
		off := add(func(p *domain.ApprovalPolicy) { p.Enabled = false })
		otherSubject := add(func(p *domain.ApprovalPolicy) { p.SubjectType = domain.SubjectSolution })

		ids := func(project, rtype, size, urgency string) map[string]bool {
			list, err := env.Policies.ListEnabledCandidates(ctx, tenantID, domain.SubjectPlan, project, rtype, size, urgency)
			if err != nil {
				t.Fatal(err)
			}
			m := map[string]bool{}
			for _, p := range list {
				m[p.ID] = true
			}
			return m
		}
		got := ids(project, "bug", "L", "normal")
		if !got[generic.ID] || !got[large.ID] || !got[scoped.ID] || got[small.ID] || got[off.ID] || got[otherSubject.ID] {
			t.Fatalf("candidates = %v", got)
		}
		unknown := ids("", "", "", "")
		if !unknown[generic.ID] || unknown[large.ID] || unknown[scoped.ID] {
			t.Fatalf("an unknown project/size must only match unconstrained policies: %v", unknown)
		}
		// the usecase picks the winner from these candidates
		res := &usecase.ResolveApproverPolicy{Repo: env.Policies}
		picked, err := res.Resolve(ctx, domain.Request{ProjectID: project, Type: domain.RequestTypeBug, Size: domain.RequestSizeL, Urgency: domain.UrgencyNormal}, domain.SubjectPlan)
		if err != nil || picked.ID != scoped.ID {
			t.Fatalf("picked %+v %v", picked, err)
		}
	})

	t.Run("ListPaginatesAndFilters", func(t *testing.T) {
		tenantID := newTenant()
		ctx := tenOf(tenantID)
		for i := 0; i < 5; i++ {
			p := policyOf(tenantID, func(p *domain.ApprovalPolicy) {
				if i == 4 {
					p.SubjectType = domain.SubjectTaskList
				}
			})
			if _, err := env.Policies.Upsert(ctx, p, 0); err != nil {
				t.Fatal(err)
			}
		}
		seen := map[string]bool{}
		token := ""
		for page := 0; page < 10; page++ {
			list, next, err := env.Policies.List(ctx, tenantID, usecase.PolicyListFilter{PageSize: 2, PageToken: token})
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range list {
				if seen[p.ID] {
					t.Fatalf("duplicate %s across pages", p.ID)
				}
				seen[p.ID] = true
			}
			if next == "" {
				break
			}
			token = next
		}
		if len(seen) != 5 {
			t.Fatalf("listed %d of 5", len(seen))
		}
		onlyPlans, _, _ := env.Policies.List(ctx, tenantID, usecase.PolicyListFilter{SubjectType: domain.SubjectPlan, PageSize: 50})
		if len(onlyPlans) != 4 {
			t.Fatalf("subject filter = %d", len(onlyPlans))
		}
	})

	t.Run("TenantIsolation", func(t *testing.T) {
		a, b := newTenant(), newTenant()
		pa := policyOf(a, nil)
		if _, err := env.Policies.Upsert(tenOf(a), pa, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := env.Policies.Get(tenOf(b), b, pa.ID); !errors.Is(err, domain.ErrApprovalPolicyNotFound) {
			t.Fatalf("Get across tenants = %v", err)
		}
		if list, _, _ := env.Policies.List(tenOf(b), b, usecase.PolicyListFilter{PageSize: 50}); len(list) != 0 {
			t.Fatal("List leaked")
		}
		if list, _ := env.Policies.ListEnabledCandidates(tenOf(b), b, domain.SubjectPlan, "", "", "", ""); len(list) != 0 {
			t.Fatal("candidates leaked: tenant A's policy must not shape tenant B's approvals")
		}
		if err := env.Policies.Delete(tenOf(b), b, pa.ID, 0); !errors.Is(err, domain.ErrApprovalPolicyNotFound) {
			t.Fatalf("Delete across tenants = %v", err)
		}
		if _, err := env.Policies.Upsert(tenOf(b), pa, 0); err == nil {
			t.Fatal("writing a policy stamped with another tenant must fail")
		}
		if _, err := env.Policies.Get(tenOf(a), a, pa.ID); err != nil {
			t.Fatalf("owner still reads it: %v", err)
		}
	})
}
