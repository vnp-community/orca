import { CodeIntelRequestContext } from './codeintel-method-table'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import { buildLayerImportsCypher } from './codeintel-structural-facts-queries'
import { buildGitNexusArgv } from './codeintel-command-whitelist'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { getCodeIntelConcurrencyGate } from './codeintel-concurrency-gate'
import { parseCypherOutput } from './gitnexus-cypher-markdown-parser'

export const LAYER_PAIRS: Record<string, { fromSeg: string, toSeg: string }> = {
  'usecase->adapter': { fromSeg: '/internal/usecase/', toSeg: '/internal/adapter/' },
  'domain->usecase': { fromSeg: '/internal/domain/', toSeg: '/internal/usecase/' },
  'domain->adapter': { fromSeg: '/internal/domain/', toSeg: '/internal/adapter/' },
  'adapter->adapter': { fromSeg: '/internal/adapter/', toSeg: '/internal/adapter/' }
}

export async function handleLayerImports(binding: CodeIntelRepoBinding, params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const pairsToRun = params.pair ? [params.pair] : Object.keys(LAYER_PAIRS)
  const pathPrefixes: string[] = params.pathPrefixes || ['backend-go/services/']

  const gate = getCodeIntelConcurrencyGate()
  const runCypher = async (pairName: string) => {
    const pairDef = LAYER_PAIRS[pairName]
    const cypher = buildLayerImportsCypher(pairDef.fromSeg, pairDef.toSeg, pathPrefixes)
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

    // Use dummy template definition to parse two columns: a.filePath, b.filePath
    const dummyTemplate = {
      text: '',
      slots: {},
      columns: [
        { header: 'a.filePath', key: 'a.filePath', type: 'string' },
        { header: 'b.filePath', key: 'b.filePath', type: 'string' }
      ]
    } as any
    const parsed = parseCypherOutput(res.stdout, dummyTemplate)

    return { pairName, rows: parsed.rows }
  }

  // Chạy song song tối đa 2
  const results: any[] = []
  for (let i = 0; i < pairsToRun.length; i += 2) {
    const batch = pairsToRun.slice(i, i + 2).map(p => runCypher(p))
    const batchResults = await Promise.all(batch)
    results.push(...batchResults)
  }

  const allRows: Array<{ pair: string, fromFile: string, toFile: string }> = []

  for (const { pairName, rows } of results) {
    for (const row of rows) {
      const fromFile = row['a.filePath'] as string
      const toFile = row['b.filePath'] as string

      // Khử trùng: adapter->adapter: giữ hàng khi hai phân đoạn adapter/<x> khác nhau
      if (pairName === 'adapter->adapter') {
        // extract /internal/adapter/<x>
        const matchFrom = fromFile.match(/\/internal\/adapter\/([^\/]+)/)
        const matchTo = toFile.match(/\/internal\/adapter\/([^\/]+)/)
        if (matchFrom && matchTo && matchFrom[1] === matchTo[1]) {
          continue // same adapter
        }
      }

      allRows.push({ pair: pairName, fromFile, toFile })
    }
  }

  // Khử trùng: khoá (pair, fromFile, dirname(toFile)), giữ toFile nhỏ nhất từ điển
  const deduplicated = new Map<string, { pair: string, fromFile: string, toFile: string }>()
  
  for (const row of allRows) {
    const toDir = row.toFile.substring(0, row.toFile.lastIndexOf('/'))
    const key = `${row.pair}|${row.fromFile}|${toDir}`
    
    const existing = deduplicated.get(key)
    if (!existing || row.toFile < existing.toFile) {
      deduplicated.set(key, row)
    }
  }

  const finalRows = Array.from(deduplicated.values())

  // sắp (pair, fromFile, toFile)
  finalRows.sort((a, b) => {
    if (a.pair !== b.pair) return a.pair.localeCompare(b.pair)
    if (a.fromFile !== b.fromFile) return a.fromFile.localeCompare(b.fromFile)
    return a.toFile.localeCompare(b.toFile)
  })

  return {
    kind: 'layerImports',
    rows: finalRows
  }
}
