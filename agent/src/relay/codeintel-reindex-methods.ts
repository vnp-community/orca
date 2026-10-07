import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { CodeIntelError } from './codeintel-errors'
import { validateReindexParams, startReindex, activeJobs, ReindexJob } from './codeintel-reindex-job'
import { runReindexJob, cancelReindex } from './codeintel-reindex-runner'
import { readReindexJournal } from './codeintel-reindex-journal'
import { enableWatch, disableWatch, getWatchingRoots } from './codeintel-index-watcher'

export function validateReindex(params: any): any {
  return validateReindexParams(params)
}

export async function handleReindex(params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  const job = await startReindex(binding, params, { config: ctx.config, log: ctx.log })

  if (job.state === 'running' || job.state === 'queued') {
    // Fire and forget background runner
    void runReindexJob(job, { config: ctx.config, log: ctx.log }).catch(() => {})
  }

  return {
    jobId: job.jobId,
    state: job.state,
    tools: job.tools,
    startedAt: job.startedAt,
    estimate: job.estimate,
    outcome: job.outcome,
    skipped: job.skipped
  }
}

export function validateReindexStatus(params: any): any {
  if (!params || typeof params !== 'object') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object')
  }
  const { workspaceRoot, jobId } = params
  if (typeof workspaceRoot !== 'string' || !workspaceRoot.trim()) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid workspaceRoot')
  }
  if (jobId !== undefined && (typeof jobId !== 'string' || !jobId.trim())) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid jobId')
  }
  return { workspaceRoot, jobId }
}

export async function handleReindexStatus(params: any, _ctx: CodeIntelRequestContext): Promise<any> {
  const { workspaceRoot, jobId } = params

  if (jobId) {
    const active = activeJobs.get(jobId)
    if (active) return { job: active }
    const journal = readReindexJournal(workspaceRoot, jobId)
    return { job: journal || null }
  }

  // Find most recent job for workspaceRoot
  for (const job of activeJobs.values()) {
    if (job.workspaceRoot === workspaceRoot || job.repoRoot === workspaceRoot) {
      return { job }
    }
  }

  return { job: null }
}

export function validateReindexCancel(params: any): any {
  if (!params || typeof params !== 'object') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object')
  }
  const { workspaceRoot, jobId } = params
  if (typeof workspaceRoot !== 'string' || !workspaceRoot.trim()) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid workspaceRoot')
  }
  if (typeof jobId !== 'string' || !jobId.trim()) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid jobId')
  }
  return { workspaceRoot, jobId }
}

export async function handleReindexCancel(params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const cancelled = await cancelReindex(params.jobId, { config: ctx.config, log: ctx.log })
  return {
    jobId: params.jobId,
    state: 'cancelled',
    cancelled
  }
}

export function validateWatch(params: any): any {
  if (!params || typeof params !== 'object') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object')
  }
  const { workspaceRoot, enabled } = params
  if (typeof workspaceRoot !== 'string' || !workspaceRoot.trim()) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid workspaceRoot')
  }
  if (typeof enabled !== 'boolean') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'enabled must be boolean')
  }
  return { workspaceRoot, enabled }
}

export async function handleWatch(params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  if (params.enabled) {
    enableWatch(binding)
  } else {
    disableWatch(binding.toplevel)
  }

  return {
    enabled: params.enabled,
    watching: getWatchingRoots()
  }
}
