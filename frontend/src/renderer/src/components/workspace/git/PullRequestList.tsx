// NEW: src/renderer/src/components/workspace/git/PullRequestList.tsx
import { useState, useEffect, useCallback } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'
import { Badge } from '../../ui/badge'
import { Button } from '../../ui/button'
import { ExternalLink, GitPullRequest, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'

type PullRequest = {
  number: number
  title: string
  state: 'open' | 'closed' | 'merged'
  url: string
  author: string
  baseBranch: string
  headBranch: string
  createdAt: string
  isDraft?: boolean
  reviewDecision?: 'APPROVED' | 'CHANGES_REQUESTED' | 'REVIEW_REQUIRED' | null
}

const REVIEW_COLORS: Record<string, string> = {
  APPROVED: 'text-green-600',
  CHANGES_REQUESTED: 'text-red-600',
  REVIEW_REQUIRED: 'text-yellow-600',
}

export function PullRequestList() {
  const { project } = useWorkspace()
  const [prs, setPrs] = useState<PullRequest[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isRefreshing, setIsRefreshing] = useState(false)

  const fetchPRs = useCallback(async (isRefresh = false) => {
    if (!project) {return}
    if (isRefresh) {setIsRefreshing(true)}
    else {setIsLoading(true)}

    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<PullRequest[]>(target, 'git.pr.list', {
        projectId: project.id,
        state: 'open',
      })
      setPrs(result)
    } catch (err: any) {
      if (!isRefresh) {
        toast.error(`Failed to load pull requests: ${  err.message ?? 'unknown error'}`)
      }
    } finally {
      setIsLoading(false)
      setIsRefreshing(false)
    }
  }, [project])

  useEffect(() => { fetchPRs() }, [fetchPRs])

  if (!project) {
    return (
      <div className="p-3 text-xs text-muted-foreground" data-testid="pr-no-project">
        No project selected
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="p-3 text-xs text-muted-foreground" data-testid="pr-loading">
        Loading pull requests...
      </div>
    )
  }

  return (
    <div className="pr-list" data-testid="pr-list">
      {/* Header with refresh */}
      <div className="flex items-center justify-between px-3 py-2 border-b">
        <span className="text-xs text-muted-foreground">
          {prs.length} open pull request{prs.length !== 1 ? 's' : ''}
        </span>
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={() => fetchPRs(true)}
          disabled={isRefreshing}
          data-testid="pr-refresh"
        >
          <RefreshCw size={10} className={isRefreshing ? 'animate-spin' : ''} />
        </Button>
      </div>

      {/* Empty state */}
      {prs.length === 0 && (
        <div className="flex flex-col items-center py-8 gap-2" data-testid="pr-empty">
          <GitPullRequest size={24} className="text-muted-foreground opacity-30" />
          <p className="text-sm text-muted-foreground">No open pull requests</p>
        </div>
      )}

      {/* PR items */}
      <div className="divide-y">
        {prs.map(pr => (
          <div
            key={pr.number}
            className="px-3 py-3 hover:bg-accent/30 transition-colors"
            data-testid={`pr-item-${pr.number}`}
          >
            <div className="flex items-start gap-2">
              <GitPullRequest
                size={14}
                className={`mt-0.5 shrink-0 ${pr.isDraft ? 'text-muted-foreground' : 'text-green-600'}`}
              />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium leading-tight truncate">{pr.title}</p>
                <p className="text-xs text-muted-foreground mt-0.5">
                  #{pr.number} · {pr.author} · {pr.headBranch} &rarr; {pr.baseBranch}
                </p>
                {pr.isDraft && (
                  <Badge variant="outline" className="text-xs mt-1 text-muted-foreground">
                    Draft
                  </Badge>
                )}
                {pr.reviewDecision && (
                  <span className={`text-xs mt-1 block ${REVIEW_COLORS[pr.reviewDecision] ?? ''}`}>
                    {pr.reviewDecision.replace(/_/g, ' ').toLowerCase()}
                  </span>
                )}
              </div>
              <a
                href={pr.url}
                target="_blank"
                rel="noopener noreferrer"
                onClick={e => e.stopPropagation()}
                data-testid={`pr-link-${pr.number}`}
              >
                <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0">
                  <ExternalLink size={10} />
                </Button>
              </a>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
