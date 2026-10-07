import { describe, it, expect, vi, beforeEach } from 'vitest'
import { validateStructuralFacts, handleStructuralFacts } from './codeintel-structural-facts'
import { CodeIntelError } from './codeintel-errors'
import { PerfCollector } from './codeintel-result-envelope'
import { CodeIntelRequestContext } from './codeintel-method-table'
import * as RepoRes from './codeintel-repo-resolution'
import * as IndexProbe from './gitnexus-index-probe'
import * as ToolDetection from './codeintel-tool-detection'

vi.mock('./codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn()
}))

vi.mock('./gitnexus-index-probe', () => ({
  gitnexusIndexProbe: vi.fn()
}))

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn().mockResolvedValue({ stdout: '{ "status": "clean", "cycles": [], "cycleCount": 0 }', stderr: '', exitCode: 0, durationMs: 0, stdoutBytes: 0 })
}))

vi.mock('./codeintel-tool-detection', () => ({
  detectCodeIntelTools: vi.fn()
}))

vi.mock('./codeintel-head-commit', () => ({
  getHeadCommit: vi.fn().mockResolvedValue('fake-commit')
}))

describe('validateStructuralFacts', () => {
  it('accepts valid params', () => {
    const params = {
      workspaceRoot: '/some/path',
      kind: 'layerImports',
      pair: 'domain->usecase',
      pathPrefixes: ['backend-go/services/', 'another/'],
      limit: 100,
      offset: 10
    }
    const validated = validateStructuralFacts(params)
    expect(validated.workspaceRoot).toBe('/some/path')
    expect(validated.kind).toBe('layerImports')
    expect(validated.pair).toBe('domain->usecase')
    expect(validated.pathPrefixes).toEqual(['backend-go/services/', 'another/'])
    expect(validated.limit).toBe(100)
    expect(validated.offset).toBe(10)
  })

  it('rejects unknown parameters', () => {
    const params = {
      workspaceRoot: '/path',
      kind: 'cycles',
      command: 'echo',
      cypher: 'MATCH'
    }
    expect(() => validateStructuralFacts(params)).toThrowError(/Unknown parameter: command/)
  })

  it('specifically asserts no command|argv|args|cypher|repo|env|cwd|timeout', () => {
    const keys = ['command', 'argv', 'args', 'cypher', 'repo', 'env', 'cwd', 'timeout']
    for (const key of keys) {
      expect(() => validateStructuralFacts({ workspaceRoot: '/path', kind: 'cycles', [key]: 'val' })).toThrowError(CodeIntelError)
    }
  })

  it('rejects invalid pair', () => {
    expect(() => validateStructuralFacts({ workspaceRoot: '/p', kind: 'cycles', pair: 'domain->usecase' }))
      .toThrowError(/pair is only allowed when kind is layerImports/)
    
    expect(() => validateStructuralFacts({ workspaceRoot: '/p', kind: 'layerImports', pair: 'invalid-pair' }))
      .toThrowError(/Invalid pair value/)
  })

  it('rejects invalid pathPrefixes', () => {
    expect(() => validateStructuralFacts({ workspaceRoot: '/p', kind: 'cycles', pathPrefixes: ['..'] }))
      .toThrowError(/Invalid pathPrefix/)
    
    expect(() => validateStructuralFacts({ workspaceRoot: '/p', kind: 'cycles', pathPrefixes: ['-a/'] }))
      .toThrowError(/Invalid pathPrefix/)
    
    const tooMany = new Array(21).fill('a/')
    expect(() => validateStructuralFacts({ workspaceRoot: '/p', kind: 'cycles', pathPrefixes: tooMany }))
      .toThrowError(/must be an array of max length 20/)
  })

  it('bounds limit and offset', () => {
    const validated = validateStructuralFacts({ workspaceRoot: '/p', kind: 'cycles', limit: 6000, offset: -10 })
    expect(validated.limit).toBe(5000)
    expect(validated.offset).toBe(0)
  })
})

describe('handleStructuralFacts', () => {
  let ctx: CodeIntelRequestContext

  beforeEach(() => {
    vi.clearAllMocks()
    ctx = {
      config: { toolEnv: {} } as any,
      log: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() } as any,
      signal: new AbortController().signal,
      deadline: Date.now() + 10000,
      notifier: { notify: vi.fn() },
      perf: new PerfCollector()
    }

    vi.mocked(RepoRes.resolveCodeIntelRepo).mockResolvedValue({
      toplevel: '/path',
      gitNexusRegistryPath: null,
      stale: false,
      worktreeMismatch: false
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

  it('throws INDEX_MISSING if state is missing', async () => {
    vi.mocked(IndexProbe.gitnexusIndexProbe).mockResolvedValue({ state: 'missing' })
    await expect(handleStructuralFacts({ workspaceRoot: '/p', kind: 'cycles' }, ctx))
      .rejects.toThrowError(/GitNexus index is missing/)
      .catch(e => expect(e.code).toBe('CODEINTEL_INDEX_MISSING'))
  })

  it('throws REINDEX_IN_PROGRESS if state is building', async () => {
    vi.mocked(IndexProbe.gitnexusIndexProbe).mockResolvedValue({ state: 'building' })
    await expect(handleStructuralFacts({ workspaceRoot: '/p', kind: 'cycles' }, ctx))
      .rejects.toThrowError(/reindexing/)
      .catch(e => expect(e.code).toBe('CODEINTEL_REINDEX_IN_PROGRESS'))
  })

  it('throws TOOL_UNAVAILABLE if gitnexus is not available', async () => {
    vi.mocked(ToolDetection.detectCodeIntelTools).mockResolvedValue({
      gitnexus: { available: false, version: null },
      codegraph: { available: false, version: null }
    } as any)
    await expect(handleStructuralFacts({ workspaceRoot: '/p', kind: 'cycles' }, ctx))
      .rejects.toThrowError(/GitNexus is not available/)
      .catch(e => expect(e.code).toBe('CODEINTEL_TOOL_UNAVAILABLE'))
  })

  it('builds result envelope with data from handler', async () => {
    // The handler for cycles is not implemented yet, so it returns empty
    const res = await handleStructuralFacts({ workspaceRoot: '/p', kind: 'cycles', limit: 10, offset: 0 }, ctx)
    expect(res.data.kind).toBe('cycles')
    expect(res.data.cycleCount).toBe(0)
    expect(res.data.cycles).toEqual([])
    expect(res.sources[0].tool).toBe('gitnexus')
    expect(res.stale).toBe(false)
  })
})
