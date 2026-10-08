import React from 'react'
import { ExternalLink, Loader2, ScanSearch } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { ReviewRiskChip } from '../shell/ReviewRiskChip'
import type { ReviewChipId } from '../review-chip-filter'
import { useAgentTurnCompletions } from './useAgentTurnCompletions'
import { openReviewFromEntryPoint } from './open-review-entry'
import {
  useCodeIntelReviewSummary,
  type ReviewSummaryFetchers
} from './useCodeIntelReviewSummary'
import type { ReviewSummaryCountId, ReviewSummaryModel } from './review-summary-model'

const COUNT_LABEL: Record<ReviewSummaryCountId, [string, string, ReviewChipId | null]> = {
  files: ['auto.components.reviewMap.ReviewSummaryPanel.files', 'files', 'files'],
  symbols: ['auto.components.reviewMap.ReviewSummaryPanel.symbols', 'symbols', 'symbols'],
  flows: ['auto.components.reviewMap.ReviewSummaryPanel.flows', 'flows', 'flows'],
  tables: ['auto.components.reviewMap.ReviewSummaryPanel.tables', 'tables', 'tables'],
  contracts: ['auto.components.reviewMap.ReviewSummaryPanel.contracts', 'contracts', 'contracts'],
  uncovered: ['auto.components.reviewMap.ReviewSummaryPanel.untested', 'untested', 'untested']
}

function Body({
  model,
  onOpen
}: {
  model: ReviewSummaryModel
  onOpen: (filter?: ReviewChipId) => void
}): React.JSX.Element {
  const { findings, progress } = model
  const hasFindings = findings.error + findings.warning + findings.info > 0
  return (
    <div className="flex flex-col gap-2 text-xs">
      {model.baseRef ? (
        <div className="text-muted-foreground">
          {translate('auto.components.reviewMap.ReviewSummaryPanel.vs', 'vs')} {model.baseRef}
        </div>
      ) : null}
      {model.freshnessState ? (
        <div className="text-muted-foreground">
          {translate('auto.components.reviewMap.ReviewSummaryPanel.index', 'Index')}:{' '}
          {model.freshnessState}
        </div>
      ) : null}
      <ReviewRiskChip risk={model.risk} />
      {model.counts.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {model.counts.map(({ id, value }) => {
            const [key, fallback, chip] = COUNT_LABEL[id]
            return (
              <button
                key={id}
                type="button"
                className="rounded border border-border px-1.5 py-0.5 tabular-nums text-foreground hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() => onOpen(chip ?? undefined)}
              >
                {value} {translate(key, fallback)}
              </button>
            )
          })}
        </div>
      ) : null}
      {hasFindings ? (
        <div className="text-foreground">
          {translate('auto.components.reviewMap.ReviewSummaryPanel.findings', 'Findings')}:{' '}
          {findings.error} {translate('auto.components.reviewMap.ReviewSummaryPanel.errors', 'errors')} ·{' '}
          {findings.warning}{' '}
          {translate('auto.components.reviewMap.ReviewSummaryPanel.warnings', 'warnings')}
        </div>
      ) : null}
      {progress ? (
        <div className="text-muted-foreground tabular-nums">
          {translate('auto.components.reviewMap.ReviewSummaryPanel.progress', 'Read')}{' '}
          {progress.seen}/{progress.total}
        </div>
      ) : null}
    </div>
  )
}

export function ReviewSummaryPanel({
  isVisible,
  fetchers
}: {
  isVisible: boolean
  fetchers?: ReviewSummaryFetchers
}): React.JSX.Element {
  const worktreeId = useAppStore((s) => s.activeWorktreeId)
  const flagOn = useAppStore((s) => s.codeIntelSupportState?.state === 'enabled')
  const completions = useAgentTurnCompletions(worktreeId)
  const latest = completions[0] ?? null
  const { state, refresh } = useCodeIntelReviewSummary({
    worktreeId,
    enabled: isVisible && flagOn,
    latestCompletionId: latest?.id ?? null,
    fetchers
  })

  const open = (filter?: ReviewChipId): void => {
    if (worktreeId) {
      openReviewFromEntryPoint(worktreeId, 'right-sidebar', { filter, completionId: latest?.id })
    }
  }

  let content: React.JSX.Element
  if (!flagOn) {
    content = (
      <p className="text-xs text-muted-foreground">
        {translate(
          'auto.components.reviewMap.ReviewSummaryPanel.disabled',
          "Code review isn't enabled for this workspace."
        )}
      </p>
    )
  } else if (state.status === 'ready') {
    content = <Body model={state.model} onOpen={open} />
  } else if (state.status === 'error' || state.status === 'blocked') {
    content = (
      <div className="flex flex-col items-start gap-2 text-xs text-muted-foreground">
        <span>
          {translate(
            'auto.components.reviewMap.ReviewSummaryPanel.unavailable',
            'Review summary is unavailable.'
          )}
        </span>
        <Button size="xs" variant="outline" onClick={refresh}>
          {translate('auto.components.reviewMap.ReviewSummaryPanel.retry', 'Retry')}
        </Button>
      </div>
    )
  } else {
    content = (
      <div className="flex items-center gap-2 text-xs text-muted-foreground" role="status">
        <Loader2 className="size-3.5 animate-spin" />
        {translate('auto.components.reviewMap.ReviewSummaryPanel.loading', 'Loading summary…')}
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-sm font-medium text-foreground">
          <ScanSearch className="size-4" />
          {translate('auto.components.reviewMap.ReviewSummaryPanel.title', 'Review')}
        </span>
        {flagOn ? (
          <Button size="xs" variant="ghost" onClick={() => open()}>
            <ExternalLink className="size-3.5" />
            {translate('auto.components.reviewMap.ReviewSummaryPanel.openFull', 'Open full')}
          </Button>
        ) : null}
      </div>
      {content}
      {flagOn && latest ? (
        <div className="flex items-center justify-between gap-2 border-t border-border pt-2 text-xs text-muted-foreground">
          <span>
            {latest.agentType} · {new Date(latest.doneAt).toLocaleTimeString()}
          </span>
          <Button size="xs" variant="outline" onClick={() => open()}>
            {translate('auto.components.reviewMap.EntryButton.short', 'Review')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

export default ReviewSummaryPanel
