import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleLayerImports, LAYER_PAIRS } from './codeintel-structural-layer-imports'
import { buildLayerImportsCypher } from './codeintel-structural-facts-queries'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as ToolRunner from './codeintel-tool-runner'
import { PerfCollector } from './codeintel-result-envelope'

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

describe('codeintel-structural-layer-imports', () => {
  let ctx: CodeIntelRequestContext
  let binding: any

  beforeEach(() => {
    vi.clearAllMocks()
    ctx = {
      config: {} as any,
      log: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() } as any,
      signal: new AbortController().signal,
      deadline: Date.now() + 10000,
      notifier: { notify: vi.fn() },
      perf: new PerfCollector()
    }
    binding = {
      toplevel: '/repo',
      gitNexusRegistryPath: '/repo/.gitnexus',
      mainCheckoutRoot: '/repo'
    }
  })

  it('buildLayerImportsCypher processes all constants and escapes quotes', () => {
    for (const [, pairAny] of Object.entries(LAYER_PAIRS)) {
      const pair = pairAny as { fromSeg: string, toSeg: string }
      const cypher = buildLayerImportsCypher(pair.fromSeg, pair.toSeg, ['backend-go/services/'])
      expect(cypher).toContain("MATCH (a:File)")
      expect(cypher).toContain("backend-go/services/")
      expect(cypher).not.toContain("DELETE")
      expect(cypher).not.toContain("SET")
    }

    // Quotes are escaped via cypherString (only single quotes are escaped)
    const cypherQuotes = buildLayerImportsCypher("from'quote", "to\"quote", ["path'with\"quotes"])
    expect(cypherQuotes).toContain("from\\'quote")
    expect(cypherQuotes).toContain("to\"quote")
    expect(cypherQuotes).toContain("path\\'with\"quotes")
  })

  it('filters and deduplicates rows correctly', async () => {
    // We simulate tool output for usecase->adapter
    vi.mocked(ToolRunner.runCodeIntelTool).mockImplementation(async (argv: any) => {
      let stdout = ''
      if (argv[1] && argv[1].includes('/internal/usecase/') && argv[1].includes('/internal/adapter/')) {
        stdout = `| a.filePath | b.filePath |
|---|---|
| backend-go/services/svc/internal/usecase/a.go | backend-go/services/svc/internal/adapter/db/foo.go |
| backend-go/services/svc/internal/usecase/a.go | backend-go/services/svc/internal/adapter/db/bar.go |
| backend-go/services/svc/internal/usecase/b.go | backend-go/services/svc/internal/adapter/api/baz.go |
| backend-go/services/svc/internal/usecase/usecasetest/test.go | backend-go/services/svc/internal/adapter/api/test.go |`
      } else if (argv[1] && argv[1].includes('/internal/adapter/') && argv[1].lastIndexOf('/internal/adapter/') !== argv[1].indexOf('/internal/adapter/')) {
        stdout = `| a.filePath | b.filePath |
|---|---|
| backend-go/services/svc/internal/adapter/db/a.go | backend-go/services/svc/internal/adapter/api/b.go |
| backend-go/services/svc/internal/adapter/db/c.go | backend-go/services/svc/internal/adapter/db/d.go |`
      } else {
        stdout = `| a.filePath | b.filePath |\n|---|---|\n`
      }

      return {
        stdout,
        stderr: '',
        exitCode: 0,
        durationMs: 100,
        stdoutBytes: 100
      }
    })

    const result = await handleLayerImports(binding, {}, ctx)

    expect(result.kind).toBe('layerImports')
    // Deduplication should pick smallest toFile per (fromFile, toDir)
    // For a.go -> db/foo.go and db/bar.go, it keeps db/bar.go
    const rows = result.rows
    
    // Check usecase->adapter
    const uaRows = rows.filter((r: any) => r.pair === 'usecase->adapter')
    expect(uaRows).toHaveLength(3)
    
    expect(uaRows[0].fromFile).toBe('backend-go/services/svc/internal/usecase/a.go')
    expect(uaRows[0].toFile).toBe('backend-go/services/svc/internal/adapter/db/bar.go')

    expect(uaRows[1].fromFile).toBe('backend-go/services/svc/internal/usecase/b.go')
    expect(uaRows[1].toFile).toBe('backend-go/services/svc/internal/adapter/api/baz.go')

    // usecasetest is included here because the filter is in cypher query.
    // In actual run, cypher WHERE filters it out. Since we mock cypher output, we just assert deduplication works.
    expect(uaRows[2].fromFile).toBe('backend-go/services/svc/internal/usecase/usecasetest/test.go')

    // Check adapter->adapter
    const aaRows = rows.filter((r: any) => r.pair === 'adapter->adapter')
    expect(aaRows).toHaveLength(1) // db->db is filtered out because same adapter
    expect(aaRows[0].fromFile).toBe('backend-go/services/svc/internal/adapter/db/a.go')
    expect(aaRows[0].toFile).toBe('backend-go/services/svc/internal/adapter/api/b.go')
    
    // Check domain->* is empty
    const duRows = rows.filter((r: any) => r.pair === 'domain->usecase')
    expect(duRows).toHaveLength(0)
  })

  it('runs only specified pair if provided', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: `| a.filePath | b.filePath |\n|---|---|\n`,
      stderr: '',
      exitCode: 0,
      durationMs: 100,
      stdoutBytes: 100
    })

    await handleLayerImports(binding, { pair: 'domain->adapter' }, ctx)

    expect(ToolRunner.runCodeIntelTool).toHaveBeenCalledTimes(1)
    const argv = vi.mocked(ToolRunner.runCodeIntelTool).mock.calls[0][0]
    expect(argv[1]).toContain('/internal/domain/')
    expect(argv[1]).toContain('/internal/adapter/')
  })
})
