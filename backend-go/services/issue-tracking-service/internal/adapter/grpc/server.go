// Package grpc implements the generated issuetrackingv1.IssueTrackingServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// Server implements issuetrackingv1.UnimplementedIssueTrackingServiceServer.
type Server struct {
	issuetrackingv1.UnimplementedIssueTrackingServiceServer

	listIssues  *usecase.ListIssues
	createIssue *usecase.CreateIssue
	linkIssue   *usecase.LinkIssue
}

func New(listIssues *usecase.ListIssues, createIssue *usecase.CreateIssue, linkIssue *usecase.LinkIssue) *Server {
	return &Server{listIssues: listIssues, createIssue: createIssue, linkIssue: linkIssue}
}

func (s *Server) ListIssues(ctx context.Context, req *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error) {
	issues, err := s.listIssues.Execute(ctx, usecase.ListIssuesInput{
		Provider:   toDomainProvider(req.GetProvider()),
		ProjectKey: req.GetProjectKey(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.Issue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, toProtoIssue(issue))
	}
	return &issuetrackingv1.ListIssuesResponse{Issues: out}, nil
}

func (s *Server) CreateIssue(ctx context.Context, req *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error) {
	issue, err := s.createIssue.Execute(ctx, usecase.CreateIssueInput{
		Provider:    toDomainProvider(req.GetProvider()),
		ProjectKey:  req.GetProjectKey(),
		Title:       req.GetTitle(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.CreateIssueResponse{Issue: toProtoIssue(issue)}, nil
}

func (s *Server) LinkIssue(ctx context.Context, req *issuetrackingv1.LinkIssueRequest) (*issuetrackingv1.LinkIssueResponse, error) {
	err := s.linkIssue.Execute(ctx, usecase.LinkIssueInput{
		IssueID: req.GetIssueId(),
		TaskID:  req.GetTaskId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.LinkIssueResponse{}, nil
}

func toDomainProvider(p issuetrackingv1.IssueProvider) domain.Provider {
	switch p {
	case issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA:
		return domain.ProviderJira
	case issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR:
		return domain.ProviderLinear
	default:
		return ""
	}
}

func toProtoIssue(i domain.Issue) *issuetrackingv1.Issue {
	return &issuetrackingv1.Issue{
		Id:    i.ID,
		Title: i.Title,
		State: i.State,
		Url:   i.URL,
	}
}
