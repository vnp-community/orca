type ChartHoverCardProps = {
  x: number
  y: number
  title?: string
  lines: string[]
}

// Why: hints only. Content duplicates the alternative table, so it is hidden from assistive tech
// and never takes focus or pointer events.
export function ChartHoverCard({
  x,
  y,
  title,
  lines
}: ChartHoverCardProps): React.JSX.Element | null {
  if (lines.length === 0 && !title) {
    return null
  }
  return (
    <div
      aria-hidden="true"
      data-chart-hover-card
      className="pointer-events-none absolute z-10 max-w-64 rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-sm"
      style={{ left: x, top: y }}
    >
      {title ? <div className="font-medium">{title}</div> : null}
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  )
}
