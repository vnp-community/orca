import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest'
import fs from 'fs'
import path from 'path'
import crypto from 'crypto'
import { handleStatus } from '../codeintel-status'
import { handleOverview } from '../codeintel-gitnexus-overview'
import { handleProcesses } from '../codeintel-gitnexus-processes'
import { handleProcess } from '../codeintel-gitnexus-process'
import { handleRoutes } from '../codeintel-gitnexus-routes'
import { handleSubgraph } from '../codeintel-gitnexus-subgraph'
import { handleImpact } from '../codeintel-gitnexus-impact'
import { handleSymbol } from '../codeintel-gitnexus-symbol'
import { handleDetectChanges } from '../codeintel-detect-changes'
import { handleStructuralFacts } from '../codeintel-structural-facts'
import { handleCodegraphSearch } from '../codeintel-codegraph-search'
import { handleFiles } from '../codeintel-codegraph-files'
import * as CypherRunner from '../gitnexus-cypher-runner'
import * as ToolRunner from '../codeintel-tool-runner'
import * as RepoRes from '../codeintel-repo-resolution'
import * as HeadCommit from '../codeintel-head-commit'
import * as ToolDetection from '../codeintel-tool-detection'
import * as IndexBasisProbe from '../codeintel-index-basis-probe'
import * as MergeBase from '../codeintel-merge-base-resolution'
import * as GitnexusProbe from '../gitnexus-index-probe'
import { PerfCollector, buildCodeIntelResult } from '../codeintel-result-envelope'
import { scanTextForLeaks } from './fixture-manifest'

vi.mock('../gitnexus-cypher-runner', () => ({
  runCypherTemplate: vi.fn()
}))

vi.mock('../codeintel-tool-runner', () => ({
  runCodeIntelTool: vi.fn()
}))

vi.mock('../codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn()
}))

vi.mock('../codeintel-head-commit', () => ({
  getHeadCommit: vi.fn()
}))

vi.mock('../codeintel-tool-detection', () => ({
  detectCodeIntelTools: vi.fn()
}))

vi.mock('../codeintel-index-basis-probe', () => ({
  probeIndexBasis: vi.fn()
}))

vi.mock('../codeintel-merge-base-resolution', () => ({
  resolveCompareRange: vi.fn()
}))

vi.mock('../gitnexus-index-probe', () => ({
  gitnexusIndexProbe: vi.fn()
}))

function getGoldenDir() {
  const rootDir = path.resolve(__dirname, '../../../../')
  const goldenDir = path.join(rootDir, 'backend-go/services/code-intel-service/testdata/agent-results')
  if (process.env.ORCA_UPDATE_GOLDEN === '1') {
    fs.mkdirSync(goldenDir, { recursive: true })
  }
  return goldenDir
}

function sortObjectKeys(obj: any): any {
  if (obj === null || typeof obj !== 'object') return obj
  if (Array.isArray(obj)) return obj.map(sortObjectKeys)
  const sorted: Record<string, any> = {}
  for (const key of Object.keys(obj).sort()) {
    sorted[key] = sortObjectKeys(obj[key])
  }
  return sorted
}

function writeOrCompare(filename: string, res: any) {
  // Sanitize perf and dates
  const clone = JSON.parse(JSON.stringify(res))
  if (clone.perf) {
    clone.perf = { totalMs: 100, queueWaitMs: 0, cliCalls: 1, cli: [], parseMs: 10, truncated: false }
  }
  if (clone.sources) {
    clone.sources.forEach((s: any) => {
      s.indexedAt = 1600000000000
    })
  }
  delete clone.perf
  delete clone.startedAt
  delete clone['codeintel.node']

  const sorted = sortObjectKeys(clone)
  const content = JSON.stringify(sorted, null, 2) + '\n'
  const filepath = path.join(getGoldenDir(), filename)

  if (process.env.ORCA_UPDATE_GOLDEN === '1') {
    fs.mkdirSync(path.dirname(filepath), { recursive: true })
    fs.writeFileSync(filepath, content)
  } else {
    if (!fs.existsSync(filepath)) {
      throw new Error(`Golden file ${filename} not found. Run with ORCA_UPDATE_GOLDEN=1.`)
    }
    const expected = fs.readFileSync(filepath, 'utf-8')
    expect(content).toEqual(expected)
  }
}

describe('agent-result-golden', () => {
  const mockCtx: any = { config: { toolEnv: { HOME: '/tmp' } }, log: { info: vi.fn(), warn: vi.fn(), error: vi.fn() }, perf: new PerfCollector() }
  const filesWritten: string[] = []

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(RepoRes.resolveCodeIntelRepo).mockResolvedValue({
      toplevel: '/repo',
      gitNexusRegistryPath: '/repo/.gitnexus',
      linkedWorktree: false,
      worktreeMismatch: false,
      stale: false,
      indexRoot: '/repo'
    })
    vi.mocked(HeadCommit.getHeadCommit).mockResolvedValue('abcdef')
    vi.mocked(ToolDetection.detectCodeIntelTools).mockResolvedValue({
      gitnexus: { available: true, version: '1.6.9', supported: true, binary: '/bin/gitnexus' },
      codegraph: { available: true, version: '1.4.1', supported: true, binary: '/bin/codegraph' }
    })
    vi.mocked(IndexBasisProbe.probeIndexBasis).mockResolvedValue({
      headCommit: 'abcdef',
      headCommitTimeMs: 1600000000000,
      mergeBase: 'abcdef',
      mergeBaseCommitTimeMs: 1600000000000,
      changedFilesNotInIndex: 0,
      dirtySinceIndex: false
    })
    vi.mocked(MergeBase.resolveCompareRange).mockResolvedValue({
      foundOid: 'mergebase',
      commitTimeMs: 1600000000000
    })
    vi.mocked(GitnexusProbe.gitnexusIndexProbe).mockResolvedValue({
      state: 'ready',
      version: '1.6.9',
      indexedAt: 1600000000000,
      commit: 'abcdef',
      lineBase: 1,
      indexRoot: '/repo'
    })
  })

  it('generates overview golden', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'OV_CLUSTERS') return { rows: [{ 'c.id': 'c1', 'c.label': 'C1', 'c.symbolCount': 10, 'c.cohesion': 0.5, 'c.keywords': [] }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'OV_COUNT') return { rows: [{ n: 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'OV_EDGES') return { rows: [{ 'ca.id': 'c1', 'cb.id': 'c2', 'r.type': 'CALLS', w: 5 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'OV_TOPFILES') return { rows: [{ 'c.id': 'c1', 's.filePath': 'foo.ts', n: 10 }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })
    const res = await handleOverview({ workspaceRoot: '/repo', topN: 200, maxEdges: 5000, edgeKinds: ['CALLS'], withTopFiles: true }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/overview.json', res)
    filesWritten.push('gitnexus-1.6.9/overview.json')
  })

  it('generates processes golden', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'PR_LIST') return { rows: [{ 'p.id': 'p1', 'p.entryPointId': 'e1' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'PR_COUNT') return { rows: [{ n: 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'NODES_BY_ID') return { rows: [{ 'n.id': 'e1', 'label(n)': 'Function', 'n.name': 'entry' }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })
    const res = await handleProcesses({ workspaceRoot: '/repo' }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/processes.json', res)
    filesWritten.push('gitnexus-1.6.9/processes.json')
  })

  it('generates process golden', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'PR_ONE') return { rows: [{ 'p.id': 'p1', 'p.stepCount': 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'PR_STEPS') return { rows: [{ 's.id': 's1', 'label(s)': 'Function', 's.name': 'step1', 'r.step': 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'STEP_EDGES') return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
      if (id === 'MEMBER_CLUSTER') return { rows: [{ 's.id': 's1', 'c.id': 'c1' }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })
    const res = await handleProcess({ workspaceRoot: '/repo', processId: 'p1' }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/process.json', res)
    filesWritten.push('gitnexus-1.6.9/process.json')
  })

  it('generates routes golden', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'RT_LIST') return { rows: [{ 'r.id': 'r1', 'r.name': '/api', 'e.type': 'HANDLES_ROUTE', 'h.id': 'File:foo.ts:foo' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'RT_COUNTS') return { rows: [{ n: 1 }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })
    const res = await handleRoutes({ workspaceRoot: '/repo' }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/routes.json', res)
    filesWritten.push('gitnexus-1.6.9/routes.json')
  })

  it('generates subgraph golden', async () => {
    vi.mocked(CypherRunner.runCypherTemplate).mockImplementation(async (binding, id, slots) => {
      if (id === 'SG_FILE_SYMBOLS') return { rows: [{ 'n.id': 's1', 'label(n)': 'Function', 'n.name': 'sym' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'SG_EDGES_AROUND') return { rows: [{ 'a.id': 's1', 'b.id': 's2', 'e.type': 'CALLS' }], rowCount: 1, skippedRows: 0, warnings: [] }
      if (id === 'NODES_BY_ID') return { rows: [{ 'n.id': 's2', 'label(n)': 'Function', 'n.name': 'sym2' }], rowCount: 1, skippedRows: 0, warnings: [] }
      return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
    })
    const res = await handleSubgraph({ workspaceRoot: '/repo', center: { file: 'foo.ts' } }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/subgraph.json', res)
    filesWritten.push('gitnexus-1.6.9/subgraph.json')
  })

  it('generates impact golden', async () => {
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
    writeOrCompare('gitnexus-1.6.9/impact-found.json', res)
    filesWritten.push('gitnexus-1.6.9/impact-found.json')
  })

  it('generates symbol golden', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify({
        symbol: { uid: 's1', kind: 'Function', name: 'sym', filePath: 'foo.ts' },
        incoming: { CALLS: [{ uid: 's2', name: 'sym2', filePath: 'bar.ts' }] }
      }),
      stderr: '', exitCode: 0, durationMs: 100, stdoutBytes: 200
    })
    const res = await handleSymbol({ workspaceRoot: '/repo', uid: 's1', includeSource: false }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/symbol-found.json', res)
    filesWritten.push('gitnexus-1.6.9/symbol-found.json')
  })

  it('generates detectChanges golden', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify([{ path: 'foo.ts', status: 'modified' }]),
      stderr: '', exitCode: 0, durationMs: 100, stdoutBytes: 200
    })
    const res = await handleDetectChanges({ workspaceRoot: '/repo', baseRef: 'main' }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/detectChanges.json', res)
    filesWritten.push('gitnexus-1.6.9/detectChanges.json')
  })

  it('generates structuralFacts golden', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify({
        status: 'cycles_found',
        cycleCount: 1,
        cycles: [{ files: ['a.ts', 'b.ts', 'a.ts'] }]
      }),
      stderr: '', exitCode: 0, durationMs: 100, stdoutBytes: 200
    })
    const res = await handleStructuralFacts({ workspaceRoot: '/repo', kind: 'cycles' }, mockCtx)
    writeOrCompare('gitnexus-1.6.9/structuralFacts.json', res)
    filesWritten.push('gitnexus-1.6.9/structuralFacts.json')
  })

  it('generates codegraphSearch golden', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify([{ file: 'foo.ts', line: 1, content: 'func foo()' }]),
      stderr: '', exitCode: 0, durationMs: 100, stdoutBytes: 200
    })
    const res = await handleCodegraphSearch({ workspaceRoot: '/repo', query: 'foo' }, mockCtx)
    writeOrCompare('codegraph-1.4.1/codegraphSearch.json', res)
    filesWritten.push('codegraph-1.4.1/codegraphSearch.json')
  })

  it('generates files golden', async () => {
    vi.mocked(ToolRunner.runCodeIntelTool).mockResolvedValue({
      stdout: JSON.stringify(['foo.ts', 'bar.ts']),
      stderr: '', exitCode: 0, durationMs: 100, stdoutBytes: 200
    })
    const res = await handleFiles({ workspaceRoot: '/repo' }, mockCtx)
    writeOrCompare('codegraph-1.4.1/files.json', res)
    filesWritten.push('codegraph-1.4.1/files.json')
  })

  it('no golden file contains perf or startedAt or codeintel.node and respects 20 KiB', () => {
    for (const relPath of filesWritten) {
      const fullPath = path.join(getGoldenDir(), relPath)
      if (!fs.existsSync(fullPath)) continue
      const raw = fs.readFileSync(fullPath, 'utf8')
      expect(raw).not.toContain('"perf"')
      expect(raw).not.toContain('"startedAt"')
      expect(raw).not.toContain('"codeintel.node"')
      expect(Buffer.byteLength(raw)).toBeLessThanOrEqual(20 * 1024)
    }
  })

  it('golden files pass leak scanning', () => {
    for (const relPath of filesWritten) {
      const fullPath = path.join(getGoldenDir(), relPath)
      if (!fs.existsSync(fullPath)) continue
      const raw = fs.readFileSync(fullPath, 'utf8')
      const findings = scanTextForLeaks(raw, relPath)
      expect(findings).toEqual([])
    }
  })

  it('writes MANIFEST.json', () => {
    if (process.env.ORCA_UPDATE_GOLDEN === '1') {
      const manifest: any = {
        tool: 'agent-results',
        version: 'v7',
        commit: 'mini-repo-c2',
        files: {}
      }
      for (const relPath of filesWritten) {
        const fullPath = path.join(getGoldenDir(), relPath)
        const bytes = fs.statSync(fullPath).size
        const sha256 = crypto.createHash('sha256').update(fs.readFileSync(fullPath)).digest('hex')
        manifest.files[relPath] = { bytes, sha256 }
      }
      fs.writeFileSync(path.join(getGoldenDir(), 'MANIFEST.json'), JSON.stringify(manifest, null, 2) + '\n')
    }
  })
})
