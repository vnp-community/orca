// Package grpc implements the generated gitgatewayv1.GitGatewayServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// Server implements gitgatewayv1.UnimplementedGitGatewayServiceServer.
type Server struct {
	gitgatewayv1.UnimplementedGitGatewayServiceServer

	getStatus             *usecase.GetStatus
	getDiff               *usecase.GetDiff
	commit                *usecase.Commit
	push                  *usecase.Push
	pull                  *usecase.Pull
	generateCommitMessage *usecase.GenerateCommitMessage

	// New, SOL-031 (TASK-192/193/194):
	createWorktree     *usecase.CreateWorktree
	removeWorktree     *usecase.RemoveWorktree
	forceDeleteBranch  *usecase.ForceDeleteBranch
	detectWorktrees    *usecase.DetectWorktrees
	prefetchCreateBase *usecase.PrefetchCreateBase
	resolvePrBase      *usecase.ResolvePrBase
	resolveMrBase      *usecase.ResolveMrBase
}

func New(
	getStatus *usecase.GetStatus,
	getDiff *usecase.GetDiff,
	commit *usecase.Commit,
	push *usecase.Push,
	pull *usecase.Pull,
	generateCommitMessage *usecase.GenerateCommitMessage,
	createWorktree *usecase.CreateWorktree,
	removeWorktree *usecase.RemoveWorktree,
	forceDeleteBranch *usecase.ForceDeleteBranch,
	detectWorktrees *usecase.DetectWorktrees,
	prefetchCreateBase *usecase.PrefetchCreateBase,
	resolvePrBase *usecase.ResolvePrBase,
	resolveMrBase *usecase.ResolveMrBase,
) *Server {
	return &Server{
		getStatus:             getStatus,
		getDiff:               getDiff,
		commit:                commit,
		push:                  push,
		pull:                  pull,
		generateCommitMessage: generateCommitMessage,
		createWorktree:        createWorktree,
		removeWorktree:        removeWorktree,
		forceDeleteBranch:     forceDeleteBranch,
		detectWorktrees:       detectWorktrees,
		prefetchCreateBase:    prefetchCreateBase,
		resolvePrBase:         resolvePrBase,
		resolveMrBase:         resolveMrBase,
	}
}

func (s *Server) GetStatus(ctx context.Context, req *gitgatewayv1.GetStatusRequest) (*gitgatewayv1.GetStatusResponse, error) {
	result, err := s.getStatus.Execute(ctx, usecase.GetStatusInput{WorktreeID: req.GetWorktreeId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.GetStatusResponse{
		Branch: result.Branch,
		Files:  toProtoFileStatuses(result.Files),
	}, nil
}

func (s *Server) GetDiff(ctx context.Context, req *gitgatewayv1.GetDiffRequest) (*gitgatewayv1.GetDiffResponse, error) {
	result, err := s.getDiff.Execute(ctx, usecase.GetDiffInput{WorktreeID: req.GetWorktreeId(), Staged: req.GetStaged()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.GetDiffResponse{UnifiedDiff: result.UnifiedDiff}, nil
}

func (s *Server) Commit(ctx context.Context, req *gitgatewayv1.CommitRequest) (*gitgatewayv1.CommitResponse, error) {
	result, err := s.commit.Execute(ctx, usecase.CommitInput{
		WorktreeID: req.GetWorktreeId(),
		Message:    req.GetMessage(),
		Paths:      req.GetPaths(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CommitResponse{CommitSha: result.CommitSHA}, nil
}

func (s *Server) Push(ctx context.Context, req *gitgatewayv1.PushRequest) (*gitgatewayv1.PushResponse, error) {
	result, err := s.push.Execute(ctx, usecase.PushInput{
		WorktreeID: req.GetWorktreeId(),
		Remote:     req.GetRemote(),
		Branch:     req.GetBranch(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.PushResponse{Success: result.Success}, nil
}

func (s *Server) Pull(ctx context.Context, req *gitgatewayv1.PullRequest) (*gitgatewayv1.PullResponse, error) {
	result, err := s.pull.Execute(ctx, usecase.PullInput{WorktreeID: req.GetWorktreeId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.PullResponse{Success: result.Success, HadConflicts: result.HadConflicts}, nil
}

// GenerateCommitMessage relays the worktree's staged diff to the Dev Server
// Agent's ai.complete (per git-gateway-service.md §3.1) via
// usecase.GenerateCommitMessage. A worktree with no relay connection maps
// to codes.FailedPrecondition through the normal apperrors.ToGRPCStatus
// path — no special-casing needed here now that the usecase always returns
// a typed AppError rather than a permanent stub sentinel.
func (s *Server) GenerateCommitMessage(ctx context.Context, req *gitgatewayv1.GenerateCommitMessageRequest) (*gitgatewayv1.GenerateCommitMessageResponse, error) {
	message, err := s.generateCommitMessage.Execute(ctx, usecase.GenerateCommitMessageInput{WorktreeID: req.GetWorktreeId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.GenerateCommitMessageResponse{Message: message}, nil
}

func (s *Server) CreateWorktree(ctx context.Context, req *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error) {
	result, err := s.createWorktree.Execute(ctx, usecase.CreateWorktreeInput{
		ProjectID: req.GetProjectId(), RepoID: req.GetRepoId(), Branch: req.GetBranch(), BaseRef: req.GetBaseRef(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CreateWorktreeResponse{WorktreeId: result.WorktreeID, Path: result.Path, HeadSha: result.HeadSHA}, nil
}

func (s *Server) RemoveWorktree(ctx context.Context, req *gitgatewayv1.RemoveWorktreeRequest) (*emptypb.Empty, error) {
	if err := s.removeWorktree.Execute(ctx, req.GetWorktreeId(), req.GetForce()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ForceDeleteBranch(ctx context.Context, req *gitgatewayv1.ForceDeleteBranchRequest) (*emptypb.Empty, error) {
	if err := s.forceDeleteBranch.Execute(ctx, req.GetWorktreeId(), req.GetBranch()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) DetectWorktrees(ctx context.Context, req *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error) {
	paths, err := s.detectWorktrees.Execute(ctx, req.GetRepoId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.DetectWorktreesResponse{OnDiskPaths: paths}, nil
}

func (s *Server) PrefetchCreateBase(ctx context.Context, req *gitgatewayv1.PrefetchCreateBaseRequest) (*gitgatewayv1.PrefetchCreateBaseResponse, error) {
	sha, err := s.prefetchCreateBase.Execute(ctx, req.GetRepoId(), req.GetBaseRef())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.PrefetchCreateBaseResponse{ResolvedSha: sha}, nil
}

func (s *Server) ResolvePrBase(ctx context.Context, req *gitgatewayv1.ResolvePrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error) {
	resolved, err := s.resolvePrBase.Execute(ctx, req.GetRepoId(), req.GetPrNumber())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ResolveBaseResponse{BaseBranch: resolved.Branch, BaseSha: resolved.SHA}, nil
}

func (s *Server) ResolveMrBase(ctx context.Context, req *gitgatewayv1.ResolveMrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error) {
	resolved, err := s.resolveMrBase.Execute(ctx, req.GetRepoId(), req.GetMrNumber())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ResolveBaseResponse{BaseBranch: resolved.Branch, BaseSha: resolved.SHA}, nil
}

func toProtoFileStatuses(files []domain.FileStatus) []*gitgatewayv1.FileStatus {
	out := make([]*gitgatewayv1.FileStatus, 0, len(files))
	for _, f := range files {
		out = append(out, &gitgatewayv1.FileStatus{Path: f.Path, State: string(f.State)})
	}
	return out
}
