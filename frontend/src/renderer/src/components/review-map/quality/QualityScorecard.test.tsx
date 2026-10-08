// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type {
  CiComparison,
  QualityGateResponse,
  QualityRun,
  QualityStep
} from '../../../../../shared/code-intel-quality-types'
import { QualityScorecard } from './QualityScorecard'

afterEach(() => cleanup())

const step = (over: Partial<QualityStep>): QualityStep =>
  ({
    id: 's1',
    profileId: 'p',
    status: 'passed',
    failureKind: '',
    exitCode: 0,
    durationMs: 1200,
    tool: 'oxlint',
    toolVersion: '1',
    errorCount: 0,
    warningCount: 0,
    infoCount: 0,
    totalCount: 0,
    truncated: false,
    outsideScopeCount: 0,
    ...over
  }) as QualityStep
const run = (over: Partial<QualityRun> = {}): QualityRun =>
  ({
    id: 'r1',
    worktreeId: 'w',
    headCommit: 'abcdef0123',
    indexCommit: '1234567890',
    scope: 'changed',
    profile: 'full',
    status: 'succeeded',
    source: 'local',
    startedAt: null,
    finishedAt: '2026-10-07T10:00:00Z',
    summary: {
      error: 1,
      warning: 2,
      info: 3,
      stepsTotal: 1,
      stepsWithFindings: 1,
      stepsFailed: 0,
      stepsEnvNotReady: 0,
      outsideScope: 0,
      truncated: false
    },
    steps: [step({})],
    workTreeChangedDuringRun: false,
    scopeWidened: false,
    indexBasis: [],
    ...over
  }) as QualityRun
const response = (
  verdict: 'pass' | 'warn' | 'fail' | 'unknown',
  over: Partial<QualityGateResponse['gate']> = {},
  comparison: CiComparison[] = []
): QualityGateResponse => ({
  gate: {
    verdict,
    reasons: [],
    mode: 'inform',
    profile: 'full@repo/v2',
    basedOn: { runIds: ['r1'], indexCommit: '1234567890', stale: false },
    ...over
  },
  waivers: [],
  evaluatedAt: '',
  profileDefinitionDigest: '',
  comparison
})
const view = (
  r: QualityGateResponse,
  runs: QualityRun[] | null = [run()],
  extra: Partial<React.ComponentProps<typeof QualityScorecard>> = {}
) =>
  render(
    <QualityScorecard
      response={r}
      runs={runs}
      cacheStale={false}
      currentHead="abcdef0123"
      {...extra}
    />
  )

describe('QualityScorecard verdicts', () => {
  it.each([
    ['pass', 'The checks of profile full all passed'],
    ['warn', 'Has warnings'],
    ['fail', 'Fail']
  ] as const)('shows the %s headline', (verdict, text) => {
    view(response(verdict))
    expect(screen.getByTestId('quality-verdict').getAttribute('data-verdict')).toBe(verdict)
    expect(screen.getByText(text)).toBeTruthy()
  })

  it('unknown never uses the pass icon or wording and names its reason', () => {
    const { container } = view(
      response('unknown', {
        reasons: [{ check: 'coverage', observed: '', threshold: '', result: 'unknown' }]
      })
    )
    const badge = container.querySelector('[data-verdict="unknown"][data-shape]')
    expect(badge?.getAttribute('data-shape')).toBe('circle-dashed')
    expect(screen.getAllByText('Not enough data to conclude').length).toBeGreaterThan(0)
    expect(container.querySelector('[data-shape="circle-check"]')).toBeNull()
    expect(container.textContent?.toLowerCase()).not.toMatch(/safe|clean|all passed/)
  })

  it('unknown without reasons says the backend gave none', () => {
    view(response('unknown'))
    expect(screen.getByText('The backend gave no reason')).toBeTruthy()
  })

  it('states the advisory mode for inform and block', () => {
    view(response('fail', { mode: 'block' }))
    expect(screen.getByText(/only warns/)).toBeTruthy()
  })
})

describe('QualityScorecard reasons', () => {
  const reasons = [
    {
      check: 'typecheck',
      observed: '2',
      threshold: '0',
      result: 'fail' as const,
      category: 'typecheck',
      waivedCount: 1
    },
    {
      check: 'coverage',
      observed: '0.62',
      threshold: '0.80',
      result: 'warn' as const,
      code: 'coverage_below',
      tool: 'cov'
    },
    { check: '<b>x</b>', observed: '', threshold: '', result: 'pass' as const }
  ]

  it('renders observed versus threshold, waived counts and result as text', () => {
    view(response('fail', { reasons }))
    expect(screen.getAllByTestId('quality-reason')).toHaveLength(3)
    expect(screen.getByText('observed 0.62, threshold 0.80')).toBeTruthy()
    expect(screen.getByText('1 waived')).toBeTruthy()
    expect(screen.getAllByText('Failed').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Warning').length).toBeGreaterThan(0)
  })

  it('renders backend text as plain text, never HTML', () => {
    const { container } = view(response('fail', { reasons }))
    expect(container.querySelector('b')).toBeNull()
    expect(screen.getByText('<b>x</b>')).toBeTruthy()
  })

  it('opens the findings list with a category or tool filter, else unfiltered', () => {
    const onShow = vi.fn()
    view(response('fail', { reasons }), [run()], { onShowFindings: onShow })
    const buttons = screen.getAllByRole('button', { name: 'Show findings' })
    fireEvent.click(buttons[0])
    fireEvent.click(buttons[1])
    fireEvent.click(buttons[2])
    expect(onShow.mock.calls.map((c) => c[0])).toEqual([
      { category: 'typecheck' },
      { tool: 'cov' },
      null
    ])
    expect(buttons[2].getAttribute('title')).toBe('Findings cannot be filtered for this check')
  })

  it('says so when the backend returned no reasons', () => {
    view(response('pass'))
    expect(screen.getByText('The backend returned no per-check reasons')).toBeTruthy()
  })
})

describe('QualityScorecard run facts', () => {
  it('lists a failed step as incomplete instead of zero findings', () => {
    view(response('fail'), [
      run({
        steps: [
          step({ id: 'tsc', status: 'timeout' }),
          step({ id: 'go', status: 'env_not_ready', failureKind: 'env', envReason: 'go missing' })
        ]
      })
    ])
    const rows = screen.getAllByText(
      'This step did not complete, so it contributed no findings and its result is unknown'
    )
    expect(rows).toHaveLength(2)
    expect(screen.getByText('go missing')).toBeTruthy()
    expect(screen.queryByText('0 errors, 0 warnings, 0 info')).toBeNull()
  })

  it('shows the steps of a passing run collapsed with the count', () => {
    view(response('pass'))
    expect(screen.getByText('Steps that ran (1)')).toBeTruthy()
  })

  it('names local and CI sources with HEAD and index', () => {
    view(response('pass', { basedOn: { runIds: ['r1', 'r2'], indexCommit: 'i', stale: false } }), [
      run(),
      run({ id: 'r2', source: 'ci' })
    ])
    const lines = screen.getByTestId('quality-provenance').textContent ?? ''
    expect(lines).toContain('Local')
    expect(lines).toContain('CI')
    expect(lines).toContain('HEAD abcdef0')
    expect(lines).toContain('index 1234567')
  })

  it('shows dirty, changed-during-run, widened-scope and outside-scope notes', () => {
    view(response('warn'), [
      run({
        dirty: true,
        workTreeChangedDuringRun: true,
        scopeWidened: true,
        summary: { ...run().summary, outsideScope: 4 }
      })
    ])
    expect(screen.getByText(/uncommitted changes during the run/)).toBeTruthy()
    expect(screen.getByText(/changed during the run/)).toBeTruthy()
    expect(screen.getByText(/scope was widened/)).toBeTruthy()
    expect(screen.getByText(/4 findings are outside the changed scope/)).toBeTruthy()
  })

  it('a stale gate keeps its data, says stale and offers a rerun that respects the lock', () => {
    const onRerun = vi.fn()
    view(
      response('pass', { basedOn: { runIds: ['r1'], indexCommit: 'i', stale: true } }),
      [run()],
      { onRerun }
    )
    expect(screen.getByText('This result is out of date')).toBeTruthy()
    expect(screen.getByText('(stale)')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Run again' }))
    expect(onRerun).toHaveBeenCalled()
  })

  it('HEAD movement alone marks the result stale', () => {
    view(
      response('pass', {
        basedOn: { runIds: ['r1'], indexCommit: 'i', stale: false, headCommit: 'old0000' }
      }),
      [run()],
      { currentHead: 'new1111' }
    )
    expect(
      screen.getByText('HEAD has moved since this result was computed.', { exact: false })
    ).toBeTruthy()
  })

  it('works with no runs loaded', () => {
    view(response('unknown', { basedOn: { runIds: [], indexCommit: '', stale: false } }), null)
    expect(screen.getByTestId('quality-scorecard')).toBeTruthy()
  })
})

describe('QualityScorecard CI comparison', () => {
  const cmp = (
    relation: CiComparison['relation'],
    extra: Partial<CiComparison> = {}
  ): CiComparison => ({ profile: 'full', headCommit: 'h', local: {}, ci: {}, relation, ...extra })

  it('local pass / CI fail uses a warning shape, shows hints and never the pass icon', () => {
    const { container } = view(
      response('pass', {}, [cmp('local_pass_ci_fail', { reasonsHint: ['Node version differs'] })])
    )
    const row = container.querySelector('[data-relation="local_pass_ci_fail"]')
    expect(row?.querySelector('[data-shape="triangle"]')).toBeTruthy()
    expect(row?.querySelector('[data-shape="circle-check"]')).toBeNull()
    expect(screen.getByText('Node version differs')).toBeTruthy()
    expect(screen.getByText(/cannot be treated as passing/)).toBeTruthy()
  })

  it.each([
    'agree_pass',
    'agree_fail',
    'local_pass_ci_fail',
    'local_fail_ci_pass',
    'local_only',
    'ci_only',
    'ci_pending',
    'sha_mismatch',
    'not_comparable',
    'unknown'
  ] as const)('has copy for %s', (relation) => {
    const { container } = view(response('pass', {}, [cmp(relation)]))
    expect(
      (container.querySelector(`[data-relation="${relation}"]`)?.textContent ?? '').length
    ).toBeGreaterThan(3)
  })

  it('only offers CI links for http(s) urls', () => {
    view(response('pass', {}, [cmp('agree_pass', { ci: { url: 'javascript:alert(1)' } })]))
    expect(screen.queryByRole('button', { name: 'Open CI run' })).toBeNull()
    cleanup()
    view(response('pass', {}, [cmp('agree_pass', { ci: { url: 'https://ci.example/run/1' } })]))
    expect(screen.getByRole('button', { name: 'Open CI run' })).toBeTruthy()
  })
})
