package grpc

import (
	"context"
	"strconv"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ArtifactUseCases are the CR-REQ-027 use cases behind the artifact RPCs. Each handler answers Unimplemented until its use case is wired.
type ArtifactUseCases struct {
	Edit   *usecase.EditRequestContent
	Reader *usecase.ArtifactReader
	Export *usecase.ExportArtifactProjection
}

func (s *Server) WithArtifact(u ArtifactUseCases) *Server { s.artifact = u; return s }

func unwired(rpc string) error { return status.Error(codes.Unimplemented, rpc+" is not wired") }

func (s *Server) EditRequestContent(ctx context.Context, req *requestv1.EditRequestContentRequest) (*requestv1.EditRequestContentResponse, error) {
	if s.artifact.Edit == nil {
		return nil, unwired("EditRequestContent")
	}
	// proto3 strings carry no presence: an empty title or body means "leave unchanged".
	patch, err := usecase.PatchFromJSON(req.GetTitle(), req.GetBody(), req.GetTitle() != "", req.GetBody() != "", req.GetAcceptanceCriteriaJson(), req.GetTypeFieldsJson())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	r, err := s.artifact.Edit.Execute(ctx, usecase.EditInput{RequestID: req.GetRequestId(), ExpectedRevision: int(req.GetExpectedRevision()), Patch: patch})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.EditRequestContentResponse{Request: toProtoRequest(r)}, nil
}

func (s *Server) ListRequestRevisions(ctx context.Context, req *requestv1.ListRequestRevisionsRequest) (*requestv1.ListRequestRevisionsResponse, error) {
	if s.artifact.Reader == nil {
		return nil, unwired("ListRequestRevisions")
	}
	after := 0
	if tok := req.GetPageToken(); tok != "" {
		n, err := strconv.Atoi(tok)
		if err != nil || n < 0 {
			return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_PAGE_TOKEN", "invalid page token", nil))
		}
		after = n
	}
	page, err := s.artifact.Reader.ListRevisions(ctx, req.GetRequestId(), after, int(req.GetPageSize()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListRequestRevisionsResponse{}
	for _, r := range page.Revisions { // summaries only: the snapshot is fetched per revision
		out.Revisions = append(out.Revisions, toProtoRevisionSummary(r))
	}
	if page.NextAfter > 0 {
		out.NextPageToken = strconv.Itoa(page.NextAfter)
	}
	return out, nil
}

func (s *Server) GetRequestRevision(ctx context.Context, req *requestv1.GetRequestRevisionRequest) (*requestv1.GetRequestRevisionResponse, error) {
	if s.artifact.Reader == nil {
		return nil, unwired("GetRequestRevision")
	}
	rev, err := s.artifact.Reader.GetRevision(ctx, req.GetRequestId(), int(req.GetRevision()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetRequestRevisionResponse{Revision: toProtoRevisionSummary(rev), SnapshotJson: string(rev.Snapshot)}, nil
}

func (s *Server) GetRequestCoverage(ctx context.Context, req *requestv1.GetRequestCoverageRequest) (*requestv1.GetRequestCoverageResponse, error) {
	if s.artifact.Reader == nil {
		return nil, unwired("GetRequestCoverage")
	}
	v, err := s.artifact.Reader.Coverage(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetRequestCoverageResponse{UncoveredAcIds: v.UncoveredACIDs}
	for _, r := range v.Rows {
		out.Rows = append(out.Rows, toProtoCoverageRow(r))
	}
	return out, nil
}

func (s *Server) GetArtifactGraph(ctx context.Context, req *requestv1.GetArtifactGraphRequest) (*requestv1.GetArtifactGraphResponse, error) {
	if s.artifact.Reader == nil {
		return nil, unwired("GetArtifactGraph")
	}
	g, err := s.artifact.Reader.Graph(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetArtifactGraphResponse{Partial: g.Partial}
	for _, e := range g.Edges {
		out.Edges = append(out.Edges, toProtoEdge(e))
	}
	return out, nil
}

func (s *Server) ExportArtifactProjection(ctx context.Context, req *requestv1.ExportArtifactProjectionRequest) (*requestv1.ExportArtifactProjectionResponse, error) {
	if s.artifact.Export == nil {
		return nil, unwired("ExportArtifactProjection")
	}
	res, err := s.artifact.Export.Execute(ctx, req.GetRequestId(), req.GetArtifactRef(), req.GetFormat())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ExportArtifactProjectionResponse{Filename: res.Filename, Content: res.Content, Digest: res.Digest}, nil
}

func (s *Server) ResolveArtifactRef(ctx context.Context, req *requestv1.ResolveArtifactRefRequest) (*requestv1.ResolveArtifactRefResponse, error) {
	if s.artifact.Reader == nil {
		return nil, unwired("ResolveArtifactRef")
	}
	e, err := s.artifact.Reader.Resolve(ctx, req.GetRef())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ResolveArtifactRefResponse{Kind: string(e.Kind), ArtifactId: e.ArtifactID, RequestId: e.RequestID}, nil
}
