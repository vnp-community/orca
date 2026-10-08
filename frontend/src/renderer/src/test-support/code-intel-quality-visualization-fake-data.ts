// Scenarios for the quality trend / coverage / hotspot / dependency blocks (FE-CV-TASK-087-20).
// Kept apart from code-intel-fake-backend.ts: the backend already exposes setHandler, so a
// scenario only needs a `{ setHandler }` target and never edits the shared fake.
import { CODE_INTEL_ERROR_CODES } from '../../../shared/code-intel-error-codes'
import { CODE_INTEL_RPC_METHODS as M } from '../../../shared/code-intel-rpc-methods'
import type { Finding } from '../../../shared/code-intel-findings-types'
import type { ModuleGraph } from '../../../shared/code-intel-graph-types'
import type {
  CoverageReport,
  QualityTrendPoint
} from '../../../shared/code-intel-quality-visualization-types'
import type { FakeChannelHandler } from './code-intel-fake-backend'

export type VisualizationState = 'loading' | 'empty' | 'error' | 'stale' | 'ready'
export type VisualizationFailure = 'timeout-in-progress' | 'output-too-large' | 'generic'

export type VisualizationScenario = {
  trend?: { state: VisualizationState; points?: number; failure?: VisualizationFailure }
  coverage?: {
    state: VisualizationState
    kind?: 'measured' | 'estimated' | 'partial'
    failure?: VisualizationFailure
  }
  hotspot?: { state: VisualizationState; failure?: VisualizationFailure }
  structure?: {
    state: VisualizationState
    kind?: 'dag' | 'cycle' | 'large'
    failure?: VisualizationFailure
  }
}

const err = (code: string, message: string, data?: unknown): Error =>
  new Error(`${code}: ${message}${data === undefined ? '' : ` | ${JSON.stringify(data)}`}`)

export function failureFor(kind: VisualizationFailure = 'generic'): Error {
  if (kind === 'timeout-in-progress') {
    return err(CODE_INTEL_ERROR_CODES.TIMEOUT, 'analysis in progress', {
      inProgress: true,
      retryAfterMs: 10
    })
  }
  if (kind === 'output-too-large') {
    return err(CODE_INTEL_ERROR_CODES.OUTPUT_TOO_LARGE, 'output too large')
  }
  return err(CODE_INTEL_ERROR_CODES.TOOL_FAILED, 'tool failed')
}

/** Counts are ratios of D5: diffCoverage is 0..1 and absent on the points that have none. */
export function buildTrendPoint(
  index: number,
  overrides: Partial<QualityTrendPoint> = {}
): QualityTrendPoint {
  const at = new Date(Date.UTC(2026, 9, 1, 8, 0, 0) + index * 3_600_000).toISOString()
  return {
    turnKey: `pane_a:${Date.parse(at)}`,
    headCommit: `a${String(index).padStart(6, '0')}c9e0`,
    baseCommit: 'b000000base',
    profileRef: 'full@repo/v1',
    verdict: index % 7 === 6 ? 'fail' : index % 3 === 2 ? 'warn' : 'pass',
    counts: { error: 12 - (index % 10), warning: 8 + (index % 3), info: 3, byCategory: {} },
    metrics: index % 4 === 1 ? {} : { diffCoverage: Math.min(1, 0.4 + index * 0.01) },
    runIds: [`run-${index}`],
    indexCommit: 'idx',
    source: index % 5 === 4 ? 'ci' : 'local',
    createdAt: at,
    ...overrides
  }
}

export function buildTrendResponse(
  count: number,
  total = count
): { points: QualityTrendPoint[]; truncated: boolean; totalCount: number } {
  return {
    points: Array.from({ length: count }, (_, i) => buildTrendPoint(i)),
    truncated: total > count,
    totalCount: total
  }
}

export function buildCoverageReport(
  kind: 'measured' | 'estimated' | 'partial' = 'measured'
): CoverageReport {
  const files = Array.from({ length: 12 }, (_, i) => ({
    path: `src/area-${i % 3}/file-${i}.ts`,
    stmts: 20 + i * 5,
    covered: 10 + i * 3,
    pct: (10 + i * 3) / (20 + i * 5),
    ...(i % 2 === 0
      ? { uncoveredRanges: [[3 + i, 5 + i] as [number, number], [40, 40] as [number, number]] }
      : {})
  }))
  return {
    source: kind === 'estimated' ? 'estimated' : 'measured',
    language: 'ts',
    headCommit: 'a41c9e0',
    baseCommit: 'b000000',
    dirty: false,
    totals: { stmts: 360, covered: 200, pct: 0.5556 },
    diff: {
      changedExecutable: 40,
      covered: 30,
      uncovered: 10,
      diffCoverage: 0.75,
      partial: kind === 'partial',
      excludedFiles: kind === 'partial' ? [{ path: 'gen/schema.ts', reason: 'generated file' }] : []
    },
    files,
    truncated: false,
    totalCount: files.length,
    toolVersions: { vitest: '3.0.0' },
    ...(kind === 'estimated' ? { estimatedNote: 'Based on test-to-source edges.' } : {})
  }
}

/** Metric keys differ per file on purpose: columns must come from the data. */
export function buildHotspotFindings(count = 6): Finding[] {
  return Array.from({ length: count }, (_, i) => ({
    findingKey: `hotspot:file-${i}`,
    rule: 'hotspot.file',
    kind: 'hotspot',
    severity: 'info',
    titleKey: 'finding.hotspot.file',
    params: {},
    subject: `src/area/file-${i}.ts`,
    evidence: [{ path: `src/area/file-${i}.ts` }],
    metrics: { churn: 30 - i, authors: 1 + (i % 4), ...(i % 2 === 0 ? { recentFixes: i } : {}) },
    ...(i === 0 ? { owner: { source: 'codeowners' as const, names: ['@platform'] } } : {}),
    scope: {},
    origin: 'preexisting',
    confidence: 'medium'
  }))
}

export function buildStructureGraph(kind: 'dag' | 'cycle' | 'large' = 'dag'): ModuleGraph {
  const size = kind === 'large' ? 90 : 8
  const nodes = Array.from({ length: size }, (_, i) => ({
    id: `src/mod-${i}`,
    kind: 'folder' as const,
    symbolCount: 10 + i
  }))
  const edges: ModuleGraph['edges'] = []
  for (let i = 0; i + 1 < size; i++) {
    edges.push({ from: nodes[i].id, to: nodes[i + 1].id, kind: 'imports', count: 1 + (i % 4) })
    edges.push({ from: nodes[i].id, to: nodes[(i + 3) % size].id, kind: 'contains', count: 1 })
  }
  if (kind === 'cycle') {
    edges.push({ from: nodes[3].id, to: nodes[1].id, kind: 'imports', count: 2 })
  }
  return { nodes, edges }
}

function stateHandler(
  state: VisualizationState,
  ready: () => unknown,
  empty: unknown,
  failure?: VisualizationFailure
): FakeChannelHandler {
  let calls = 0
  return () => {
    calls++
    if (state === 'loading') {
      return new Promise(() => undefined)
    }
    if (state === 'error' || (state === 'stale' && calls > 1)) {
      throw failureFor(failure)
    }
    return state === 'empty' ? empty : ready()
  }
}

function structureEnvelope(
  params: Record<string, unknown>,
  graph: ModuleGraph,
  truncated: boolean,
  totalCount: number
): unknown {
  return {
    worktreeId: String(params.worktreeId ?? ''),
    view: 'structure',
    sources: [],
    headCommit: null,
    stale: false,
    truncated,
    totalCount,
    etag: 'etag-structure',
    fromCache: false,
    generatedAt: new Date(0).toISOString(),
    data: graph
  }
}

/** Registers handlers for the four blocks; `stale` answers once, then fails (a failed refresh). */
export function registerQualityVisualizationScenario(
  backend: { setHandler: (method: string, handler: FakeChannelHandler) => void },
  scenario: VisualizationScenario
): void {
  const { trend, coverage, hotspot, structure } = scenario
  if (trend) {
    const count = trend.points ?? 12
    backend.setHandler(
      M.QUALITY_TREND,
      stateHandler(
        trend.state,
        () => buildTrendResponse(count),
        { points: [], truncated: false, totalCount: 0 },
        trend.failure
      )
    )
  }
  if (coverage) {
    backend.setHandler(
      M.QUALITY_COVERAGE,
      stateHandler(
        coverage.state,
        () => ({ report: buildCoverageReport(coverage.kind) }),
        { report: null, reason: 'no coverage artifact was found' },
        coverage.failure
      )
    )
  }
  if (hotspot) {
    const rows = stateHandler(
      hotspot.state,
      () => ({ findings: buildHotspotFindings(), dismissedCount: 0 }),
      { findings: [], dismissedCount: 0 },
      hotspot.failure
    )
    backend.setHandler(M.FINDINGS, (params) => {
      const rules = params.rules
      // Why: other findings consumers keep working; only the hotspot request is scripted.
      return Array.isArray(rules) && rules.includes('hotspot.file')
        ? rows(params)
        : { findings: [], dismissedCount: 0 }
    })
  }
  if (structure) {
    const graph = buildStructureGraph(structure.kind)
    const run = stateHandler(
      structure.state,
      () => graph,
      { nodes: [], edges: [] },
      structure.failure
    )
    backend.setHandler(M.STRUCTURE, async (params) => {
      const out = (await run(params)) as ModuleGraph
      return structureEnvelope(
        params,
        out,
        structure.kind === 'large',
        structure.kind === 'large' ? 400 : out.nodes.length
      )
    })
  }
}
