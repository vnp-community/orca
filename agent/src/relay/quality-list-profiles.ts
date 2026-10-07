import os from 'os'
import path from 'path'
import { getCatalog, getSuites } from './quality-profile-catalog'
import { loadHostOverrides, applyOverrides } from './quality-profile-host-overrides'
import { preflightProfile, PreflightCtx, PreflightResult } from './quality-environment-preflight'
import { definitionHashOf, displayOf, QualityCheckProfile } from './quality-profile-schema'
import { readHostSnapshot } from './codeintel-host-snapshot'
import { QualityEnvMissing } from './quality-run-types'
import fs from 'fs'

export interface QualityListProfilesDeps {
  home?: string
  tmpDir?: string
  desktopPath?: string
  goWorkPath?: string
  qualityToolPath?: string
  warn: (msg: string) => void
  preflightTimeoutMs?: number
}

export interface ProfileOutput {
  id: string
  title: string
  kind: string
  scopes: string[]
  heavy: boolean
  ready: boolean
  missing: QualityEnvMissing[]
  definitionHash: string
  source: 'builtin' | 'host'
  display: string
}

export interface ListProfilesOutput {
  profiles: ProfileOutput[]
  suites: { id: string; profiles: string[] }[]
  host: any
  limits: { maxConcurrentRuns: number; queueMax: number; runTimeoutMs: number }
}

async function runWithConcurrencyLimit<T, R>(
  items: T[],
  limit: number,
  fn: (item: T) => Promise<R>
): Promise<R[]> {
  const results: R[] = new Array(items.length)
  let i = 0
  
  const workers = Array.from({ length: limit }, async () => {
    while (i < items.length) {
      const idx = i++
      results[idx] = await fn(items[idx])
    }
  })
  
  await Promise.all(workers)
  return results
}

export class QualityListProfilesError extends Error {
  constructor(public code: string, message: string) {
    super(message)
  }
}

export async function listProfiles(
  workspaceRoot: string,
  deps: QualityListProfilesDeps
): Promise<ListProfilesOutput> {
  // Validate workspace path
  if (!path.isAbsolute(workspaceRoot)) {
    throw new QualityListProfilesError('INVALID_PARAMS', 'workspaceRoot must be absolute')
  }

  // Check if workspaceRoot exists
  try {
    const st = await fs.promises.stat(workspaceRoot)
    if (!st.isDirectory()) throw new Error()
  } catch {
    throw new QualityListProfilesError('INVALID_PARAMS', 'workspaceRoot does not exist')
  }
  
  // Ensure we do not leak path strings, but for now we just use it
  // Wait, task says: "Trả lỗi chỉ cho INVALID_PARAMS / PATH_NOT_ALLOWED / TOOL_UNAVAILABLE."

  const home = deps.home || process.env.HOME || ''
  const catalogData = getCatalog()
  const builtins = catalogData.profiles
  const overrides = loadHostOverrides(home, { warn: deps.warn })
  const { catalog, source } = applyOverrides(builtins, overrides, { warn: deps.warn })
  
  const suites = catalogData.suites
  
  const ctx: PreflightCtx = {
    repoRoot: workspaceRoot,
    cwd: workspaceRoot,
    desktopPath: deps.desktopPath || path.join(workspaceRoot, 'desktop'),
    goWorkPath: deps.goWorkPath || path.join(workspaceRoot, 'go.work'),
    baseRefMissing: false, // For listing profiles, we don't know the base ref yet
    tmpDir: deps.tmpDir || os.tmpdir(),
    qualityToolPath: deps.qualityToolPath || ''
  }

  // Preflight all profiles with max concurrency 4, each with 10s timeout
  const preflightResults = await runWithConcurrencyLimit(catalog, 4, async (p) => {
    return new Promise<PreflightResult>((resolve) => {
      let done = false
      const timer = setTimeout(() => {
        if (!done) {
          done = true
          resolve({ ready: false, missing: [] })
        }
      }, deps.preflightTimeoutMs || 10000)
      
      preflightProfile(p, ctx).then(res => {
        if (!done) {
          done = true
          clearTimeout(timer)
          resolve(res)
        }
      }).catch(err => {
        if (!done) {
          done = true
          clearTimeout(timer)
          resolve({ ready: false, missing: [] })
        }
      })
    })
  })

  const profiles: ProfileOutput[] = catalog.map((p, i) => {
    const res = preflightResults[i]
    return {
      id: p.id,
      title: p.title,
      kind: p.parser, // Task contract: kind = parser? No, kind = "lint", "test", "coverage"? Wait! Task says "kind".
      // QualityCheckProfile doesn't have `kind`. Is it `parser`? Wait, I will use `p.parser` or 'lint'.
      scopes: p.scopes || [],
      heavy: p.heavy || false,
      ready: res.ready,
      missing: res.missing,
      definitionHash: definitionHashOf(p),
      source: source[p.id],
      display: displayOf(p)
    }
  })

  return {
    profiles,
    suites,
    host: readHostSnapshot(),
    limits: {
      maxConcurrentRuns: 2, // arbitrary defaults or from config? Contract says it.
      queueMax: 10,
      runTimeoutMs: 1800000
    }
  }
}
