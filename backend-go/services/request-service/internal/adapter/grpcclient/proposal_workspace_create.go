package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure creates the proposal worktree, or returns the one that exists. The base is in.BaseRef when the Request has one, else the
// repository default; the same base is used for the first creation and for a recreation after the worktree was removed, so the
// branch cannot start somewhere else the second time.
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
	base, err := w.startFor(tctx, repoID, in.BaseRef)
	if err != nil {
		return usecase.Workspace{}, err
	}
	branch, err := w.branchFor(ctx, tctx, in, repoID)
	if err != nil {
		return usecase.Workspace{}, err
	}
	key := "request-proposal:" + in.RequestID
	ws, err := w.create(tctx, in.ProjectID, repoID, branch, base, key)
	if errors.Is(err, errProposalWorktreeExists) {
		// A worktree made before the gateway stored the key with its record has nothing to find by key; the project's own
		// records still say which worktree holds this Request's branch.
		var found bool
		ws, found, err = w.existingFor(ctx, in.ProjectID, repoID, branch, base.ref)
		if err == nil && !found && branch == proposalBranch(in.RequestNumber) {
			// The path belongs to something no record of this Request knows (left by an earlier database, or another project on
			// the same repository): build on the Request's own name instead of failing the run.
			own := domain.ProposalBranchFor(in.RequestNumber, in.RequestID)
			slog.WarnContext(ctx, "the proposal worktree path is taken by a worktree no record knows; using the Request's own branch",
				slog.String("request_id", in.RequestID), slog.String("taken", branch), slog.String("branch", own))
			branch = own
			ws, err = w.create(tctx, in.ProjectID, repoID, branch, base, key)
			if errors.Is(err, errProposalWorktreeExists) {
				ws, found, err = w.existingFor(ctx, in.ProjectID, repoID, branch, base.ref)
			}
		}
		if err == nil && !found && ws.WorktreeID == "" {
			err = fmt.Errorf("%w: could not create the proposal worktree: its path exists but no record of this Request's branch was found", domain.ErrOpenSpecAgentFailed)
		}
	}
	if err != nil {
		return usecase.Workspace{}, err
	}
	// The idempotency key also matches a worktree that was removed since: such a record answers with a path that is gone.
	// A status call tells the two apart, and a fresh key creates the worktree again on the surviving branch.
	if _, serr := w.git.GetStatus(tctx, &gitgatewayv1.GetStatusRequest{WorktreeId: ws.WorktreeID}); serr != nil {
		ws, err = w.create(tctx, in.ProjectID, repoID, branch, base, fmt.Sprintf("%s:%d", key, w.now().UnixNano()))
		if err != nil {
			return usecase.Workspace{}, err
		}
	}
	return ws, nil
}

// branchFor picks the proposal branch. The usual name is request/<n>-proposal; when a branch of that name already exists and no
// worktree record of this project holds it, it was left by something else (Request numbers start again after a database reset,
// two projects can share a repository), and git-gateway would check it out and build the new change on its old commits. The
// Request's own name avoids that. A failed lookup keeps the usual name: the create call is still the final judge.
func (w *ProposalWorkspace) branchFor(ctx, tctx context.Context, in usecase.EnsureWorkspaceInput, repoID string) (string, error) {
	branch := proposalBranch(in.RequestNumber)
	own := domain.ProposalBranchFor(in.RequestNumber, in.RequestID)
	refs, err := w.git.SearchRefs(tctx, &gitgatewayv1.SearchRefsRequest{RepoId: repoID, Query: branch})
	if err != nil {
		slog.WarnContext(ctx, "could not look up the proposal branch; keeping the usual name", slog.String("branch", branch), slog.Any("error", err))
		return branch, nil
	}
	exists, ownExists := false, false
	for _, r := range refs.GetRefs() {
		switch strings.TrimPrefix(r, "heads/") {
		case branch:
			exists = true
		case own:
			ownExists = true
		}
	}
	if ownExists {
		return own, nil // an earlier run of this Request already moved to its own name
	}
	if !exists {
		return branch, nil
	}
	if _, known, err := w.recordedFor(ctx, in.ProjectID, repoID, branch); err != nil || known {
		return branch, err
	}
	slog.WarnContext(ctx, "the proposal branch exists but no worktree record of this project holds it; using the Request's own branch",
		slog.String("request_id", in.RequestID), slog.String("taken", branch), slog.String("branch", own))
	return own, nil
}

// recordedFor says whether project-service has any worktree record (active or not) of branch on this repo.
func (w *ProposalWorkspace) recordedFor(ctx context.Context, projectID, repoID, branch string) (string, bool, error) {
	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return "", false, err
	}
	resp, err := w.projects.ListWorktrees(pctx, &projectv1.ListWorktreesRequest{ProjectId: projectID})
	if err != nil {
		return "", false, mapWorkspaceError("list the project's worktrees", err)
	}
	for _, wt := range resp.GetWorktrees() {
		if wt.GetBranch() == branch && (wt.GetRepoId() == repoID || wt.GetRepoId() == "") {
			return wt.GetId(), true, nil
		}
	}
	return "", false, nil
}

// startPoint is where the proposal branch starts. explicit marks a ref the person chose.
type startPoint struct {
	ref      string
	explicit bool
}

// candidates are the spellings handed to `git worktree add`, in order. A chosen name is never passed bare: a branch the clone
// knows only as origin/<name> would make git create a LOCAL branch called <name> and check that out instead of the new
// proposal branch (seen with git 2.54), so the worktree would sit on, and later commit to, the person's release branch. The
// fully qualified forms resolve or fail cleanly, and the first that resolves wins: local branch, remote-tracking branch, tag.
func (s startPoint) candidates() []string {
	if !s.explicit || strings.HasPrefix(s.ref, "refs/") {
		return []string{s.ref}
	}
	out := []string{"refs/heads/" + s.ref, "refs/remotes/origin/" + s.ref, "refs/tags/" + s.ref}
	if strings.HasPrefix(s.ref, "origin/") {
		out = append(out, "refs/remotes/"+s.ref) // typed as the remote-tracking name already
	}
	return out
}

func (w *ProposalWorkspace) startFor(ctx context.Context, repoID, requested string) (startPoint, error) {
	if ref := strings.TrimSpace(requested); ref != "" {
		return startPoint{ref: ref, explicit: true}, nil
	}
	def, err := w.git.BaseRefDefault(ctx, &gitgatewayv1.BaseRefDefaultRequest{RepoId: repoID})
	if err != nil {
		return startPoint{}, mapWorkspaceError("resolve the default branch", err)
	}
	return startPoint{ref: def.GetRef()}, nil
}

// errProposalWorktreeExists says git-gateway refused to create the worktree because its path is already taken.
var errProposalWorktreeExists = errors.New("the proposal worktree path already exists")

// create asks git-gateway for the worktree, trying each spelling of the start point until one resolves. When none does, the
// error is REQUEST_BASE_REF_NOT_FOUND naming the ref the person chose, which the run shows to them; any other failure stops at once.
func (w *ProposalWorkspace) create(ctx context.Context, projectID, repoID, branch string, base startPoint, key string) (usecase.Workspace, error) {
	var missing error
	for _, spelling := range base.candidates() {
		ws, err := w.createFrom(ctx, projectID, repoID, branch, spelling, base.ref, key)
		if err == nil || !domain.IsBaseRefNotFound(err) {
			return ws, err
		}
		missing = err
	}
	return usecase.Workspace{}, missing
}

func (w *ProposalWorkspace) createFrom(ctx context.Context, projectID, repoID, branch, baseRef, chosen, key string) (usecase.Workspace, error) {
	resp, err := w.git.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: projectID, RepoId: repoID, Branch: branch, BaseRef: baseRef, IdempotencyKey: &key,
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return usecase.Workspace{}, errProposalWorktreeExists
		}
		if suggestions, missing := gatewayBaseRefMissing(err, chosen); missing {
			return usecase.Workspace{}, domain.ErrRequestBaseRefNotFound(chosen, suggestions)
		}
		return usecase.Workspace{}, mapWorkspaceError("create the proposal worktree", err)
	}
	return usecase.Workspace{WorktreeID: resp.GetWorktreeId(), Path: resp.GetPath(), Branch: branch, BaseRef: baseRef}, nil
}

// existingFor finds the active worktree project-service has recorded for the Request's proposal branch on this repo. It keeps the
// base that worktree was cut from: the Request's ref is frozen once a worktree exists, so a different current value is only logged.
func (w *ProposalWorkspace) existingFor(ctx context.Context, projectID, repoID, branch, baseRef string) (usecase.Workspace, bool, error) {
	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return usecase.Workspace{}, false, err
	}
	resp, err := w.projects.ListWorktrees(pctx, &projectv1.ListWorktreesRequest{ProjectId: projectID})
	if err != nil {
		return usecase.Workspace{}, false, mapWorkspaceError("list the project's worktrees", err)
	}
	for _, wt := range resp.GetWorktrees() {
		if wt.GetActive() && wt.GetBranch() == branch && (wt.GetRepoId() == repoID || wt.GetRepoId() == "") {
			kept := baseRef
			if recorded := wt.GetBaseRef(); recorded != "" {
				kept = recorded
				if recorded != baseRef {
					slog.InfoContext(ctx, "the proposal worktree keeps the base it was cut from", slog.String("branch", branch),
						slog.String("worktree_base", recorded), slog.String("requested_base", baseRef))
				}
			}
			return usecase.Workspace{WorktreeID: wt.GetId(), Path: wt.GetPath(), Branch: branch, BaseRef: kept}, true, nil
		}
	}
	return usecase.Workspace{}, false, nil
}

// repoFor takes the repo the caller named, else the project's default one.
func (w *ProposalWorkspace) repoFor(ctx context.Context, in usecase.EnsureWorkspaceInput) (string, error) {
	if in.RepoID != "" {
		return in.RepoID, nil
	}
	return defaultRepoID(ctx, w.projects, in.ProjectID)
}
