import { describe, it, expect, vi } from 'vitest'
import { getAffectedTests } from './codegraph-affected-tests'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as ToolRunner from './codeintel-tool-runner'

describe('codegraph-affected-tests', () => {
  const ctx = { config: {}, log: { error: vi.fn() }, deadline: Date.now() + 1000 } as unknown as CodeIntelRequestContext

  it('filters out invalid files and returns empty if none valid', async () => {
    const res = await getAffectedTests({ toplevel: '/tmp/repo' }, ['-x', '--foo'], ctx)
    expect(res.changedFiles.length).toBe(0)
    expect(res.affectedTests.length).toBe(0)
  })

  it('chunks files, deduplicates tests, and respects limit', async () => {
    const spy = vi.spyOn(ToolRunner, 'runCodeIntelTool').mockResolvedValue({
      stdout: JSON.stringify({ changedFiles: ['a.ts'], affectedTests: [{ file: 'a.test.ts', name: 't1' }] }),
      stderr: '', exitCode: 0, durationMs: 1, stdoutBytes: 10
    })

    const files = Array.from({ length: 600 }, (_, i) => `f${i}.ts`)
    const res = await getAffectedTests({ toplevel: '/tmp/repo' }, files, ctx)
    
    expect(spy).toHaveBeenCalledTimes(2) // 600 chunks to 500 and 100
    expect(spy.mock.calls[0][4].stdinText?.split('\n').length).toBe(501) // 500 files + empty newline
    expect(spy.mock.calls[1][4].stdinText?.split('\n').length).toBe(101) // 100 files + empty newline
    
    expect(res.changedFiles).toEqual(['a.ts']) // Set removes duplicates
    expect(res.affectedTests.length).toBe(1)
  })
})
