import { useEffect, useRef, useState } from 'react'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { translate } from '@/i18n/i18n'
import { usePerceivedLoadingStage } from '@/hooks/usePerceivedLoadingStage'
import type { ReviewDataApi } from '../review-shell-data'
import type { IndexStatusView, ReviewError } from '../review-wire-types'

export type ReindexButtonProps = {
  worktreeId: string
  status: IndexStatusView | null
  api: ReviewDataApi
  /** Called after the backend accepted (or already runs) a job so the caller re-polls status. */
  onStarted: () => void
  remote?: boolean
}

export function ReindexButton({
  worktreeId,
  status,
  api,
  onStarted,
  remote
}: ReindexButtonProps): React.JSX.Element {
  const [starting, setStarting] = useState(false)
  const [cooldown, setCooldown] = useState(0)
  const [error, setError] = useState<ReviewError | null>(null)
  const [confirmFull, setConfirmFull] = useState(false)
  const inFlight = useRef(false)

  const job = status?.activeJob ?? null
  const running = starting || status?.overall === 'BUILDING' || job !== null
  const stage = usePerceivedLoadingStage(running, { remote })

  const cooling = cooldown > 0
  useEffect(() => {
    if (!cooling) {
      return
    }
    const id = setInterval(() => setCooldown((c) => Math.max(0, c - 1)), 1000)
    return () => clearInterval(id)
  }, [cooling])

  async function start(mode: 'incremental' | 'full'): Promise<void> {
    // Ref guard: two quick clicks must produce exactly one call.
    if (inFlight.current) {
      return
    }
    inFlight.current = true
    setStarting(true)
    setError(null)
    setConfirmFull(false)
    try {
      const res = await api.reindex(worktreeId, mode)
      if (res.ok || res.error.kind === 'reindex-in-progress') {
        onStarted()
      } else if (res.error.kind === 'rate-limited') {
        setCooldown(res.error.retryAfterSeconds ?? 300)
      } else {
        setError(res.error)
      }
    } finally {
      inFlight.current = false
      setStarting(false)
    }
  }

  const disabled = running || cooldown > 0
  return (
    <div className="space-y-2" data-stage={stage}>
      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled}
          onClick={() => void start('incremental')}
        >
          {stage === 'spinner' || stage === 'stages' ? (
            <Loader2 className="animate-spin motion-reduce:animate-none" aria-hidden />
          ) : (
            <RefreshCw aria-hidden />
          )}
          {translate('auto.components.reviewMap.shell.reindex.refresh', 'Refresh index')}
        </Button>
        {confirmFull ? (
          <>
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={disabled}
              onClick={() => void start('full')}
            >
              {translate(
                'auto.components.reviewMap.shell.reindex.fullConfirm',
                'Rebuild everything'
              )}
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => setConfirmFull(false)}>
              {translate('auto.components.reviewMap.shell.cancel', 'Cancel')}
            </Button>
          </>
        ) : (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={disabled}
            onClick={() => setConfirmFull(true)}
          >
            {translate('auto.components.reviewMap.shell.reindex.full', 'Rebuild all')}
          </Button>
        )}
      </div>
      {job ? (
        <div className="space-y-1">
          {/* percent:null means unknown: indeterminate bar, never a fake 0%. */}
          {job.percent === null ? (
            <div
              role="progressbar"
              aria-label={job.stage}
              className="h-2 w-full animate-pulse rounded-full bg-primary/20 motion-reduce:animate-none"
            />
          ) : (
            <Progress value={job.percent} aria-label={job.stage} />
          )}
          {job.stage ? <p className="text-muted-foreground">{job.stage}</p> : null}
        </div>
      ) : null}
      {cooldown > 0 ? (
        <p role="status" className="text-muted-foreground">
          {translate(
            'auto.components.reviewMap.shell.reindex.cooldown',
            'Available again in {{seconds}}s',
            {
              seconds: cooldown
            }
          )}
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="text-destructive">
          {error.message}
        </p>
      ) : null}
    </div>
  )
}
