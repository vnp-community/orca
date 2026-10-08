import { SeverityGlyph } from './SeverityGlyph'
import {
  SEVERITY_ENCODING,
  VERDICT_ENCODING,
  labelOf,
  type QualitySeverityLevel,
  type QualityVerdictLevel
} from './severity-encoding'

type ChartLegendProps =
  | { kind: 'severity'; levels: QualitySeverityLevel[] }
  | { kind: 'verdict'; levels: QualityVerdictLevel[] }

// Why: built straight from the encoding tables so legend, badges and marks cannot drift apart.
export function ChartLegend(props: ChartLegendProps): React.JSX.Element {
  const entries =
    props.kind === 'severity'
      ? props.levels.map((l) => ({ key: l, entry: SEVERITY_ENCODING[l] }))
      : props.levels.map((l) => ({ key: l, entry: VERDICT_ENCODING[l] }))
  return (
    <ul
      className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-foreground"
      data-chart-legend={props.kind}
    >
      {entries.map(({ key, entry }) => (
        <li key={key} className="flex items-center gap-1.5" data-level={key}>
          <svg
            width="22"
            height="14"
            viewBox="0 0 22 14"
            aria-hidden="true"
            className={entry.textClass}
          >
            <line
              x1="0"
              y1="7"
              x2="22"
              y2="7"
              stroke="currentColor"
              strokeWidth="2"
              strokeDasharray={entry.strokeDash ?? undefined}
            />
          </svg>
          <SeverityGlyph shape={entry.shape} className={entry.textClass} />
          <span>{labelOf(entry)}</span>
        </li>
      ))}
    </ul>
  )
}
