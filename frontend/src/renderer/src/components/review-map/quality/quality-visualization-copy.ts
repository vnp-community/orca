/**
 * quality-visualization-copy.ts — FE-CV-TASK-087-20
 *
 * English copy of the quality visualization blocks (coverage, trend, hotspot, dependency).
 * Keys are read by name, so the locale coverage test enumerates this table. Call `qv(key)` at
 * render time, never at module scope. Chart-level strings live in quality-chart-copy.
 *
 * @module components/review-map/quality/quality-visualization-copy
 */

import { createQualityCopy } from './quality-copy-factory'

export const QUALITY_VISUALIZATION_COPY = {
  'block.refresh': 'Refresh',
  'block.unavailable': 'This view needs a worktree that code intelligence can address.',
  'block.stale': 'The last refresh failed; showing the previously loaded data.',
  'error.generic': 'Could not load this data.',
  'error.timeout': 'The analysis is taking too long. Try again in a moment.',
  'error.tooLarge': 'The result is too large to show. Narrow the scope and try again.',
  'coverage.title': 'Test coverage',
  'coverage.description':
    'Coverage of the changed lines and per file. Darker tiles have a larger share of statements not covered.',
  'coverage.diffTitle': 'Diff coverage',
  'coverage.treemapTitle': 'Coverage by file',
  'coverage.sizeLabel': 'statements',
  'coverage.intensityLabel': 'not covered %',
  'coverage.estimatedBanner': 'Estimated from test edges, not measured coverage.',
  'coverage.measuredLine': 'Measured coverage.',
  'coverage.totals': 'Whole report: {{percent}}% of {{stmts}} statements covered.',
  'coverage.dirty': 'The working tree had uncommitted changes when this was measured.',
  'coverage.noReport': 'No coverage data for this scope yet.',
  'coverage.noReportReason': 'No coverage data for this scope yet. Reason: {{reason}}',
  'coverage.noDiff': 'Diff coverage is not available for this scope.',
  'coverage.noDiffReason': 'Diff coverage is not available for this scope. Reason: {{reason}}',
  'coverage.noChangedLines': 'There are no changed executable lines in this scope.',
  'coverage.runCheck': 'Run check',
  'coverage.partial': 'Partial scope: some changed files are not measured.',
  'coverage.excluded': 'Excluded files ({{count}})',
  'coverage.excludedItem': '{{path}}: {{reason}}',
  'coverage.noStatements': '{{count}} files without statements are not drawn.',
  'coverage.uncoveredTitle': 'Lines not covered',
  'coverage.uncoveredNone': 'No uncovered line ranges were reported.',
  'coverage.uncoveredFile': '{{path}} ({{count}} lines)',
  'coverage.uncoveredRange': 'Lines {{from}}-{{to}}',
  'coverage.uncoveredLine': 'Line {{line}}',
  'coverage.openDiff': 'Open diff of {{path}} at line {{line}}',
  'coverage.uncoveredMore':
    'Showing the {{shown}} files with the most uncovered lines of {{total}}.',
  'trend.title': 'Trend by turn',
  'trend.titleCommit': 'Trend by commit',
  'trend.description':
    'Findings per point. A gap in a line means that no number was recorded for that point.',
  'trend.groupBy': 'Group by',
  'trend.groupTurn': 'Turn',
  'trend.groupCommit': 'Commit',
  'trend.empty': 'No quality runs have been recorded for this worktree yet.',
  'trend.needTwo': 'At least two turns are needed to show a trend.',
  'trend.latest': 'Latest point: {{error}} errors, {{warning}} warnings, {{info}} info.',
  'trend.clickHint': 'Select a turn to compare it with the previous one.',
  'trend.diffCoverage': 'Diff coverage',
  'trend.diffCoverageMissing': '{{count}} points have no diff coverage number.',
  'trend.sourceLocal': 'Local',
  'trend.sourceCi': 'CI',
  'trend.sourceUnknown': 'Unknown source',
  'trend.details': 'Per-point details',
  'trend.colPoint': 'Point',
  'trend.colSource': 'Source',
  'trend.colVerdict': 'Verdict',
  'trend.colCoverage': 'Diff coverage',
  'trend.headNote': 'Figures at HEAD {{commit}} · {{source}}',
  'hotspot.title': 'Hotspots',
  'hotspot.description':
    'Files flagged by the structure analysis (rule hotspot.file) over the configured history window. This is not a quality score.',
  'hotspot.empty': 'No hotspot data yet.',
  'hotspot.emptyReason':
    'No hotspot data yet. Hotspots appear once the structure analysis is on and has enough history.',
  'hotspot.moreRows': '{{count}} more files exist on the backend and are not loaded.',
  'hotspot.allMetrics': 'All metrics ({{count}})',
  'hotspot.colFile': 'File',
  'hotspot.selected': 'Selected: {{path}}',
  'hotspot.owner': 'Owner: {{owner}}',
  'hotspot.openDiff': 'Open diff',
  'dependency.title': 'Dependencies',
  'dependency.description':
    'Rows are importing modules, columns are imported modules. Outlined blocks and ▲ mark cyclic groups.',
  'dependency.emptyReason': 'No dependency data yet for this scope.',
  'dependency.backendTruncated': 'The backend returned {{shown}} of {{total}} modules.',
  'dependency.edgesTitle': 'Edges between {{from}} and {{to}}',
  'dependency.edgeLine': '{{from}} imports {{to}} ({{count}})',
  'dependency.noEdge': 'No import edge between these modules.',
  'dependency.openStructure': 'Open in Structure lens',
  'dependency.close': 'Close'
} as const

export type QualityVisualizationCopyKey = keyof typeof QUALITY_VISUALIZATION_COPY

export const qv = createQualityCopy('visualization', QUALITY_VISUALIZATION_COPY)
