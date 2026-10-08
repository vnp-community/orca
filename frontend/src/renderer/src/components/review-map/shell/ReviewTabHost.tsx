import { Suspense, useCallback } from 'react'
import { lazyWithRetry as lazy } from '@/lib/lazy-with-retry'
import { useAppStore } from '@/store'
import { toast } from 'sonner'
import { openReviewDiffAtSymbol } from '@/lib/review-diff-navigation'
import { translate } from '@/i18n/i18n'
import { ReviewLoadingStage } from './ReviewLoadingStage'

const ReviewWorkspace = lazy(() => import('./ReviewWorkspace'))

type Props = {
  worktreeId: string
  /** Unified tab id; closing the tab is offered on unsupported/disabled screens. */
  tabId: string
}

/** Mounts the Review workspace inside a tab group body (code-split: only loads when a review tab shows). */
export function ReviewTabHost({ worktreeId, tabId }: Props): React.JSX.Element {
  const closeUnifiedTab = useAppStore((s) => s.closeUnifiedTab)

  const onOpenDiff = useCallback(
    (path: string, line?: number) => {
      const scope = useAppStore.getState().reviewUiByWorktree[worktreeId]?.scope ?? null
      const result = openReviewDiffAtSymbol(worktreeId, { filePath: path }, scope, { line })
      if (!result.ok) {
        toast.error(
          result.reason === 'path-not-allowed'
            ? translate(
                'auto.components.reviewMap.impact.openPathBlocked',
                'This path is outside the worktree and was not opened.'
              )
            : translate(
                'auto.components.reviewMap.impact.openUnavailable',
                'The diff for this file could not be opened.'
              )
        )
      }
    },
    [worktreeId]
  )

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col" data-testid="review-tab-host">
      <Suspense
        fallback={
          <ReviewLoadingStage
            stage="spinner"
            stageLabel={translate('auto.components.reviewMap.shell.host.loading', 'Loading review')}
          />
        }
      >
        <ReviewWorkspace
          worktreeId={worktreeId}
          onOpenDiff={onOpenDiff}
          onCloseTab={() => closeUnifiedTab(tabId)}
        />
      </Suspense>
    </div>
  )
}
