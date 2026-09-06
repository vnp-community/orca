// GitPanel.tsx (TDD-FE-12, TASK-FE-011)
import { useState, lazy, Suspense } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useGit } from '../../../hooks/useGit'
import { useAppStore } from '../../../store'
import { getRepoIdFromWorktreeId } from '../../../../../shared/worktree-id'
import type { GitStatus } from '../../../types/workspace-types'
import { StagingArea } from './StagingArea'
import { CommitForm } from './CommitForm'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'

// Why (CR-PW-001): a single hardcoded "(no branch)" string used to cover 3 unrelated states
// (RPC failure, detached HEAD, and "no worktree selected" — the last now handled by
// WorkspaceLayout gating GitPanel behind currentWorktree). Telling them apart avoids reporting a
// transient host/relay failure as if the repo genuinely had no branch.
function describeBranch(gitStatus: GitStatus | null, gitStatusError: boolean): string {
  if (gitStatusError) {
    return 'Git status unavailable'
  }
  if (gitStatus?.branch) {
    return gitStatus.branch
  }
  if (gitStatus?.branchUnavailable === 'detached-head') {
    return 'Detached HEAD'
  }
  if (gitStatus?.branchUnavailable === 'status-unavailable') {
    return 'Git unavailable'
  }
  return '—'
}

const GitHistory = lazy(() => import('./GitHistory').then((m) => ({ default: m.GitHistory })))
const BranchManager = lazy(() =>
  import('./BranchManager').then((m) => ({ default: m.BranchManager }))
)
const DiffViewer = lazy(() => import('./DiffViewer').then((m) => ({ default: m.DiffViewer })))
const PullRequestList = lazy(() =>
  import('./PullRequestList').then((m) => ({ default: m.PullRequestList }))
)

type GitTab = 'changes' | 'history' | 'branches' | 'pullrequests'

export function GitPanel() {
  const { gitStatus, gitStatusError, project, currentWorktree, emit } = useWorkspace()
  const { getDiff, push, isPushing } = useGit()
  const [activeTab, setActiveTab] = useState<GitTab>('changes')
  const [selectedDiff, setSelectedDiff] = useState<string | null>(null)

  // Why (CR-PW-002): a project can have several repos (AssignRepoToProject); label which one
  // this branch belongs to instead of adding a second worktree picker — Workspace intentionally
  // reuses the sidebar's selection as its only picker (roadmap decision #8).
  const currentRepo = useAppStore((s) =>
    currentWorktree
      ? s.repos.find((r) => r.id === getRepoIdFromWorktreeId(currentWorktree.id))
      : undefined
  )

  // Why (crash reported by user): this used to build its own callRuntimeRpc('git.push', …)
  // call with a {projectId, branch, remote} shape the backend has never accepted
  // (real schema: {worktree, publish?, pushTarget?, forceWithLease?} — see
  // backend/src/main/runtime/rpc/methods/git-params.ts). useGit().push() already
  // sends the correct worktree-scoped request (FIX BUG-FE-HLD-002) — reuse it
  // instead of duplicating a second, broken push implementation.
  const handleSync = async () => {
    if (!project || !gitStatus || !currentWorktree) {
      return
    }
    try {
      await push(gitStatus.branch ?? 'main')
      emit('git.push', { branch: gitStatus.branch ?? 'main' })
      toast.success('Push complete')
    } catch (err: unknown) {
      toast.error(`Push failed: ${err instanceof Error ? err.message : 'unknown error'}`)
    }
  }

  const handleViewDiff = async (path: string) => {
    setSelectedDiff(path)
    await getDiff(path)
  }

  const TABS: { id: GitTab; label: string }[] = [
    { id: 'changes', label: 'Changes' },
    { id: 'history', label: 'History' },
    { id: 'branches', label: 'Branches' },
    { id: 'pullrequests', label: 'Pull Requests' }
  ]

  return (
    <div className="git-panel flex flex-col h-full" data-testid="git-panel">
      {/* Header: branch info + sync */}
      <div className="flex items-center gap-2 px-3 py-2 border-b bg-muted/30 text-sm">
        {currentRepo && (
          <span className="text-xs text-muted-foreground" data-testid="git-panel-repo-label">
            {currentRepo.displayName}
          </span>
        )}
        <span className="font-mono text-xs font-medium" data-testid="git-panel-branch">
          {describeBranch(gitStatus, gitStatusError)}
        </span>
        {gitStatus && (
          <span className="text-xs text-muted-foreground">
            &uarr;{gitStatus.aheadBy ?? 0} &darr;{gitStatus.behindBy ?? 0}
          </span>
        )}
        <button
          onClick={handleSync}
          disabled={isPushing}
          className="ml-auto flex items-center gap-1 text-xs px-2 py-1 border rounded hover:bg-accent disabled:opacity-50"
          data-testid="sync-button"
        >
          {isPushing && <Loader2 size={10} className="animate-spin" />}
          Sync
        </button>
      </div>

      {/* Tab bar */}
      <div className="flex border-b text-sm shrink-0">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            data-testid={`git-tab-${tab.id}`}
            className={`px-3 py-2 text-xs border-b-2 transition-colors ${
              activeTab === tab.id
                ? 'border-primary text-primary font-medium'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-auto">
        <Suspense fallback={<div className="p-4 text-sm text-muted-foreground">Loading...</div>}>
          {activeTab === 'changes' && (
            <div>
              <StagingArea onViewDiff={handleViewDiff} />
              <CommitForm />
              {selectedDiff && (
                <div className="border-t">
                  <DiffViewer filePath={selectedDiff} />
                </div>
              )}
            </div>
          )}
          {activeTab === 'history' && <GitHistory />}
          {activeTab === 'branches' && <BranchManager />}
          {activeTab === 'pullrequests' && <PullRequestList />}
        </Suspense>
      </div>

      {/* Push progress output */}
      {isPushing && (
        <div
          className="push-progress px-3 py-2 bg-muted border-t text-xs font-mono overflow-auto max-h-24"
          data-testid="push-progress"
        >
          Pushing...
        </div>
      )}
    </div>
  )
}
