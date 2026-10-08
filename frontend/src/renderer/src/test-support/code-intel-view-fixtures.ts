/**
 * code-intel-view-fixtures.ts — FE-CV-TASK-050-08
 *
 * Overlay, reading order, impact, symbol, module graph, C4 and data-flow fixtures
 * (re-exported from code-intel-fixtures.ts).
 */

import type {
  C4ComponentView,
  ChangeOverlay,
  ComponentGroup,
  ContainerRef,
  DataFlow,
  ImpactGraph,
  ModuleGraph,
  ReadingStep,
  SequenceModel,
  SymbolDetail
} from '../../../shared/code-intel-types'
import { SYMBOL_REF_SERVICE, SYMBOL_REF_TEST } from './code-intel-symbol-fixtures'

// ---------------------------------------------------------------------------
// §4.3 ReadingStep / ComponentGroup / ChangeOverlay
// ---------------------------------------------------------------------------

export const READING_STEPS: ReadingStep[] = [
  {
    stepKey: 'step-1',
    n: 1,
    file: 'services/order/create.go',
    symbols: [SYMBOL_REF_SERVICE],
    hunks: [{ startLine: 10, endLine: 42 }],
    reason: 'dependency-of',
    dependsOn: [],
    tests: [SYMBOL_REF_TEST],
    layer: 'usecase'
  },
  {
    stepKey: 'step-2',
    n: 2,
    file: 'services/order/create_test.go',
    symbols: [SYMBOL_REF_TEST],
    hunks: [{ startLine: 5, endLine: 30 }],
    reason: 'test',
    dependsOn: ['step-1'],
    tests: [],
    layer: 'test'
  }
]

export const COMPONENT_GROUPS: ComponentGroup[] = [
  {
    componentId: 'order.usecase',
    containerId: 'order-service',
    label: 'Order usecase',
    files: 2,
    symbols: 2,
    added: 40,
    removed: 5,
    riskPoints: 3,
    stepKeys: ['step-1', 'step-2']
  }
]

function overlay(overrides: Partial<ChangeOverlay> = {}): ChangeOverlay {
  return {
    scope: { baseRef: 'main', mode: 'worktree', includesUncommitted: true },
    changedFiles: [],
    changedSymbols: [],
    affectedFlows: [],
    affectedClusters: [],
    touchedTables: [],
    touchedContracts: [],
    uncoveredSymbols: [],
    violations: [],
    readingOrder: [],
    components: [],
    risk: { level: 'LOW', score: 0, incomplete: false, confidence: 'high', reasons: [], modelVersion: '1' },
    indexFreshness: { state: 'fresh', dirtyFiles: 0, unindexedFiles: [], generatedAt: '2026-10-07T00:00:00Z' },
    limits: {
      truncated: { files: false, symbols: false, flows: false, steps: false, impact: false },
      totalCounts: {}
    },
    ...overrides
  }
}

const CHANGED_FILE_BASE = {
  area: 'order',
  isTest: false,
  isGenerated: false,
  isDoc: false,
  mappingConfidence: 'exact'
} as const

/** Small overlay: 3 files, 1 renamed, reading order + components */
export const CHANGE_OVERLAY_SMALL: ChangeOverlay = overlay({
  changedFiles: [
    { ...CHANGED_FILE_BASE, path: 'services/order/create.go', status: 'modified', added: 30, removed: 5 },
    { ...CHANGED_FILE_BASE, path: 'services/order/new.go', status: 'added', added: 10 },
    { ...CHANGED_FILE_BASE, path: 'services/order/renamed.go', oldPath: 'services/order/old.go', status: 'renamed' }
  ],
  changedSymbols: [
    { symbol: SYMBOL_REF_SERVICE, changeKind: 'modified', linesChanged: 12, flows: 1, tested: 'yes' }
  ],
  readingOrder: READING_STEPS,
  components: COMPONENT_GROUPS,
  risk: { level: 'MEDIUM', score: 3, incomplete: false, confidence: 'medium', reasons: [], modelVersion: '1' },
  limits: {
    truncated: { files: false, symbols: false, flows: false, steps: false, impact: false },
    totalCounts: { files: 3, symbols: 1 }
  }
})

/** Medium overlay: 60 changed files */
export const CHANGE_OVERLAY_MEDIUM: ChangeOverlay = overlay({
  changedFiles: Array.from({ length: 60 }, (_, i) => ({
    ...CHANGED_FILE_BASE,
    path: `services/order/file-${i}.go`,
    status: 'modified' as const
  })),
  limits: {
    truncated: { files: false, symbols: false, flows: false, steps: false, impact: false },
    totalCounts: { files: 60 }
  }
})

/** Truncated overlay: backend cut the file list */
export const CHANGE_OVERLAY_TRUNCATED: ChangeOverlay = overlay({
  changedFiles: CHANGE_OVERLAY_SMALL.changedFiles,
  limits: {
    truncated: { files: true, symbols: false, flows: false, steps: true, impact: false },
    totalCounts: { files: 5000 }
  }
})

/** Empty overlay on an unborn HEAD */
export const CHANGE_OVERLAY_EMPTY_UNBORN: ChangeOverlay = overlay({ emptyReason: 'unborn-head' })

// ---------------------------------------------------------------------------
// §4.2 Impact, symbol detail, module graph
// ---------------------------------------------------------------------------

/** v7 limitation: no edges, only levels */
export const IMPACT_GRAPH_NO_EDGES: ImpactGraph = {
  target: SYMBOL_REF_SERVICE,
  direction: 'upstream',
  risk: 'HIGH',
  impactedCount: 1,
  levels: [{ depth: 1, symbols: [{ symbol: SYMBOL_REF_TEST, via: 'calls', direct: true }] }],
  affectedFlows: [{ flowId: 'flow-1', label: 'Create order', stepCount: 4 }],
  affectedClusters: [{ id: 'c1', label: 'order', hits: 2, impact: 'high' }],
  testsCovering: [SYMBOL_REF_TEST]
}

export const SYMBOL_DETAIL_WITH_SOURCE: SymbolDetail = {
  symbol: SYMBOL_REF_SERVICE,
  incoming: { calls: [{ key: SYMBOL_REF_TEST.key, name: 'TestCreate', filePath: SYMBOL_REF_TEST.filePath, line: 12 }] },
  outgoing: {},
  flows: [{ id: 'flow-1', label: 'Create order', stepCount: 4, step: 2 }],
  source: { text: 'func Create() {}', startLine: 10, endLine: 10, truncated: false },
  sourceOmitted: null
}

export const SYMBOL_DETAIL_SOURCE_OMITTED: SymbolDetail = {
  ...SYMBOL_DETAIL_WITH_SOURCE,
  source: null,
  sourceOmitted: 'sensitive_path'
}

/** One page of a module graph; the paging token travels in the envelope (nextPageToken) */
export const MODULE_GRAPH_PAGE_1: ModuleGraph = {
  nodes: [
    { id: 'services/order', kind: 'folder', symbolCount: 10 },
    { id: 'services/order/create.go', kind: 'file', language: 'go', symbolCount: 3, loc: 42 }
  ],
  edges: [{ from: 'services/order', to: 'services/order/create.go', kind: 'contains', count: 1 }]
}
export const MODULE_GRAPH_PAGE_1_TOKEN = 'page-2'

// ---------------------------------------------------------------------------
// §4.4 C4, data flow, sequence
// ---------------------------------------------------------------------------

export const CONTAINER_REFS: ContainerRef[] = [
  { id: 'order-service', name: 'order-service', path: 'backend-go/services/order', kind: 'service' },
  { id: 'frontend', name: 'frontend', path: 'frontend', kind: 'frontend' }
]

export const C4_COMPONENT_VIEW: C4ComponentView = {
  container: CONTAINER_REFS[0],
  components: [
    {
      id: 'order.usecase',
      name: 'usecase',
      kind: 'usecase',
      path: 'services/order/usecase',
      descriptionSource: 'none',
      symbolCount: 12,
      origin: 'derived',
      packagePaths: ['services/order/usecase'],
      hidden: false
    }
  ],
  relations: [],
  externals: [],
  warnings: [],
  overridesVersion: '0',
  hasOverrides: false
}

export const DATA_FLOW_PARTIAL: DataFlow = {
  id: 'flow-1',
  label: 'Create order',
  trigger: { kind: 'ws-channel', name: 'order.create' },
  steps: [
    {
      n: 1,
      from: { container: 'frontend', componentId: 'ui', name: 'UI', kind: 'ui' },
      to: { container: 'order-service', componentId: 'order.usecase', name: 'usecase', kind: 'component' },
      kind: 'rpc',
      sync: true,
      confidence: 0.9,
      origin: 'static-name',
      evidence: []
    }
  ],
  stores: [],
  completeness: 'partial',
  gaps: [{ afterStep: 1, code: 'no-callee', message: 'callee not resolved' }],
  services: ['order-service'],
  relatedProcesses: []
}

export const SEQUENCE_MODEL: SequenceModel = {
  participants: [
    { id: 'ui', label: 'UI', kind: 'ui' },
    { id: 'order', label: 'order', kind: 'component' }
  ],
  messages: [
    { n: 1, from: 'ui', to: 'order', label: 'create', kind: 'rpc', sync: true, dashedReturn: false, confidence: 0.9 }
  ]
}

