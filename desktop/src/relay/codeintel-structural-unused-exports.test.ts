import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleUnusedExports } from './codeintel-structural-unused-exports'
import { buildUnusedExportsCypher } from './codeintel-structural-facts-queries'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as ToolRunner from './codeintel-tool-runner'
import { PerfCollector } from './codeintel-result-envelope'

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

describe('codeintel-structural-unused-exports', () => {
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

  it('generates correct cypher query', () => {
    const cypher = buildUnusedExportsCypher(['foo/', 'bar/'])
    expect(cypher).toContain('MATCH (f:Function)')
    expect(cypher).toContain('f.isExported = true')
    expect(cypher).toContain("NOT f.filePath ENDS WITH '_test.go'")
    expect(cypher).toContain("NOT f.filePath CONTAINS '/cmd/'")
    expect(cypher).toContain("NOT EXISTS { MATCH (x)-[r:CodeRelation]->(f) WHERE r.type IN ['CALLS', 'ACCESSES', 'IMPORTS'] }")
    expect(cypher).toContain("f.filePath STARTS WITH 'foo/' OR f.filePath STARTS WITH 'bar/'")
  })

  it('parses output, maps to SymbolRef and handles key collisions', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: `| f.id | f.name | label | f.filePath | f.startLine | f.endLine |
|---|---|---|---|---|---|
| 1 | foo | Function | a.go | 10 | 20 |
| 2 | bar | Function | b.go | 15 | 25 |
| 3 | foo | Function | a.go | 30 | 40 |`,
      stderr: '',
      exitCode: 0,
      durationMs: 100,
      stdoutBytes: 100
    })

    const result = await handleUnusedExports(binding, { offset: 0, limit: 10 }, ctx)

    expect(result.kind).toBe('unusedExports')
    expect(result.rows).toHaveLength(3)

    // Check SymbolRef properties
    const ref1 = result.rows[0].symbol
    expect(ref1.uid).toBe('1')
    expect(ref1.name).toBe('foo')
    expect(ref1.kind).toBe('function')
    expect(ref1.filePath).toBe('a.go')
    expect(ref1.startLine).toBe(11) // 1-based (10 + 1)
    expect(ref1.endLine).toBe(21) // 1-based (20 + 1)

    // Check key collision handling
    // a.go's first foo
    expect(ref1.key).toBe('function:a.go:foo')

    // a.go's second foo
    const ref3 = result.rows[2].symbol
    expect(ref3.key).toBe('function:a.go:foo#L31')

    expect(result.warnings).toContain('key_collision')
    expect(result.pagination).toBeUndefined()
  })
})
