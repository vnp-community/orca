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
  skipped: string[]
}

const MAX_QUEUE = 4
export const activeJobs = new Map<string, ReindexJob>()
export const queue: ReindexJob[] = []

export function clearReindexStateForTest() {
  activeJobs.clear()
  queue.length = 0
}

export async function startReindex(
  binding: CodeIntelRepoBinding,
  params: any,
  deps: { config: any, log: any, getIndexedCommit?: (repo: string) => Promise<string|null> }
): Promise<ReindexJob> {
  if (process.env.ORCA_CODEINTEL_REINDEX === 'off') {
    return createSkippedJob(binding, params, 'cancelled', '')
  }

  const trigger = params.trigger || 'manual'

  if (binding.linkedWorktree && trigger !== 'manual') {
    return createSkippedJob(binding, params, 'cancelled', 'skipped_scope_repo_root')
  }

  let tools = params.tools
  if (!tools || tools.length === 0) {
    const bins = await detectCodeIntelBinaries(deps.config)
    tools = []
    if (bins.gitnexus) tools.push('gitnexus')
    if (bins.codegraph) tools.push('codegraph')
  }

  if (params.expectHead) {
    assertGitRef(params.expectHead, 'expectHead')
    const head = await getHeadCommit(binding.workspaceRoot)
    if (head && head !== params.expectHead) {
      return createSkippedJob(binding, params, 'completed', 'already_up_to_date')
    }
  }

  if (params.ifStale && deps.getIndexedCommit) {
    const indexed = await deps.getIndexedCommit(binding.toplevel)
    const head = await getHeadCommit(binding.workspaceRoot)
    if (head && indexed === head && !binding.worktreeMismatch) {
      return createSkippedJob(binding, params, 'completed', 'already_up_to_date')
    }
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

function createSkippedJob(binding: CodeIntelRepoBinding, params: any, state: ReindexState, outcome: ReindexOutcome): ReindexJob {
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
    skipped: []
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
