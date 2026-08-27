# TASK-159: Wire the 8 new usecases into `git-gateway-service`'s gRPC server (Bucket 3)

**From Solution:** SOL-023 (Bucket 3)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `services/git-gateway-service/internal/adapter/grpc/server.go`, `services/git-gateway-service/cmd/server/main.go`
**Depends on:** TASK-157, TASK-158
**Status:** `[x]` DONE — merge reconciliation completed (merged into `integration/missing-v1` across merges 6-10, git-gateway-service server.go/proto conflicts manually reconciled) — implemented in worktree `agent-a5714e047dcaed0fc`, committed as `56c5fbeff`, builds/tests green in isolation. Touches the same `gitgateway.proto`/`server.go`/`main.go` as the concurrent git.*/files.*/worktree.* work — needs manual reconciliation at merge. Found `DevServerReachability` bug: the task doc's placeholder `GetDevServers()` was wrong, real fix uses `GetFleetHealth`'s `resp.GetStatuses()`.

---

## Changes to make

### `internal/adapter/grpc/server.go`

Add the 8 new usecase fields to `Server` and `New(...)`'s parameter list,
following the exact pattern the existing 6 fields (`getStatus`,
`getDiff`, ...) already use:

```go
type Server struct {
	gitgatewayv1.UnimplementedGitGatewayServiceServer

	getStatus              *usecase.GetStatus
	getDiff                *usecase.GetDiff
	commit                 *usecase.Commit
	push                   *usecase.Push
	pull                   *usecase.Pull
	generateCommitMessage  *usecase.GenerateCommitMessage
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
		getStatus:              getStatus,
		getDiff:                getDiff,
		commit:                 commit,
		push:                   push,
		pull:                   pull,
		generateCommitMessage:  generateCommitMessage,
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
```

Add the 8 RPC methods, following `GetStatus`'s exact translate-only shape:

```go
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
```

Add `"google.golang.org/protobuf/types/known/emptypb"` to the import block
if not already present.

### `cmd/server/main.go`

Construct the 8 new usecases (see TASK-157/TASK-158 for their exact
constructor signatures) next to this service's existing usecase
constructors, and pass all 14 into `grpc.New(...)`'s call in the same
order as `Server`'s new field list above.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./... && go vet ./...
```
