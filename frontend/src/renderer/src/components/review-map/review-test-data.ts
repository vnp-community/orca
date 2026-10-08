import type {
  ChangeOverlayView,
  IndexStatusView,
  ReadingProgress,
  ReadingStepView
} from './review-wire-types'

/** Test-only builders shared by review shell/reading-order tests. */
export function makeStep(n: number, over: Partial<ReadingStepView> = {}): ReadingStepView {
  return {
    stepKey: `s${n}`,
    n,
    file: `src/f${n}.ts`,
    symbols: [],
    hunks: [{ startLine: 10 + n, endLine: 20 + n }],
    reason: 'leaf',
    dependsOn: [],
    tests: [],
    layer: 'core',
    ...over
  }
}

export function makeOverlay(over: Partial<ChangeOverlayView> = {}): ChangeOverlayView {
  const steps = [makeStep(1), makeStep(2), makeStep(3)]
  return {
    scope: {
      baseRef: 'origin/main',
      baseOid: 'b'.repeat(40),
      mergeBase: 'm'.repeat(40),
      headOid: 'h'.repeat(40),
      mode: 'worktree',
      includesUncommitted: true
    },
    changedFiles: steps.map((s) => ({ path: s.file, status: 'modified' as const })),
    changedSymbols: [],
    affectedFlows: [],
    touchedTables: [],
    touchedContracts: [],
    uncoveredSymbols: [],
    violations: [],
    readingOrder: steps,
    components: [],
    risk: { level: 'MEDIUM', incomplete: false, reasons: [] },
    indexFreshness: null,
    limits: { truncated: {}, totalCounts: {} },
    ...over
  }
}

export function makeStatus(over: Partial<IndexStatusView> = {}): IndexStatusView {
  return { overall: 'READY', tools: [], scopeMismatch: false, indexBasis: [], ...over }
}

export const EMPTY_PROGRESS: ReadingProgress = { version: 1, entries: {}, lastFocusedKey: null }
