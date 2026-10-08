package domain

import (
	"testing"
	"time"
)

func strp(s string) *string { return &s }

func TestSelectPolicy_SpecificityThenPriorityThenAge(t *testing.T) {
	old, young := time.Unix(100, 0), time.Unix(200, 0)
	ctx := PolicyContext{ProjectID: "p1", RequestType: "bug", Size: "L", Urgency: "urgent"}
	generic := ApprovalPolicy{ID: "generic", Enabled: true, Priority: 99, CreatedAt: old}
	bySize := ApprovalPolicy{ID: "by-size", Enabled: true, Size: strp("L"), CreatedAt: old}
	bySizeHi := ApprovalPolicy{ID: "by-size-hi", Enabled: true, Size: strp("L"), Priority: 5, CreatedAt: young}
	bySizeOld := ApprovalPolicy{ID: "by-size-old", Enabled: true, Size: strp("L"), Priority: 5, CreatedAt: old}
	byProjectAndType := ApprovalPolicy{ID: "pt", Enabled: true, ProjectID: strp("p1"), RequestType: strp("bug"), CreatedAt: young}
	wrongSize := ApprovalPolicy{ID: "wrong", Enabled: true, Size: strp("S"), ProjectID: strp("p1"), RequestType: strp("bug"), Urgency: strp("urgent")}
	disabled := ApprovalPolicy{ID: "off", Enabled: false, ProjectID: strp("p1"), RequestType: strp("bug"), Size: strp("L"), Urgency: strp("urgent")}

	pick := func(ps ...ApprovalPolicy) string {
		p, ok := SelectPolicy(ps, ctx)
		if !ok {
			return ""
		}
		return p.ID
	}
	if got := pick(generic, bySize); got != "by-size" {
		t.Errorf("more specific wins over priority: %s", got)
	}
	if got := pick(bySize, bySizeHi); got != "by-size-hi" {
		t.Errorf("priority breaks specificity ties: %s", got)
	}
	if got := pick(bySizeHi, bySizeOld); got != "by-size-old" {
		t.Errorf("age breaks priority ties: %s", got)
	}
	if got := pick(bySize, byProjectAndType); got != "pt" {
		t.Errorf("two matching fields beat one: %s", got)
	}
	if got := pick(wrongSize, disabled); got != "" {
		t.Errorf("non-matching and disabled policies are skipped: %s", got)
	}
}

func TestDefaultPolicy_PerSubjectUrgencyAndSize(t *testing.T) {
	pre := DefaultPolicy(SubjectPreDeploy, PolicyContext{Urgency: "urgent"})
	if len(pre.Approvers) != 1 || pre.Approvers[0].String() != "role:admin" || pre.AllowRequesterApprove || *pre.DueAfter != 2*time.Hour {
		t.Errorf("pre_deploy: %+v", pre)
	}
	plan := DefaultPolicy(SubjectPlan, PolicyContext{Size: "L", Urgency: "normal"})
	if plan.AllowRequesterApprove || *plan.DueAfter != 7*24*time.Hour || plan.Approvers[0].Kind != PrincipalKindReporter {
		t.Errorf("large plan needs a second pair of eyes: %+v", plan)
	}
	if hf := DefaultPolicy(SubjectRequestType, PolicyContext{RequestType: "hotfix"}); hf.AllowRequesterApprove {
		t.Error("hotfix type confirmation must not be self-approved")
	}
	if sol := DefaultPolicy(SubjectSolution, PolicyContext{Size: "S", Urgency: "urgent"}); !sol.AllowRequesterApprove || *sol.DueAfter != 4*time.Hour {
		t.Errorf("solution: %+v", sol)
	}
}
