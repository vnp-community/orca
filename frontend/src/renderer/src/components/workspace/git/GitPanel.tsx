// GitPanel.tsx (TDD-FE-12, TASK-FE-011)
import { useState, lazy, Suspense } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useGit } from '../../../hooks/useGit'
import { StagingArea } from './StagingArea'
import { CommitForm } from './CommitForm'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'

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
  const { gitStatus, project, currentWorktree, emit } = useWorkspace()
  const { getDiff, push, isPushing } = useGit()
  const [activeTab, setActiveTab] = useState<GitTab>('changes')
  const [selectedDiff, setSelectedDiff] = useState<string | null>(null)

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
        <span className="font-mono text-xs font-medium">{gitStatus?.branch ?? '(no branch)'}</span>
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
