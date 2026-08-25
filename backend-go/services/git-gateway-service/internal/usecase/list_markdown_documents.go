package usecase

import (
	"context"
	"strings"
)

// ListMarkdownDocumentsUseCase is a thin wrapper over the same
// FilesystemExecutor.Glob ListAllFilesUseCase uses, filtered to
// *.md/*.mdx server-side — not a duplicate walk implementation.
type ListMarkdownDocumentsUseCase struct {
	resolver ConnectionResolver
	local    FilesystemExecutor
	relay    FilesystemExecutor
}

func NewListMarkdownDocumentsUseCase(resolver ConnectionResolver, local, relay FilesystemExecutor) *ListMarkdownDocumentsUseCase {
	return &ListMarkdownDocumentsUseCase{resolver: resolver, local: local, relay: relay}
}

func (uc *ListMarkdownDocumentsUseCase) Execute(ctx context.Context, worktreeID string, maxResults int) ([]string, error) {
	exec, conn, err := dispatchFilesystemExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return nil, err
	}
	all, err := exec.Glob(ctx, conn.RepoPath, "", 0) // unfiltered walk; maxResults applied after the .md/.mdx filter below
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(all))
	for _, p := range all {
		if strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".mdx") {
			out = append(out, p)
			if maxResults > 0 && len(out) >= maxResults {
				break
			}
		}
	}
	return out, nil
}
