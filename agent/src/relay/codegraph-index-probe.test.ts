import { describe, it, expect, vi } from 'vitest'
import { codegraphIndexProbe } from './codegraph-index-probe'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as ToolRunner from './codeintel-tool-runner'

vi.mock('./codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn().mockImplementation((p) => Promise.resolve({ toplevel: p }))
}))

describe('codegraph-index-probe', () => {
  it('returns stale and rootMismatch for linked worktree', async () => {
    vi.spyOn(ToolRunner, 'runCodeIntelTool').mockResolvedValue({
      stdout: JSON.stringify({ worktreeMismatch: true, pendingChanges: 5 }),
      stderr: '', exitCode: 0, durationMs: 1, stdoutBytes: 10
    })
    
    const ctx = { config: {}, log: { error: vi.fn() }, deadline: Date.now() + 1000 } as unknown as CodeIntelRequestContext
    const res = await codegraphIndexProbe('/tmp/linked', ctx)
    
    expect(res.state).toBe('stale')
    expect(res.rootMismatch).not.toBeNull()
    expect(res.pendingChanges).toBeNull()
  })

  it('adds warning codegraph_has_no_commit if stale by pending but no commit', async () => {
    vi.spyOn(ToolRunner, 'runCodeIntelTool').mockResolvedValue({
      stdout: JSON.stringify({ pendingChanges: 5 }),
      stderr: '', exitCode: 0, durationMs: 1, stdoutBytes: 10
    })
    
    const ctx = { config: {}, log: { error: vi.fn() }, deadline: Date.now() + 1000 } as unknown as CodeIntelRequestContext
    const res = await codegraphIndexProbe('/tmp/repo', ctx)
    
    expect(res.state).toBe('stale')
    expect(res.warnings).toContain('codegraph_has_no_commit')
    expect(res.pendingChanges).toBe(5)
  })

  it('returns building state if state is building', async () => {
    vi.spyOn(ToolRunner, 'runCodeIntelTool').mockResolvedValue({
      stdout: JSON.stringify({ state: 'building', pendingChanges: 0 }),
      stderr: '', exitCode: 0, durationMs: 1, stdoutBytes: 10
    })
    
    const ctx = { config: {}, log: { error: vi.fn() }, deadline: Date.now() + 1000 } as unknown as CodeIntelRequestContext
    const res = await codegraphIndexProbe('/tmp/repo', ctx)
    
    expect(res.state).toBe('building')
    expect(res.pendingChanges).toBe(0)
  })
})
