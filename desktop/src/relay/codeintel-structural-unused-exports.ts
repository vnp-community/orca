import { CodeIntelRequestContext } from './codeintel-method-table'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import { buildUnusedExportsCypher } from './codeintel-structural-facts-queries'
import { buildGitNexusArgv } from './codeintel-command-whitelist'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { getCodeIntelConcurrencyGate } from './codeintel-concurrency-gate'
import { parseCypherOutput } from './gitnexus-cypher-markdown-parser'
import { buildSymbolRef, assignUniqueKeys, SymbolRef } from './codeintel-symbol-ref'

export async function handleUnusedExports(binding: CodeIntelRepoBinding, params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const pathPrefixes: string[] = params.pathPrefixes || ['backend-go/services/']
  const cypher = buildUnusedExportsCypher(pathPrefixes)
  const argv = buildGitNexusArgv({ verb: 'cypher', query: cypher }, binding.gitNexusRegistryPath || binding.toplevel)

  const gate = getCodeIntelConcurrencyGate()
  const tmpPerf: any[] = []
  
  const res = await runCodeIntelTool(
    argv,
    binding.toplevel,
    ctx.config,
    gate,
    { tool: 'gitnexus', deadline: ctx.deadline, signal: ctx.signal },
    { perf: tmpPerf }
  )

  if (tmpPerf.length > 0) {
    ctx.perf.recordCli(tmpPerf[0])
  }

  const dummyTemplate = {
    text: '',
    slots: {},
    columns: [
      { header: 'f.id', key: 'id', type: 'string' },
      { header: 'f.name', key: 'name', type: 'string' },
      { header: 'label', key: 'label', type: 'string' },
      { header: 'f.filePath', key: 'filePath', type: 'string' },
      { header: 'f.startLine', key: 'startLine', type: 'int' },
      { header: 'f.endLine', key: 'endLine', type: 'int' }
    ]
  } as any

  const parsed = parseCypherOutput(res.stdout, dummyTemplate)

  const warnings: string[] = []
  const allRefs: SymbolRef[] = []

  for (const r of parsed.rows) {
    const raw = {
      id: r['f.id'] as string,
      name: r['f.name'] as string,
      label: r['label'] as string,
      filePath: r['f.filePath'] as string,
      startLine: typeof r['f.startLine'] === 'number' ? r['f.startLine'] : null,
      endLine: typeof r['f.endLine'] === 'number' ? r['f.endLine'] : null
    }
    allRefs.push(buildSymbolRef(raw, warnings))
  }

  // Handle key collisions
  assignUniqueKeys(allRefs, warnings)

  const rows = allRefs.map(ref => ({ symbol: ref }))

  return {
    kind: 'unusedExports',
    rows,
    warnings: warnings.length > 0 ? Array.from(new Set(warnings)) : undefined
  }
}
