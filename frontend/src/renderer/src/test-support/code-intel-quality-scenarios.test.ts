/**
 * Drives the real quality slice and parsers through the shared fake backend (FE-CV-TASK-087-08):
 * every start error, gate verdict, CI relation, interruption and the progress/finished push path.
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { parseCodeIntelPushEvent } from '../../../shared/code-intel-parsers'
import { createCodeIntelQualitySlice } from '../store/slices/code-intel-quality-state'
import type { CodeIntelQualitySlice } from '../store/slices/code-intel-quality-state'
import type { QualitySliceData } from '../store/slices/code-intel-quality-slice-context'
import { createFakeCodeIntelBackend } from './code-intel-fake-backend'
import type { FakeCodeIntelBackend } from './code-intel-fake-backend'
import {
  CI_RELATIONS,
  createFakeQualityCall,
  installQualityScenario,
  pushQualityRun
} from './code-intel-quality-scenarios'
import type { QualityScenario } from './code-intel-quality-scenarios'

const WT = 'worktree-1'
let backend: FakeCodeIntelBackend
let state: QualitySliceData
let slice: CodeIntelQualitySlice

async function flush(): Promise<void> {
  for (let i = 0; i < 20; i++) {
    await Promise.resolve()
  }
}

function setup(scenario: QualityScenario): void {
  installQualityScenario(backend, scenario)
  state = { codeIntelQualityByWorktree: {}, codeIntelEventsState: 'streaming' }
  slice = createCodeIntelQualitySlice(
    (fn) => {
      state = { ...state, ...fn(state) }
    },
    () => state,
    { call: createFakeQualityCall(backend) }
  )
  backend.subscribe({
    onEvent: (frame) => {
      const event = parseCodeIntelPushEvent(frame)
      if (event && event.event !== 'reindexProgress') {
        slice.applyQualityPushEvent(event)
      }
    },
    onClose: () => undefined
  })
}

const wt = () => state.codeIntelQualityByWorktree[WT]

beforeEach(() => {
  backend = createFakeCodeIntelBackend()
  backend.setSettings({ codeIntelEnabled: true, qualityGateEnabled: true })
})

describe('gate verdicts through the contract shapes', () => {
  it.each([
    ['gate-pass', 'pass'],
    ['gate-warn', 'warn'],
    ['gate-fail', 'fail'],
    ['gate-unknown', 'unknown']
  ] as const)('%s parses to %s', async (scenario, verdict) => {
    setup(scenario)
    await slice.loadQualityGate(WT)
    expect(wt().gate?.data.gate.verdict).toBe(verdict)
    expect(wt().gate?.data.gate.profile).toBe('full@repo/v2')
  })

  it('keeps all nine CI relations', async () => {
    setup('ci-relations')
    await slice.loadQualityGate(WT)
    expect(wt().gate?.data.comparison.map((c) => c.relation)).toEqual([...CI_RELATIONS])
  })

  it('exposes dirty / changed-during-run / widened-scope flags and failed steps', async () => {
    setup('dirty-run')
    await slice.loadQualityRuns(WT)
    expect(wt().runs?.data[0]).toMatchObject({
      dirty: true,
      workTreeChangedDuringRun: true,
      scopeWidened: true
    })
    setup('failed-step')
    await slice.loadQualityRuns(WT)
    expect(wt().runs?.data[0].steps[0].status).toBe('timeout')
  })

  it('profiles keep ready=false with missing[]', async () => {
    setup('gate-pass')
    await slice.loadQualityProfiles(WT)
    const security = wt().profiles?.data.runnableProfiles.find((p) => p.id === 'security')
    expect(security).toMatchObject({ ready: false, heavy: true })
    expect(security?.missing[0]).toMatchObject({ check: 'semgrep', hint: 'brew install semgrep' })
  })
})

describe('start errors', () => {
  const request = { profile: 'full', scope: 'changed' as const }

  it('ENV_NOT_READY keeps missing[]', async () => {
    setup('env-not-ready')
    await slice.startQualityRun(WT, request)
    expect(wt().runError).toMatchObject({
      kind: 'env-not-ready',
      missing: [{ check: 'node_modules', hint: 'pnpm install' }]
    })
  })

  it('PROFILE_UNKNOWN keeps available[]', async () => {
    setup('profile-unknown')
    await slice.startQualityRun(WT, request)
    expect(wt().runError).toMatchObject({ kind: 'profile-unknown', available: ['full'] })
  })

  it('RUN_IN_PROGRESS attaches to the other run', async () => {
    setup('run-in-progress')
    await slice.startQualityRun(WT, request)
    expect(wt().run).toMatchObject({ runId: 'run-other', phase: 'running' })
  })

  it('RATE_LIMITED keeps retryAfterSeconds and NOT_AUTHORIZED is forbidden', async () => {
    setup('rate-limited')
    await slice.startQualityRun(WT, request)
    expect(wt().runError).toMatchObject({ kind: 'rate-limited', retryAfterSeconds: 7 })
    setup('forbidden')
    await slice.startQualityRun(WT, request)
    expect(wt().runError?.kind).toBe('forbidden')
  })

  it('QUALITY_GATE_DISABLED surfaces as quality-disabled on the gate', async () => {
    setup('gate-pass')
    backend.setSettings({ qualityGateEnabled: false })
    await slice.loadQualityGate(WT)
    expect(wt().errors.gate?.kind).toBe('quality-disabled')
  })
})

describe('push stream', () => {
  async function started(): Promise<void> {
    setup('gate-warn')
    await slice.loadQualityGate(WT)
    await slice.startQualityRun(WT, { profile: 'full', scope: 'changed' })
  }

  it('progress with unknown percent, then a known one', async () => {
    await started()
    pushQualityRun(backend, ['progress-null'])
    expect(wt().run).toMatchObject({
      percent: null,
      stage: 'lint',
      stepIndex: 1,
      stepCount: 2,
      message: 'running lint'
    })
    pushQualityRun(backend, ['progress-40'])
    expect(wt().run?.percent).toBe(40)
  })

  it.each(['succeeded', 'failed', 'cancelled', 'interrupted'] as const)(
    'finished %s ends the run and reloads the gate',
    async (status) => {
      await started()
      const before = backend.callsTo('codeIntel.quality.gate').length
      pushQualityRun(backend, ['progress-null', status])
      await flush()
      expect(wt().run).toMatchObject({ phase: 'finished', status })
      expect(backend.callsTo('codeIntel.quality.gate').length).toBe(before + 1)
    }
  )

  it('gateChanged reloads only the gate', async () => {
    await started()
    const gates = backend.callsTo('codeIntel.quality.gate').length
    pushQualityRun(backend, ['gate-changed'])
    await flush()
    expect(backend.callsTo('codeIntel.quality.gate').length).toBe(gates + 1)
  })
})
