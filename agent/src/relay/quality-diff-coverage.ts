import type { CoverBlock } from './quality-coverage-go-profile'
import { isFileExcluded } from './quality-coverage-go-profile'
import type { ChangedFileDiff } from './quality-diff-changed-lines'

export interface UncoveredRange {
  start: number
  end: number
}

export interface FileDiffCoverage {
  path: string
  executable: number
  covered: number
  uncovered: number
  pct: number
  uncoveredRanges: UncoveredRange[]
}

export interface ExcludedCoverageFile {
  path: string
  reason: 'not_go' | 'test' | 'generated' | 'no_coverage_blocks' | 'doc'
}

export interface DiffCoverageSummary {
  changedExecutable: number
  covered: number
  uncovered: number
  diffCoverage: number | null
  reason?: string
  files: FileDiffCoverage[]
  excludedFiles: ExcludedCoverageFile[]
  partial: boolean
  modules: string[]
  noTests: string[]
}

export interface CoverageReportTotals {
  stmts: number
  covered: number
  pct: number
}

export interface CoverageReport {
  source: 'measured'
  language: string
  mode: string
  headCommit?: string | null
  baseCommit?: string | null
  dirty?: boolean
  totals: CoverageReportTotals
  diff: DiffCoverageSummary
  files: FileDiffCoverage[]
  truncated: boolean
  totalCount: number
  toolVersions: Record<string, string>
}

export function computeDiffCoverage(
  blocksByFile: Record<string, CoverBlock[]>,
  addedFiles: ChangedFileDiff[],
  opts?: {
    mode?: string
    toolVersion?: string
    modules?: string[]
    noTests?: string[]
    partial?: boolean
  }
): DiffCoverageSummary {
  const resultFiles: FileDiffCoverage[] = []
  const excludedFiles: ExcludedCoverageFile[] = []

  let totalChangedExecutable = 0
  let totalCovered = 0
  let totalUncovered = 0

  for (const f of addedFiles) {
    if (!f.path.endsWith('.go')) {
      excludedFiles.push({ path: f.path, reason: 'not_go' })
      continue
    }

    if (f.path.endsWith('_test.go')) {
      excludedFiles.push({ path: f.path, reason: 'test' })
      continue
    }

    if (isFileExcluded(f.path)) {
      excludedFiles.push({ path: f.path, reason: 'generated' })
      continue
    }

    const blocks = blocksByFile[f.path]
    if (!blocks || blocks.length === 0) {
      excludedFiles.push({ path: f.path, reason: 'no_coverage_blocks' })
      continue
    }

    const uncoveredLines: number[] = []
    let fileExec = 0
    let fileCov = 0

    for (const added of f.addedLines) {
      const matchingBlocks = blocks.filter(
        b => b.startLine <= added.line && added.line <= b.endLine
      )

      if (matchingBlocks.length === 0) {
        // Line is not part of any executable block (comments, whitespace, signatures)
        continue
      }

      fileExec++
      // Conservative K4: if any block has count === 0, line is considered uncovered
      const isUncovered = matchingBlocks.some(b => b.count === 0)
      if (isUncovered) {
        uncoveredLines.push(added.line)
      } else {
        fileCov++
      }
    }

    if (fileExec > 0) {
      const fileUncov = fileExec - fileCov
      totalChangedExecutable += fileExec
      totalCovered += fileCov
      totalUncovered += fileUncov

      resultFiles.push({
        path: f.path,
        executable: fileExec,
        covered: fileCov,
        uncovered: fileUncov,
        pct: fileExec > 0 ? fileCov / fileExec : 0,
        uncoveredRanges: buildRanges(uncoveredLines)
      })
    }
  }

  const diffCoverage =
    totalChangedExecutable > 0 ? totalCovered / totalChangedExecutable : null

  return {
    changedExecutable: totalChangedExecutable,
    covered: totalCovered,
    uncovered: totalUncovered,
    diffCoverage,
    reason: totalChangedExecutable === 0 ? 'no_executable_changed_lines' : undefined,
    files: resultFiles,
    excludedFiles,
    partial: opts?.partial ?? false,
    modules: opts?.modules ?? [],
    noTests: opts?.noTests ?? []
  }
}

export function buildRanges(lines: number[]): UncoveredRange[] {
  if (lines.length === 0) return []
  const sorted = Array.from(new Set(lines)).sort((a, b) => a - b)
  const ranges: UncoveredRange[] = []

  let start = sorted[0]
  let prev = sorted[0]

  for (let i = 1; i < sorted.length; i++) {
    const cur = sorted[i]
    if (cur === prev + 1) {
      prev = cur
    } else {
      ranges.push({ start, end: prev })
      start = cur
      prev = cur
    }
  }
  ranges.push({ start, end: prev })

  return ranges
}

export function shapeReportPayload(report: CoverageReport): CoverageReport {
  let files = [...report.files]
  const totalCount = files.length
  let truncated = false

  // Sort files by coverage ascending so lowest coverage files are prioritized
  files.sort((a, b) => a.pct - b.pct)

  if (files.length > 2000) {
    files = files.slice(0, 2000)
    truncated = true
  }

  // Filter uncoveredRanges: keep only for changed files or pct < 0.5
  for (const f of files) {
    if (f.pct >= 0.5) {
      f.uncoveredRanges = []
    }
  }

  const candidate: CoverageReport = {
    ...report,
    files,
    truncated,
    totalCount
  }

  const jsonStr = JSON.stringify(candidate)
  if (jsonStr.length > 1024 * 1024) {
    // Truncate further if payload exceeds 1 MiB
    candidate.files = candidate.files.slice(0, Math.floor(candidate.files.length / 2))
    candidate.truncated = true
  }

  return candidate
}
