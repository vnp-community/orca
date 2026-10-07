import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { codeIntelCache, generateCacheKey } from './codeintel-short-lived-cache'
import { runCypherTemplate } from './gitnexus-cypher-runner'
import { getHeadCommit } from './codeintel-head-commit'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { detectCodeIntelTools } from './codeintel-tool-detection'
import { buildSymbolRef, assignUniqueKeys } from './codeintel-symbol-ref'

export async function handleRoutes(params: any, ctx: CodeIntelRequestContext) {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  
  const cacheKey = generateCacheKey({
    registryPath: binding.gitNexusRegistryPath || binding.toplevel,
    versionMarker: binding.stale ? 'stale' : 'ready',
    method: 'routes',
    params: { offset: params.offset, limit: params.limit }
  })

  const payload = await codeIntelCache.getOrCompute(cacheKey, async () => {
    const listP = runCypherTemplate(binding, 'RT_LIST', { offset: params.offset || 0, limit: params.limit || 200 }, ctx)
    const countP = runCypherTemplate(binding, 'RT_COUNTS', {}, ctx)

    const [listRes, countRes] = await Promise.all([listP, countP])

    let totalCount = 0
    let hasGoRoute = false
    for (const r of countRes.rows) {
      totalCount += (r['n'] as number)
      // Check for go files
      // actually RT_COUNTS doesn't give file extension, it gives label(h)
    }

    const warnings: string[] = []
    
    // Check coverage
    try {
      const fs = await import('fs')
      const path = await import('path')
      if (fs.existsSync(path.join(binding.toplevel, 'backend-go'))) {
        warnings.push('routes_coverage_js_only')
      }
    } catch {}

    const routesMap = new Map<string, any>()
    const edges: any[] = []

    const refsToAssign: any[] = []

    for (const r of listRes.rows) {
      const rId = r['r.id'] as string
      if (!routesMap.has(rId)) {
        routesMap.set(rId, {
          id: rId,
          path: r['r.name'] as string,
          method: r['r.method'],
          filePath: r['r.filePath'],
          middleware: [],
          responseKeys: [],
          errorKeys: []
        })
      }

      const eType = r['e.type'] as string
      const side = eType === 'HANDLES_ROUTE' ? 'server' : 'client'
      
      const hId = r['h.id'] as string
      // handler is file (per CR-002: "handler là SymbolRef kiểu file (không phải symbol)")
      // wait, we need to extract filePath and name from hId
      // e.g. File:some/path:file.ts
      const handlerSymbol = buildSymbolRef({
        id: hId,
        label: 'File', // actually we don't have label(h) here, but CR says it's mostly File
        name: hId.split(':').pop() || '',
        filePath: hId.split(':')[1] || ''
      }, warnings)
      
      refsToAssign.push(handlerSymbol)

      edges.push({
        route: rId,
        handler: handlerSymbol,
        kind: eType,
        side,
        confidence: r['e.confidence']
      })
    }

    assignUniqueKeys(refsToAssign, warnings)

    return {
      data: {
        routes: Array.from(routesMap.values()),
        edges
      },
      totalCount,
      truncated: false,
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
    totalCount: payload.totalCount || null,
    warnings: payload.warnings,
    perf: ctx.perf.build(),
    data: payload.data
  })
}
