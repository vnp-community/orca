import { Badge } from '../ui/badge'
import { cn } from '../../lib/utils'
import { chartCopy } from './quality-chart-copy'
import { SeverityGlyph } from './SeverityGlyph'
import {
  VERDICT_ENCODING,
  labelOf,
  toVerdictLevel,
  type QualityVerdictLevel
} from './severity-encoding'

const TINT_CLASS: Record<QualityVerdictLevel, string> = {
  pass: 'border-quality-pass-border bg-quality-pass-background',
  warn: 'border-quality-warning-border bg-quality-warning-background',
  fail: 'border-quality-error-border bg-quality-error-background',
  unknown: 'border-dashed border-border'
}

type GateVerdictBadgeProps = {
  verdict: string
  stale?: boolean
  compact?: boolean
  className?: string
}

export function GateVerdictBadge({
  verdict,
  stale = false,
  compact = false,
  className
}: GateVerdictBadgeProps): React.JSX.Element {
  const level = toVerdictLevel(verdict)
  const entry = VERDICT_ENCODING[level]
  // Why: unknown must read as "cannot conclude", never as a pass.
  const label = level === 'unknown' && !compact ? chartCopy('verdict.unknownLong') : labelOf(entry)
  return (
    <Badge
      variant="outline"
      data-verdict={level}
      data-shape={entry.shape}
      data-stale={stale ? 'true' : undefined}
      className={cn(TINT_CLASS[level], className)}
    >
      <SeverityGlyph shape={entry.shape} className={entry.textClass} />
      <span>{label}</span>
      {stale ? <span>({chartCopy('stale')})</span> : null}
    </Badge>
  )
}
