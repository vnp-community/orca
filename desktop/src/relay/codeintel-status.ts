import os from 'os'
import fs from 'fs'
import { CodeIntelRequestContext, getIndexProbes } from './codeintel-method-table'
import { CodeIntelError } from './codeintel-errors'
import { detectCodeIntelTools } from './codeintel-tool-detection'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { buildCodeIntelResult } from './codeintel-result-envelope'
import { readCodeIntelLimits } from './codeintel-limits'
import { readHostSnapshot } from './codeintel-host-snapshot'
import { probeIndexBasis } from './codeintel-index-basis-probe'
import { classifyIndexBasis } from './codeintel-index-basis'

export async function handleStatus(
  params: { workspaceRoot: string; baseRef?: string },
  ctx: CodeIntelRequestContext
) {
  if (process.platform === 'win32') {
    throw new CodeIntelError('CODEINTEL_TOOL_UNAVAILABLE', 'Code Intel is not supported on Windows', { reason: 'unsupported_platform' })
  }

  let binding: any = null
  try {
    binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  } catch (err: any) {
    if (err instanceof CodeIntelError && (err.code === 'CODEINTEL_REPO_NOT_REGISTERED' || err.code === 'CODEINTEL_PATH_NOT_ALLOWED')) {
      binding = null
    } else {
      throw err
    }
  }

  const tools = await detectCodeIntelTools(ctx.config)
  const probes = getIndexProbes()
  const sources: any[] = []
  const warnings: string[] = []
  
  let globalHeadCommit: string | null = null

  if (binding) {
    const results = await Promise.allSettled(probes.map(p => p.probe(params.workspaceRoot, ctx)))
    const baseRef = params.baseRef || 'origin/HEAD'
    
    for (let i = 0; i < results.length; i++) {
      const res = results[i]
      const tool = probes[i].tool
      
      if (res.status === 'fulfilled') {
        const val = res.value
        if (val.state !== 'ready') {
          sources.push({ tool, ...val })
          continue
        }

        const indexRoot = (val as any).indexRoot || params.workspaceRoot
        
        let rootMatches = false
        try {
          rootMatches = fs.realpathSync(indexRoot) === fs.realpathSync(params.workspaceRoot)
        } catch {
          rootMatches = false
        }

        const probeRes = await probeIndexBasis(params.workspaceRoot, {
          indexedCommit: val.commit || null,
          indexedAtMs: val.indexedAt ? new Date(val.indexedAt).getTime() : null,
          baseRef
        })

        if (!globalHeadCommit && probeRes.headCommit) {
          globalHeadCommit = probeRes.headCommit
        }

        let pendingChanges = null
        if (tool === 'codegraph' && rootMatches && (val as any).pendingChanges) {
          pendingChanges = (val as any).pendingChanges
        }

        const basis = classifyIndexBasis({
          tool: tool as 'gitnexus' | 'codegraph',
          toolUsable: true,
          indexExists: true,
          rootMatches,
          indexedCommit: val.commit || null,
          indexedAtMs: val.indexedAt ? new Date(val.indexedAt).getTime() : null,
          headCommit: probeRes.headCommit,
          headCommitTimeMs: probeRes.headCommitTimeMs,
          mergeBase: probeRes.mergeBase,
          mergeBaseCommitTimeMs: probeRes.mergeBaseCommitTimeMs,
          dirtySinceIndex: probeRes.dirtySinceIndex,
          pendingChanges
        })

        if (tool === 'gitnexus' && probeRes.headCommit && val.commit && val.commit !== probeRes.headCommit) {
          if (!warnings.includes('index_commit_differs_from_head')) {
            warnings.push('index_commit_differs_from_head')
          }
        }

        sources.push({
          tool,
          state: val.state,
          version: val.version,
          indexedAt: val.indexedAt,
          commit: val.commit,
          lineBase: val.lineBase,
          indexRoot,
          indexScope: basis.indexScope,
          freshness: basis.freshness,
          headCommit: probeRes.headCommit,
          mergeBase: probeRes.mergeBase,
          dirtySinceIndex: probeRes.dirtySinceIndex,
          changedFilesNotInIndex: probeRes.changedFilesNotInIndex,
          pendingChanges,
          rootMismatch: binding.worktreeMismatch ? { worktreeRoot: params.workspaceRoot, indexRoot } : null
        })
      } else {
        warnings.push(`Probe for ${tool} failed: ${res.reason.message}`)
        sources.push({ tool, state: 'unknown', version: null, indexedAt: null, commit: null, lineBase: 1 })
      }
    }
  }

  if (!sources.some(s => s.tool === 'gitnexus')) {
    sources.push({ tool: 'gitnexus', state: 'unknown', version: tools.gitnexus.version, indexedAt: null, commit: null, lineBase: 1 })
  }
  if (!sources.some(s => s.tool === 'codegraph')) {
    sources.push({ tool: 'codegraph', state: 'unknown', version: tools.codegraph.version, indexedAt: null, commit: null, lineBase: 1 })
  }

  let envelopeStale = false
  for (const s of sources) {
    if (s.state === 'ready') {
      if (s.commit && s.headCommit && s.commit !== s.headCommit) envelopeStale = true
      if (s.rootMismatch) envelopeStale = true
      if (s.pendingChanges && (s.pendingChanges.added > 0 || s.pendingChanges.modified > 0 || s.pendingChanges.removed > 0)) envelopeStale = true
    }
  }

  return buildCodeIntelResult({
    sources,
    headCommit: globalHeadCommit,
    stale: envelopeStale,
    truncated: false,
    totalCount: null,
    warnings,
    perf: ctx.perf.build(),
    data: {
      host: readHostSnapshot(),
      limits: readCodeIntelLimits(process.env, ctx.log),
      tools: {
        gitnexus: tools.gitnexus,
        codegraph: tools.codegraph
      }
    }
  })
}
