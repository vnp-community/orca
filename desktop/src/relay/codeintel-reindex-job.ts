import { randomUUID } from 'node:crypto'
import fs from 'node:fs'
import { CodeIntelError } from './codeintel-errors'
import { assertGitRef } from './codeintel-params-validation'
import { getHeadCommit } from './codeintel-head-commit'
import { detectCodeIntelBinaries } from './codeintel-tool-detection'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'

export type ReindexState = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'
export type ReindexOutcome = 'already_up_to_date' | 'superseded' | 'skipped_scope_repo_root' | 'queue_full' | ''

export interface ReindexJob {
  jobId: string
  state: ReindexState
  workspaceRoot: string
  repoRoot: string
  mode: string
  tools: string[]
  trigger: string
  startedAt: number
  estimate: number | null
  outcome: ReindexOutcome
  skipped: string[] | any[]
}

const MAX_QUEUE = 4
export const activeJobs = new Map<string, ReindexJob>()
export const queue: ReindexJob[] = []

export function clearReindexStateForTest() {
  activeJobs.clear()
  queue.length = 0
}

export function validateReindexParams(params: any): any {
  if (!params || typeof params !== 'object') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object')
  }

  const validKeys = ['workspaceRoot', 'mode', 'tools', 'trigger', 'ifStale', 'expectHead', '_trace']
  for (const key of Object.keys(params)) {
    if (!validKeys.includes(key)) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Unknown parameter: ${key}`, { field: key })
    }
  }

  const { workspaceRoot, mode, tools, trigger, ifStale, expectHead } = params

  if (typeof workspaceRoot !== 'string' || !workspaceRoot.trim()) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid workspaceRoot', { field: 'workspaceRoot' })
  }

  const t = trigger || 'manual'
  if (t !== 'manual' && t !== 'agent_done' && t !== 'head_change') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid trigger', { field: 'trigger' })
  }

  if (ifStale !== undefined && typeof ifStale !== 'boolean') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'ifStale must be boolean', { field: 'ifStale' })
  }

  if (expectHead !== undefined) {
    if (typeof expectHead !== 'string' || !/^[0-9a-f]{7,64}$/.test(expectHead)) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid expectHead format', { field: 'expectHead' })
    }
  }

  if (mode !== undefined && mode !== 'full' && mode !== 'incremental') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid mode', { field: 'mode' })
  }

  if (tools !== undefined) {
    if (!Array.isArray(tools) || !tools.every(x => typeof x === 'string')) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'tools must be array of strings', { field: 'tools' })
    }
  }

  return { ...params, trigger: t }
}

export async function startReindex(
  binding: CodeIntelRepoBinding,
  params: any,
  deps: { config: any, log: any, getIndexedCommit?: (repo: string) => Promise<string|null>, checkFreshness?: () => Promise<boolean> }
): Promise<ReindexJob> {
  if (process.env.ORCA_CODEINTEL_REINDEX === 'off') {
    return createSkippedJob(binding, params, 'cancelled', '')
  }

  const trigger = params.trigger || 'manual'

  if (params.expectHead) {
    const head = await getHeadCommit(binding.workspaceRoot)
    if (head !== params.expectHead) {
      return createSkippedJob(binding, params, 'completed', 'superseded')
    }
  }

  if (binding.linkedWorktree) {
    if (trigger === 'manual') {
      throw new CodeIntelError('CODEINTEL_PATH_NOT_ALLOWED', 'Cannot manually reindex linked worktree', { hint: 'reindex_linked_worktree_unsupported' })
    } else {
      const skipped = [
        { tool: 'gitnexus', reason: 'index_root_is_main_checkout' },
        { tool: 'codegraph', reason: 'index_root_is_main_checkout' }
      ]
      return createSkippedJob(binding, params, 'completed', 'skipped_scope_repo_root', skipped)
    }
  }

  if (params.ifStale && deps.checkFreshness) {
    const isFresh = await deps.checkFreshness()
    if (isFresh) {
      return createSkippedJob(binding, params, 'completed', 'already_up_to_date')
    }
  } else if (params.ifStale && deps.getIndexedCommit) {
    // Fallback if checkFreshness not provided
    const indexed = await deps.getIndexedCommit(binding.toplevel)
    const head = await getHeadCommit(binding.workspaceRoot)
    if (head && indexed === head && !binding.worktreeMismatch) {
      return createSkippedJob(binding, params, 'completed', 'already_up_to_date')
    }
  }

  let tools = params.tools
  if (!tools || tools.length === 0) {
    const bins = await detectCodeIntelBinaries(deps.config)
    tools = []
    if (bins.gitnexus) tools.push('gitnexus')
    if (bins.codegraph) tools.push('codegraph')
  }

  const repoRootPath = fs.existsSync(binding.toplevel) ? fs.realpathSync(binding.toplevel) : binding.toplevel
  const existingActive = Array.from(activeJobs.values()).find(j => 
    (j.state === 'running' || j.state === 'queued') && 
    (fs.existsSync(j.repoRoot) ? fs.realpathSync(j.repoRoot) : j.repoRoot) === repoRootPath
  )
  
  if (existingActive) {
    throw new CodeIntelError('CODEINTEL_REINDEX_IN_PROGRESS', 'Reindex already in progress for this repo', { jobId: existingActive.jobId })
  }

  if (queue.length >= MAX_QUEUE) {
    return createSkippedJob(binding, params, 'cancelled', 'queue_full')
  }

  const job: ReindexJob = {
    jobId: 'ri_' + randomUUID().replace(/-/g, '').substring(0, 23),
    state: 'queued',
    workspaceRoot: binding.workspaceRoot,
    repoRoot: binding.toplevel,
    mode: params.mode || 'incremental',
    tools,
    trigger,
    startedAt: Date.now(),
    estimate: null,
    outcome: '',
    skipped: []
  }

  queue.push(job)
  activeJobs.set(job.jobId, job)

  return job
}

function createSkippedJob(binding: CodeIntelRepoBinding, params: any, state: ReindexState, outcome: ReindexOutcome, skipped: any[] = []): ReindexJob {
  return {
    jobId: 'ri_' + randomUUID().replace(/-/g, '').substring(0, 23),
    state,
    workspaceRoot: binding.workspaceRoot,
    repoRoot: binding.toplevel,
    mode: params.mode || 'incremental',
    tools: params.tools || [],
    trigger: params.trigger || 'manual',
    startedAt: Date.now(),
    estimate: null,
    outcome,
    skipped
  }
}

export function getJob(jobId: string): ReindexJob | null {
  if (jobId === 'latest') {
    const active = Array.from(activeJobs.values())
    if (active.length > 0) return active[active.length - 1]
    return null
  }
  return activeJobs.get(jobId) || null
}
