import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { codeIntelCache, generateCacheKey } from './codeintel-short-lived-cache'
import { runCypherTemplate } from './gitnexus-cypher-runner'
import { getHeadCommit } from './codeintel-head-commit'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { detectCodeIntelTools } from './codeintel-tool-detection'
import { buildSymbolRef, assignUniqueKeys } from './codeintel-symbol-ref'
import { CodeIntelError } from './codeintel-errors'

export async function handleProcess(params: any, ctx: CodeIntelRequestContext) {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  
  const cacheKey = generateCacheKey({
    registryPath: binding.gitNexusRegistryPath || binding.toplevel,
    versionMarker: binding.stale ? 'stale' : 'ready',
    method: 'process',
    params: { processId: params.processId }
  })

  const payload = await codeIntelCache.getOrCompute(cacheKey, async () => {
    const prRes = await runCypherTemplate(binding, 'PR_ONE', { id: params.processId }, ctx)
    if (prRes.rows.length === 0) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Process not found', { reason: 'not_found' })
    }

    const flowRaw = prRes.rows[0]
    const stepCount = flowRaw['p.stepCount'] as number
    const truncated = stepCount > 200

    const pSteps = runCypherTemplate(binding, 'PR_STEPS', { id: params.processId, limit: 200 }, ctx)
    // We cannot run STEP_EDGES and MEMBER_CLUSTER until we have step nodes
    const stepsRes = await pSteps

    const nodeIds = Array.from(new Set(stepsRes.rows.map(r => r['s.id'] as string)))

    let edgesRes: any = { rows: [] }
    let clusterRes: any = { rows: [] }
    if (nodeIds.length > 0) {
      const [e, c] = await Promise.all([
        runCypherTemplate(binding, 'STEP_EDGES', { ids: nodeIds, kinds: ['CALLS'] }, ctx),
        runCypherTemplate(binding, 'MEMBER_CLUSTER', { ids: nodeIds }, ctx)
      ])
      edgesRes = e
      clusterRes = c
    }

    const clusterMap = new Map<string, string>()
    for (const r of clusterRes.rows) {
      clusterMap.set(r['s.id'] as string, r['c.id'] as string)
    }

    const warnings: string[] = []
    if (truncated) {
      warnings.push('process_steps_truncated')
    }

    const steps = stepsRes.rows.map(r => {
      const sId = r['s.id'] as string
      const symbol = buildSymbolRef({
        id: sId,
        label: r['label(s)'] as string,
        name: r['s.name'] as string,
        filePath: r['s.filePath'] as string,
        startLine: r['s.startLine'] as number,
        endLine: r['s.endLine'] as number
      }, warnings)

      return {
        step: r['r.step'],
        symbol,
        cluster: clusterMap.get(sId) || null,
        filePath: r['s.filePath'],
        startLine: symbol.startLine
      }
    })

    const refsToAssign = steps.map(s => s.symbol)
    assignUniqueKeys(refsToAssign, warnings)

    const edges = edgesRes.rows.map((r: any) => ({
      from: r['a.id'],
      to: r['b.id'],
      kind: r['e.type'],
      confidence: r['e.confidence'],
      reason: r['e.reason']
    }))

    const flow = {
      id: flowRaw['p.id'],
      label: flowRaw['p.label'],
      processType: flowRaw['p.processType'],
      stepCount,
      communities: flowRaw['p.communities'] || []
    }

    return {
      data: { flow, steps, edges },
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
