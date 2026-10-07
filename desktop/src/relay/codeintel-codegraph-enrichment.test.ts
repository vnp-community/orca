import { describe, it, expect, vi } from 'vitest'
import { enrichSymbol, enrichImpact, enrichSubgraph } from './codeintel-codegraph-enrichment'
import * as ToolRunner from './codeintel-tool-runner'
import { CodeIntelRequestContext } from './codeintel-method-table'

vi.mock('./codegraph-sqlite-reader', () => ({
  openCodeGraphDb: vi.fn().mockReturnValue(null),
  closeCodeGraphDb: vi.fn(),
  findNodes: vi.fn(),
  callersById: vi.fn(),
  calleesById: vi.fn()
}))

vi.mock('./codegraph-affected-tests', () => ({
  getAffectedTests: vi.fn().mockResolvedValue({ changedFiles: [], affectedTests: [{ file: 'a.test.ts', name: 't1' }], truncated: false })
}))

describe('codeintel-codegraph-enrichment', () => {
  const ctx = { config: {}, log: { error: vi.fn() }, deadline: Date.now() + 1000 } as unknown as CodeIntelRequestContext

  it('enrichImpact adds testsCovering when includeTests is true', async () => {
    const impactData = { nodes: [{ filePath: 'a.ts' }] }
    const res = await enrichImpact(impactData, { toplevel: '/tmp/repo' }, ctx, [], true)
    expect(res.testsCovering.length).toBe(1)
  })

  it('enrichSymbol uses cli fallback when sqlite is off', async () => {
    vi.spyOn(ToolRunner, 'runCodeIntelTool').mockResolvedValue({
      stdout: JSON.stringify([{ id: 123, type: 'function', name: 'foo', path: 'a.ts', signature: '()' }]),
      stderr: '', exitCode: 0, durationMs: 1, stdoutBytes: 10
    })

    const symbol = { uid: '1', kind: 'function', name: 'foo', qualifiedName: 'foo', filePath: 'a.ts', key: 'k', startLine: 1, endLine: 2 }
    const warnings: string[] = []
    const res = await enrichSymbol(symbol, { toplevel: '/tmp/repo' }, ctx, warnings)
    
    expect((res as any).signature).toBe('()')
    expect(warnings).toContain('codegraph_name_based_resolution')
  })
})
