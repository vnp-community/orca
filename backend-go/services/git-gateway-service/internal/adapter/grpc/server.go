// Package grpc implements the generated gitgatewayv1.GitGatewayServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
}

func New(
	getStatus *usecase.GetStatus,
	getDiff *usecase.GetDiff,
	commit *usecase.Commit,
	push *usecase.Push,
	pull *usecase.Pull,
	generateCommitMessage *usecase.GenerateCommitMessage,
) *Server {
	return &Server{
		getStatus:             getStatus,
		getDiff:               getDiff,
		commit:                commit,
		push:                  push,
		pull:                  pull,
		generateCommitMessage: generateCommitMessage,
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

// GenerateCommitMessage always fails with codes.Unimplemented — per
// git-gateway-service.md §3.1 this RPC relays to the Dev Server Agent's
// ai.complete, which is not wired in this scaffold
// (usecase.ErrGenerateCommitMessageNotImplemented). This bypasses
// apperrors.ToGRPCStatus, whose Kind taxonomy has no Unimplemented case,
// since this RPC is deliberately and permanently a stub in this scaffold
// rather than a runtime failure mode that taxonomy models.
func (s *Server) GenerateCommitMessage(ctx context.Context, req *gitgatewayv1.GenerateCommitMessageRequest) (*gitgatewayv1.GenerateCommitMessageResponse, error) {
	_, err := s.generateCommitMessage.Execute(ctx, usecase.GenerateCommitMessageInput{WorktreeID: req.GetWorktreeId()})
	if errors.Is(err, usecase.ErrGenerateCommitMessageNotImplemented) {
		return nil, status.Error(codes.Unimplemented, err.Error())
	}
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.GenerateCommitMessageResponse{}, nil
}

func toProtoFileStatuses(files []domain.FileStatus) []*gitgatewayv1.FileStatus {
	out := make([]*gitgatewayv1.FileStatus, 0, len(files))
	for _, f := range files {
		out = append(out, &gitgatewayv1.FileStatus{Path: f.Path, State: string(f.State)})
	}
	return out
}
