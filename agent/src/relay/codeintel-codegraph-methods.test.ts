import { describe, it, expect, vi } from 'vitest'
import { handleCodegraphSearch } from './codeintel-codegraph-search'
import { handleFiles } from './codeintel-codegraph-files'
import { CODEINTEL_METHODS, CodeIntelRequestContext } from './codeintel-method-table'
import * as ToolRunner from './codeintel-tool-runner'
import { CodeIntelError } from './codeintel-errors'

vi.mock('./codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn().mockImplementation((p) => Promise.resolve({ toplevel: p }))
}))

describe('codegraph-methods', () => {
  const ctx = { config: {}, log: { error: vi.fn() }, deadline: Date.now() + 1000 } as unknown as CodeIntelRequestContext

  it('handleCodegraphSearch processes output', async () => {
    vi.spyOn(ToolRunner, 'runCodeIntelTool').mockResolvedValue({
      stdout: JSON.stringify([{ id: 1, type: 'function', name: 'foo', path: 'a.ts' }]),
      stderr: '', exitCode: 0, durationMs: 1, stdoutBytes: 10
    })

    const res = await handleCodegraphSearch({ workspaceRoot: '/tmp/repo', search: 'foo' }, ctx)
    expect(res.results.length).toBe(1)
    expect(res.results[0].kind).toBe('function')
    expect(res.sources[0].tool).toBe('codegraph')
  })

  it('handleFiles processes output', async () => {
    vi.spyOn(ToolRunner, 'runCodeIntelTool').mockResolvedValue({
      stdout: JSON.stringify([{ path: 'a.ts', language: 'typescript', nodeCount: 1, size: 10 }]),
      stderr: '', exitCode: 0, durationMs: 1, stdoutBytes: 10
    })

    const res = await handleFiles({ workspaceRoot: '/tmp/repo', filter: 'a', limit: 10 }, ctx)
    expect(res.files.length).toBe(1)
    expect(res.truncated).toBe(false)
  })

  it('validates search params', () => {
    const validate = CODEINTEL_METHODS['codeintel.codegraphSearch'].validate
    expect(() => validate({ workspaceRoot: '/tmp', search: '' })).toThrowError(CodeIntelError)
    
    const p = validate({ workspaceRoot: '/tmp', search: 'foo', limit: 1000 })
    expect(p.limit).toBe(50) // clamped
  })

  it('validates files params', () => {
    const validate = CODEINTEL_METHODS['codeintel.files'].validate
    expect(() => validate({ workspaceRoot: '/tmp', filter: '-x' })).toThrowError(CodeIntelError)
    expect(() => validate({ workspaceRoot: '/tmp', filter: '../a' })).toThrowError(CodeIntelError)
    
    const p = validate({ workspaceRoot: '/tmp', filter: 'a', limit: 6000 })
    expect(p.limit).toBe(5000) // clamped
  })
})
