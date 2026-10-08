import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { QUALITY_CHART_COPY } from '../quality-chart-copy'
import {
  describeCoverage,
  describeGrid,
  describeSeriesRange,
  formatChartNumber
} from '../chart-text-summary'

describe('chart-text-summary', () => {
  it('describes rise, fall and flat with numbers only', () => {
    expect(describeSeriesRange({ label: 'Errors', points: [3, 5, 9] })).toBe(
      'Errors: rose from 3 to 9 over 3 points'
    )
    expect(describeSeriesRange({ label: 'Errors', points: [9, 2] })).toContain('fell from 9 to 2')
    expect(describeSeriesRange({ label: 'Errors', points: [4, 4] })).toContain('stayed at 4')
  })

  it('names missing points and handles degenerate input', () => {
    expect(describeSeriesRange({ label: 'E', points: [1, null, 3] })).toContain(
      '1 points have no value'
    )
    expect(describeSeriesRange({ label: 'E', points: [null, null] })).toContain(
      'no values recorded'
    )
    expect(describeSeriesRange({ label: 'E', points: [] })).toContain('no values recorded')
    expect(describeSeriesRange({ label: 'E', points: [7] })).toContain('one value, 7')
  })

  it('describes coverage without dividing by zero', () => {
    expect(describeCoverage({ covered: 0, total: 0, source: 'measured' })).toBe(
      'No coverage data for this scope'
    )
    expect(describeCoverage({ covered: 62, total: 100, source: 'estimated' })).toContain(
      'estimated'
    )
    expect(describeCoverage({ covered: 5, total: 2, source: 'measured' })).toContain('100%')
  })

  it('describes grids and formats numbers', () => {
    expect(describeGrid({ rows: 3, columns: 2, shown: 3, total: 9 })).toContain('showing 3 of 9')
    expect(formatChartNumber(Number.NaN)).toBe('—')
    expect(formatChartNumber(1.23456)).toBe('1.23')
  })

  it('default copy contains no conclusive wording', () => {
    const banned = /\b(safe|clean|improved|better|worse|healthy)\b/i
    for (const [key, text] of Object.entries(QUALITY_CHART_COPY)) {
      if (key.startsWith('summary.') || key.startsWith('gauge.') || key.startsWith('stackedBar.')) {
        expect(text, key).not.toMatch(banned)
      }
    }
    const src = readFileSync(
      join(process.cwd(), 'src/renderer/src/components/quality-charts/chart-text-summary.ts'),
      'utf8'
    )
    expect(src).not.toMatch(/an toàn|sạch|cải thiện|tốt hơn|xấu đi/)
  })
})
