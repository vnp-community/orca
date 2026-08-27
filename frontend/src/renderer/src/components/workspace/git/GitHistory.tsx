import { useEffect } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useAppStore } from '../../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../../runtime/runtime-rpc-client'
import { toRuntimeWorktreeSelector } from '../../../runtime/runtime-worktree-selector'
import type { GitCommit } from '../../../store/slices/git-panel'
import type { GitHistoryResult } from '../../../../../shared/git-history'

export function GitHistory() {
  const { currentWorktree } = useWorkspace()
  const gitHistory = useAppStore((s) => s.gitHistory)

  // Why (same crash class as GitPanel.tsx's push): this called the nonexistent
  // 'git.getLog' with a {projectId} shape — the real method is 'git.history'
  // and requires a {worktree} selector (backend/src/main/runtime/rpc/methods/git.ts).
  useEffect(() => {
    if (!currentWorktree) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<GitHistoryResult>(target, 'git.history', {
      worktree: toRuntimeWorktreeSelector(currentWorktree.id),
      limit: 50
    })
      .then((result) => {
        const commits: GitCommit[] = result.items.map((item) => ({
          hash: item.id,
          shortHash: item.displayId ?? item.id.slice(0, 7),
          message: item.subject,
          author: item.author ?? 'unknown',
          date: item.timestamp ?? 0
        }))
        useAppStore.getState().setGitHistory(commits)
      })
      .catch(() => {
        // Silently fail — panel shows the empty state below
      })
  }, [currentWorktree])

  return (
    <div className="git-history p-2" data-testid="git-history">
      {gitHistory.map((commit) => (
        <div key={commit.hash} className="py-2 border-b last:border-0">
          <div className="flex items-baseline gap-2">
            <code className="text-xs text-muted-foreground font-mono">{commit.shortHash}</code>
            <span className="text-sm truncate">{commit.message}</span>
          </div>
          <div className="text-xs text-muted-foreground mt-0.5">
            {commit.author} · {new Date(commit.date).toLocaleDateString()}
          </div>
        </div>
      ))}
      {gitHistory.length === 0 && (
        <div className="text-sm text-muted-foreground py-4 text-center">No commits yet</div>
      )}
    </div>
  )
}
