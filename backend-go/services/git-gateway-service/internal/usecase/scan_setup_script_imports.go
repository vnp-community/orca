package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type ScanSetupScriptImportsInput struct {
	RepoID string
}

// ScanSetupScriptImports resolves repoID's owning host and scans its setup
// script imports. Repo-scoped — see CheckHooks' doc comment for why.
type ScanSetupScriptImports struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewScanSetupScriptImports(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *ScanSetupScriptImports {
	return &ScanSetupScriptImports{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *ScanSetupScriptImports) Execute(ctx context.Context, in ScanSetupScriptImportsInput) ([]string, error) {
	if in.RepoID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REPO_ID", "repo_id is required", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve repo's owning host", err)
	}
	paths, err := executor.ScanSetupScriptImports(ctx, repoPath)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_SCAN_SETUP_SCRIPT_IMPORTS_FAILED", "failed to scan setup script imports", err)
	}
	return paths, nil
}
