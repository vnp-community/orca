import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleOverview } from './codeintel-gitnexus-overview'
import { handleProcesses } from './codeintel-gitnexus-processes'
import { handleProcess } from './codeintel-gitnexus-process'
import { handleRoutes } from './codeintel-gitnexus-routes'
import { handleSubgraph } from './codeintel-gitnexus-subgraph'
import { handleImpact } from './codeintel-gitnexus-impact'
import { handleSymbol } from './codeintel-gitnexus-symbol'
import * as CypherRunner from './gitnexus-cypher-runner'
import * as ToolRunner from './codeintel-tool-runner'
import * as RepoRes from './codeintel-repo-resolution'
import * as HeadCommit from './codeintel-head-commit'
import * as ToolDetection from './codeintel-tool-detection'

vi.mock('./gitnexus-cypher-runner', () => ({
  runCypherTemplate: vi.fn()
}))

vi.mock('./codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

vi.mock('./codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn()
}))

vi.mock('./codeintel-head-commit', () => ({
  getHeadCommit: vi.fn()
}))

vi.mock('./codeintel-tool-detection', () => ({
  detectCodeIntelTools: vi.fn()
}))

describe('codeintel-gitnexus-methods', () => {
  const mockCtx: any = { config: {}, log: {}, perf: { build: vi.fn(() => ({ truncated: false })), recordCli: vi.fn() } }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(RepoRes.resolveCodeIntelRepo).mockResolvedValue({
      toplevel: '/repo',
      gitNexusRegistryPath: '/repo/.gitnexus',
      linkedWorktree: false,
      worktreeMismatch: false,
      stale: false
    })
    vi.mocked(HeadCommit.getHeadCommit).mockResolvedValue('commit1')
    vi.mocked(ToolDetection.detectCodeIntelTools).mockResolvedValue({
      gitnexus: { available: true, version: '1.6.9' },
      codegraph: { available: false, version: null }
    })
  })

  it('handleOverview', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'OV_CLUSTERS') return { rows: [{ 'c.id': 'c1', 'c.label': 'C1', 'c.symbolCount': 10, 'c.cohesion': 0.5, 'c.keywords': [] }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'OV_COUNT') return { rows: [{ n: 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'OV_EDGES') return { rows: [{ 'ca.id': 'c1', 'cb.id': 'c1', 'r.type': 'CALLS', w: 5 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'OV_TOPFILES') return { rows: [{ 'c.id': 'c1', 's.filePath': 'foo.ts', n: 10 }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })

    const res = await handleOverview({ workspaceRoot: '/repo', topN: 200, maxEdges: 5000, edgeKinds: ['CALLS'], withTopFiles: true }, mockCtx)
    expect(res.data.nodes[0].id).toBe('c1')
    expect(res.data.nodes[0].topFiles).toContain('foo.ts')
    expect(res.data.edges[0].weight).toBe(5)
  })

  it('handleProcesses', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'PR_LIST') return { rows: [{ 'p.id': 'p1', 'p.entryPointId': 'e1' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'PR_COUNT') return { rows: [{ n: 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'NODES_BY_ID') return { rows: [{ 'n.id': 'e1', 'label(n)': 'Function', 'n.name': 'entry' }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })

    const res = await handleProcesses({ workspaceRoot: '/repo' }, mockCtx)
    expect(res.data[0].id).toBe('p1')
    expect(res.data[0].entry.name).toBe('entry')
  })

  it('handleProcess', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'PR_ONE') return { rows: [{ 'p.id': 'p1', 'p.stepCount': 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'PR_STEPS') return { rows: [{ 's.id': 's1', 'label(s)': 'Function', 's.name': 'step1', 'r.step': 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'STEP_EDGES') return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
      if (id === 'MEMBER_CLUSTER') return { rows: [{ 's.id': 's1', 'c.id': 'c1' }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })

    const res = await handleProcess({ workspaceRoot: '/repo', processId: 'p1' }, mockCtx)
    expect(res.data.flow.id).toBe('p1')
    expect(res.data.steps[0].symbol.name).toBe('step1')
    expect(res.data.steps[0].cluster).toBe('c1')
  })

  it('handleRoutes', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'RT_LIST') return { rows: [{ 'r.id': 'r1', 'r.name': '/api', 'e.type': 'HANDLES_ROUTE', 'h.id': 'File:foo.ts:foo' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'RT_COUNTS') return { rows: [{ n: 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })

    const res = await handleRoutes({ workspaceRoot: '/repo' }, mockCtx)
    expect(res.data.routes[0].id).toBe('r1')
    expect(res.data.edges[0].side).toBe('server')
  })

  it('handleSubgraph', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'SG_FILE_SYMBOLS') return { rows: [{ 'n.id': 's1', 'label(n)': 'Function', 'n.name': 'sym' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'SG_EDGES_AROUND') return { rows: [{ 'a.id': 's1', 'b.id': 's2', 'e.type': 'CALLS' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'NODES_BY_ID') return { rows: [{ 'n.id': 's2', 'label(n)': 'Function', 'n.name': 'sym2' }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })

    const res = await handleSubgraph({ workspaceRoot: '/repo', center: { file: 'foo.ts' } }, mockCtx)
    expect(res.data.nodes[0].uid).toBe('s2') // because s1 might be filtered or present depending on logic, actually both s1 and s2
    expect(res.data.edges[0].kind).toBe('CALLS')
  })

  it('handleImpact', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify({
        target: { id: 's1', type: 'Function', name: 'sym' },
        direction: 'upstream',
        risk: 'HIGH',
        byDepth: [{ depth: 1, symbols: [{ id: 's2', name: 'sym2', relationType: 'CALLS' }] }]
      }),
      stderr: '', exitCode: 0, durationMs: 100, stdoutBytes: 200
    })

    const res = await handleImpact({ workspaceRoot: '/repo', target: { uid: 's1' } }, mockCtx)
    expect(res.data.target.uid).toBe('s1')
    expect(res.data.levels[0].symbols[0].symbol.uid).toBe('s2')
  })

  it('handleSymbol', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify({
        symbol: { uid: 's1', kind: 'Function', name: 'sym', filePath: 'foo.ts' },
        incoming: { CALLS: [{ uid: 's2', name: 'sym2', filePath: 'bar.ts' }] }
      }),
      stderr: '', exitCode: 0, durationMs: 100, stdoutBytes: 200
    })

    const res = await handleSymbol({ workspaceRoot: '/repo', uid: 's1', includeSource: false }, mockCtx)
    expect(res.data.symbol.uid).toBe('s1')
    expect(res.data.incoming.calls[0].uid).toBe('s2')
  })
})
