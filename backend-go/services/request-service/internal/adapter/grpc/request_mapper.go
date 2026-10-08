package grpc

import (
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toProtoRequest(r domain.Request) *requestv1.Request {
	out := &requestv1.Request{
		Id:                     r.ID,
		ProjectId:              r.ProjectID,
		Number:                 r.Number,
		Title:                  r.Title,
		Body:                   r.Body,
		SourceProvider:         string(r.SourceProvider),
		SourceRef:              r.SourceRef,
		SourceUrl:              r.SourceURL,
		SourceSite:             r.SourceSite,
		Type:                   string(r.Type),
		TypeSource:             string(r.TypeSource),
		Size:                   string(r.Size),
		Urgency:                string(r.Urgency),
		ClassificationReason:   r.ClassificationReason,
		Status:                 string(r.Status),
		ReturnedFromStage:      string(r.ReturnedFromStage),
		ReturnReason:           r.ReturnReason,
		ReturnedCategory:       string(r.ReturnedCategory),
		ClassificationAttempts: int32(r.ClassificationAttempts),
		PlanTaskId:             r.PlanTaskID,
		ReporterId:             r.ReporterID,
		CreatedAt:              timestamppb.New(r.CreatedAt),
		UpdatedAt:              timestamppb.New(r.UpdatedAt),
		Version:                r.Version,
		// JSON members go out canonical so clients and digests agree regardless of the database's own JSON spacing.
		AcceptanceCriteriaJson: canonicalJSONString(r.AcceptanceCriteriaJSON),
		TypeFieldsJson:         canonicalJSONString(r.TypeFieldsJSON),
		ContentRevision:        int32(r.ContentRevision),
		ContentSchemaVersion:   int32(r.ContentSchemaVersion),
		ContentDigest:          r.ContentDigest,
	}
	if !r.SourceHints.IsZero() {
		out.SourceHints = &requestv1.SourceHints{IssueType: r.SourceHints.IssueType, Labels: r.SourceHints.Labels, Priority: r.SourceHints.Priority, TypeHint: r.SourceHints.TypeHint}
	}
	if r.Confidence != nil {
		c := *r.Confidence
		out.Confidence = &c
	}
	return out
}

// filterFromProto validates enum values so a typo is InvalidArgument instead of an empty page.
func filterFromProto(req *requestv1.ListRequestsRequest) (usecase.ListFilter, error) {
	f := usecase.ListFilter{
		ProjectID:      req.GetProjectId(),
		PageSize:       int(req.GetPageSize()),
		PageToken:      req.GetPageToken(),
		SourceProvider: req.GetSourceProvider(),
		SourceSite:     req.GetSourceSite(),
		SourceRef:      req.GetSourceRef(),
	}
	for _, s := range req.GetStatus() {
		st, err := domain.ParseRequestStatus(s)
		if err != nil {
			return usecase.ListFilter{}, err
		}
		f.Statuses = append(f.Statuses, st)
	}
	for _, t := range req.GetType() {
		rt, err := domain.ParseRequestType(t)
		if err != nil {
			return usecase.ListFilter{}, err
		}
		f.Types = append(f.Types, rt)
	}
	return f, nil
}
