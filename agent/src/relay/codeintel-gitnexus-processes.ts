import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { codeIntelCache, generateCacheKey } from './codeintel-short-lived-cache'
import { runCypherTemplate } from './gitnexus-cypher-runner'
import { getHeadCommit } from './codeintel-head-commit'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { detectCodeIntelTools } from './codeintel-tool-detection'
import { buildSymbolRef, assignUniqueKeys } from './codeintel-symbol-ref'

export async function handleProcesses(params: any, ctx: CodeIntelRequestContext) {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  
  const cacheKey = generateCacheKey({
    registryPath: binding.gitNexusRegistryPath || binding.toplevel,
    versionMarker: binding.stale ? 'stale' : 'ready',
    method: 'processes',
    params: { offset: params.offset, limit: params.limit }
  })

  const payload = await codeIntelCache.getOrCompute(cacheKey, async () => {
    const listP = runCypherTemplate(binding, 'PR_LIST', { offset: params.offset || 0, limit: params.limit || 50 }, ctx)
    const countP = runCypherTemplate(binding, 'PR_COUNT', {}, ctx)

    const [listRes, countRes] = await Promise.all([listP, countP])

    const totalCount = countRes.rows[0]?.['n'] as number || 0

    const nodeIdsToFetch = new Set<string>()
    for (const r of listRes.rows) {
      if (r['p.entryPointId']) nodeIdsToFetch.add(r['p.entryPointId'] as string)
      if (r['p.terminalId']) nodeIdsToFetch.add(r['p.terminalId'] as string)
    }

    const idList = Array.from(nodeIdsToFetch)
    let nodesRes: any = { rows: [] }
    if (idList.length > 0) {
      nodesRes = await runCypherTemplate(binding, 'NODES_BY_ID', { ids: idList }, ctx)
    }

    const nodeMap = new Map<string, any>()
    for (const r of nodesRes.rows) {
      nodeMap.set(r['n.id'], {
        id: r['n.id'],
        label: r['label(n)'],
        name: r['n.name'],
        filePath: r['n.filePath'],
        startLine: r['n.startLine'],
        endLine: r['n.endLine']
      })
    }

    const data = listRes.rows.map(r => {
      let entry: any = null
      let terminal: any = null

      if (r['p.entryPointId']) {
        const n = nodeMap.get(r['p.entryPointId'] as string)
        if (n) {
          entry = buildSymbolRef(n)
        }
      }
      if (r['p.terminalId']) {
        const n = nodeMap.get(r['p.terminalId'] as string)
        if (n) {
          terminal = buildSymbolRef(n)
        }
      }

      return {
        id: r['p.id'],
        label: r['p.label'],
        processType: r['p.processType'],
        stepCount: r['p.stepCount'],
        communities: r['p.communities'] || [],
        entry,
        terminal
      }
    })

    const warnings: string[] = []
    const refsToAssign = data.flatMap(d => [d.entry, d.terminal]).filter(Boolean)
    assignUniqueKeys(refsToAssign, warnings)

    return {
      data,
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
