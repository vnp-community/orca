import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { codeIntelCache, generateCacheKey } from './codeintel-short-lived-cache'
import { runCypherTemplate } from './gitnexus-cypher-runner'
import { getHeadCommit } from './codeintel-head-commit'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { detectCodeIntelTools } from './codeintel-tool-detection'
import { buildSymbolRef, assignUniqueKeys } from './codeintel-symbol-ref'
import { CodeIntelError } from './codeintel-errors'
import { enrichSubgraph } from './codeintel-codegraph-enrichment'

export async function handleSubgraph(params: any, ctx: CodeIntelRequestContext) {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  
  if (!params.center || typeof params.center !== 'object') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid center')
  }

  const keys = Object.keys(params.center)
  if (keys.length !== 1 || !['symbol', 'file', 'cluster'].includes(keys[0])) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Center must have exactly one of: symbol, file, cluster')
  }

  const centerType = keys[0]
  const centerValue = params.center[centerType]
  const depth = centerType === 'cluster' || centerType === 'file' ? 1 : (params.depth || 1)
  const kinds = params.kinds || ['CALLS', 'IMPORTS', 'EXTENDS', 'IMPLEMENTS']
  const limit = params.limit || 800

  const cacheKey = generateCacheKey({
    registryPath: binding.gitNexusRegistryPath || binding.toplevel,
    versionMarker: binding.stale ? 'stale' : 'ready',
    method: 'subgraph',
    params: { center: params.center, depth, kinds, limit }
  })

  const payload = await codeIntelCache.getOrCompute(cacheKey, async () => {
    let frontier: string[] = []
    let nodesToFetch = new Set<string>()
    const edgesMap = new Map<string, any>()
    let truncated = false

    if (centerType === 'cluster') {
      const memRes = await runCypherTemplate(binding, 'SG_CLUSTER_MEMBERS', { id: centerValue, limit }, ctx)
      frontier = memRes.rows.map((r: any) => r['s.id'] as string)
      for (const id of frontier) nodesToFetch.add(id)
    } else if (centerType === 'file') {
      const fileRes = await runCypherTemplate(binding, 'SG_FILE_SYMBOLS', { path: centerValue, limit }, ctx)
      frontier = fileRes.rows.map((r: any) => r['n.id'] as string)
      for (const id of frontier) nodesToFetch.add(id)
    } else {
      frontier = [centerValue]
      nodesToFetch.add(centerValue)
    }

    for (let d = 0; d < depth; d++) {
      if (frontier.length === 0) break
      if (nodesToFetch.size >= limit || edgesMap.size >= 4000) {
        truncated = true
        break
      }

      const nextFrontier = new Set<string>()
      // batch frontier to max 300
      for (let i = 0; i < frontier.length; i += 300) {
        const batch = frontier.slice(i, i + 300)
        const eRes = await runCypherTemplate(binding, 'SG_EDGES_AROUND', { frontier: batch, kinds, limit: 4000 }, ctx)
        for (const r of eRes.rows) {
          const a = r['a.id'] as string
          const b = r['b.id'] as string
          if (!edgesMap.has(`${a}::${b}::${r['e.type']}`)) {
            edgesMap.set(`${a}::${b}::${r['e.type']}`, {
              from: a,
              to: b,
              kind: r['e.type'],
              confidence: r['e.confidence'],
              reason: r['e.reason']
            })
            if (!nodesToFetch.has(a)) nextFrontier.add(a)
            if (!nodesToFetch.has(b)) nextFrontier.add(b)
            nodesToFetch.add(a)
            nodesToFetch.add(b)
          }
        }
      }
      frontier = Array.from(nextFrontier)
    }

    const allNodeIds = Array.from(nodesToFetch)
    if (allNodeIds.length > limit) {
      allNodeIds.length = limit
      truncated = true
    }

    const nodeRows: any[] = []
    for (let i = 0; i < allNodeIds.length; i += 300) {
      const batch = allNodeIds.slice(i, i + 300)
      const res = await runCypherTemplate(binding, 'NODES_BY_ID', { ids: batch }, ctx)
      nodeRows.push(...res.rows)
    }

    const warnings: string[] = []
    let nodes = nodeRows.map(r => buildSymbolRef({
      id: r['n.id'] as string,
      label: r['label(n)'] as string,
      name: r['n.name'] as string,
      filePath: r['n.filePath'] as string,
      startLine: r['n.startLine'] as number,
      endLine: r['n.endLine'] as number
    }, warnings))

    // filter Const, Variable, Property, Section if not center
    if (centerType === 'symbol') {
      nodes = nodes.filter(n => {
        if (n.uid === centerValue) return true
        return !['Const', 'Variable', 'Property', 'Section'].includes(n.rawKind || n.kind)
      })
    }

    assignUniqueKeys(nodes, warnings)

    const baseSubgraph = {
      center: params.center,
      nodes,
      edges: Array.from(edgesMap.values())
    }

    const enrichedSubgraph = await enrichSubgraph(baseSubgraph, binding, ctx, warnings)

    return {
      data: enrichedSubgraph,
      truncated,
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
