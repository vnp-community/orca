import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { codeIntelCache, generateCacheKey } from './codeintel-short-lived-cache'
import { runCypherTemplate } from './gitnexus-cypher-runner'
import { getHeadCommit } from './codeintel-head-commit'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { detectCodeIntelTools } from './codeintel-tool-detection'

export async function handleOverview(params: any, ctx: CodeIntelRequestContext) {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  
  const cacheKey = generateCacheKey({
    registryPath: binding.gitNexusRegistryPath || binding.toplevel,
    versionMarker: binding.stale ? 'stale' : 'ready',
    method: 'overview',
    params: { topN: params.topN, maxEdges: params.maxEdges, edgeKinds: params.edgeKinds, withTopFiles: params.withTopFiles }
  })

  const payload = await codeIntelCache.getOrCompute(cacheKey, async () => {
    const clustersRes = await runCypherTemplate(binding, 'OV_CLUSTERS', { topN: params.topN }, ctx)
    const countRes = await runCypherTemplate(binding, 'OV_COUNT', {}, ctx)

    const clusterIds = clustersRes.rows.map(r => r['c.id'] as string)
    let topFilesRes: any = { rows: [] }
    let edgesRes: any = { rows: [] }

    if (clusterIds.length > 0) {
      const promises = []
      promises.push(runCypherTemplate(binding, 'OV_EDGES', { kinds: params.edgeKinds, maxEdges: params.maxEdges }, ctx))
      if (params.withTopFiles) {
        promises.push(runCypherTemplate(binding, 'OV_TOPFILES', { ids: clusterIds }, ctx))
      }
      const pRes = await Promise.all(promises)
      edgesRes = pRes[0]
      if (params.withTopFiles) {
        topFilesRes = pRes[1]
      }
    }

    const totalCount = countRes.rows[0]?.['n'] as number || 0

    const fileMap = new Map<string, Array<{ path: string, n: number }>>()
    for (const r of topFilesRes.rows) {
      const id = r['c.id'] as string
      const arr = fileMap.get(id) || []
      arr.push({ path: r['s.filePath'] as string, n: r['n'] as number })
      fileMap.set(id, arr)
    }

    const getArea = (path: string) => {
      const parts = path.split('/')
      return parts[0] || 'unknown'
    }

    const getLang = (path: string) => {
      if (path.endsWith('.ts') || path.endsWith('.tsx')) return 'typescript'
      if (path.endsWith('.js') || path.endsWith('.jsx')) return 'javascript'
      if (path.endsWith('.go')) return 'go'
      if (path.endsWith('.py')) return 'python'
      return 'unknown'
    }

    const nodes = clustersRes.rows.map(r => {
      const id = r['c.id'] as string
      const files = fileMap.get(id) || []
      files.sort((a, b) => b.n - a.n)
      const topFiles = files.slice(0, 3).map(f => f.path)
      
      const langs = files.map(f => getLang(f.path))
      let dominantLanguage = 'unknown'
      if (langs.length > 0) {
        const counts = new Map<string, number>()
        let maxL = '', maxC = 0
        for (const l of langs) {
          const c = (counts.get(l) || 0) + 1
          counts.set(l, c)
          if (c > maxC) { maxC = c; maxL = l }
        }
        dominantLanguage = maxL
      }

      const areas = files.map(f => getArea(f.path))
      let area = 'unknown'
      if (areas.length > 0) {
         const counts = new Map<string, number>()
         let maxL = '', maxC = 0
         for (const a of areas) {
           const c = (counts.get(a) || 0) + 1
           counts.set(a, c)
           if (c > maxC) { maxC = c; maxL = a }
         }
         area = maxL
      }

      return {
        id,
        label: r['c.label'] as string,
        symbolCount: r['c.symbolCount'] as number,
        cohesion: r['c.cohesion'] as number,
        keywords: r['c.keywords'] as string[] || [],
        topFiles,
        dominantLanguage,
        area
      }
    })

    const edges = edgesRes.rows
      .filter((r: any) => clusterIds.includes(r['ca.id']) && clusterIds.includes(r['cb.id']))
      .map((r: any) => {
        const w = r['w'] as number
        const from = r['ca.id'] as string
        const to = r['cb.id'] as string
        const kind = r['r.type'] as string
        return {
          from,
          to,
          weight: w,
          kinds: { [kind]: w }
        }
      })

    const edgeMap = new Map<string, any>()
    for (const e of edges) {
      const k = `${e.from}::${e.to}`
      const existing = edgeMap.get(k)
      if (existing) {
        existing.weight += e.weight
        existing.kinds = { ...existing.kinds, ...e.kinds }
      } else {
        edgeMap.set(k, { from: e.from, to: e.to, weight: e.weight, kinds: e.kinds })
      }
    }

    const finalEdges = Array.from(edgeMap.values())
    const truncated = params.topN < totalCount || finalEdges.length >= params.maxEdges

    return {
      data: { nodes, edges: finalEdges },
      totalCount,
      truncated
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
