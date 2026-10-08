import { ChartFrame } from './ChartFrame'
import type { ChartFrameIdentity } from './chart-frame-shared-props'
import { chartCopy } from './quality-chart-copy'
import { SeverityGlyph } from './SeverityGlyph'
import { SEVERITY_ENCODING, labelOf, type QualitySeverityLevel } from './severity-encoding'

const LEVELS: QualitySeverityLevel[] = ['error', 'warning', 'info']

// Why: literal class names so Tailwind generates them from the quality tokens.
const STRIP_CLASS: Record<QualitySeverityLevel, string> = {
  error: 'bg-quality-error',
  warning: 'bg-quality-warning',
  info: 'bg-quality-info',
  unknown: 'bg-quality-unknown'
}

type StackedSeverityBarProps = {
  frame: ChartFrameIdentity
  counts: { error: number; warning: number; info: number }
  /** True only when the caller knows the checks actually ran; zero findings is not "clean" otherwise. */
  ranCheck?: boolean
}

function safeCount(value: number): number {
  return Number.isFinite(value) && value > 0 ? Math.floor(value) : 0
}

export function StackedSeverityBar({
  frame,
  counts,
  ranCheck = false
}: StackedSeverityBarProps): React.JSX.Element {
  const values = LEVELS.map((level) => ({
    level,
    count: safeCount(counts[level as keyof typeof counts])
  }))
  const total = values.reduce((sum, v) => sum + v.count, 0)
  const summary =
    total === 0
      ? ranCheck
        ? chartCopy('stackedBar.noFindings')
        : chartCopy('stackedBar.noData')
      : `${chartCopy('stackedBar.total', { count: total })}: ${values
          .map((v) => `${labelOf(SEVERITY_ENCODING[v.level])} ${v.count}`)
          .join(', ')}`
  return (
    <ChartFrame
      {...frame}
      status={frame.status ?? 'ready'}
      minHeight={frame.minHeight ?? 56}
      summary={summary}
      legend={frame.legend}
      table={{
        caption: frame.title,
        columns: [
          { key: 'severity', label: chartCopy('table.severity') },
          { key: 'count', label: chartCopy('table.count'), align: 'end' }
        ],
        rows: values.map((v) => ({ severity: labelOf(SEVERITY_ENCODING[v.level]), count: v.count }))
      }}
    >
      {() =>
        total === 0 ? (
          <p className="p-1 text-sm text-muted-foreground">{summary}</p>
        ) : (
          <div className="flex w-full gap-1" data-stacked-bar>
            {values
              .filter((v) => v.count > 0)
              .map((v) => {
                const entry = SEVERITY_ENCODING[v.level]
                return (
                  <div
                    key={v.level}
                    data-level={v.level}
                    className="flex min-w-14 flex-col gap-1"
                    style={{ flexGrow: v.count, flexBasis: 0 }}
                  >
                    <div className={`h-2 rounded-sm ${STRIP_CLASS[v.level]}`} />
                    <div className="flex items-center gap-1 text-xs text-foreground">
                      <SeverityGlyph shape={entry.shape} className={entry.textClass} />
                      <span>{labelOf(entry)}</span>
                      <span className="tabular-nums">{v.count}</span>
                    </div>
                  </div>
                )
              })}
          </div>
        )
      }
    </ChartFrame>
  )
}
