import { describe, expect, it } from 'vitest'
import type { QualityRun, QualityStep } from '../../../../../shared/code-intel-quality-types'
import {
  comparisonIsDisagreement,
  isIncompleteStep,
  reasonCodeKeySegment,
  reasonFilter,
  shortCommit,
  summarizeRuns
} from './quality-scorecard-model'

const step = (status: QualityStep['status'], id = 's'): QualityStep =>
  ({ id, status }) as QualityStep
const run = (
  over: Omit<Partial<QualityRun>, 'summary'> & { summary?: Partial<QualityRun['summary']> }
): QualityRun =>
  ({
    id: 'r',
    source: 'local',
    finishedAt: 't',
    headCommit: 'abcdef012345',
    indexCommit: 'i',
    steps: [],
    workTreeChangedDuringRun: false,
    scopeWidened: false,
    ...over,
    summary: { error: 0, warning: 0, info: 0, outsideScope: 0, truncated: false, ...over.summary }
  }) as QualityRun

describe('summarizeRuns', () => {
  it('sums severity counts and outside-scope across runs and keeps one source per run', () => {
    const s = summarizeRuns([
      run({ id: 'a', summary: { error: 1, warning: 2, outsideScope: 3 } }),
      run({ id: 'b', source: 'ci', summary: { error: 4, info: 5 } })
    ])
    expect(s.counts).toEqual({ error: 5, warning: 2, info: 5 })
    expect(s.outsideScope).toBe(3)
    expect(s.sources.map((x) => [x.runId, x.source])).toEqual([
      ['a', 'local'],
      ['b', 'ci']
    ])
    expect(s.ran).toBe(true)
  })

  it('flags dirty, changed-during-run, widened scope and truncation', () => {
    const s = summarizeRuns([
      run({ dirty: true, scopeWidened: true, summary: { truncated: true } }),
      run({ workTreeChangedDuringRun: true })
    ])
    expect(s).toMatchObject({
      dirty: true,
      changedDuringRun: true,
      scopeWidened: true,
      truncated: true
    })
  })

  it('an empty run list means nothing ran', () => {
    expect(summarizeRuns([])).toMatchObject({
      ran: false,
      steps: [],
      counts: { error: 0, warning: 0, info: 0 }
    })
  })

  it('failed, timed out, not-ready, cancelled and unknown steps are incomplete, never "0 findings"', () => {
    const rows = summarizeRuns([
      run({
        steps: [
          step('passed', '1'),
          step('findings', '2'),
          step('failed', '3'),
          step('timeout', '4'),
          step('env_not_ready', '5'),
          step('skipped', '6'),
          step('unknown', '7')
        ]
      })
    ]).steps
    expect(rows.map((r) => r.incomplete)).toEqual([false, false, true, true, true, false, true])
    expect(isIncompleteStep(step('cancelled'))).toBe(true)
  })

  it('step keys stay unique even without step ids', () => {
    const rows = summarizeRuns([run({ steps: [step('passed', ''), step('passed', '')] })]).steps
    expect(new Set(rows.map((r) => r.key)).size).toBe(2)
  })
})

describe('small helpers', () => {
  it('shortCommit', () => {
    expect(shortCommit('abcdef012345')).toBe('abcdef0')
    expect(shortCommit('')).toBeNull()
    expect(shortCommit(undefined)).toBeNull()
  })

  it('reasonCodeKeySegment only accepts plain identifiers', () => {
    expect(reasonCodeKeySegment('coverage_below')).toBe('coverage_below')
    expect(reasonCodeKeySegment('a.b')).toBeNull()
    expect(reasonCodeKeySegment('')).toBeNull()
    expect(reasonCodeKeySegment(undefined)).toBeNull()
  })

  it('reasonFilter prefers category, then tool, else null', () => {
    expect(reasonFilter({ category: 'lint', tool: 'oxlint' })).toEqual({ category: 'lint' })
    expect(reasonFilter({ tool: 'oxlint' })).toEqual({ tool: 'oxlint' })
    expect(reasonFilter({})).toBeNull()
  })

  it('comparisonIsDisagreement', () => {
    expect(comparisonIsDisagreement({ relation: 'local_pass_ci_fail' } as never)).toBe(true)
    expect(comparisonIsDisagreement({ relation: 'agree_pass' } as never)).toBe(false)
  })
})
