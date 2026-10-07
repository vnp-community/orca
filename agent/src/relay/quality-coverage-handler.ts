import { CodeIntelError } from './codeintel-errors'
import { getCoverageReport } from './quality-coverage-collector'
import { getQualityRunManager } from './quality-run-manager'

export interface QualityCoverageParams {
  workspaceRoot: string
  runId: string
}

export interface QualityCoverageResult {
  runId: string
  available: boolean
  reason?: string
  report?: any
}

export async function handleQualityCoverage(
  params: QualityCoverageParams,
  ctx: any
): Promise<QualityCoverageResult> {
  const manager = getQualityRunManager()
  const run = manager.getRun(params.runId)

  if (run) {
    if (run.state === 'queued' || run.state === 'running' || run.state === 'cancelling') {
      throw new CodeIntelError('CODEINTEL_RUN_IN_PROGRESS', `Run ${params.runId} is currently executing`, {
        runId: params.runId
      })
    }
    if (run.state === 'cancelled') {
      throw new CodeIntelError('CODEINTEL_RUN_CANCELLED', `Run ${params.runId} was cancelled`, {
        runId: params.runId
      })
    }
  }

  const report = getCoverageReport(params.workspaceRoot, params.runId)
  if (!report) {
    if (!run) {
      throw new CodeIntelError('CODEINTEL_RUN_NOT_FOUND', `Run not found: ${params.runId}`, {
        runId: params.runId
      })
    }

    // Check if run had coverage step
    const hasCoverageStep = run.plan.steps.some(
      s => s.profile.kind === 'coverage' || s.profile.id.startsWith('coverage-')
    )

    if (!hasCoverageStep) {
      return {
        runId: params.runId,
        available: false,
        reason: 'no_coverage_step'
      }
    }

    return {
      runId: params.runId,
      available: false,
      reason: 'expired'
    }
  }

  return {
    runId: params.runId,
    available: true,
    report
  }
}
