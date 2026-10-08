package domain

import (
	"reflect"
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

func TestApprovalExtend_MovesDeadlineAndRearmsReminder(t *testing.T) {
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	due, reminded := now.Add(time.Hour), now.Add(-time.Minute)
	a := Approval{Status: ApprovalStatusPending, DueAt: &due, RemindedAt: &reminded, Version: 3}
	if err := a.Extend(7200, now); err != nil || !a.DueAt.Equal(due.Add(2*time.Hour)) || a.RemindedAt != nil || a.Version != 4 {
		t.Fatalf("%+v %v", a, err)
	}
	overdue := now.Add(-time.Hour)
	b := Approval{Status: ApprovalStatusPending, DueAt: &overdue}
	if err := b.Extend(60, now); err != nil || !b.DueAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("an overdue deadline extends from now: %+v %v", b, err)
	}
	c := Approval{Status: ApprovalStatusPending}
	if err := c.Extend(60, now); err != nil || !c.DueAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("no deadline extends from now: %+v %v", c, err)
	}
	for _, bad := range []int{0, -1, maxApprovalExtendSeconds + 1} {
		if err := (&Approval{Status: ApprovalStatusPending}).Extend(bad, now); err != ErrApprovalExtendInvalid {
			t.Fatalf("%d: %v", bad, err)
		}
	}
	if err := (&Approval{Status: ApprovalStatusApproved}).Extend(60, now); err != ErrApprovalNotPending {
		t.Fatalf("closed approvals cannot be extended: %v", err)
	}
}

func TestApprovalComment_RedactsSecretsAndBoundsSize(t *testing.T) {
	now := time.Now()
	a := Approval{Status: ApprovalStatusPending}
	if err := a.Reject("u", "rotate key AKIAIOSFODNN7EXAMPLE now", now); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(a.Comment, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("secret stored in comment: %q", a.Comment)
	}
	long := strings.Repeat("x", 2001)
	if err := (&Approval{Status: ApprovalStatusPending}).Approve("u", long, now); err != ErrApprovalCommentTooLong {
		t.Fatalf("approve comment cap: %v", err)
	}
	b := Approval{Status: ApprovalStatusPending}
	if err := b.Approve("u", "", now); err != nil || b.Comment != "" {
		t.Fatalf("approve comment is optional: %v", err)
	}
}

func TestApprovalSubjectStage_FlowAndStatusRules(t *testing.T) {
	cr, _ := FlowFor(RequestTypeChangeRequest)
	hf, _ := FlowFor(RequestTypeHotfix)
	spike, _ := FlowFor(RequestTypeSpike)
	cases := []struct {
		flow FlowDefinition
		size RequestSize
		st   SubjectType
		want bool
	}{
		{cr, RequestSizeM, SubjectSolution, true}, {cr, RequestSizeM, SubjectPlan, true}, {cr, RequestSizeM, SubjectPhase, true},
		{cr, RequestSizeM, SubjectFindings, false}, {hf, RequestSizeM, SubjectSolution, false}, {hf, RequestSizeM, SubjectPreDeploy, true},
		{spike, RequestSizeM, SubjectFindings, true}, {spike, RequestSizeM, SubjectPlan, false},
	}
	for _, c := range cases {
		if got := ApprovalSubjectAllowedByFlow(c.flow, c.size, true, c.st); got != c.want {
			t.Errorf("%s/%s allowed=%v want %v", c.flow.Type, c.st, got, c.want)
		}
	}
	if !ApprovalSubjectAllowedByFlow(FlowDefinition{}, "", false, SubjectRequestType) || ApprovalSubjectAllowedByFlow(FlowDefinition{}, "", false, SubjectPlan) {
		t.Error("an untyped request only has the request_type gate")
	}
	if !ApprovalAllowedInStatus(SubjectPlan, RequestStatusAwaitingPlanApproval) || ApprovalAllowedInStatus(SubjectPlan, RequestStatusExecuting) ||
		!ApprovalAllowedInStatus(SubjectPreDeploy, RequestStatusExecuting) {
		t.Error("status gating wrong")
	}
	phased := Request{Type: RequestTypeChangeRequest, Size: RequestSizeM}
	flat := Request{Type: RequestTypeOpsRequest, Size: RequestSizeM}
	if ApprovalReturnStage(SubjectPreDeploy, RequestStatusExecuting, phased) != ReturnStagePhase ||
		ApprovalReturnStage(SubjectPreDeploy, RequestStatusExecuting, flat) != ReturnStageTask ||
		ApprovalReturnStage(SubjectPreDeploy, RequestStatusAwaitingPlanApproval, flat) != ReturnStagePlan {
		t.Error("pre_deploy return stage wrong")
	}
}

func TestApprovalSubjectDigest_IsSeparatorSafe(t *testing.T) {
	if SubjectDigest(SubjectPlan, "ab", "c") == SubjectDigest(SubjectPlan, "a", "bc") || SubjectDigest(SubjectPlan, "x") == SubjectDigest(SubjectSolution, "x") {
		t.Fatal("digest must separate parts and subject types")
	}
	if SubjectDigest(SubjectPlan, "x") != SubjectDigest(SubjectPlan, "x") || len(SubjectDigest(SubjectPlan, "x")) != 64 {
		t.Fatal("digest must be stable 64 hex chars")
	}
}

func TestRedactSecrets_MasksAndDropsUnscannedTail(t *testing.T) {
	if got := RedactSecrets("key AKIAIOSFODNN7EXAMPLE end"); strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("not redacted: %q", got)
	}
	if got := RedactSecrets("plain text"); got != "plain text" {
		t.Fatalf("clean text must be untouched: %q", got)
	}
	big := strings.Repeat("a", secretScanWindow) + "AKIAIOSFODNN7EXAMPLE"
	got := RedactSecrets(big)
	if strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") || !strings.HasSuffix(got, "[TRUNCATED: unscanned remainder dropped]") {
		t.Fatalf("a secret past the scan window must not survive: tail %q", got[len(got)-60:])
	}
}

func TestApprovalPolicyValidate_And_ApproverStringsRoundTrip(t *testing.T) {
	ps, err := ParseApprovers([]string{"reporter", "user:u1", "team:t1", "role:admin"})
	if err != nil || len(ps) != 4 || !reflect.DeepEqual(ApproverStrings(ps), []string{"reporter", "user:u1", "team:t1", "role:admin"}) {
		t.Fatalf("%v %v", ps, err)
	}
	for _, bad := range []string{"user:", "team", "robot:x", ""} {
		if _, err := ParseApprovers([]string{bad}); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	p := ApprovalPolicy{SubjectType: SubjectPlan, Approvers: ps}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	neg := -time.Second
	p.DueAfter = &neg
	if err := p.Validate(); err == nil {
		t.Fatal("negative due must be rejected")
	}
}
