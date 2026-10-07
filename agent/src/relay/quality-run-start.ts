import { changedFiles } from './quality-changed-files'
import { planRun } from './quality-run-planning'
import { preflightProfile, PreflightCtx } from './quality-environment-preflight'
import { getCatalog } from './quality-profile-catalog'
import { applyOverrides, loadHostOverrides } from './quality-profile-host-overrides'
import { dirtyFingerprint } from './quality-dirty-fingerprint'
import { QualityEnvMissing } from './quality-run-types'
import path from 'path'
import os from 'os'

export interface QualityRunStartParams {
  workspaceRoot: string
  profile?: string
  profiles?: string[]
  suites?: string[]
  base?: string
}

export interface QualityRunStartDeps {
  runManager: any
  home?: string
  tmpDir?: string
  desktopPath?: string
  goWorkPath?: string
  qualityToolPath?: string
  warn: (msg: string) => void
  resolveBin: (bin: string) => string
  gitCommonDir: string
  readGoWork: () => string[]
  now: () => number
}

export class QualityRunStartError extends Error {
  constructor(public code: string, public reason?: string, public data?: any) {
    super(reason || code)
  }
}

export async function startRun(
  params: QualityRunStartParams,
  deps: QualityRunStartDeps
) {
  const t0 = deps.now()
  
  if (!path.isAbsolute(params.workspaceRoot)) {
    throw new QualityRunStartError('INVALID_PARAMS', 'workspaceRoot must be absolute')
  }

  let headCommit = ''
  let dirtyFinger = ''
  const scope = params.base ? 'changed' : 'worktree'

  let files: string[] | null = null
  try {
    files = await changedFiles({ root: params.workspaceRoot, scope, base: params.base })
  } catch (e: any) {
    if (e.message.includes('base_required')) throw new QualityRunStartError('INVALID_PARAMS', 'base_required')
    if (e.message.includes('invalid_base')) throw new QualityRunStartError('INVALID_PARAMS', 'invalid_base')
    if (e.message.includes('missing_base')) throw new QualityRunStartError('CODEINTEL_ENV_NOT_READY', 'base_ref_missing', { missing: [{ reason: 'base_ref_missing' }] })
    throw e
  }

  try {
    dirtyFinger = await dirtyFingerprint(params.workspaceRoot)
  } catch {
    // ignore
  }

  const catalogData = getCatalog()
  const home = deps.home || process.env.HOME || ''
  const overrides = loadHostOverrides(home, { warn: deps.warn })
  const { catalog } = applyOverrides(catalogData.profiles, overrides, { warn: deps.warn })

  let plan
  try {
    plan = planRun({
      profileIds: params.profile ? [params.profile] : params.profiles,
      suiteIds: params.suites,
      catalog,
      suites: catalogData.suites,
      workspaceRoot: params.workspaceRoot,
      changedFiles: files,
      tmpRunDir: path.join(deps.tmpDir || os.tmpdir(), 'orca-quality', 'tmp-run'),
      base: params.base,
      sourceEnv: process.env,
      deps: {
        resolveBin: deps.resolveBin,
        gitCommonDir: deps.gitCommonDir,
        readGoWork: deps.readGoWork
      }
    })
  } catch (e: any) {
    if (e.message.startsWith('Plan error: PROFILE_UNKNOWN')) {
      throw new QualityRunStartError('PROFILE_UNKNOWN')
    }
    throw e
  }

  const ctx: PreflightCtx = {
    repoRoot: params.workspaceRoot,
    cwd: params.workspaceRoot,
    desktopPath: deps.desktopPath || path.join(params.workspaceRoot, 'desktop'),
    goWorkPath: deps.goWorkPath || path.join(params.workspaceRoot, 'go.work'),
    baseRefMissing: false,
    tmpDir: deps.tmpDir || os.tmpdir(),
    qualityToolPath: deps.qualityToolPath || ''
  }

  const finalSteps: any[] = []
  let allMissing = true
  let commonReason = ''
  
  const preflightPromises = plan.steps.map(async (step) => {
    const prof = catalog.find(p => p.id === step.id)
    if (!prof) return { step, ready: true, missing: [] }
    
    // Check if we have time left in our 800ms budget
    const elapsed = deps.now() - t0
    if (elapsed > 800) {
      return { step, ready: true, missing: [] } // assume ready, manager will check
    }

    try {
      const res = await Promise.race([
        preflightProfile(prof, ctx),
        new Promise<any>(resolve => setTimeout(() => resolve(null), 800 - elapsed))
      ])
      if (res === null) {
        // timeout, assume ready
        return { step, ready: true, missing: [] }
      }
      return { step, ready: res.ready, missing: res.missing }
    } catch {
      return { step, ready: true, missing: [] }
    }
  })

  const results = await Promise.all(preflightPromises)

  const missingList: QualityEnvMissing[] = []
  
  for (const { step, ready, missing } of results) {
    if (!ready) {
      finalSteps.push({ id: step.id, title: catalog.find(p => p.id === step.id)?.title || step.id, state: 'env_not_ready' })
      missingList.push(...missing)
      if (missing.length > 0) {
        if (!commonReason) commonReason = missing[0].reason
        else if (commonReason !== missing[0].reason) commonReason = 'multiple'
      }
    } else {
      allMissing = false
      finalSteps.push({ id: step.id, title: catalog.find(p => p.id === step.id)?.title || step.id })
    }
  }

  if (results.length > 0 && allMissing) {
    throw new QualityRunStartError('CODEINTEL_ENV_NOT_READY', commonReason === 'multiple' ? '' : commonReason, { missing: missingList })
  }

  // Actually we need to pass steps to runManager.
  // Wait, runManager executes `PlannedStep`. It does not know about `env_not_ready`!
  // Unless we attach it.
  const managerSteps = plan.steps.map(s => {
    const r = results.find(x => x.step.id === s.id)
    if (r && !r.ready) {
      return { ...s, preflightMissing: r.missing }
    }
    return s
  })

  let submitRes
  try {
    submitRes = await deps.runManager.submit({
      workspaceRoot: params.workspaceRoot,
      steps: managerSteps
    })
  } catch (e: any) {
    if (e.code === 'worktree_busy') throw new QualityRunStartError('WORKTREE_BUSY', undefined, { runId: e.runId })
    if (e.code === 'queue_full') throw new QualityRunStartError('QUEUE_FULL')
    throw e
  }

  return {
    runId: submitRes.runId,
    state: submitRes.state,
    profile: params.profile, // or suites depending on input
    scope: scope,
    steps: finalSteps,
    headCommit,
    dirtyFingerprint: dirtyFinger || submitRes.dirtyFingerprint,
    queuePosition: submitRes.queuePosition
  }
}

export async function handleStartRun(params: QualityRunStartParams, ctx?: any) {
  const { getQualityRunManager } = await import('./quality-run-manager')
  const manager = getQualityRunManager()
  const deps: QualityRunStartDeps = {
    runManager: manager,
    warn: (msg: string) => ctx?.log?.warn?.(msg),
    resolveBin: (b: string) => b,
    gitCommonDir: path.join(params.workspaceRoot, '.git'),
    readGoWork: () => [],
    now: () => Date.now(),
    ...(ctx?.deps ?? {})
  }
  return startRun(params, deps)
}
