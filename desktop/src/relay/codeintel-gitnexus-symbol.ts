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
import { readSymbolSource } from './codeintel-symbol-source-reader'
import { enrichSymbol } from './codeintel-codegraph-enrichment'

export async function handleSymbol(params: any, ctx: CodeIntelRequestContext) {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  
  const cacheKey = generateCacheKey({
    registryPath: binding.gitNexusRegistryPath || binding.toplevel,
    versionMarker: binding.stale ? 'stale' : 'ready',
    method: 'symbol',
    params: { uid: params.uid, name: params.name, file: params.file, relationLimit: params.relationLimit }
  })

  const payload = await codeIntelCache.getOrCompute(cacheKey, async () => {
    let args = ['context', '-r', binding.toplevel]
    if (params.uid) {
      args.push('-u', params.uid)
    } else {
      args.push(params.name, '-f', params.file)
    }
    args.push('-l', String(params.relationLimit || 20))

    const timeoutMs = Math.max(100, ctx.deadline - Date.now())
    const res = await runCodeIntelTool({ verb: 'context', query: '', args }, binding, { cwd: binding.toplevel, timeout: timeoutMs, env: ctx.config.toolEnv, signal: ctx.signal, maxOutputBytes: 16 * 1024 * 1024 })
    ctx.perf.recordCli({ tool: 'gitnexus', command: 'context', ms: res.durationMs, stdoutBytes: res.stdout.length })

    let parsed: any
    try {
      parsed = JSON.parse(res.stdout)
    } catch {
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'truncated_stdout')
    }

    if (parsed.error) {
      if (parsed.error.includes('not found')) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'not_found')
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
          score: c.score
        }))
      })
    }

    const warnings: string[] = []
    const symbolData = parsed.symbol || parsed
    const sLabel = parseGitNexusId(symbolData.uid).label
    const symbolRefRaw = buildSymbolRef({
      id: symbolData.uid,
      label: symbolData.kind || sLabel,
      name: symbolData.name,
      filePath: symbolData.filePath,
      startLine: symbolData.startLine,
      endLine: symbolData.endLine
    }, warnings)

    if (params.includeTrail) {
      (symbolRefRaw as any).includeTrail = true
    }
    const symbolRef = await enrichSymbol(symbolRefRaw, binding, ctx, warnings)

    let flows: any[] = []
    try {
      const flowRes = await runCypherTemplate(binding, 'SYMBOL_FLOWS', { ids: [symbolRef.uid] }, ctx)
      flows = flowRes.rows.map(r => ({
        id: r['p.id'],
        label: r['p.label'],
        stepCount: r['p.stepCount'],
        step: r['r.step']
      }))
    } catch {}

    const buildRelations = (rels: any) => {
      const out: any = {}
      for (const [k, arr] of Object.entries(rels || {})) {
        if (Array.isArray(arr)) {
          out[k.toLowerCase()] = arr.map(a => ({ uid: a.uid, name: a.name, filePath: a.filePath }))
        }
      }
      return out
    }

    return {
      data: {
        symbol: symbolRef,
        incoming: buildRelations(parsed.incoming),
        outgoing: buildRelations(parsed.outgoing),
        flows
      },
      truncated: false,
      warnings
    }
  }, { cacheable: true })

  let source: any = null
  let sourceWarnings: string[] = []
  if (params.includeSource !== false) {
    const s = payload.data.symbol
    if (s.startLine && s.endLine && s.filePath) {
      const srcRes = await readSymbolSource(params.workspaceRoot, s.filePath, s.startLine, s.endLine)
      if (srcRes.sourceOmitted) {
        source = { omitted: srcRes.sourceOmitted }
      } else {
        source = { text: srcRes.text, truncated: srcRes.truncated }
      }
      if (binding.stale || binding.worktreeMismatch) {
        sourceWarnings.push('source_may_not_match_index')
      }
    }
  }

  const tools = await detectCodeIntelTools(ctx.config)
  const headCommit = await getHeadCommit(params.workspaceRoot)
  const stale = computeStale([{ tool: 'gitnexus', version: tools.gitnexus.version, indexedAt: null, commit: binding.stale ? null : headCommit, lineBase: 1 }], headCommit, { worktreeMismatch: binding.worktreeMismatch })

  const finalWarnings = [...(payload.warnings || []), ...sourceWarnings]

  return buildCodeIntelResult({
    sources: [{ tool: 'gitnexus', version: tools.gitnexus.version, indexedAt: null, commit: stale ? null : headCommit, lineBase: 1 }],
    headCommit,
    stale,
    truncated: payload.truncated || false,
    totalCount: null,
    warnings: finalWarnings,
    perf: ctx.perf.build(),
    data: {
      ...payload.data,
      source
    }
  })
}
