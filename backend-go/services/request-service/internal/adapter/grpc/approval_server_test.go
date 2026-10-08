package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestApprovalMapping_EverySubjectAndStatusRoundTrips(t *testing.T) {
	for _, st := range domain.AllSubjectTypes {
		p := toProtoSubject(st)
		if p == requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_UNSPECIFIED || toDomainSubject(p) != st {
			t.Errorf("subject %s -> %v -> %s", st, p, toDomainSubject(p))
		}
	}
	for _, s := range []domain.ApprovalStatus{domain.ApprovalStatusPending, domain.ApprovalStatusApproved, domain.ApprovalStatusRejected, domain.ApprovalStatusCancelled, domain.ApprovalStatusExpired} {
		p := toProtoStatus(s)
		if p == requestv1.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED || toDomainStatus(p) != s {
			t.Errorf("status %s -> %v -> %s", s, p, toDomainStatus(p))
		}
	}
	if toDomainSubject(requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_UNSPECIFIED) != "" || toDomainStatus(requestv1.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED) != "" {
		t.Error("unspecified must map to the empty filter")
	}
}

func TestApprovalMapping_ApprovalAndPolicyFields(t *testing.T) {
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	by, key := "u1", "k"
	a := toProtoApproval(domain.Approval{ID: "a", SubjectType: domain.SubjectTaskList, Status: domain.ApprovalStatusRejected, DecidedBy: &by, IdempotencyKey: &key,
		DueAt: &now, CreatedAt: now, UpdatedAt: now, Version: 3, SubjectDigest: "d", SelfApprovalAllowed: true, Comment: "c"})
	if a.SubjectType != requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_TASK_LIST || a.DecidedBy != "u1" || a.IdempotencyKey != "k" ||
		a.DueAt.AsTime() != now || a.Version != 3 || a.DecidedAt != nil || !a.SelfApprovalAllowed {
		t.Fatalf("%+v", a)
	}
	due := 90 * time.Second
	size := "L"
	pp := toProtoPolicy(domain.ApprovalPolicy{ID: "p", SubjectType: domain.SubjectPlan, Size: &size, DueAfter: &due, Approvers: []domain.Principal{{Kind: domain.PrincipalKindReporter}, {Kind: domain.PrincipalKindTeam, ID: "t"}}, CreatedAt: now, UpdatedAt: now})
	if pp.Size != "L" || pp.GetDueAfterSeconds() != 90 || len(pp.Approvers) != 2 || pp.Approvers[1] != "team:t" || pp.ProjectId != "" {
		t.Fatalf("%+v", pp)
	}
	back, err := toDomainPolicy(pp)
	if err != nil || *back.Size != "L" || *back.DueAfter != due || back.ProjectID != nil || len(back.Approvers) != 2 || back.TenantID != "" {
		t.Fatalf("%+v %v (tenant must never come from the body)", back, err)
	}
	if _, err := toDomainPolicy(&requestv1.ApprovalPolicy{Approvers: []string{"user:"}}); err == nil {
		t.Fatal("bad principal must be rejected")
	}
}

type listOnlyRepo struct{ usecase.ApprovalRepository }

func (listOnlyRepo) List(context.Context, string, usecase.ApprovalListFilter) ([]domain.Approval, string, error) {
	return nil, "", nil
}

func userCtx() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1")
}

func TestApprovalServer_RequestApprovalOnlyAllowsPreDeployOrClosedGate(t *testing.T) {
	s := NewApprovalServer(ApprovalUseCases{Request: &usecase.RequestApprovalFromAPI{Repo: listOnlyRepo{}}})
	_, err := s.RequestApproval(userCtx(), &requestv1.RequestApprovalRequest{RequestId: "r", SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_SOLUTION})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("SOLUTION without a closed gate = %v", err)
	}
	_, err = s.RequestApproval(tenant.WithTenantID(context.Background(), "t1"), &requestv1.RequestApprovalRequest{RequestId: "r", SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PRE_DEPLOY})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous = %v", err)
	}
	_, err = s.RequestApproval(tenant.WithActorType(userCtx(), tenant.ActorAgent), &requestv1.RequestApprovalRequest{RequestId: "r", SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PRE_DEPLOY})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("agent = %v", err)
	}
}

func TestApprovalServer_UnwiredRPCsAnswerUnimplementedNotPanic(t *testing.T) {
	s := NewApprovalServer(ApprovalUseCases{})
	ctx := userCtx()
	calls := map[string]error{}
	_, calls["RequestApproval"] = s.RequestApproval(ctx, &requestv1.RequestApprovalRequest{})
	_, calls["Approve"] = s.Approve(ctx, &requestv1.ApproveRequest{})
	_, calls["Reject"] = s.Reject(ctx, &requestv1.RejectRequest{})
	_, calls["Cancel"] = s.Cancel(ctx, &requestv1.ApprovalServiceCancelRequest{})
	_, calls["GetApproval"] = s.GetApproval(ctx, &requestv1.GetApprovalRequest{})
	_, calls["ListApprovals"] = s.ListApprovals(ctx, &requestv1.ListApprovalsRequest{})
	_, calls["ListPendingForUser"] = s.ListPendingForUser(ctx, &requestv1.ListPendingForUserRequest{})
	_, calls["ExtendApproval"] = s.ExtendApproval(ctx, &requestv1.ExtendApprovalRequest{})
	for name, err := range calls {
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("%s = %v", name, err)
		}
	}
}

func TestApprovalServer_ListApprovalsReadsTenantFromContextOnly(t *testing.T) {
	s := NewApprovalServer(ApprovalUseCases{List: &usecase.ListApprovals{Repo: listOnlyRepo{}}})
	if _, err := s.ListApprovals(userCtx(), &requestv1.ListApprovalsRequest{RequestId: "r"}); err != nil {
		t.Fatal(err)
	}
	// no tenant in ctx: refused, whatever the body says
	if _, err := s.ListApprovals(context.Background(), &requestv1.ListApprovalsRequest{RequestId: "r"}); err == nil {
		t.Fatal("a call without a tenant must fail")
	}
}

func TestApprovalPolicyAdminServer_NonAdminIsPermissionDenied(t *testing.T) {
	s := NewApprovalPolicyAdminServer(&usecase.ManageApprovalPolicies{})
	ctx := tenant.WithRole(userCtx(), "user")
	if _, err := s.ListApprovalPolicies(ctx, &requestv1.ListApprovalPoliciesRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("list = %v", err)
	}
	if _, err := s.UpsertApprovalPolicy(ctx, &requestv1.UpsertApprovalPolicyRequest{Policy: &requestv1.ApprovalPolicy{Approvers: []string{"reporter"}}}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("upsert = %v", err)
	}
	if _, err := s.DeleteApprovalPolicy(ctx, &requestv1.DeleteApprovalPolicyRequest{Id: "x"}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("delete = %v", err)
	}
	if _, err := NewApprovalPolicyAdminServer(nil).ListApprovalPolicies(ctx, &requestv1.ListApprovalPoliciesRequest{}); status.Code(err) != codes.Unimplemented {
		t.Errorf("unwired = %v", err)
	}
}
