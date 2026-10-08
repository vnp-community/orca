import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { translate } from '@/i18n/i18n'
import type { ReviewProgressSaveStatus } from '@/store/slices/review-progress'

type Props = {
  seen: number
  total: number
  saveStatus: ReviewProgressSaveStatus
  localOnly?: boolean
  oversize?: boolean
  onRetrySave: () => void
}

/** Live announcement is delayed 500 ms so rapid ticking does not flood screen readers. */
function useDelayedValue<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return v
}

export function ReadingProgressBar({
  seen,
  total,
  saveStatus,
  localOnly,
  oversize,
  onRetrySave
}: Props): React.JSX.Element {
  const announced = useDelayedValue(`${seen}/${total}`, 500)
  const pct = total === 0 ? 0 : Math.round((seen / total) * 100)
  return (
    <div className="space-y-1 px-3 py-2" data-testid="reading-progress">
      <div className="flex items-center justify-between text-xs">
        <span>
          {translate(
            'auto.components.reviewMap.readingOrder.progress',
            '{{seen}} of {{total}} read',
            { seen, total }
          )}
        </span>
        {/* Never says "Saved" until the backend confirmed it. */}
        {saveStatus === 'error' ? (
          <span className="flex items-center gap-1 text-destructive">
            {localOnly
              ? translate(
                  'auto.components.reviewMap.readingOrder.localOnly',
                  'Not saved (read-only access)'
                )
              : translate('auto.components.reviewMap.readingOrder.notSaved', 'Not saved')}
            {!localOnly ? (
              <Button type="button" size="xs" variant="ghost" onClick={onRetrySave}>
                {translate('auto.components.reviewMap.readingOrder.retry', 'Retry')}
              </Button>
            ) : null}
          </span>
        ) : saveStatus === 'saved' ? (
          <span className="text-muted-foreground">
            {translate('auto.components.reviewMap.readingOrder.saved', 'Saved')}
          </span>
        ) : (
          <span className="text-muted-foreground">
            {translate('auto.components.reviewMap.readingOrder.saving', 'Saving')}
          </span>
        )}
      </div>
      <Progress
        value={pct}
        aria-label={translate(
          'auto.components.reviewMap.readingOrder.progressAria',
          'Reading progress'
        )}
        className="h-1.5"
      />
      {oversize ? (
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.components.reviewMap.readingOrder.oversize',
            'Progress is too large; part of it is not saved.'
          )}
        </p>
      ) : null}
      <span aria-live="polite" className="sr-only">
        {announced}
      </span>
    </div>
  )
}
