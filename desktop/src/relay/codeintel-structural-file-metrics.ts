import { CodeIntelRequestContext } from './codeintel-method-table'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import { buildImportInDegreeCypher, buildFileSizesCypher } from './codeintel-structural-facts-queries'
import { buildGitNexusArgv } from './codeintel-command-whitelist'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { getCodeIntelConcurrencyGate } from './codeintel-concurrency-gate'
import { parseCypherOutput } from './gitnexus-cypher-markdown-parser'

export async function handleImportInDegree(binding: CodeIntelRepoBinding, params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const pathPrefixes: string[] = params.pathPrefixes || ['backend-go/services/']
  const offset = typeof params.offset === 'number' ? params.offset : 0
  const limit = typeof params.limit === 'number' ? params.limit : 100
  const limitN = offset + limit

  const cypher = buildImportInDegreeCypher(pathPrefixes, limitN)
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
      { header: 'b.filePath', key: 'file', type: 'string' },
      { header: 'count(DISTINCT a)', key: 'inDegree', type: 'int' }
    ]
  } as any

  const parsed = parseCypherOutput(res.stdout, dummyTemplate)

  const rows = parsed.rows.map(r => ({
    file: r['b.filePath'] as string,
    inDegree: r['count(DISTINCT a)'] as number
  }))

  return {
    kind: 'importInDegree',
    rows
  }
}

export async function handleFileSizes(binding: CodeIntelRepoBinding, params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const pathPrefixes: string[] = params.pathPrefixes || ['backend-go/services/']

  const gate = getCodeIntelConcurrencyGate()

  const runCypher = async (label: string) => {
    const cypher = buildFileSizesCypher(label, pathPrefixes)
    const argv = buildGitNexusArgv({ verb: 'cypher', query: cypher }, binding.gitNexusRegistryPath || binding.toplevel)
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
        { header: 'f.filePath', key: 'file', type: 'string' },
        { header: 'count(s)', key: 'count', type: 'int' },
        { header: 'sum(s.endLine - s.startLine + 1)', key: 'sumLines', type: 'int' },
        { header: 'max(s.endLine - s.startLine + 1)', key: 'maxLines', type: 'int' }
      ]
    } as any

    const parsed = parseCypherOutput(res.stdout, dummyTemplate)
    return parsed.rows
  }

  const [funcRows, methodRows] = await Promise.all([
    runCypher('Function'),
    runCypher('Method')
  ])

  const fileMap = new Map<string, { functions: number, totalLines: number, longestLines: number }>()

  const processRows = (rows: any[]) => {
    for (const r of rows) {
      const file = r['f.filePath'] as string
      const count = r['count(s)'] as number || 0
      const sumLines = r['sum(s.endLine - s.startLine + 1)'] as number || 0
      const maxLines = r['max(s.endLine - s.startLine + 1)'] as number || 0

      if (!fileMap.has(file)) {
        fileMap.set(file, { functions: 0, totalLines: 0, longestLines: 0 })
      }

      const existing = fileMap.get(file)!
      existing.functions += count
      existing.totalLines += sumLines
      existing.longestLines = Math.max(existing.longestLines, maxLines)
    }
  }

  processRows(funcRows)
  processRows(methodRows)

  const mergedRows = Array.from(fileMap.entries()).map(([file, metrics]) => ({
    file,
    functions: metrics.functions,
    totalLines: metrics.totalLines,
    longest: {
      name: '',
      lines: metrics.longestLines
    }
  }))

  mergedRows.sort((a, b) => a.file.localeCompare(b.file))

  return {
    kind: 'fileSizes',
    rows: mergedRows,
    warnings: ['longest_symbol_name_unavailable']
  }
}
