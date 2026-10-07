import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { parseCodeGraphQuery } from './codegraph-cli-output'
import { CodeIntelConcurrencyGate } from './codeintel-concurrency-gate'
import { buildSymbolRefFromCodeGraph } from './codeintel-symbol-ref-codegraph'

const searchGate = new CodeIntelConcurrencyGate()

export async function handleCodegraphSearch(params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)

  const args = ['query', binding.toplevel, params.search, '-j']
  if (params.limit) {
    args.push('--limit', String(params.limit))
  }
  if (params.kind) {
    args.push('--kind', params.kind)
  }

  const res = await runCodeIntelTool(
    args,
    binding.toplevel,
    ctx.config,
    searchGate,
    { deadline: ctx.deadline, tool: 'codegraph', signal: ctx.signal },
    ctx
  )

  const rawNodes = parseCodeGraphQuery(res.stdout)
  const results = []
  
  for (const node of rawNodes) {
    const ref = buildSymbolRefFromCodeGraph(node)
    if (ref) {
      results.push(ref)
    }
  }

  return {
    results,
    sources: [{ tool: 'codegraph', commit: null, lineBase: 1 }]
  }
}
