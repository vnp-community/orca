# TASK-076: Wire shape 1/2 RPCs into `scm-integration-service`'s gRPC server + composition root

**From Solution:** SOL-012
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `services/scm-integration-service/internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-073, TASK-074, TASK-075
**Status:** `[x]` DONE — implemented in worktree `agent-aac2382028c6ce920` (branch `worktree-agent-aac2382028c6ce920`), **committed** as `ce750c490`. `go build`/`go vet`/`gofmt -l` clean, `buf generate`/`buf breaking` clean (additive-only). Pending merge to main + one-line RegisterRealChannels/main.go wiring.

---

## Context

Wires the 7 new usecases (TASK-073/074) into the generated
`ScmIntegrationServiceServer` interface, following `server.go`'s existing
`Execute` → proto-mapping pattern exactly (`apperrors.ToGRPCStatus` on
error, `toDomainProvider`/new `toProtoX` helpers on success), then
constructs and injects them from `main.go`'s composition root.

---

## Changes to make

### Step 1: Extend `Server` struct + constructor

**File:** `services/scm-integration-service/internal/adapter/grpc/server.go`

Find:

```go
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
	}
}
```

Replace with:

```go
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

	mergePullRequest             *usecase.MergePullRequest
	requestPullRequestReviewers  *usecase.RequestPullRequestReviewers
	removePullRequestReviewers   *usecase.RemovePullRequestReviewers
	setPullRequestAutoMerge      *usecase.SetPullRequestAutoMerge
	updateIssue                  *usecase.UpdateIssue
	getPullRequestForBranch      *usecase.GetPullRequestForBranch
	resolveRepoSlug              *usecase.ResolveRepoSlug
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
	mergePullRequest *usecase.MergePullRequest,
	requestPullRequestReviewers *usecase.RequestPullRequestReviewers,
	removePullRequestReviewers *usecase.RemovePullRequestReviewers,
	setPullRequestAutoMerge *usecase.SetPullRequestAutoMerge,
	updateIssue *usecase.UpdateIssue,
	getPullRequestForBranch *usecase.GetPullRequestForBranch,
	resolveRepoSlug *usecase.ResolveRepoSlug,
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

		mergePullRequest:            mergePullRequest,
		requestPullRequestReviewers: requestPullRequestReviewers,
		removePullRequestReviewers:  removePullRequestReviewers,
		setPullRequestAutoMerge:     setPullRequestAutoMerge,
		updateIssue:                 updateIssue,
		getPullRequestForBranch:     getPullRequestForBranch,
		resolveRepoSlug:             resolveRepoSlug,
	}
}
```

### Step 2: Add RPC methods

Append to `server.go`, after the existing `RevokeAuth` method:

```go
func (s *Server) MergePullRequest(ctx context.Context, req *scmintegrationv1.MergePullRequestRequest) (*scmintegrationv1.MergePullRequestResponse, error) {
	result, err := s.mergePullRequest.Execute(ctx, usecase.MergePullRequestParams{
		TenantID: req.GetTenantId(), Provider: toDomainProvider(req.GetProvider()), Repo: req.GetRepo(),
		Number: req.GetNumber(), MergeMethod: req.GetMergeMethod(), CommitTitle: req.GetCommitTitle(), CommitMessage: req.GetCommitMessage(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.MergePullRequestResponse{
		PullRequest: toProtoPullRequest(result.PullRequest), Merged: result.Merged, Sha: result.SHA,
	}, nil
}

func (s *Server) RequestPullRequestReviewers(ctx context.Context, req *scmintegrationv1.RequestPullRequestReviewersRequest) (*scmintegrationv1.PullRequest, error) {
	pr, err := s.requestPullRequestReviewers.Execute(ctx, usecase.RequestPullRequestReviewersParams{
		TenantID: req.GetTenantId(), Provider: toDomainProvider(req.GetProvider()), Repo: req.GetRepo(),
		Number: req.GetNumber(), ReviewerLogins: req.GetReviewerLogins(), TeamSlugs: req.GetTeamSlugs(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoPullRequest(pr), nil
}

func (s *Server) RemovePullRequestReviewers(ctx context.Context, req *scmintegrationv1.RemovePullRequestReviewersRequest) (*scmintegrationv1.PullRequest, error) {
	pr, err := s.removePullRequestReviewers.Execute(ctx, usecase.RemovePullRequestReviewersParams{
		TenantID: req.GetTenantId(), Provider: toDomainProvider(req.GetProvider()), Repo: req.GetRepo(),
		Number: req.GetNumber(), ReviewerLogins: req.GetReviewerLogins(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoPullRequest(pr), nil
}

func (s *Server) SetPullRequestAutoMerge(ctx context.Context, req *scmintegrationv1.SetPullRequestAutoMergeRequest) (*scmintegrationv1.PullRequest, error) {
	pr, err := s.setPullRequestAutoMerge.Execute(ctx, usecase.SetPullRequestAutoMergeParams{
		TenantID: req.GetTenantId(), Provider: toDomainProvider(req.GetProvider()), Repo: req.GetRepo(),
		Number: req.GetNumber(), Enabled: req.GetEnabled(), MergeMethod: req.GetMergeMethod(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoPullRequest(pr), nil
}

func (s *Server) UpdateIssue(ctx context.Context, req *scmintegrationv1.UpdateIssueRequest) (*scmintegrationv1.Issue, error) {
	patch := usecase.IssuePatch{AddLabels: req.GetAddLabels(), RemoveLabels: req.GetRemoveLabels(), Assignees: req.GetAssignees()}
	if req.Title != nil {
		v := req.GetTitle()
		patch.Title = &v
	}
	if req.Body != nil {
		v := req.GetBody()
		patch.Body = &v
	}
	if req.State != nil {
		v := req.GetState()
		patch.State = &v
	}
	issue, err := s.updateIssue.Execute(ctx, usecase.UpdateIssueParams{
		TenantID: req.GetTenantId(), Provider: toDomainProvider(req.GetProvider()), Repo: req.GetRepo(),
		Number: req.GetNumber(), Patch: patch,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoIssue(issue), nil
}

func (s *Server) GetPullRequestForBranch(ctx context.Context, req *scmintegrationv1.GetPullRequestForBranchRequest) (*scmintegrationv1.GetPullRequestForBranchResponse, error) {
	result, err := s.getPullRequestForBranch.Execute(ctx, usecase.GetPullRequestForBranchParams{
		TenantID: req.GetTenantId(), Provider: toDomainProvider(req.GetProvider()), Repo: req.GetRepo(), HeadBranch: req.GetHeadBranch(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &scmintegrationv1.GetPullRequestForBranchResponse{Found: result.Found}
	if result.Found {
		resp.PullRequest = toProtoPullRequest(result.PullRequest)
	}
	return resp, nil
}

func (s *Server) ResolveRepoSlug(ctx context.Context, req *scmintegrationv1.ResolveRepoSlugRequest) (*scmintegrationv1.ResolveRepoSlugResponse, error) {
	result, err := s.resolveRepoSlug.Execute(ctx, usecase.ResolveRepoSlugParams{
		TenantID: req.GetTenantId(), Provider: toDomainProvider(req.GetProvider()), Candidate: req.GetCandidate(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.ResolveRepoSlugResponse{Owner: result.Owner, Name: result.Name, Slug: result.Slug}, nil
}
```

### Step 3: `toProtoIssue`/`toProtoPullRequest` gain `Number`

Find:

```go
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
```

Replace with:

```go
func toProtoIssue(i domain.Issue) *scmintegrationv1.Issue {
	return &scmintegrationv1.Issue{
		Id:     i.ID,
		Title:  i.Title,
		State:  i.State,
		Url:    i.URL,
		Number: i.Number,
	}
}

func toProtoPullRequest(pr domain.PullRequest) *scmintegrationv1.PullRequest {
	return &scmintegrationv1.PullRequest{
		Id:     pr.ID,
		Url:    pr.URL,
		State:  pr.State,
		Number: pr.Number,
	}
}
```

### Step 4: Wire into `main.go`'s composition root

**File:** `services/scm-integration-service/cmd/server/main.go`

Find:

```go
	revokeAuthUC := usecase.NewRevokeAuth(credentials)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	scmintegrationv1.RegisterScmIntegrationServiceServer(grpcServer, scmgrpc.New(
		listIssuesUC, createPullRequestUC, listPullRequestsUC, getRateLimitStatusUC,
		getAuthStatusUC, startOAuthFlowUC, completeOAuthFlowUC, revokeAuthUC,
	))
```

Replace with:

```go
	revokeAuthUC := usecase.NewRevokeAuth(credentials)

	mergePullRequestUC := usecase.NewMergePullRequest(credentials, registry)
	requestPullRequestReviewersUC := usecase.NewRequestPullRequestReviewers(credentials, registry)
	removePullRequestReviewersUC := usecase.NewRemovePullRequestReviewers(credentials, registry)
	setPullRequestAutoMergeUC := usecase.NewSetPullRequestAutoMerge(credentials, registry)
	updateIssueUC := usecase.NewUpdateIssue(credentials, registry)
	getPullRequestForBranchUC := usecase.NewGetPullRequestForBranch(credentials, registry)
	resolveRepoSlugUC := usecase.NewResolveRepoSlug(credentials, registry)

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	scmintegrationv1.RegisterScmIntegrationServiceServer(grpcServer, scmgrpc.New(
		listIssuesUC, createPullRequestUC, listPullRequestsUC, getRateLimitStatusUC,
		getAuthStatusUC, startOAuthFlowUC, completeOAuthFlowUC, revokeAuthUC,
		mergePullRequestUC, requestPullRequestReviewersUC, removePullRequestReviewersUC,
		setPullRequestAutoMergeUC, updateIssueUC, getPullRequestForBranchUC, resolveRepoSlugUC,
	))
```

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./... && go vet ./...
go test ./... -count=1
```

Expected: clean build; `Server` satisfies the generated
`scmintegrationv1.ScmIntegrationServiceServer` interface for every RPC
added by TASK-071/072.
