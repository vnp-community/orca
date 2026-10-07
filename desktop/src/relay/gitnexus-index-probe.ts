import fs from 'fs'
import path from 'path'
import { CodeIntelRequestContext, registerIndexProbe } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { readGitNexusRegistry, findRegistryEntry } from './gitnexus-registry-reader'
import { getHeadCommit } from './codeintel-head-commit'

let metaCache: { mtimeMs: number, schemaVersion?: number, supported: boolean } = { mtimeMs: 0, supported: true }

export function isGitNexusReindexing(repoRoot: string): boolean {
  return false // to be overridden by SOL-004
}

export async function gitnexusIndexProbe(workspaceRoot: string, ctx: CodeIntelRequestContext): Promise<any> {
  const binding = await resolveCodeIntelRepo(workspaceRoot, ctx.config, ctx.log)
  const registry = readGitNexusRegistry(ctx.config)
  const entry = findRegistryEntry(registry, binding.toplevel)

  if (!entry) {
    return { state: 'missing', lineBase: 1 }
  }

  const storageDir = entry.storagePath || path.join(binding.toplevel, '.gitnexus')
  const lbugPath = path.join(storageDir, 'lbug')
  
  let hasLbug = false
  try {
    hasLbug = fs.statSync(lbugPath).isFile()
  } catch {}

  if (!hasLbug) {
    return { state: 'missing', lineBase: 1 }
  }

  if (isGitNexusReindexing(binding.toplevel)) {
    return { state: 'building', lineBase: 1 }
  }

  const headCommit = await getHeadCommit(workspaceRoot)
  const indexedCommit = entry.lastCommit || ''
  
  let state = 'ready'
  if (indexedCommit !== headCommit || binding.worktreeMismatch) {
    state = 'stale'
  }

  // Count missing shadow files
  const indicators: string[] = []
  try {
    const files = fs.readdirSync(storageDir)
    let shadowCount = 0
    for (const f of files) {
      if (f.startsWith('lbug.wal.missing-shadow.')) {
        shadowCount++
      }
    }
    if (shadowCount > 0) {
      indicators.push(`wal_missing_shadow_files:${shadowCount}`)
    }
  } catch {}

  // read meta.json
  const metaPath = path.join(storageDir, 'meta.json')
  let schemaVersion: number | undefined
  try {
    const stat = fs.statSync(metaPath)
    if (stat.mtimeMs === metaCache.mtimeMs) {
      schemaVersion = metaCache.schemaVersion
    } else {
      const start = Date.now()
      const content = fs.readFileSync(metaPath, 'utf8')
      const parsed = JSON.parse(content)
      schemaVersion = parsed.schemaVersion
      metaCache = { mtimeMs: stat.mtimeMs, schemaVersion, supported: schemaVersion === 5 }
      
      if (Date.now() - start > 200) {
        // Just cache the decision if it takes too long
      }
    }
  } catch {}

  return {
    state,
    indexedCommit,
    indexedAt: entry.indexedAt ? new Date(entry.indexedAt).toISOString() : null,
    branch: entry.branch || 'main',
    stats: entry.stats || {},
    ...(schemaVersion !== undefined && { schemaVersion }),
    storagePath: storageDir,
    ...(indicators.length > 0 && { indicators }),
    lineBase: 1
  }
}

registerIndexProbe('gitnexus', gitnexusIndexProbe)

export async function probeGitNexusIndex(repoRoot: string): Promise<{ indexed: boolean; commit?: string; stale?: boolean }> {
  try {
    const metaPath = path.join(repoRoot, '.gitnexus', 'meta.json')
    if (!fs.existsSync(metaPath)) {
      return { indexed: false }
    }
    const content = fs.readFileSync(metaPath, 'utf8')
    const parsed = JSON.parse(content)
    return {
      indexed: true,
      commit: parsed.lastCommit || parsed.commit,
      stale: false
    }
  } catch {
    return { indexed: false }
  }
}
