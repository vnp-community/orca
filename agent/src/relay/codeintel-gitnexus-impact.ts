import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { codeIntelCache, generateCacheKey } from './codeintel-short-lived-cache'
import { getHeadCommit } from './codeintel-head-commit'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { detectCodeIntelTools } from './codeintel-tool-detection'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { CodeIntelError } from './codeintel-errors'
import { runCypherTemplate } from './gitnexus-cypher-runner'
import { buildSymbolRef, parseGitNexusId } from './codeintel-symbol-ref'
import { enrichImpact } from './codeintel-codegraph-enrichment'

export async function handleImpact(params: any, ctx: CodeIntelRequestContext) {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  
  const cacheKey = generateCacheKey({
    registryPath: binding.gitNexusRegistryPath || binding.toplevel,
    versionMarker: binding.stale ? 'stale' : 'ready',
    method: 'impact',
    params: { target: params.target, direction: params.direction, depth: params.depth, limit: params.limit, includeTests: params.includeTests }
  })

  const payload = await codeIntelCache.getOrCompute(cacheKey, async () => {
    let uid = params.target.uid
    if (!uid && params.target.key) {
      const fileRes = await runCypherTemplate(binding, 'FILE_SYMBOLS_BATCH', { paths: [params.target.filePath] }, ctx)
      for (const r of fileRes.rows) {
        const s = buildSymbolRef({ id: r['n.id'] as string, label: r['label(n)'] as string, name: r['n.name'] as string, filePath: r['n.filePath'] as string })
        if (s.key === params.target.key) {
          uid = s.uid
          break
        }
      }
    }
    
    let args = ['impact', '-r', binding.toplevel]
    if (uid) {
      args.push('-u', uid)
    } else {
      args.push(params.target.name)
      if (params.target.file) args.push('-f', params.target.file)
      if (params.target.kind) args.push('--kind', params.target.kind)
    }

    if (params.direction === 'downstream') args.push('-d', 'downstream')
    args.push('--depth', String(params.depth || 2))
    args.push('-l', String(params.limit || 300))
    if (params.includeTests) args.push('--include-tests')

    const timeoutMs = Math.max(100, ctx.deadline - Date.now())
    const res = await runCodeIntelTool({ verb: 'impact', query: '' /* not cypher */, args }, binding, { cwd: binding.toplevel, timeout: timeoutMs, env: ctx.config.toolEnv, signal: ctx.signal, maxOutputBytes: 16 * 1024 * 1024 })
    ctx.perf.recordCli({ tool: 'gitnexus', command: 'impact', ms: res.durationMs, stdoutBytes: res.stdout.length })

    let parsed: any
    try {
      parsed = JSON.parse(res.stdout)
    } catch {
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'truncated_stdout')
    }

    if (parsed.error) {
      if (parsed.error.includes('not found')) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'not_found')
      }
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', parsed.error)
    }

    if (parsed.status === 'ambiguous' || parsed.candidates) {
      throw new CodeIntelError('CODEINTEL_AMBIGUOUS_SYMBOL', 'Ambiguous symbol', {
        candidates: (parsed.candidates || []).map((c: any) => ({
          uid: c.uid,
          name: c.name,
          kind: c.kind,
          filePath: c.filePath,
          line: typeof c.line === 'number' ? c.line + 1 : undefined,
          score: c.score,
          impactedCount: c.impactedCount,
          risk: c.risk
        }))
      })
    }

    const warnings: string[] = []
    if (!params.includeTests) warnings.push('tests_excluded')

    // fetch flows for affected
    const affectedNodeIds = new Set<string>()
    for (const d of parsed.byDepth || []) {
      for (const s of d.symbols || []) {
        affectedNodeIds.add(s.id)
      }
    }

    let affectedFlows: any[] = []
    if (affectedNodeIds.size > 0) {
      const flowRes = await runCypherTemplate(binding, 'SYMBOL_FLOWS', { ids: Array.from(affectedNodeIds) }, ctx)
      const flowMap = new Map<string, any>()
      for (const r of flowRes.rows) {
        flowMap.set(r['p.id'] as string, { id: r['p.id'], label: r['p.label'], stepCount: r['p.stepCount'] })
      }
      affectedFlows = Array.from(flowMap.values())
    }

    const targetRef = buildSymbolRef({ id: parsed.target.id, label: parsed.target.type, name: parsed.target.name || '' }, warnings)
    
    const levels = (parsed.byDepth || []).map((d: any) => {
      return {
        depth: d.depth,
        symbols: (d.symbols || []).map((s: any) => {
          const sLabel = parseGitNexusId(s.id).label
          const ref = buildSymbolRef({ id: s.id, label: sLabel, name: s.name, filePath: s.filePath }, warnings)
          return {
            symbol: ref,
            via: s.relationType,
            confidence: s.confidence,
            direct: d.depth === 1
          }
        })
      }
    })

    const testsCovering = params.includeTests ? (parsed.testsCovering || []) : []
    const affectedClusters = (parsed.summary?.affected_modules || []).map((m: any) => ({ name: m.name, hits: m.hits, impact: m.impact }))

    const baseData = {
      target: targetRef,
      direction: parsed.direction || params.direction || 'upstream',
      risk: parsed.risk,
      impactedCount: parsed.impactedCount,
      levels,
      affectedFlows,
      affectedClusters,
      testsCovering,
      rawSummary: parsed.summary
    }

    const enrichedData = await enrichImpact(
      baseData,
      binding,
      ctx,
      warnings,
      !!params.includeTests
    )

    return {
      data: enrichedData,
      truncated: parsed.pagination?.truncated || false,
      warnings
    }
  }, { cacheable: true })

  const tools = await detectCodeIntelTools(ctx.config)
  const headCommit = await getHeadCommit(params.workspaceRoot)
  const stale = computeStale([{ tool: 'gitnexus', version: tools.gitnexus.version, indexedAt: null, commit: binding.stale ? null : headCommit, lineBase: 1 }], headCommit, { worktreeMismatch: binding.worktreeMismatch })

  return buildCodeIntelResult({
    sources: [{ tool: 'gitnexus', version: tools.gitnexus.version, indexedAt: null, commit: stale ? null : headCommit, lineBase: 1 }],
    headCommit,
    stale,
    truncated: payload.truncated || false,
    totalCount: null,
    warnings: payload.warnings,
    perf: ctx.perf.build(),
    data: payload.data
  })
}
