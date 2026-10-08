import { chartCopy } from './quality-chart-copy'

export function formatChartNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return '—'
  }
  return String(Number(value.toFixed(2)))
}

// Why: summaries state measurements only; they must never judge ("improved", "safe").
export function describeSeriesRange(input: {
  label: string
  points: readonly (number | null)[]
}): string {
  const { label } = input
  const numeric = input.points.filter(
    (p): p is number => typeof p === 'number' && Number.isFinite(p)
  )
  const missing = input.points.length - numeric.length
  let sentence: string
  if (numeric.length === 0) {
    sentence = chartCopy('summary.noValues', { label })
  } else if (numeric.length === 1) {
    sentence = chartCopy('summary.single', { label, value: formatChartNumber(numeric[0]) })
  } else {
    const first = numeric[0]
    const last = numeric.at(-1)!
    const params = {
      label,
      first: formatChartNumber(first),
      last: formatChartNumber(last),
      count: input.points.length
    }
    sentence = chartCopy(
      last > first ? 'summary.rangeUp' : last < first ? 'summary.rangeDown' : 'summary.rangeFlat',
      params
    )
  }
  return missing > 0 ? `${sentence}. ${chartCopy('summary.missing', { count: missing })}` : sentence
}

export function describeCoverage(input: {
  covered: number
  total: number
  source: 'measured' | 'estimated'
}): string {
  if (!Number.isFinite(input.total) || input.total <= 0) {
    return chartCopy('summary.coverageNone')
  }
  const covered = Math.min(Math.max(0, input.covered), input.total)
  const text = chartCopy('summary.coverage', {
    percent: formatChartNumber((covered / input.total) * 100),
    covered: formatChartNumber(covered),
    total: formatChartNumber(input.total)
  })
  return input.source === 'estimated' ? `${text} (${chartCopy('summary.coverageEstimated')})` : text
}

export function describeGrid(input: {
  rows: number
  columns: number
  shown: number
  total: number
}): string {
  return chartCopy('summary.grid', {
    rows: input.rows,
    columns: input.columns,
    shown: input.shown,
    total: input.total
  })
}

export function describeTreemap(input: {
  count: number
  sizeLabel: string
  intensityLabel: string
}): string {
  return chartCopy('summary.treemap', {
    count: input.count,
    size: input.sizeLabel,
    intensity: input.intensityLabel
  })
}

export function describeMatrix(input: { nodes: number; cells: number; blocks: number }): string {
  return chartCopy('summary.matrix', input)
}
