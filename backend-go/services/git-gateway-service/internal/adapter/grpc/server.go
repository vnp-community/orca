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

	clone                  *usecase.Clone
	initRepo               *usecase.InitRepo
	baseRefDefault         *usecase.BaseRefDefault
	searchRefs             *usecase.SearchRefs
	checkHooks             *usecase.CheckHooks
	readIssueCommand       *usecase.ReadIssueCommand
	writeIssueCommand      *usecase.WriteIssueCommand
	scanSetupScriptImports *usecase.ScanSetupScriptImports
}

func New(
	getStatus *usecase.GetStatus,
	getDiff *usecase.GetDiff,
	commit *usecase.Commit,
	push *usecase.Push,
	pull *usecase.Pull,
	generateCommitMessage *usecase.GenerateCommitMessage,
	clone *usecase.Clone,
	initRepo *usecase.InitRepo,
	baseRefDefault *usecase.BaseRefDefault,
	searchRefs *usecase.SearchRefs,
	checkHooks *usecase.CheckHooks,
	readIssueCommand *usecase.ReadIssueCommand,
	writeIssueCommand *usecase.WriteIssueCommand,
	scanSetupScriptImports *usecase.ScanSetupScriptImports,
) *Server {
	return &Server{
		getStatus:             getStatus,
		getDiff:               getDiff,
		commit:                commit,
		push:                  push,
		pull:                  pull,
		generateCommitMessage: generateCommitMessage,

		clone:                  clone,
		initRepo:               initRepo,
		baseRefDefault:         baseRefDefault,
		searchRefs:             searchRefs,
		checkHooks:             checkHooks,
		readIssueCommand:       readIssueCommand,
		writeIssueCommand:      writeIssueCommand,
		scanSetupScriptImports: scanSetupScriptImports,
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

func (s *Server) Clone(ctx context.Context, req *gitgatewayv1.CloneRequest) (*gitgatewayv1.CloneResponse, error) {
	result, err := s.clone.Execute(ctx, usecase.CloneInput{
		DevServerID: req.GetDevServerId(), URL: req.GetUrl(), DestPath: req.GetDestPath(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CloneResponse{WorktreePath: result.WorktreePath, DefaultBranch: result.DefaultBranch}, nil
}

func (s *Server) InitRepo(ctx context.Context, req *gitgatewayv1.InitRepoRequest) (*gitgatewayv1.InitRepoResponse, error) {
	result, err := s.initRepo.Execute(ctx, usecase.InitRepoInput{
		DevServerID: req.GetDevServerId(), DestPath: req.GetDestPath(), DefaultBranch: req.GetDefaultBranch(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.InitRepoResponse{Path: result.Path, DefaultBranch: result.DefaultBranch}, nil
}

func (s *Server) BaseRefDefault(ctx context.Context, req *gitgatewayv1.BaseRefDefaultRequest) (*gitgatewayv1.BaseRefDefaultResponse, error) {
	ref, err := s.baseRefDefault.Execute(ctx, usecase.BaseRefDefaultInput{WorktreeID: req.GetWorktreeId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.BaseRefDefaultResponse{Ref: ref}, nil
}

func (s *Server) SearchRefs(ctx context.Context, req *gitgatewayv1.SearchRefsRequest) (*gitgatewayv1.SearchRefsResponse, error) {
	refs, err := s.searchRefs.Execute(ctx, usecase.SearchRefsInput{WorktreeID: req.GetWorktreeId(), Query: req.GetQuery()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.SearchRefsResponse{Refs: refs}, nil
}

func (s *Server) CheckHooks(ctx context.Context, req *gitgatewayv1.CheckHooksRequest) (*gitgatewayv1.CheckHooksResponse, error) {
	result, err := s.checkHooks.Execute(ctx, usecase.CheckHooksInput{WorktreeID: req.GetWorktreeId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CheckHooksResponse{InstalledHooks: result.InstalledHooks, OrcaHooksCurrent: result.OrcaHooksCurrent}, nil
}

func (s *Server) ReadIssueCommand(ctx context.Context, req *gitgatewayv1.ReadIssueCommandRequest) (*gitgatewayv1.ReadIssueCommandResponse, error) {
	result, err := s.readIssueCommand.Execute(ctx, usecase.ReadIssueCommandInput{WorktreeID: req.GetWorktreeId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ReadIssueCommandResponse{Content: result.Content, Exists: result.Exists}, nil
}

func (s *Server) WriteIssueCommand(ctx context.Context, req *gitgatewayv1.WriteIssueCommandRequest) (*emptypb.Empty, error) {
	err := s.writeIssueCommand.Execute(ctx, usecase.WriteIssueCommandInput{WorktreeID: req.GetWorktreeId(), Content: req.GetContent()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ScanSetupScriptImports(ctx context.Context, req *gitgatewayv1.ScanSetupScriptImportsRequest) (*gitgatewayv1.ScanSetupScriptImportsResponse, error) {
	paths, err := s.scanSetupScriptImports.Execute(ctx, usecase.ScanSetupScriptImportsInput{WorktreeID: req.GetWorktreeId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ScanSetupScriptImportsResponse{ImportedPaths: paths}, nil
}

func toProtoFileStatuses(files []domain.FileStatus) []*gitgatewayv1.FileStatus {
	out := make([]*gitgatewayv1.FileStatus, 0, len(files))
	for _, f := range files {
		out = append(out, &gitgatewayv1.FileStatus{Path: f.Path, State: string(f.State)})
	}
	return out
}
