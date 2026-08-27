# TASK-WT-01-05: Wire BR-WT-01/04 and [A1]/[A2]/[A3] into `CreateWorktree.Execute`

**From Solution:** SOL-WT-01
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`
**Depends on:** TASK-WT-01-01, TASK-WT-01-02, TASK-WT-01-03, TASK-WT-01-04
**Status:** `[x]` DONE — create_worktree.go rewritten with BR-WT-01/04 + [A1]/[A2] validation (lineage-capture struct dropped — doesn't exist in this codebase's actual CreateWorktreeRequest, which only has fields 1-4); wscompat worktree.create forwards name/path. go build+test clean.

---

## Context

This is the integration step: [SOL-WT-01](../solutions/SOL-WT-01-tao-worktree.md)'s validate-before-dispatch step, added as a pre-check block ahead of `CreateWorktree.Execute`'s existing resolve→dispatch→translate body (`create_worktree.go:41-71`), without changing that shape — same pattern `git-gateway-service.md` §2 already allows (a fourth "validate" step, per SOL-WT-01's rationale).

## Changes to make

Replace `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`'s `CreateWorktreeInput` and `Execute`:

```go
type CreateWorktreeInput struct {
	ProjectID, RepoID, Branch, BaseRef, Name, Path string
	Lineage                                        domain.WorktreeLineageCapture
}

func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
	name := in.Name
	if name == "" {
		name = sanitizeBranchForPathUsecase(in.Branch)
	}
	if err := domain.ValidateWorktreeName(name); err != nil { // BR-WT-01
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_NAME_INVALID", err.Error(), err)
	}

	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-WT-04 — count active worktrees for this repo before attempting
	// git. Fails OPEN on a ListWorktrees error: a transient bookkeeping
	// read failure must not block worktree creation.
	if existing, err := uc.projects.ListWorktrees(ctx, in.ProjectID); err == nil {
		count := 0
		for _, w := range existing {
			if w.RepoID == in.RepoID && w.Active {
				count++
			}
		}
		if count >= 20 {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_LIMIT_EXCEEDED", "maximum 20 worktrees per repository", nil)
		}
	}

	// [A1] — duplicate-path pre-check + alternate-name suggestion via the
	// already-required ListWorktreePaths; best-effort, git itself is still
	// the final authority if this call fails.
	onDisk, _ := executor.ListWorktreePaths(ctx, repoPath)
	taken := make(map[string]bool, len(onDisk))
	for _, p := range onDisk {
		taken[p] = true
	}
	targetPath := in.Path
	if targetPath == "" {
		targetPath = repoPath + "-" + name
	}
	if taken[targetPath] {
		suggested := domain.SuggestAlternateName(name, taken)
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindAlreadyExists, "WORKTREE_PATH_EXISTS",
			fmt.Sprintf("path already exists; try %q", suggested), nil)
	}

	result, err := executor.CreateWorktree(ctx, repoPath, in.Branch, in.BaseRef, targetPath)
	if err != nil {
		if isBaseRefNotFoundErr(err) { // [A2]
			if branches, listErr := executor.ListLocalBranches(ctx, repoPath); listErr == nil {
				names := make([]string, 0, len(branches))
				for _, b := range branches {
					names = append(names, b.Name)
				}
				return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_BASE_REF_NOT_FOUND",
					fmt.Sprintf("branch %q not found; available: %s", in.BaseRef, strings.Join(names, ", ")), err)
			}
		}
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", "git worktree add failed", err)
	}

	worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch, in.Lineage)
	if err != nil {
		if compErr := executor.RemoveWorktree(ctx, result.Path, true); compErr != nil {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED",
				fmt.Sprintf("worktree created but bookkeeping failed (%v) and rollback also failed (%v) — orphaned at %s, will surface via worktree.detectedList", err, compErr, result.Path), err)
		}
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED", "worktree created but bookkeeping failed; rolled back cleanly", err)
	}
	return domain.WorktreeResult{WorktreeID: worktree.ID, Path: result.Path, HeadSHA: result.HeadSHA}, nil
}

// isBaseRefNotFoundErr classifies git's stderr — same pragmatic string-match
// approach this package already uses elsewhere (e.g. localgit's
// strings.HasPrefix(baseRef, "-") flag-injection guard).
func isBaseRefNotFoundErr(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "invalid reference") || strings.Contains(msg, "unknown revision") || strings.Contains(msg, "not a valid object name")
}

// sanitizeBranchForPathUsecase mirrors localgit.sanitizeBranchForPath — this
// package cannot import an internal/adapter/localgit unexported helper, so
// it's duplicated here as a small, obviously-equivalent function rather than
// exporting one across a layer boundary this package doesn't otherwise
// depend on.
func sanitizeBranchForPathUsecase(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}
```

Add `"strings"` to the file's imports alongside the existing `"context"`/`"fmt"`.

Update the gRPC adapter call site, `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go`'s `CreateWorktree` handler (`server.go:678-695`), to pass the two new fields through and return `SuggestedName` on the `WORKTREE_PATH_EXISTS` path:

```go
func (s *Server) CreateWorktree(ctx context.Context, req *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error) {
	result, err := s.createWorktree.Execute(ctx, usecase.CreateWorktreeInput{
		ProjectID: req.GetProjectId(), RepoID: req.GetRepoId(), Branch: req.GetBranch(), BaseRef: req.GetBaseRef(),
		Name: req.GetName(), Path: req.GetPath(),
		Lineage: domain.WorktreeLineageCapture{
			ParentWorktreeID:        req.GetParentWorktreeId(),
			Origin:                  req.GetOrigin(),
			CaptureSource:           req.GetCaptureSource(),
			TaskID:                  req.GetTaskId(),
			OrchestrationRunID:      req.GetOrchestrationRunId(),
			CoordinatorHandle:       req.GetCoordinatorHandle(),
			CreatedByTerminalHandle: req.GetCreatedByTerminalHandle(),
		},
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CreateWorktreeResponse{WorktreeId: result.WorktreeID, Path: result.Path, HeadSha: result.HeadSHA}, nil
}
```

Update `wscompat`'s `worktree.create` channel (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:36-74`) to accept/forward `name`/`path`:

```go
		type createArgs struct {
			ProjectID string `json:"projectId"`
			RepoID    string `json:"repoId"`
			Branch    string `json:"branch"`
			BaseRef   string `json:"baseRef"`
			Name      string `json:"name"`
			Path      string `json:"path"`
			// ...existing lineage fields unchanged...
		}
		// ...
		resp, err := gitClient.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
			ProjectId: in.ProjectID, RepoId: in.RepoID, Branch: in.Branch, BaseRef: in.BaseRef,
			Name: nonEmptyPtr(in.Name), Path: nonEmptyPtr(in.Path),
			// ...existing lineage fields unchanged...
		})
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/... ./services/api-gateway/...
```

Expected: clean build once [TASK-WT-01-01](./TASK-WT-01-01-proto-name-path-suggested-name.md)'s stubs are regenerated and [TASK-WT-01-02](./TASK-WT-01-02-domain-validate-name-suggest-alternate.md)/[TASK-WT-01-03](./TASK-WT-01-03-project-client-list-worktrees.md)/[TASK-WT-01-04](./TASK-WT-01-04-executor-target-path-diskspace.md) are already applied. Full behavior tests land in [TASK-WT-01-07](./TASK-WT-01-07-tests.md).
