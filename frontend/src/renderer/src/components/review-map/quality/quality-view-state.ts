/**
 * quality-view-state.ts — FE-CV-TASK-087-04
 *
 * Reduces gate/run/load facts to what the quality lens shows. `primary` is the most important
 * condition; `notices` are banners that can sit above a scorecard. Everything is inline and
 * persistent (never a toast), and a failed or unfinished run never reads as a clean result.
 *
 * @module components/review-map/quality/quality-view-state
 */

import type { CodeIntelErrorKind } from '../../../../../shared/code-intel-parsers'
import type {
  QualityGateResponse,
  QualityRun
} from '../../../../../shared/code-intel-quality-types'
import type {
  ActiveQualityRun,
  QualityRunError
} from '../../../store/slices/code-intel-quality-state'

export type QualityViewKind =
  | 'loading'
  | 'not-run'
  | 'ready'
  | 'ready-empty'
  | 'load-error'
  | 'forbidden'
  | 'running'
  | 'run-failed'
  | 'run-cancelled'
  | 'env-not-ready'
  | 'profile-unknown'
  | 'rate-limited'
  | 'run-forbidden'
  | 'offline'
  | 'result-stale'
  | 'index-stale'
  | 'truncated'

export type QualityViewInput = {
  gateStatus: 'idle' | 'loading' | 'ready' | 'error'
  gate: QualityGateResponse | null
  gateErrorKind: CodeIntelErrorKind | null
  resultStale: boolean
  indexBehindHead: boolean
  runs: readonly QualityRun[] | null
  run: ActiveQualityRun | null
  runError: QualityRunError | null
}

export type QualityViewState = {
  primary: QualityViewKind
  notices: QualityViewKind[]
  showScorecard: boolean
  /** Running is impossible right now (offline / no permission), as opposed to merely busy. */
  lockRun: boolean
}

const RUN_ERROR_KIND: Partial<Record<CodeIntelErrorKind, QualityViewKind>> = {
  'env-not-ready': 'env-not-ready',
  'profile-unknown': 'profile-unknown',
  'rate-limited': 'rate-limited',
  forbidden: 'run-forbidden',
  offline: 'offline'
}

function behindGate(input: QualityViewInput): QualityRun[] {
  const ids = new Set(input.gate?.gate.basedOn.runIds ?? [])
  return (input.runs ?? []).filter((r) => ids.has(r.id))
}

/** True when the runs behind the verdict ran fully and produced nothing to report. */
function ranWithoutFindings(input: QualityViewInput): boolean {
  const runs = behindGate(input)
  return (
    runs.length > 0 &&
    runs.every(
      (r) =>
        r.status === 'succeeded' &&
        r.summary.error + r.summary.warning + r.summary.info === 0 &&
        r.summary.stepsFailed === 0 &&
        r.summary.stepsEnvNotReady === 0
    )
  )
}

export function deriveQualityViewState(input: QualityViewInput): QualityViewState {
  const notices: QualityViewKind[] = []
  const hasGate = input.gate !== null
  const neverRan =
    (hasGate &&
      input.gate?.gate.basedOn.runIds.length === 0 &&
      input.gate.gate.verdict === 'unknown') ||
    (!hasGate && input.runs !== null && input.runs.length === 0 && input.gateStatus !== 'loading')
  const offline = input.gateErrorKind === 'offline' || input.runError?.kind === 'offline'
  const runForbidden = input.runError?.kind === 'forbidden'
  const lockRun = offline || runForbidden || input.gateErrorKind === 'forbidden'

  if (hasGate && input.resultStale) {
    notices.push('result-stale')
  }
  if (hasGate && input.indexBehindHead) {
    notices.push('index-stale')
  }
  if (behindGate(input).some((r) => r.summary.truncated)) {
    notices.push('truncated')
  }
  if (offline && hasGate) {
    notices.push('offline')
  }

  let primary: QualityViewKind
  if (input.run && input.run.phase !== 'finished') {
    primary = 'running'
  } else if (input.runError) {
    primary = RUN_ERROR_KIND[input.runError.kind] ?? 'load-error'
  } else if (input.run?.phase === 'finished' && input.run.status === 'cancelled') {
    primary = 'run-cancelled'
  } else if (
    input.run?.phase === 'finished' &&
    (input.run.status === 'failed' ||
      input.run.status === 'interrupted' ||
      input.run.status === 'unknown')
  ) {
    primary = 'run-failed'
  } else if (input.gateErrorKind === 'forbidden' && !hasGate) {
    primary = 'forbidden'
  } else if (input.gateErrorKind === 'offline' && !hasGate) {
    primary = 'offline'
  } else if (input.gateStatus === 'error' && !hasGate) {
    primary = 'load-error'
  } else if (
    !hasGate &&
    (input.gateStatus === 'loading' || input.gateStatus === 'idle') &&
    !neverRan
  ) {
    primary = 'loading'
  } else if (neverRan) {
    primary = 'not-run'
  } else if (ranWithoutFindings(input)) {
    primary = 'ready-empty'
  } else {
    primary = 'ready'
  }

  return {
    primary,
    notices: notices.filter((n) => n !== primary),
    showScorecard: hasGate && primary !== 'not-run',
    lockRun
  }
}
