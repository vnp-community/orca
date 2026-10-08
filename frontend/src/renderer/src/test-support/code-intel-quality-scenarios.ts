/**
 * Scenario fixtures for the quality channels on the shared fake code-intel backend
 * (FE-CV-TASK-087-08). Each scenario sets channel data/handlers on a backend instance, so unit,
 * integration and web e2e tests all speak the same contract shapes (§3.2 / §4.7).
 */

import {
  codeIntelErrorKindForCode,
  parseCodeIntelErrorMessage
} from '../../../shared/code-intel-error-codes'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import type { QualityCall } from '../store/slices/code-intel-quality-slice-context'
import type { FakeCodeIntelBackend } from './code-intel-fake-backend'

const M = CODE_INTEL_RPC_METHODS

export type GateVerdictFixture = 'pass' | 'warn' | 'fail' | 'unknown'

export const CI_RELATIONS = [
  'agree_pass',
  'agree_fail',
  'local_pass_ci_fail',
  'local_fail_ci_pass',
  'local_only',
  'ci_only',
  'ci_pending',
  'sha_mismatch',
  'not_comparable'
] as const

export function qualityRunFixture(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'run-1',
    worktreeId: 'worktree-1',
    headCommit: 'a41c9e0aaaaaaaa',
    indexCommit: 'd819812bbbbbbbb',
    indexBasis: [],
    scope: 'changed',
    profile: 'full',
    status: 'succeeded',
    source: 'local',
    startedAt: '2026-10-07T10:00:00Z',
    finishedAt: '2026-10-07T10:01:00Z',
    summary: {
      error: 2,
      warning: 5,
      info: 9,
      stepsTotal: 2,
      stepsWithFindings: 2,
      stepsFailed: 0,
      stepsEnvNotReady: 0,
      outsideScope: 0,
      truncated: false
    },
    steps: [
      {
        id: 'lint',
        profileId: 'lint',
        status: 'findings',
        failureKind: '',
        exitCode: 1,
        durationMs: 1500,
        tool: 'oxlint',
        toolVersion: '1.2',
        errorCount: 0,
        warningCount: 5,
        infoCount: 9,
        totalCount: 14,
        truncated: false,
        outsideScopeCount: 0
      },
      {
        id: 'typecheck',
        profileId: 'typecheck',
        status: 'findings',
        failureKind: '',
        exitCode: 1,
        durationMs: 4200,
        tool: 'tsc',
        toolVersion: '5.4',
        errorCount: 2,
        warningCount: 0,
        infoCount: 0,
        totalCount: 2,
        truncated: false,
        outsideScopeCount: 0
      }
    ],
    workTreeChangedDuringRun: false,
    scopeWidened: false,
    ...over
  }
}

export function qualityGateFixture(
  verdict: GateVerdictFixture,
  over: Record<string, unknown> = {}
): Record<string, unknown> {
  const reasons =
    verdict === 'unknown'
      ? []
      : [
          {
            check: 'typecheck',
            observed: '2',
            threshold: '0',
            result: verdict === 'pass' ? 'pass' : 'fail',
            category: 'typecheck'
          },
          {
            check: 'coverage',
            observed: '0.62',
            threshold: '0.80',
            result: verdict === 'warn' ? 'warn' : 'pass',
            code: 'coverage_below',
            tool: 'cov'
          }
        ]
  return {
    gate: {
      verdict,
      reasons,
      mode: 'inform',
      profile: 'full@repo/v2',
      basedOn: {
        runIds: verdict === 'unknown' ? [] : ['run-1'],
        indexCommit: 'd819812bbbbbbbb',
        stale: false,
        headCommit: 'a41c9e0aaaaaaaa'
      },
      ...over
    },
    waivers: [],
    evaluatedAt: '2026-10-07T10:02:00Z',
    profileDefinitionDigest: 'digest',
    comparison: []
  }
}

export const QUALITY_PROFILES_FIXTURE = {
  profile: {
    name: 'full',
    mode: 'inform',
    version: 2,
    definition: {
      schemaVersion: 1,
      coverage: { required: true, diffCoverageWarnBelow: 0.8, diffCoverageFailBelow: 0.5 }
    }
  },
  origin: 'repo',
  version: 2,
  runnableProfiles: [
    {
      id: 'full',
      title: 'Full',
      kind: 'suite',
      ready: true,
      heavy: false,
      scopes: ['changed', 'worktree', 'commitRange'],
      missing: []
    },
    {
      id: 'security',
      title: 'Security',
      kind: 'tool',
      ready: false,
      heavy: true,
      scopes: ['worktree'],
      missing: [{ check: 'semgrep', reason: 'not installed', hint: 'brew install semgrep' }]
    }
  ]
}

export type QualityScenario =
  | 'gate-pass'
  | 'gate-warn'
  | 'gate-fail'
  | 'gate-unknown'
  | 'failed-step'
  | 'dirty-run'
  | 'ci-relations'
  | 'env-not-ready'
  | 'profile-unknown'
  | 'run-in-progress'
  | 'rate-limited'
  | 'forbidden'

const FAILURE_BY_SCENARIO: Partial<Record<QualityScenario, [string, string, unknown?]>> = {
  'env-not-ready': [
    'CODEINTEL_ENV_NOT_READY',
    'environment is not ready',
    { missing: [{ check: 'node_modules', reason: 'absent', hint: 'pnpm install' }] }
  ],
  'profile-unknown': ['CODEINTEL_PROFILE_UNKNOWN', 'unknown profile', { available: ['full'] }],
  'run-in-progress': [
    'CODEINTEL_RUN_IN_PROGRESS',
    'a run is already active',
    { runId: 'run-other' }
  ],
  'rate-limited': ['CODEINTEL_RATE_LIMITED', 'slow down', { retryAfterSeconds: 7 }],
  forbidden: ['CODEINTEL_NOT_AUTHORIZED', 'review_write required']
}

export function installQualityScenario(
  backend: FakeCodeIntelBackend,
  scenario: QualityScenario
): void {
  backend.setChannelData(M.QUALITY_PROFILE_GET, QUALITY_PROFILES_FIXTURE)
  const verdict: GateVerdictFixture = scenario.startsWith('gate-')
    ? (scenario.slice(5) as GateVerdictFixture)
    : 'warn'
  backend.setChannelData(M.QUALITY_GATE, qualityGateFixture(verdict))
  backend.setChannelData(M.QUALITY_RUNS, {
    runs: verdict === 'unknown' ? [] : [qualityRunFixture()]
  })
  backend.setHandler(M.QUALITY_START, () => ({
    run: qualityRunFixture({ status: 'running', finishedAt: null })
  }))
  backend.setHandler(M.QUALITY_CANCEL, () => ({ run: qualityRunFixture({ status: 'running' }) }))
  backend.setChannelData(M.QUALITY_RUN, qualityRunFixture())

  if (scenario === 'failed-step') {
    backend.setChannelData(M.QUALITY_RUNS, {
      runs: [
        qualityRunFixture({
          status: 'failed',
          steps: [
            {
              id: 'test',
              profileId: 'test',
              status: 'timeout',
              failureKind: '',
              exitCode: 0,
              durationMs: 60000,
              tool: 'vitest',
              toolVersion: '4',
              errorCount: 0,
              warningCount: 0,
              infoCount: 0,
              totalCount: 0,
              truncated: false,
              outsideScopeCount: 0
            }
          ]
        })
      ]
    })
  }
  if (scenario === 'dirty-run') {
    backend.setChannelData(M.QUALITY_RUNS, {
      runs: [qualityRunFixture({ dirty: true, workTreeChangedDuringRun: true, scopeWidened: true })]
    })
  }
  if (scenario === 'ci-relations') {
    backend.setChannelData(M.QUALITY_GATE, {
      ...qualityGateFixture('pass'),
      comparison: CI_RELATIONS.map((relation) => ({
        profile: 'full',
        headCommit: 'a41c9e0',
        local: {},
        ci: {},
        relation
      }))
    })
  }
  const failure = FAILURE_BY_SCENARIO[scenario]
  if (failure) {
    backend.setHandler(M.QUALITY_START, () => {
      throw new Error(
        `${failure[0]}: ${failure[1]}${failure[2] ? ` | ${JSON.stringify(failure[2])}` : ''}`
      )
    })
  }
}

/** Run-progress push (percent null = unknown), then the terminal frame. */
export function pushQualityRun(
  backend: FakeCodeIntelBackend,
  frames: (
    | 'progress-null'
    | 'progress-40'
    | 'succeeded'
    | 'failed'
    | 'cancelled'
    | 'interrupted'
    | 'gate-changed'
  )[]
): void {
  for (const frame of frames) {
    if (frame === 'progress-null' || frame === 'progress-40') {
      backend.pushQuality({
        event: 'quality.progress',
        runId: 'run-1',
        stage: 'lint',
        stepIndex: 1,
        stepCount: 2,
        percent: frame === 'progress-40' ? 40 : null,
        message: 'running lint'
      })
    } else if (frame === 'gate-changed') {
      backend.pushQuality({
        event: 'quality.gateChanged',
        headCommit: 'a41c9e0',
        previousVerdict: 'warn',
        verdict: 'pass',
        profile: 'full@repo/v2'
      })
    } else {
      backend.pushQuality({
        event: 'quality.finished',
        runId: 'run-1',
        status: frame,
        summary: {},
        headCommit: 'a41c9e0'
      })
    }
  }
}

/** QualityCall backed by the fake backend, classifying errors like the real client does. */
export function createFakeQualityCall(backend: FakeCodeIntelBackend): QualityCall {
  return async (worktreeId, method, params) => {
    const response = await backend.callEnvelope(method, {
      ...backend.selector,
      worktreeId,
      ...params
    })
    if (response.ok) {
      return { ok: true, result: response.result }
    }
    const parsed = parseCodeIntelErrorMessage(response.error?.message ?? '')
    const kind = codeIntelErrorKindForCode(parsed.code)
    return {
      ok: false,
      error: {
        kind,
        code: parsed.code,
        message: parsed.text,
        data: parsed.data,
        retryable: kind === 'offline' || kind === 'rate-limited'
      }
    }
  }
}
