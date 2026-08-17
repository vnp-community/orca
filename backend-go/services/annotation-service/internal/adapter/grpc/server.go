// Package grpc implements the generated annotationv1.AnnotationServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
	"github.com/stablyai/orca-go/services/annotation-service/internal/usecase"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
)

// Server implements annotationv1.UnimplementedAnnotationServiceServer.
type Server struct {
	annotationv1.UnimplementedAnnotationServiceServer

	createAnnotation *usecase.CreateAnnotation
	listAnnotations  *usecase.ListAnnotations
	updateAnnotation *usecase.UpdateAnnotation
	deleteAnnotation *usecase.DeleteAnnotation
}

func New(
	create *usecase.CreateAnnotation,
	list *usecase.ListAnnotations,
	update *usecase.UpdateAnnotation,
	del *usecase.DeleteAnnotation,
) *Server {
	return &Server{
		createAnnotation: create,
		listAnnotations:  list,
		updateAnnotation: update,
		deleteAnnotation: del,
	}
}

func (s *Server) CreateAnnotation(ctx context.Context, req *annotationv1.CreateAnnotationRequest) (*annotationv1.CreateAnnotationResponse, error) {
	anchor := req.GetAnchor()
	annotation, err := s.createAnnotation.Execute(ctx, usecase.CreateAnnotationInput{
		RepoID:    anchor.GetRepoId(),
		FilePath:  anchor.GetFilePath(),
		Line:      anchor.GetLine(),
		Ref:       anchor.GetRef(),
		Content:   req.GetContent(),
		RequestID: req.GetRequestId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &annotationv1.CreateAnnotationResponse{Annotation: toProtoAnnotation(annotation)}, nil
}

func (s *Server) ListAnnotations(ctx context.Context, req *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
	out, err := s.listAnnotations.Execute(ctx, usecase.ListAnnotationsInput{
		RepoID:    req.GetRepoId(),
		FilePath:  req.GetFilePath(),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	annotations := make([]*annotationv1.Annotation, 0, len(out.Annotations))
	for _, a := range out.Annotations {
		annotations = append(annotations, toProtoAnnotation(a))
	}
	return &annotationv1.ListAnnotationsResponse{Annotations: annotations, NextPageToken: out.NextPageToken}, nil
}

func (s *Server) UpdateAnnotation(ctx context.Context, req *annotationv1.UpdateAnnotationRequest) (*annotationv1.UpdateAnnotationResponse, error) {
	annotation, err := s.updateAnnotation.Execute(ctx, usecase.UpdateAnnotationInput{
		ID:       req.GetId(),
		Content:  req.GetContent(),
		Resolved: req.GetResolved(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &annotationv1.UpdateAnnotationResponse{Annotation: toProtoAnnotation(annotation)}, nil
}

func (s *Server) DeleteAnnotation(ctx context.Context, req *annotationv1.DeleteAnnotationRequest) (*annotationv1.DeleteAnnotationResponse, error) {
	if err := s.deleteAnnotation.Execute(ctx, usecase.DeleteAnnotationInput{ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &annotationv1.DeleteAnnotationResponse{}, nil
}

func toProtoAnnotation(a domain.Annotation) *annotationv1.Annotation {
	out := &annotationv1.Annotation{
		Id:       a.ID,
		TenantId: a.TenantID,
		AuthorId: a.AuthorID,
		Anchor: &annotationv1.Anchor{
			RepoId:   a.Anchor.RepoID,
			FilePath: a.Anchor.FilePath,
			Line:     a.Anchor.Line,
			Ref:      a.Anchor.Ref,
		},
		Content:  a.Content,
		Resolved: a.Resolved,
	}
	if !a.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(a.CreatedAt)
	}
	if !a.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(a.UpdatedAt)
	}
	return out
}
