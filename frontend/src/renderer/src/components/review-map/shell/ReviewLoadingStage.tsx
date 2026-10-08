import { Loader2 } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import type { PerceivedLoadingStage } from '@/hooks/usePerceivedLoadingStage'

type Props = {
  stage: PerceivedLoadingStage
  /** Shown from the `stages` step on, e.g. "Reading the change set". */
  stageLabel?: string
  rows?: number
}

/** Fixed-height placeholder so the layout does not jump when data arrives. */
export function ReviewLoadingStage({ stage, stageLabel, rows = 6 }: Props): React.JSX.Element {
  return (
    <div
      role="status"
      aria-busy={stage !== 'idle'}
      data-stage={stage}
      className="flex min-h-40 flex-col gap-2 p-4"
    >
      {stage === 'spinner' || stage === 'stages' ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden />
          {stage === 'stages' && stageLabel ? <span>{stageLabel}</span> : null}
        </div>
      ) : null}
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className={stage === 'busy' ? 'h-8 bg-muted/40' : 'h-8'} />
      ))}
    </div>
  )
}
