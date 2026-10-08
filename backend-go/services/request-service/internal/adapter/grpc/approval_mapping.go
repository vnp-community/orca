package grpc

import (
	"time"

	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	subjectEnumPrefix = "APPROVAL_SUBJECT_TYPE_"
	statusEnumPrefix  = "APPROVAL_STATUS_"
)

// toDomainSubject maps the proto enum to the stored string ("APPROVAL_SUBJECT_TYPE_TASK_LIST" -> "task_list").
func toDomainSubject(t requestv1.ApprovalSubjectType) domain.SubjectType {
	if t == requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_UNSPECIFIED {
		return ""
	}
	return domain.SubjectType(strings.ToLower(strings.TrimPrefix(t.String(), subjectEnumPrefix)))
}

func toProtoSubject(st domain.SubjectType) requestv1.ApprovalSubjectType {
	return requestv1.ApprovalSubjectType(requestv1.ApprovalSubjectType_value[subjectEnumPrefix+strings.ToUpper(string(st))])
}

func toDomainStatus(s requestv1.ApprovalStatus) domain.ApprovalStatus {
	if s == requestv1.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED {
		return ""
	}
	return domain.ApprovalStatus(strings.ToLower(strings.TrimPrefix(s.String(), statusEnumPrefix)))
}

func toProtoStatus(s domain.ApprovalStatus) requestv1.ApprovalStatus {
	return requestv1.ApprovalStatus(requestv1.ApprovalStatus_value[statusEnumPrefix+strings.ToUpper(string(s))])
}

func tsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func toProtoApproval(a domain.Approval) *requestv1.Approval {
	p := &requestv1.Approval{
		Id: a.ID, TenantId: a.TenantID, RequestId: a.RequestID, SubjectType: toProtoSubject(a.SubjectType), SubjectId: a.SubjectID,
		Stage: a.Stage, Status: toProtoStatus(a.Status), RequestedBy: a.RequestedBy, DecidedAt: tsPtr(a.DecidedAt), Comment: a.Comment,
		DueAt: tsPtr(a.DueAt), Version: a.Version, SubjectDigest: a.SubjectDigest, SelfApprovalAllowed: a.SelfApprovalAllowed,
		RemindedAt: tsPtr(a.RemindedAt), CreatedAt: timestamppb.New(a.CreatedAt), UpdatedAt: timestamppb.New(a.UpdatedAt),
	}
	if a.DecidedBy != nil {
		p.DecidedBy = *a.DecidedBy
	}
	if a.IdempotencyKey != nil {
		p.IdempotencyKey = *a.IdempotencyKey
	}
	return p
}

func toProtoPending(p usecase.PendingApproval) *requestv1.Approval {
	out := toProtoApproval(p.Approval)
	out.RequestTitle, out.RequestType, out.RequestNumber = p.RequestTitle, p.RequestType, p.RequestNumber
	return out
}

func toProtoPolicy(p domain.ApprovalPolicy) *requestv1.ApprovalPolicy {
	out := &requestv1.ApprovalPolicy{
		Id: p.ID, TenantId: p.TenantID, SubjectType: toProtoSubject(p.SubjectType), Approvers: domain.ApproverStrings(p.Approvers),
		AllowRequesterApprove: p.AllowRequesterApprove, Priority: int32(p.Priority), Enabled: p.Enabled, Version: p.Version,
		CreatedBy: p.CreatedBy, CreatedAt: timestamppb.New(p.CreatedAt), UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
	if p.ProjectID != nil {
		out.ProjectId = *p.ProjectID
	}
	if p.RequestType != nil {
		out.RequestType = *p.RequestType
	}
	if p.Size != nil {
		out.Size = *p.Size
	}
	if p.Urgency != nil {
		out.Urgency = *p.Urgency
	}
	if p.DueAfter != nil {
		secs := int32(p.DueAfter.Seconds())
		out.DueAfterSeconds = &secs
	}
	return out
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// toDomainPolicy never trusts tenant_id/created_by from the body: the use case sets them from ctx.
func toDomainPolicy(p *requestv1.ApprovalPolicy) (domain.ApprovalPolicy, error) {
	approvers, err := domain.ParseApprovers(p.GetApprovers())
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}
	out := domain.ApprovalPolicy{
		ID: p.GetId(), ProjectID: emptyToNil(p.GetProjectId()), SubjectType: toDomainSubject(p.GetSubjectType()),
		RequestType: emptyToNil(p.GetRequestType()), Size: emptyToNil(p.GetSize()), Urgency: emptyToNil(p.GetUrgency()),
		Approvers: approvers, AllowRequesterApprove: p.GetAllowRequesterApprove(), Priority: int(p.GetPriority()), Enabled: p.GetEnabled(),
	}
	if p.DueAfterSeconds != nil {
		d := time.Duration(p.GetDueAfterSeconds()) * time.Second
		out.DueAfter = &d
	}
	return out, nil
}
