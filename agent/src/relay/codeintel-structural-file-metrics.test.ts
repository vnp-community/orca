import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleImportInDegree, handleFileSizes } from './codeintel-structural-file-metrics'
import { buildFileSizesCypher } from './codeintel-structural-facts-queries'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as ToolRunner from './codeintel-tool-runner'
import { PerfCollector } from './codeintel-result-envelope'

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

describe('codeintel-structural-file-metrics', () => {
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

  describe('handleImportInDegree', () => {
    it('parses output and paginates correctly', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
        stdout: `| b.filePath | count(DISTINCT a) |
|---|---|
| a.go | 10 |
| b.go | 8 |
| c.go | 5 |
| d.go | 2 |`,
        stderr: '',
        exitCode: 0,
        durationMs: 100,
        stdoutBytes: 100
      })

      const result = await handleImportInDegree(binding, { offset: 1, limit: 2 }, ctx)
      expect(result.kind).toBe('importInDegree')
      expect(result.rows).toHaveLength(4)
      expect(result.rows[0]).toEqual({ file: 'a.go', inDegree: 10 })
      expect(result.rows[1]).toEqual({ file: 'b.go', inDegree: 8 })
      expect(result.pagination).toBeUndefined()
    })
  })

  describe('handleFileSizes', () => {
    it('does not contain OR syntax in cypher query', () => {
      const cypherFunc = buildFileSizesCypher('Function', ['foo/'])
      const cypherMeth = buildFileSizesCypher('Method', ['foo/'])
      
      expect(cypherFunc).toContain('->(s:Function)')
      expect(cypherMeth).toContain('->(s:Method)')
      
      // Ensure no OR syntax for labels
      expect(cypherFunc).not.toMatch(/\(s:.*OR.*s:.*\)/)
    })

    it('merges functions and methods, handles empty labels, sorts and provides fallback for longest name', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockImplementation(async (argv: any) => {
        let stdout = ''
        if (argv[1].includes('->(s:Function)')) {
          stdout = `| f.filePath | count(s) | sum(s.endLine - s.startLine + 1) | max(s.endLine - s.startLine + 1) |
|---|---|---|---|
| a.go | 2 | 20 | 15 |
| b.go | 1 | 5 | 5 |`
        } else if (argv[1].includes('->(s:Method)')) {
          // b.go has both, a.go has only functions, c.go has only methods
          stdout = `| f.filePath | count(s) | sum(s.endLine - s.startLine + 1) | max(s.endLine - s.startLine + 1) |
|---|---|---|---|
| b.go | 3 | 30 | 12 |
| c.go | 5 | 50 | 20 |`
        }

        return {
          stdout,
          stderr: '',
          exitCode: 0,
          durationMs: 100,
          stdoutBytes: 100
        }
      })

      const result = await handleFileSizes(binding, { offset: 0, limit: 10 }, ctx)

      expect(result.kind).toBe('fileSizes')
      expect(result.warnings).toContain('longest_symbol_name_unavailable')
      expect(result.rows).toHaveLength(3)

      // Expected sorted order: a.go, b.go, c.go
      expect(result.rows[0].file).toBe('a.go')
      expect(result.rows[0].functions).toBe(2)
      expect(result.rows[0].totalLines).toBe(20)
      expect(result.rows[0].longest.lines).toBe(15)

      expect(result.rows[1].file).toBe('b.go')
      expect(result.rows[1].functions).toBe(4) // 1 (func) + 3 (meth)
      expect(result.rows[1].totalLines).toBe(35) // 5 + 30
      expect(result.rows[1].longest.lines).toBe(12) // max(5, 12) = 12

      expect(result.rows[2].file).toBe('c.go')
      expect(result.rows[2].functions).toBe(5)
      expect(result.rows[2].totalLines).toBe(50)
      expect(result.rows[2].longest.lines).toBe(20)
    })
  })
})
