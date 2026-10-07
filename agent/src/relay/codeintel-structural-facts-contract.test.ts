import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleStructuralFacts } from './codeintel-structural-facts'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as RepoRes from './codeintel-repo-resolution'
import * as IndexProbe from './gitnexus-index-probe'
import * as ToolDetection from './codeintel-tool-detection'
import * as ToolRunner from './codeintel-tool-runner'
import { PerfCollector } from './codeintel-result-envelope'
import { codeIntelCache } from './codeintel-short-lived-cache'

vi.mock('./codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn()
}))

vi.mock('./gitnexus-index-probe', () => ({
  gitnexusIndexProbe: vi.fn()
}))

vi.mock('./codeintel-tool-detection', () => ({
  detectCodeIntelTools: vi.fn()
}))

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

vi.mock('./codeintel-head-commit', () => ({
  getHeadCommit: vi.fn().mockResolvedValue('fake-commit')
}))

describe('codeintel-structural-facts contract tests', () => {
  let ctx: CodeIntelRequestContext

  beforeEach(() => {
    vi.clearAllMocks()
    codeIntelCache.invalidate()
    
    ctx = {
      config: { toolEnv: {} } as any,
      log: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() } as any,
      signal: new AbortController().signal,
      deadline: Date.now() + 10000,
      notifier: { notify: vi.fn() },
      perf: new PerfCollector()
    }

    vi.mocked(RepoRes.resolveCodeIntelRepo).mockResolvedValue({
      toplevel: '/repo',
      gitNexusRegistryPath: '/repo/.gitnexus',
      stale: false,
      worktreeMismatch: false,
      mainCheckoutRoot: '/repo'
    } as any)

    vi.mocked(IndexProbe.gitnexusIndexProbe).mockResolvedValue({
      state: 'ready',
      indexedCommit: 'fake-commit'
    })

    vi.mocked(ToolDetection.detectCodeIntelTools).mockResolvedValue({
      gitnexus: { available: true, version: '1.6.13' },
      codegraph: { available: false, version: null }
    } as any)
  })

  it('layerImports contract matches', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: `| a.filePath | b.filePath |
|---|---|
| src/a.ts | src/b.ts |`,
      stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100
    })

    const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
    expect(res.data.kind).toBe('layerImports')
    expect(res.data.rows).toBeInstanceOf(Array)
    expect(res.truncated).toBe(false)
    expect(res.totalCount).toBe(1)
    expect(res.sources[0].tool).toBe('gitnexus')
  })

  it('cycles contract matches', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify({ status: 'clean', cycles: [], cycleCount: 0 }),
      stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100
    })

    const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'cycles', offset: 0, limit: 10 }, ctx)
    expect(res.data.kind).toBe('cycles')
    expect(res.data.cycles).toBeInstanceOf(Array)
    expect(res.truncated).toBe(false)
    expect(res.totalCount).toBe(0)
  })

  it('importInDegree contract matches', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: `| b.filePath | count(DISTINCT a) |
|---|---|
| a.go | 10 |`,
      stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100
    })

    const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'importInDegree', offset: 0, limit: 10 }, ctx)
    expect(res.data.kind).toBe('importInDegree')
    expect(res.data.rows).toBeInstanceOf(Array)
    expect(res.data.rows[0].file).toBe('a.go')
    expect(res.data.rows[0].inDegree).toBe(10)
    expect(res.truncated).toBe(false)
    expect(res.totalCount).toBe(1)
  })

  it('fileSizes contract matches', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockImplementation(async (argv: any) => {
      let stdout = ''
      if (argv[1].includes('->(s:Function)')) {
        stdout = `| f.filePath | count(s) | sum(s.endLine - s.startLine + 1) | max(s.endLine - s.startLine + 1) |
|---|---|---|---|
| a.go | 2 | 20 | 15 |`
      } else {
        stdout = `| f.filePath | count(s) | sum(s.endLine - s.startLine + 1) | max(s.endLine - s.startLine + 1) |
|---|---|---|---|`
      }
      return { stdout, stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100 }
    })

    const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'fileSizes', offset: 0, limit: 10 }, ctx)
    expect(res.data.kind).toBe('fileSizes')
    expect(res.data.rows).toBeInstanceOf(Array)
    expect(res.data.rows[0].functions).toBe(2)
    expect(res.truncated).toBe(false)
    expect(res.totalCount).toBe(1)
  })

  it('unusedExports contract matches', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: `| f.id | f.name | label | f.filePath | f.startLine | f.endLine |
|---|---|---|---|---|---|
| 1 | foo | Function | a.go | 10 | 20 |`,
      stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100
    })

    const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'unusedExports', offset: 0, limit: 10 }, ctx)
    expect(res.data.kind).toBe('unusedExports')
    expect(res.data.rows).toBeInstanceOf(Array)
    expect(res.data.rows[0].symbol).toBeDefined()
    expect(res.data.rows[0].symbol.name).toBe('foo')
    expect(res.data.rows[0].symbol.filePath).toBe('a.go')
    expect(res.truncated).toBe(false)
    expect(res.totalCount).toBe(1)
  })

  describe('pagination and truncation', () => {
    it('paginates rows correctly and marks truncated=true when remaining', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
        stdout: `| a.filePath | b.filePath |
|---|---|
| src/1.ts | src/a.ts |
| src/2.ts | src/b.ts |
| src/3.ts | src/c.ts |
| src/4.ts | src/d.ts |
| src/5.ts | src/e.ts |`,
        stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100
      })

      const res = await handleStructuralFacts(
        { workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 1, limit: 2 },
        ctx
      )
      expect(res.totalCount).toBe(5)
      expect(res.truncated).toBe(true)
      expect(res.data.rows).toHaveLength(2)
      expect(res.data.rows[0].fromFile).toBe('src/2.ts')
      expect(res.data.rows[1].fromFile).toBe('src/3.ts')
    })

    it('paginates cycles correctly', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
        stdout: JSON.stringify({
          status: 'cycles_found',
          cycleCount: 3,
          cycles: [
            { files: ['backend-go/services/a.ts', 'b.ts', 'backend-go/services/a.ts'] },
            { files: ['backend-go/services/c.ts', 'd.ts', 'backend-go/services/c.ts'] },
            { files: ['backend-go/services/e.ts', 'f.ts', 'backend-go/services/e.ts'] }
          ]
        }),
        stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100
      })

      const res = await handleStructuralFacts(
        { workspaceRoot: '/repo', kind: 'cycles', offset: 1, limit: 1 },
        ctx
      )
      expect(res.totalCount).toBe(3)
      expect(res.truncated).toBe(true)
      expect(res.data.cycles).toHaveLength(1)
      expect(res.data.cycles[0].files).toEqual(['backend-go/services/c.ts', 'd.ts', 'backend-go/services/c.ts'])
    })
  })

  describe('short-lived caching', () => {
    it('caches successful result and does not invoke runner second time', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
        stdout: `| a.filePath | b.filePath |\n|---|---|\n| src/1.ts | src/2.ts |`,
        stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 50
      })

      const res1 = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
      expect(vi.mocked(ToolRunner.runCodeIntelTool)).toHaveBeenCalledTimes(1)

      const res2 = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
      expect(vi.mocked(ToolRunner.runCodeIntelTool)).toHaveBeenCalledTimes(1)
      expect(res2.data).toEqual(res1.data)
    })

    it('invalidates cache when codeIntelCache.invalidate is called', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
        stdout: `| a.filePath | b.filePath |\n|---|---|\n| src/1.ts | src/2.ts |`,
        stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 50
      })

      await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
      expect(vi.mocked(ToolRunner.runCodeIntelTool)).toHaveBeenCalledTimes(1)

      codeIntelCache.invalidate()

      await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
      expect(vi.mocked(ToolRunner.runCodeIntelTool)).toHaveBeenCalledTimes(2)
    })

    it('does not cache errors', async () => {
      vi.mocked(ToolRunner.runCodeIntelTool).mockRejectedValueOnce(new Error('Cypher execution failed'))

      await expect(
        handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
      ).rejects.toThrow('Cypher execution failed')

      vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
        stdout: `| a.filePath | b.filePath |\n|---|---|\n| src/1.ts | src/2.ts |`,
        stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 50
      })

      const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
      expect(res.data.rows).toHaveLength(1)
      expect(vi.mocked(ToolRunner.runCodeIntelTool)).toHaveBeenCalledTimes(2)
    })
  })

  describe('size limits (> 8 MiB)', () => {
    it('truncates rows if payload exceeds 8 MiB', async () => {
      const hugeRow = 'x'.repeat(2 * 1024 * 1024)
      vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
        stdout: `| a.filePath | b.filePath |
|---|---|
| ${hugeRow}1 | b.ts |
| ${hugeRow}2 | b.ts |
| ${hugeRow}3 | b.ts |
| ${hugeRow}4 | b.ts |
| ${hugeRow}5 | b.ts |`,
        stderr: '', exitCode: 0, durationMs: 10, stdoutBytes: 100
      })

      const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'layerImports', pair: 'usecase->adapter', offset: 0, limit: 10 }, ctx)
      expect(res.truncated).toBe(true)
      const size = Buffer.byteLength(JSON.stringify(res.data), 'utf8')
      expect(size).toBeLessThanOrEqual(8 * 1024 * 1024)
    })
  })
})

