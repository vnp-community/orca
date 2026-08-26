// Package grpc implements the generated scmintegrationv1.ScmIntegrationServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// Server implements scmintegrationv1.UnimplementedScmIntegrationServiceServer.
type Server struct {
	scmintegrationv1.UnimplementedScmIntegrationServiceServer

	listIssues         *usecase.ListIssues
	createPullRequest  *usecase.CreatePullRequest
	listPullRequests   *usecase.ListPullRequests
	getRateLimitStatus *usecase.GetRateLimitStatus
	getAuthStatus      *usecase.GetAuthStatus
	startOAuthFlow     *usecase.StartOAuthFlow
	completeOAuthFlow  *usecase.CompleteOAuthFlow
	revokeAuth         *usecase.RevokeAuth

	setIntegrationCredential       *usecase.SetIntegrationCredential
	getIntegrationCredentialStatus *usecase.GetIntegrationCredentialStatus
	listIntegrationCredentials     *usecase.ListIntegrationCredentials
}

func New(
	listIssues *usecase.ListIssues,
	createPullRequest *usecase.CreatePullRequest,
	listPullRequests *usecase.ListPullRequests,
	getRateLimitStatus *usecase.GetRateLimitStatus,
	getAuthStatus *usecase.GetAuthStatus,
	startOAuthFlow *usecase.StartOAuthFlow,
	completeOAuthFlow *usecase.CompleteOAuthFlow,
	revokeAuth *usecase.RevokeAuth,
	setIntegrationCredential *usecase.SetIntegrationCredential,
	getIntegrationCredentialStatus *usecase.GetIntegrationCredentialStatus,
	listIntegrationCredentials *usecase.ListIntegrationCredentials,
) *Server {
	return &Server{
		listIssues:         listIssues,
		createPullRequest:  createPullRequest,
		listPullRequests:   listPullRequests,
		getRateLimitStatus: getRateLimitStatus,
		getAuthStatus:      getAuthStatus,
		startOAuthFlow:     startOAuthFlow,
		completeOAuthFlow:  completeOAuthFlow,
		revokeAuth:         revokeAuth,

		setIntegrationCredential:       setIntegrationCredential,
		getIntegrationCredentialStatus: getIntegrationCredentialStatus,
		listIntegrationCredentials:     listIntegrationCredentials,
	}
}

func (s *Server) ListIssues(ctx context.Context, req *scmintegrationv1.ListIssuesRequest) (*scmintegrationv1.ListIssuesResponse, error) {
	issues, err := s.listIssues.Execute(ctx, usecase.ListIssuesInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
		Repo:     req.GetRepo(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*scmintegrationv1.Issue, 0, len(issues))
	for _, i := range issues {
		out = append(out, toProtoIssue(i))
	}
	return &scmintegrationv1.ListIssuesResponse{Issues: out}, nil
}

func (s *Server) CreatePullRequest(ctx context.Context, req *scmintegrationv1.CreatePullRequestRequest) (*scmintegrationv1.CreatePullRequestResponse, error) {
	pr, err := s.createPullRequest.Execute(ctx, usecase.CreatePullRequestParams{
		TenantID:   req.GetTenantId(),
		Provider:   toDomainProvider(req.GetProvider()),
		Repo:       req.GetRepo(),
		Title:      req.GetTitle(),
		Body:       req.GetBody(),
		HeadBranch: req.GetHeadBranch(),
		BaseBranch: req.GetBaseBranch(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.CreatePullRequestResponse{PullRequest: toProtoPullRequest(pr)}, nil
}

func (s *Server) ListPullRequests(ctx context.Context, req *scmintegrationv1.ListPullRequestsRequest) (*scmintegrationv1.ListPullRequestsResponse, error) {
	prs, err := s.listPullRequests.Execute(ctx, usecase.ListPullRequestsInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
		Repo:     req.GetRepo(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*scmintegrationv1.PullRequest, 0, len(prs))
	for _, pr := range prs {
		out = append(out, toProtoPullRequest(pr))
	}
	return &scmintegrationv1.ListPullRequestsResponse{PullRequests: out}, nil
}

func (s *Server) GetRateLimitStatus(ctx context.Context, req *scmintegrationv1.GetRateLimitStatusRequest) (*scmintegrationv1.GetRateLimitStatusResponse, error) {
	status, err := s.getRateLimitStatus.Execute(ctx, usecase.GetRateLimitStatusInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.GetRateLimitStatusResponse{
		Remaining: int32(status.Remaining),
		Limit:     int32(status.Limit),
		ResetUnix: status.ResetAt.Unix(),
	}, nil
}

func (s *Server) GetAuthStatus(ctx context.Context, req *scmintegrationv1.GetAuthStatusRequest) (*scmintegrationv1.GetAuthStatusResponse, error) {
	connected, err := s.getAuthStatus.Execute(ctx, usecase.GetAuthStatusInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.GetAuthStatusResponse{Connected: connected}, nil
}

func (s *Server) StartOAuthFlow(ctx context.Context, req *scmintegrationv1.StartOAuthFlowRequest) (*scmintegrationv1.StartOAuthFlowResponse, error) {
	result, err := s.startOAuthFlow.Execute(ctx, usecase.StartOAuthFlowInput{
		TenantID:    req.GetTenantId(),
		UserID:      req.GetUserId(),
		Provider:    toDomainProvider(req.GetProvider()),
		RedirectURI: req.GetRedirectUri(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.StartOAuthFlowResponse{AuthorizationUrl: result.AuthorizationURL, State: result.State}, nil
}

func (s *Server) CompleteOAuthFlow(ctx context.Context, req *scmintegrationv1.CompleteOAuthFlowRequest) (*scmintegrationv1.CompleteOAuthFlowResponse, error) {
	connected, err := s.completeOAuthFlow.Execute(ctx, usecase.CompleteOAuthFlowInput{
		TenantID:    req.GetTenantId(),
		UserID:      req.GetUserId(),
		Provider:    toDomainProvider(req.GetProvider()),
		Code:        req.GetCode(),
		State:       req.GetState(),
		RedirectURI: req.GetRedirectUri(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.CompleteOAuthFlowResponse{Connected: connected}, nil
}

func (s *Server) RevokeAuth(ctx context.Context, req *scmintegrationv1.RevokeAuthRequest) (*scmintegrationv1.RevokeAuthResponse, error) {
	if err := s.revokeAuth.Execute(ctx, usecase.RevokeAuthInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.RevokeAuthResponse{}, nil
}

func (s *Server) SetIntegrationCredential(ctx context.Context, req *scmintegrationv1.SetIntegrationCredentialRequest) (*scmintegrationv1.SetIntegrationCredentialResponse, error) {
	if err := s.setIntegrationCredential.Execute(ctx, usecase.SetIntegrationCredentialInput{
		TenantID:   req.GetTenantId(),
		Provider:   toDomainProvider(req.GetProvider()),
		Token:      req.GetToken(),
		ConfigJSON: req.GetConfigJson(),
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.SetIntegrationCredentialResponse{}, nil
}

func (s *Server) GetIntegrationCredentialStatus(ctx context.Context, req *scmintegrationv1.GetIntegrationCredentialStatusRequest) (*scmintegrationv1.GetIntegrationCredentialStatusResponse, error) {
	result, err := s.getIntegrationCredentialStatus.Execute(ctx, usecase.GetIntegrationCredentialStatusInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.GetIntegrationCredentialStatusResponse{
		Configured: result.Configured,
		ConfigJson: result.ConfigJSON,
	}, nil
}

func (s *Server) ListIntegrationCredentials(ctx context.Context, req *scmintegrationv1.ListIntegrationCredentialsRequest) (*scmintegrationv1.ListIntegrationCredentialsResponse, error) {
	providers, err := s.listIntegrationCredentials.Execute(ctx, req.GetTenantId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]scmintegrationv1.ScmProvider, 0, len(providers))
	for _, p := range providers {
		out = append(out, toProtoProvider(p))
	}
	return &scmintegrationv1.ListIntegrationCredentialsResponse{ConfiguredProviders: out}, nil
}

func toDomainProvider(p scmintegrationv1.ScmProvider) domain.ScmProvider {
	switch p {
	case scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB:
		return domain.ScmProviderGitHub
	case scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB:
		return domain.ScmProviderGitLab
	case scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET:
		return domain.ScmProviderBitbucket
	case scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS:
		return domain.ScmProviderAzureDevOps
	case scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA:
		return domain.ScmProviderGitea
	default:
		return ""
	}
}

func toProtoProvider(p domain.ScmProvider) scmintegrationv1.ScmProvider {
	switch p {
	case domain.ScmProviderGitHub:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB
	case domain.ScmProviderGitLab:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB
	case domain.ScmProviderBitbucket:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET
	case domain.ScmProviderAzureDevOps:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS
	case domain.ScmProviderGitea:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA
	default:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_UNSPECIFIED
	}
}

func toProtoIssue(i domain.Issue) *scmintegrationv1.Issue {
	return &scmintegrationv1.Issue{
		Id:    i.ID,
		Title: i.Title,
		State: i.State,
		Url:   i.URL,
	}
}

func toProtoPullRequest(pr domain.PullRequest) *scmintegrationv1.PullRequest {
	return &scmintegrationv1.PullRequest{
		Id:    pr.ID,
		Url:   pr.URL,
		State: pr.State,
	}
}
