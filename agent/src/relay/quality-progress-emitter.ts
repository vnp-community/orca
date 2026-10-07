import { ActiveRun } from './quality-run-manager'
import { emitCodeIntelNotification } from './codeintel-notification-sink'

export interface ProgressEmitterDeps {
  emit: (method: string, params: any) => void
  now: () => number
  minIntervalMs?: number
  redact: (text: string) => string
}

export function createProgressEmitter(deps: ProgressEmitterDeps) {
  const minIntervalMs = deps.minIntervalMs ?? 1000
  const lastEmitTime = new Map<string, number>()
  const lastStage = new Map<string, string>()

  function onStage(run: ActiveRun, stage: string, stepIndex: number) {
    const now = deps.now()
    const prevTime = lastEmitTime.get(run.runId) || 0
    const prevStage = lastStage.get(run.runId)

    if (stage === prevStage && (now - prevTime) < minIntervalMs) {
      return
    }

    lastEmitTime.set(run.runId, now)
    lastStage.set(run.runId, stage)

    const stepCount = run.plan.steps.length
    let percent = null
    if (stepCount > 0) {
      const completed = run.journal.steps.filter(s => ['passed', 'failed', 'timeout', 'cancelled', 'skipped'].includes(s.state)).length
      percent = Math.floor(100 * completed / stepCount)
    }

    deps.emit('quality.progress', {
      runId: run.runId,
      workspaceRoot: run.workspaceRoot,
      stage,
      stepIndex,
      percent,
      message: deps.redact(`Running step ${stepIndex} (${stage.replace(/^step:/, '')})`).substring(0, 200)
    })
  }

  function emitFinished(run: ActiveRun, errorCode?: string) {
    const summaryStr = run.journal.steps.map(s => `${s.id}=${s.state}`).join('; ')
    
    deps.emit('quality.finished', {
      runId: run.runId,
      workspaceRoot: run.workspaceRoot,
      status: run.state,
      summary: summaryStr.substring(0, 500),
      steps: run.journal.steps.map(s => ({
        id: s.id,
        status: s.state,
        exitCode: s.exitCode ?? null,
        durationMs: s.durationMs ?? 0,
        truncated: false
      })),
      headCommit: null,
      dirtyFingerprint: run.startFingerprint,
      workTreeChangedDuringRun: false, // will calculate if end != start
      startedAt: run.startedAt,
      finishedAt: run.journal.finishedAt || deps.now(),
      errorCode: errorCode ?? null
    })
  }

  return { onStage, emitFinished }
}
