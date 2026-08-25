package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type ScanSetupScriptImportsInput struct {
	WorktreeID string
}

// ScanSetupScriptImports follows GetStatus's exact resolve -> dispatch -> translate shape.
type ScanSetupScriptImports struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewScanSetupScriptImports(resolver ConnectionResolver, local, relay GitExecutor) *ScanSetupScriptImports {
	return &ScanSetupScriptImports{resolver: resolver, local: local, relay: relay}
}

func (uc *ScanSetupScriptImports) Execute(ctx context.Context, in ScanSetupScriptImportsInput) ([]string, error) {
	if in.WorktreeID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	paths, err := executor.ScanSetupScriptImports(ctx, repoPath)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_SCAN_SETUP_SCRIPT_IMPORTS_FAILED", "failed to scan setup script imports", err)
	}
	return paths, nil
}
