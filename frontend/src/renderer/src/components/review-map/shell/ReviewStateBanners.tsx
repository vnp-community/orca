import { CloudOff, Info, TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type { ReviewBanner } from '../review-view-state'

type Props = {
  banners: readonly ReviewBanner[]
  onRetry: () => void
  onApplyNewData: () => void
  onRefreshIndex: () => void
}

const t = translate
const k = 'auto.components.reviewMap.shell.banner.'

/** Persistent inline banners; never toasts. Stable order = order of `banners`. */
export function ReviewStateBanners({
  banners,
  onRetry,
  onApplyNewData,
  onRefreshIndex
}: Props): React.JSX.Element | null {
  if (banners.length === 0) {
    return null
  }
  return (
    <div className="flex flex-col gap-1" data-testid="review-banners">
      {banners.map((b) => {
        const Icon =
          b.id === 'offline-cached' ? CloudOff : b.id === 'new-data' ? Info : TriangleAlert
        let text = ''
        let action: { label: string; run: () => void } | null = null
        switch (b.id) {
          case 'stale':
            text = t(
              `${k}stale`,
              'The index is behind HEAD ({{indexed}} to {{head}}); results may miss recent changes.',
              {
                indexed: b.indexedCommit?.slice(0, 7) ?? '?',
                head: b.headCommit?.slice(0, 7) ?? '?'
              }
            )
            action = {
              label: t('auto.components.reviewMap.shell.reindex.refresh', 'Refresh index'),
              run: onRefreshIndex
            }
            break
          case 'truncated':
            text = t(`${k}truncated`, 'Showing {{shown}} of {{total}} files.', {
              shown: b.shown,
              total: b.total
            })
            break
          case 'scope-mismatch':
            text = t(
              `${k}scopeMismatch`,
              'The index covers a different checkout than this worktree.'
            )
            break
          case 'offline-cached':
            text = t(`${k}offline`, 'Offline: showing the last loaded data.')
            action = { label: t(`${k}retry`, 'Try again'), run: onRetry }
            break
          case 'error-cached':
            text = t(`${k}errorCached`, 'Could not refresh: {{message}}', {
              message: b.error.message
            })
            action = { label: t(`${k}retry`, 'Try again'), run: onRetry }
            break
          case 'new-data':
            text = t(`${k}newData`, 'New data is available.')
            action = { label: t(`${k}apply`, 'Update now'), run: onApplyNewData }
            break
        }
        return (
          <div
            key={b.id}
            role="status"
            data-banner={b.id}
            className="flex items-center gap-2 rounded-md border bg-muted/50 px-3 py-1.5 text-xs"
          >
            <Icon className="size-3.5 shrink-0" aria-hidden />
            <span className="min-w-0 flex-1">{text}</span>
            {action ? (
              <Button type="button" size="xs" variant="ghost" onClick={action.run}>
                {action.label}
              </Button>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}
