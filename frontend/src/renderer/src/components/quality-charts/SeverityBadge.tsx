import { Badge } from '../ui/badge'
import { cn } from '../../lib/utils'
import { SeverityGlyph } from './SeverityGlyph'
import {
  SEVERITY_ENCODING,
  labelOf,
  toSeverityLevel,
  type QualitySeverityLevel
} from './severity-encoding'

// Why: literal class names so Tailwind can generate them; unknown has no tint token by design.
const TINT_CLASS: Record<QualitySeverityLevel, string> = {
  error: 'border-quality-error-border bg-quality-error-background',
  warning: 'border-quality-warning-border bg-quality-warning-background',
  info: 'border-quality-info-border bg-quality-info-background',
  unknown: 'border-dashed border-border'
}

type SeverityBadgeProps = {
  severity: string
  count?: number
  withLabel?: boolean
  className?: string
}

export function SeverityBadge({
  severity,
  count,
  withLabel = true,
  className
}: SeverityBadgeProps): React.JSX.Element {
  const level = toSeverityLevel(severity)
  const entry = SEVERITY_ENCODING[level]
  const label = labelOf(entry)
  const safeCount =
    count === undefined || !Number.isFinite(count) || count < 0 ? undefined : Math.floor(count)
  const ariaLabel = safeCount === undefined ? label : `${label}: ${safeCount}`
  return (
    <Badge
      variant="outline"
      data-severity={level}
      data-shape={entry.shape}
      aria-label={withLabel ? undefined : ariaLabel}
      // Why: tint background with level colour only on border/glyph; text stays --foreground (contrast on tint).
      className={cn(TINT_CLASS[level], className)}
    >
      <SeverityGlyph shape={entry.shape} className={entry.textClass} />
      {withLabel ? <span>{label}</span> : null}
      {safeCount !== undefined ? <span className="tabular-nums">{safeCount}</span> : null}
    </Badge>
  )
}
