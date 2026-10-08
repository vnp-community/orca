import { translate } from '@/i18n/i18n'
import type { ChangeOverlayView, IndexStatusView } from '../review-wire-types'
import { IndexFreshnessChip } from './IndexFreshnessChip'
import type { ReindexButtonProps } from './ReindexButton'
import { ReviewRiskChip } from './ReviewRiskChip'
import { ReviewScopePicker } from './ReviewScopePicker'
import type { ComponentProps, ReactNode } from 'react'

type Props = {
  branchName: string | null
  scopePicker: ComponentProps<typeof ReviewScopePicker>
  status: IndexStatusView | null
  reindex: Omit<ReindexButtonProps, 'status'>
  overlay: ChangeOverlayView | null
  /** Extra chips (e.g. the quality gate chip); each renders nothing when its feature is off. */
  trailing?: ReactNode
}

export function ReviewHeaderBar({
  branchName,
  scopePicker,
  status,
  reindex,
  overlay,
  trailing
}: Props): React.JSX.Element {
  return (
    <div className="flex flex-wrap items-center gap-2 px-3 py-2" data-testid="review-header">
      <h1 className="truncate text-sm font-medium">
        {translate('auto.components.reviewMap.shell.header.title', 'Review')}
        {branchName ? (
          <span className="ml-1 font-normal text-muted-foreground">· {branchName}</span>
        ) : null}
      </h1>
      <ReviewScopePicker {...scopePicker} />
      <div className="ml-auto flex items-center gap-2">
        <IndexFreshnessChip status={status} reindex={reindex} />
        <ReviewRiskChip risk={overlay?.risk ?? null} />
        {trailing}
      </div>
    </div>
  )
}
