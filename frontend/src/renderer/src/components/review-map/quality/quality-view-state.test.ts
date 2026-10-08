import { describe, expect, it } from 'vitest'
import type {
  QualityGateResponse,
  QualityRun
} from '../../../../../shared/code-intel-quality-types'
import type { ActiveQualityRun } from '../../../store/slices/code-intel-quality-state'
import { deriveQualityViewState } from './quality-view-state'
import type { QualityViewInput } from './quality-view-state'

const gateResponse = (
  verdict: 'pass' | 'warn' | 'fail' | 'unknown',
  runIds: string[] = ['r1']
): QualityGateResponse => ({
  gate: {
    verdict,
    reasons: [],
    mode: 'inform',
    profile: 'full@repo/v1',
    basedOn: { runIds, indexCommit: 'i', stale: false }
  },
  waivers: [],
  evaluatedAt: '',
  profileDefinitionDigest: '',
  comparison: []
})
const run = (
  over: Partial<QualityRun['summary']> = {},
  status: QualityRun['status'] = 'succeeded'
): QualityRun =>
  ({
    id: 'r1',
    status,
    summary: {
      error: 0,
      warning: 0,
      info: 0,
      stepsFailed: 0,
      stepsEnvNotReady: 0,
      truncated: false,
      ...over
    }
  }) as QualityRun
const active = (over: Partial<ActiveQualityRun> = {}): ActiveQualityRun => ({
  runId: 'r',
  profile: 'full',
  scope: 'changed',
  phase: 'running',
  stage: '',
  percent: null,
  message: '',
  startedAt: 0,
  ...over
})
const base: QualityViewInput = {
  gateStatus: 'ready',
  gate: gateResponse('pass'),
  gateErrorKind: null,
  resultStale: false,
  indexBehindHead: false,
  runs: [run({ warning: 2 })],
  run: null,
  runError: null
}
const derive = (over: Partial<QualityViewInput>) => deriveQualityViewState({ ...base, ...over })

describe('deriveQualityViewState', () => {
  it('shows the scorecard when a gate with findings is known', () => {
    expect(derive({})).toMatchObject({ primary: 'ready', showScorecard: true, lockRun: false })
  })

  it('says "ran without findings" only when the runs behind the gate fully succeeded with zero findings', () => {
    expect(derive({ runs: [run()] }).primary).toBe('ready-empty')
    expect(derive({ runs: [run({ stepsFailed: 1 })] }).primary).toBe('ready')
    expect(derive({ runs: [run({ stepsEnvNotReady: 1 })] }).primary).toBe('ready')
    expect(derive({ runs: [run({}, 'failed')] }).primary).toBe('ready')
    expect(derive({ runs: null }).primary).toBe('ready')
  })

  it('an unknown verdict with no run behind it is "not run", with no scorecard', () => {
    const s = derive({ gate: gateResponse('unknown', []), runs: [] })
    expect(s).toMatchObject({ primary: 'not-run', showScorecard: false })
  })

  it('no gate and no runs after loading is "not run"', () => {
    expect(derive({ gate: null, runs: [], gateStatus: 'ready' }).primary).toBe('not-run')
  })

  it('loading and idle without data are loading', () => {
    expect(derive({ gate: null, runs: null, gateStatus: 'loading' }).primary).toBe('loading')
    expect(derive({ gate: null, runs: null, gateStatus: 'idle' }).primary).toBe('loading')
  })

  it('an active run takes priority and keeps the previous scorecard', () => {
    expect(derive({ run: active() })).toMatchObject({ primary: 'running', showScorecard: true })
    expect(derive({ run: active({ phase: 'starting', runId: null }) }).primary).toBe('running')
    expect(derive({ run: active({ phase: 'cancelling' }) }).primary).toBe('running')
  })

  it('finished failed, interrupted and unknown runs are never a result', () => {
    for (const status of ['failed', 'interrupted', 'unknown'] as const) {
      expect(derive({ run: active({ phase: 'finished', status }) }).primary).toBe('run-failed')
    }
    expect(derive({ run: active({ phase: 'finished', status: 'cancelled' }) }).primary).toBe(
      'run-cancelled'
    )
    expect(derive({ run: active({ phase: 'finished', status: 'succeeded' }) }).primary).toBe(
      'ready'
    )
  })

  it('start errors map to their own screen and lock running when it cannot work', () => {
    expect(derive({ runError: { kind: 'env-not-ready', message: '' } }).primary).toBe(
      'env-not-ready'
    )
    expect(derive({ runError: { kind: 'profile-unknown', message: '' } }).primary).toBe(
      'profile-unknown'
    )
    expect(derive({ runError: { kind: 'rate-limited', message: '' } }).primary).toBe('rate-limited')
    expect(derive({ runError: { kind: 'forbidden', message: '' } })).toMatchObject({
      primary: 'run-forbidden',
      lockRun: true
    })
    expect(derive({ runError: { kind: 'offline', message: '' } })).toMatchObject({
      primary: 'offline',
      lockRun: true
    })
    expect(derive({ runError: { kind: 'unknown', message: '' } }).primary).toBe('load-error')
  })

  it('load errors: forbidden, offline and generic, but a cached gate survives with a notice', () => {
    expect(
      derive({ gate: null, runs: null, gateStatus: 'error', gateErrorKind: 'forbidden' })
    ).toMatchObject({ primary: 'forbidden', lockRun: true })
    expect(
      derive({ gate: null, runs: null, gateStatus: 'error', gateErrorKind: 'offline' }).primary
    ).toBe('offline')
    expect(
      derive({ gate: null, runs: null, gateStatus: 'error', gateErrorKind: 'tool-failed' }).primary
    ).toBe('load-error')
    const cached = derive({ gateStatus: 'error', gateErrorKind: 'offline' })
    expect(cached.primary).toBe('ready')
    expect(cached.notices).toContain('offline')
    expect(cached.lockRun).toBe(true)
  })

  it('adds banners for staleness, index and truncation without replacing the scorecard', () => {
    const s = derive({
      resultStale: true,
      indexBehindHead: true,
      runs: [run({ warning: 1, truncated: true })]
    })
    expect(s.primary).toBe('ready')
    expect(s.notices).toEqual(['result-stale', 'index-stale', 'truncated'])
  })
})
