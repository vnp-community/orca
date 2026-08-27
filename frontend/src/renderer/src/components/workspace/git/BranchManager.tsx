import { useCallback, useEffect } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useAppStore } from '../../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../../runtime/runtime-rpc-client'
import { toRuntimeWorktreeSelector } from '../../../runtime/runtime-worktree-selector'
import { Button } from '../../ui/button'
import { Badge } from '../../ui/badge'
import { GitBranch } from 'lucide-react'
import { toast } from 'sonner'
import type { RuntimeGitLocalBranches } from '../../../../../shared/runtime-types'
import type { GitBranch as GitBranchRow } from '../../../store/slices/git-panel'

// Lists local branches + checkout. Branch CREATION intentionally has no UI
// here: Orca's git model is worktree-per-branch (a new branch is checked out
// by creating a new worktree, not by switching branches in place) and the
// backend has no "create a branch in the current worktree" RPC — only
// checkoutRuntimeGitBranch(existing branch) exists
// (backend/src/main/runtime/orca-runtime-git.ts). Surfacing a "Create" button
// against a nonexistent method silently broke the same way the checkout list
// did (BUG-FE-HLD, GitPanel.tsx crash report) — left out rather than faked.

export function BranchManager() {
  const { currentWorktree } = useWorkspace()
  const branches = useAppStore((s) => s.branches)

  const fetchBranches = useCallback(async () => {
    if (!currentWorktree) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const result = await callRuntimeRpc<RuntimeGitLocalBranches>(target, 'git.localBranches', {
      worktree: toRuntimeWorktreeSelector(currentWorktree.id)
    })
    const rows: GitBranchRow[] = result.branches.map((name) => ({
      name,
      isRemote: false,
      isCurrent: name === result.current,
      aheadBy: 0,
      behindBy: 0
    }))
    useAppStore.getState().setBranches(rows)
  }, [currentWorktree])

  useEffect(() => {
    fetchBranches().catch(() => {
      // Silently fail — panel shows the empty state below
    })
  }, [fetchBranches])

  const checkout = async (branch: string) => {
    if (!currentWorktree) {
      return
    }
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'git.checkout', {
        worktree: toRuntimeWorktreeSelector(currentWorktree.id),
        branch
      })
      await fetchBranches()
    } catch (err: unknown) {
      toast.error(`Checkout failed: ${err instanceof Error ? err.message : 'unknown error'}`)
    }
  }

  return (
    <div className="branch-manager p-2 space-y-3" data-testid="branch-manager">
      {/* Branch list */}
      <div className="space-y-0.5">
        {branches.map((b) => (
          <div
            key={b.name}
            className={`flex items-center gap-2 px-2 py-1.5 rounded text-sm ${b.isCurrent ? 'bg-accent' : 'hover:bg-accent/50'}`}
            data-testid={`branch-${b.name}`}
          >
            <GitBranch size={12} className="text-muted-foreground shrink-0" />
            <span className="flex-1 truncate">{b.name}</span>
            {b.isCurrent && <Badge className="text-xs">current</Badge>}
            {!b.isCurrent && (
              <Button
                size="sm"
                variant="ghost"
                className="h-5 text-xs"
                onClick={() => checkout(b.name)}
              >
                Checkout
              </Button>
            )}
          </div>
        ))}
        {branches.length === 0 && (
          <div className="text-sm text-muted-foreground py-4 text-center">No branches found</div>
        )}
      </div>
    </div>
  )
}
