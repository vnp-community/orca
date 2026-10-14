package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProposalWorkspace keeps the OpenSpec proposal of a Request on a local branch `request/<number>-proposal`, in its own
// worktree. Every git action goes through git-gateway; this adapter runs no command and builds no shell string.
type ProposalWorkspace struct {
	git      gitgatewayv1.GitGatewayServiceClient
	projects projectv1.ProjectServiceClient
	now      func() time.Time
}

var _ usecase.ProposalWorkspace = (*ProposalWorkspace)(nil)

func NewProposalWorkspace(git gitgatewayv1.GitGatewayServiceClient, projects projectv1.ProjectServiceClient) *ProposalWorkspace {
	return &ProposalWorkspace{git: git, projects: projects, now: func() time.Time { return time.Now().UTC() }}
}

func proposalBranch(number int64) string { return domain.ProposalBranch(number) }

func (w *ProposalWorkspace) Ensure(ctx context.Context, in usecase.EnsureWorkspaceInput) (usecase.Workspace, error) {
	if in.ProjectID == "" || in.RequestID == "" || in.RequestNumber <= 0 {
		return usecase.Workspace{}, fmt.Errorf("%w: project, request id and number are required", domain.ErrEngineRepoNotResolved)
	}
	repoID, err := w.repoFor(ctx, in)
	if err != nil {
		return usecase.Workspace{}, err
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return usecase.Workspace{}, err
	}
	base, err := w.git.BaseRefDefault(tctx, &gitgatewayv1.BaseRefDefaultRequest{RepoId: repoID})
	if err != nil {
		return usecase.Workspace{}, mapWorkspaceError("resolve the default branch", err)
	}
	branch := proposalBranch(in.RequestNumber)
	key := "request-proposal:" + in.RequestID
	ws, err := w.create(tctx, in.ProjectID, repoID, branch, base.GetRef(), key)
	if err != nil {
		return usecase.Workspace{}, err
	}
	// The idempotency key also matches a worktree that was removed since: such a record answers with a path that is gone.
	// A status call tells the two apart, and a fresh key creates the worktree again on the surviving branch.
	if _, serr := w.git.GetStatus(tctx, &gitgatewayv1.GetStatusRequest{WorktreeId: ws.WorktreeID}); serr != nil {
		ws, err = w.create(tctx, in.ProjectID, repoID, branch, base.GetRef(), fmt.Sprintf("%s:%d", key, w.now().UnixNano()))
		if err != nil {
			return usecase.Workspace{}, err
		}
	}
	return ws, nil
}

func (w *ProposalWorkspace) create(ctx context.Context, projectID, repoID, branch, baseRef, key string) (usecase.Workspace, error) {
	resp, err := w.git.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: projectID, RepoId: repoID, Branch: branch, BaseRef: baseRef, IdempotencyKey: &key,
	})
	if err != nil {
		return usecase.Workspace{}, mapWorkspaceError("create the proposal worktree", err)
	}
	return usecase.Workspace{WorktreeID: resp.GetWorktreeId(), Path: resp.GetPath(), Branch: branch, BaseRef: baseRef}, nil
}

// repoFor takes the repo the caller named, else the project's default one (lowest position): the repo tasks execute on,
// so the proposal and the code it asks for live in the same repository.
func (w *ProposalWorkspace) repoFor(ctx context.Context, in usecase.EnsureWorkspaceInput) (string, error) {
	if in.RepoID != "" {
		return in.RepoID, nil
	}
	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := w.projects.ListRepos(pctx, &projectv1.ListReposRequest{ProjectId: in.ProjectID})
	if err != nil {
		return "", mapWorkspaceError("list the project's repos", err)
	}
	if len(resp.GetRepos()) == 0 {
		return "", fmt.Errorf("%w: project %s has no repo", domain.ErrEngineRepoNotResolved, in.ProjectID)
	}
	return resp.GetRepos()[0].GetId(), nil
}

func (w *ProposalWorkspace) ReadFile(ctx context.Context, ws usecase.Workspace, relPath string, maxBytes int) ([]byte, error) {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 || maxBytes > usecase.MaxProposalReadBytes {
		maxBytes = usecase.MaxProposalReadBytes
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := w.git.ReadFile(tctx, &gitgatewayv1.ReadFileRequest{WorktreeId: ws.WorktreeID, Path: clean})
	if err != nil {
		return nil, mapWorkspaceError("read "+clean, err)
	}
	if resp.GetEncoding() == "base64" {
		return nil, fmt.Errorf("%w: %s is not a text file", domain.ErrOpenSpecInvalidOutput, clean)
	}
	if len(resp.GetContent()) > maxBytes {
		return nil, fmt.Errorf("%w: %s is larger than %d bytes", domain.ErrOpenSpecInvalidOutput, clean, maxBytes)
	}
	return resp.GetContent(), nil
}

func (w *ProposalWorkspace) WriteFile(ctx context.Context, ws usecase.Workspace, relPath string, content []byte) error {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return err
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	if _, err := w.git.WriteFile(tctx, &gitgatewayv1.WriteFileRequest{WorktreeId: ws.WorktreeID, Path: clean, Content: content, Encoding: "utf8", CreateParents: true}); err != nil {
		return mapWorkspaceError("write "+clean, err)
	}
	return nil
}

func (w *ProposalWorkspace) Stat(ctx context.Context, ws usecase.Workspace, relPath string) (usecase.PathInfo, error) {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return usecase.PathInfo{}, err
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return usecase.PathInfo{}, err
	}
	resp, err := w.git.StatFile(tctx, &gitgatewayv1.StatFileRequest{WorktreeId: ws.WorktreeID, Path: clean})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return usecase.PathInfo{}, nil
		}
		return usecase.PathInfo{}, mapWorkspaceError("stat "+clean, err)
	}
	return usecase.PathInfo{Exists: resp.GetExists(), IsDir: resp.GetIsDirectory()}, nil
}

func (w *ProposalWorkspace) ListDir(ctx context.Context, ws usecase.Workspace, relPath string) ([]usecase.DirEntry, error) {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return nil, err
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := w.git.ReadDir(tctx, &gitgatewayv1.ReadDirRequest{WorktreeId: ws.WorktreeID, Path: clean})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, mapWorkspaceError("list "+clean, err)
	}
	out := make([]usecase.DirEntry, 0, len(resp.GetEntries()))
	for _, e := range resp.GetEntries() {
		out = append(out, usecase.DirEntry{Name: e.GetName(), IsDir: e.GetIsDirectory()})
	}
	return out, nil
}

func (w *ProposalWorkspace) ChangedPaths(ctx context.Context, ws usecase.Workspace) ([]usecase.ChangedPath, error) {
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := w.git.GetStatus(tctx, &gitgatewayv1.GetStatusRequest{WorktreeId: ws.WorktreeID})
	if err != nil {
		return nil, mapWorkspaceError("read the proposal status", err)
	}
	out := make([]usecase.ChangedPath, 0, len(resp.GetFiles()))
	for _, f := range resp.GetFiles() {
		out = append(out, usecase.ChangedPath{Path: f.GetPath(), OldPath: f.GetOldPath(), State: f.GetState()})
	}
	return out, nil
}

// DiscardAll resets every changed path. Untracked files are removed by git-gateway's Discard (git clean), tracked ones
// restored, and a path that fails to discard fails the call: a retry must start from a clean branch head.
func (w *ProposalWorkspace) DiscardAll(ctx context.Context, ws usecase.Workspace) error {
	changed, err := w.ChangedPaths(ctx, ws)
	if err != nil || len(changed) == 0 {
		return err
	}
	paths := make([]string, 0, len(changed))
	for _, c := range changed {
		paths = append(paths, c.Path)
		if c.OldPath != "" {
			paths = append(paths, c.OldPath)
		}
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	resp, err := w.git.BulkDiscard(tctx, &gitgatewayv1.BulkDiscardRequest{WorktreeId: ws.WorktreeID, Paths: paths})
	if err != nil {
		return mapWorkspaceError("discard changes", err)
	}
	if !resp.GetSuccess() {
		return fmt.Errorf("%w: could not discard %s", domain.ErrOpenSpecAgentFailed, strings.Join(resp.GetFailedPaths(), ", "))
	}
	return nil
}

// Commit commits exactly paths. git-gateway's Commit takes the staged set when paths is empty, which would sweep in
// anything else in the worktree, so an empty list is refused here.
func (w *ProposalWorkspace) Commit(ctx context.Context, ws usecase.Workspace, message string, paths []string) (string, error) {
	if len(paths) == 0 || strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("%w: a commit needs a message and the paths to commit", domain.ErrOpenSpecInvalidOutput)
	}
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		c, err := cleanRelPath(p)
		if err != nil {
			return "", err
		}
		clean = append(clean, c)
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	if _, err := w.git.Stage(tctx, &gitgatewayv1.StageRequest{WorktreeId: ws.WorktreeID, Paths: clean}); err != nil {
		return "", mapWorkspaceError("stage the proposal", err)
	}
	resp, err := w.git.Commit(tctx, &gitgatewayv1.CommitRequest{WorktreeId: ws.WorktreeID, Message: message, Paths: clean})
	if err != nil {
		return "", mapWorkspaceError("commit the proposal", err)
	}
	return resp.GetCommitSha(), nil
}

// Remove drops the worktree and never the branch: the proposal stays reviewable after cleanup.
func (w *ProposalWorkspace) Remove(ctx context.Context, ws usecase.Workspace) error {
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	_, err = w.git.RemoveWorktree(tctx, &gitgatewayv1.RemoveWorktreeRequest{WorktreeId: ws.WorktreeID, Force: true})
	if err != nil && status.Code(err) != codes.NotFound {
		return mapWorkspaceError("remove the proposal worktree", err)
	}
	return nil
}

// cleanRelPath accepts only a path inside the worktree, so a document name from an agent can never address another place.
func cleanRelPath(p string) (string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	c := path.Clean(p)
	if p == "" || strings.ContainsRune(p, 0) || len(c) > 512 || path.IsAbs(c) || c == ".." || strings.HasPrefix(c, "../") || c == "." {
		return "", fmt.Errorf("%w: %q is not a path inside the proposal", domain.ErrOpenSpecOutOfScopeChange, p)
	}
	return c, nil
}

func mapWorkspaceError(doing string, err error) error {
	var ae interface{ GRPCStatus() *status.Status }
	if errors.As(err, &ae) {
		switch ae.GRPCStatus().Code() {
		case codes.NotFound:
			return fmt.Errorf("%w: could not %s: %v", domain.ErrEngineRepoNotResolved, doing, err)
		case codes.Unavailable, codes.DeadlineExceeded:
			return fmt.Errorf("%w: could not %s: %v", domain.ErrEngineNoConnection, doing, err)
		}
	}
	return fmt.Errorf("%w: could not %s: %v", domain.ErrOpenSpecAgentFailed, doing, err)
}
