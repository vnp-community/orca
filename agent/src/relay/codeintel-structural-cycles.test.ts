import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleCycles } from './codeintel-structural-cycles'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as ToolRunner from './codeintel-tool-runner'
import { PerfCollector } from './codeintel-result-envelope'

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

describe('codeintel-structural-cycles', () => {
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

  it('parses valid output and filters by pathPrefixes', async () => {
    const mockOutput = {
      status: 'cycles_found',
      cycleCount: 2,
      cycles: [
        { files: ['backend-go/services/app/a.go', 'backend-go/services/app/b.go'] },
        { files: ['other/foo.go', 'other/bar.go'] }
      ]
    }

    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify(mockOutput),
      stderr: '',
      exitCode: 0,
      durationMs: 100,
      stdoutBytes: 100
    })

    const params = { pathPrefixes: ['backend-go/services/'] }
    const result = await handleCycles(binding, params, ctx)

    expect(result.kind).toBe('cycles')
    expect(result.status).toBe('cycles_found')
    expect(result.cycleCount).toBe(2)
    // Only the first cycle should be kept because its first file matches prefix
    expect(result.cycles).toHaveLength(1)
    expect(result.cycles[0].files).toEqual(['backend-go/services/app/a.go', 'backend-go/services/app/b.go'])
    
    // Assert argv
    const args = vi.mocked(ToolRunner.runCodeIntelTool).mock.calls[0][0]
    expect(args).toEqual(['check', '--cycles', '--json', '-r', '/repo/.gitnexus'])
  })

  it('sorts cycles lexicographically', async () => {
    const mockOutput = {
      status: 'clean',
      cycleCount: 0,
      cycles: [
        { files: ['z.go', 'y.go'] },
        { files: ['a.go', 'b.go'] }
      ]
    }

    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify(mockOutput),
      stderr: '',
      exitCode: 0,
      durationMs: 100,
      stdoutBytes: 100
    })

    const result = await handleCycles(binding, {}, ctx)
    expect(result.cycles[0].files).toEqual(['a.go', 'b.go'])
    expect(result.cycles[1].files).toEqual(['z.go', 'y.go'])
  })

  it('throws truncated_stdout on malformed JSON', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: '{ "status": "cycles_found", "cycleCount":',
      stderr: '',
      exitCode: 0,
      durationMs: 100,
      stdoutBytes: 100
    })

    await expect(handleCycles(binding, {}, ctx))
      .rejects.toThrowError(/truncated_stdout/)
      .catch(e => expect(e.data.reason).toBe('truncated_stdout'))
  })

  it('throws unknown_shape on missing keys or invalid status', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify({ status: 'invalid_status', cycleCount: 1, cycles: [] }),
      stderr: '',
      exitCode: 0,
      durationMs: 100,
      stdoutBytes: 100
    })

    await expect(handleCycles(binding, {}, ctx))
      .rejects.toThrowError(/unknown_shape/)
  })
})
