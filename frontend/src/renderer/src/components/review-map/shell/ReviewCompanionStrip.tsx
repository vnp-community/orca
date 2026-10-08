/**
 * ReviewCompanionStrip.tsx — FE-CV-TASK-060-08 / 090-06 / 093-04
 *
 * Row under the summary chips: export-report menu, agent-turn switcher (with the verification
 * line) and the AI summary card. Each part hides itself when its flag or data is absent.
 *
 * @module components/review-map/shell/ReviewCompanionStrip
 */

import { ReviewAiSummaryCard } from '../ai-summary/ReviewAiSummaryCard'
import { ReviewReportMenu } from '../report/ReviewReportMenu'
import { ReviewTurnSwitcher } from '../turns/ReviewTurnSwitcher'
import type { useReviewCompanions } from './use-review-companions'
import type { useReviewWorkspaceModel } from './useReviewWorkspaceModel'

export function ReviewCompanionStrip({
  worktreeId,
  m,
  c,
  onOpenDiff
}: {
  worktreeId: string
  m: ReturnType<typeof useReviewWorkspaceModel>
  c: ReturnType<typeof useReviewCompanions>
  onOpenDiff: (path: string, line?: number) => void
}): React.JSX.Element {
  return (
    <>
      <ReviewReportMenu
        worktreeId={worktreeId}
        projectId={m.worktree?.projectId}
        base={m.summary?.baseRef ?? null}
        provider={undefined}
        repoSlug={m.worktree?.displayName ?? worktreeId}
      />
      {c.turnMarkers.length > 0 || c.turnSaveFailed ? (
        <div className="basis-full">
          <ReviewTurnSwitcher
            markers={c.turnMarkers}
            mode={c.turnMode}
            onModeChange={c.setTurnMode}
            selectedTurnId={c.selectedTurnId}
            onSelectTurn={c.setSelectedTurnId}
            onCompare={c.setTurnCompare}
            verificationByTurn={c.verificationByTurn}
            saveFailed={c.turnSaveFailed}
            onRetrySave={c.retryTurnSave}
          />
        </div>
      ) : null}
      <div className="basis-full empty:hidden">
        <ReviewAiSummaryCard ai={c.aiSummary} changedFiles={c.changedFileSet} onOpenFile={onOpenDiff} />
      </div>
    </>
  )
}
