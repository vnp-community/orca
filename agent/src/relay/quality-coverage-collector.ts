import fs from 'node:fs'
import path from 'node:path'
import { parseGoCoverProfile, readGoWorkModules, type CoverBlock } from './quality-coverage-go-profile'
import { getAddedLines } from './quality-diff-changed-lines'
import {
  computeDiffCoverage,
  shapeReportPayload,
  type CoverageReport,
  type FileDiffCoverage
} from './quality-diff-coverage'

interface StoredCoverage {
  report: CoverageReport
  createdAt: number
}

// In-memory store for coverage reports by workspaceRoot and runId with 1h TTL
const coverageStore = new Map<string, StoredCoverage>()
const COVERAGE_TTL_MS = 60 * 60 * 1000

function storeKey(workspaceRoot: string, runId: string): string {
  return `${workspaceRoot}::${runId}`
}

export function saveCoverageReport(workspaceRoot: string, runId: string, report: CoverageReport): void {
  // Prune expired
  const now = Date.now()
  for (const [k, v] of coverageStore.entries()) {
    if (now - v.createdAt > COVERAGE_TTL_MS) {
      coverageStore.delete(k)
    }
  }

  coverageStore.set(storeKey(workspaceRoot, runId), {
    report,
    createdAt: now
  })
}

export function getCoverageReport(workspaceRoot: string, runId: string): CoverageReport | null {
  const item = coverageStore.get(storeKey(workspaceRoot, runId))
  if (!item) return null
  if (Date.now() - item.createdAt > COVERAGE_TTL_MS) {
    coverageStore.delete(storeKey(workspaceRoot, runId))
    return null
  }
  return item.report
}

export async function collectGoCoverage(opts: {
  workspaceRoot: string
  runId: string
  runDir: string
  baseCommit?: string | null
  headCommit?: string | null
  dirty?: boolean
  stepOutputs: { moduleName: string; profilePath: string; success: boolean }[]
  scope: string
}): Promise<CoverageReport> {
  const modules = readGoWorkModules(opts.workspaceRoot)
  const blocksByFile: Record<string, CoverBlock[]> = {}

  let totalStmts = 0
  let totalCoveredStmts = 0
  const measuredModules: string[] = []
  const noTestModules: string[] = []

  for (const step of opts.stepOutputs) {
    if (!step.success || !fs.existsSync(step.profilePath)) {
      noTestModules.push(step.moduleName)
      continue
    }

    try {
      const content = fs.readFileSync(step.profilePath, 'utf8')
      const parsed = parseGoCoverProfile(content, { modules })
      measuredModules.push(step.moduleName)

      for (const b of parsed.blocks) {
        if (!blocksByFile[b.file]) {
          blocksByFile[b.file] = []
        }
        blocksByFile[b.file].push(b)
        totalStmts += b.numStmt
        if (b.count > 0) {
          totalCoveredStmts += b.numStmt
        }
      }
    } catch {
      noTestModules.push(step.moduleName)
    }
  }

  // Get added lines in worktree
  const addedLinesRes = await getAddedLines(opts.workspaceRoot, {
    mergeBaseOf: opts.baseCommit ?? null,
    to: 'worktree'
  })

  const diffCoverage = computeDiffCoverage(blocksByFile, addedLinesRes.files, {
    modules: measuredModules,
    noTests: noTestModules,
    partial: opts.scope === 'changed'
  })

  // Build files list
  const files: FileDiffCoverage[] = diffCoverage.files

  const totals = {
    stmts: totalStmts,
    covered: totalCoveredStmts,
    pct: totalStmts > 0 ? totalCoveredStmts / totalStmts : 0
  }

  const report: CoverageReport = {
    source: 'measured',
    language: 'go',
    mode: 'set',
    headCommit: opts.headCommit ?? null,
    baseCommit: opts.baseCommit ?? null,
    dirty: opts.dirty ?? false,
    totals,
    diff: diffCoverage,
    files,
    truncated: false,
    totalCount: files.length,
    toolVersions: { go: 'go1.22' }
  }

  const shaped = shapeReportPayload(report)
  saveCoverageReport(opts.workspaceRoot, opts.runId, shaped)
  return shaped
}
