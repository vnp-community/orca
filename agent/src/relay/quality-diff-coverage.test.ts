import { describe, it, expect } from 'vitest'
import {
  computeDiffCoverage,
  buildRanges,
  shapeReportPayload,
  type CoverageReport
} from './quality-diff-coverage'
import type { CoverBlock } from './quality-coverage-go-profile'
import type { ChangedFileDiff } from './quality-diff-changed-lines'

describe('quality-diff-coverage', () => {
  it('correctly builds line ranges for uncovered lines', () => {
    expect(buildRanges([])).toEqual([])
    expect(buildRanges([5])).toEqual([{ start: 5, end: 5 }])
    expect(buildRanges([5, 6, 7, 10, 11])).toEqual([
      { start: 5, end: 7 },
      { start: 10, end: 11 }
    ])
  })

  it('computes diff coverage accurately with conservative K4 rule', () => {
    const blocksByFile: Record<string, CoverBlock[]> = {
      'pkg/calc.go': [
        { file: 'pkg/calc.go', startLine: 5, startCol: 1, endLine: 7, endCol: 1, numStmt: 1, count: 1 },
        { file: 'pkg/calc.go', startLine: 10, startCol: 1, endLine: 12, endCol: 1, numStmt: 1, count: 0 }
      ]
    }

    const addedFiles: ChangedFileDiff[] = [
      {
        path: 'pkg/calc.go',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [
          { line: 4, text: '// comment' }, // non-executable
          { line: 6, text: 'return a + b' }, // covered
          { line: 11, text: 'return a - b' } // uncovered
        ]
      },
      {
        path: 'pkg/calc_test.go',
        status: 'A',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'func TestCalc() {}' }]
      },
      {
        path: 'README.md',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'Title' }]
      }
    ]

    const res = computeDiffCoverage(blocksByFile, addedFiles)
    expect(res.changedExecutable).toBe(2)
    expect(res.covered).toBe(1)
    expect(res.uncovered).toBe(1)
    expect(res.diffCoverage).toBe(0.5)
    expect(res.files.length).toBe(1)
    expect(res.files[0].uncoveredRanges).toEqual([{ start: 11, end: 11 }])

    // Excluded files
    expect(res.excludedFiles).toContainEqual({ path: 'pkg/calc_test.go', reason: 'test' })
    expect(res.excludedFiles).toContainEqual({ path: 'README.md', reason: 'not_go' })
  })

  it('returns null diffCoverage and reason when changed executable lines is 0', () => {
    const blocksByFile: Record<string, CoverBlock[]> = {
      'pkg/calc.go': [
        { file: 'pkg/calc.go', startLine: 5, startCol: 1, endLine: 7, endCol: 1, numStmt: 1, count: 1 }
      ]
    }

    const addedFiles: ChangedFileDiff[] = [
      {
        path: 'pkg/calc.go',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [
          { line: 1, text: '// only comments added' }
        ]
      }
    ]

    const res = computeDiffCoverage(blocksByFile, addedFiles)
    expect(res.changedExecutable).toBe(0)
    expect(res.diffCoverage).toBeNull()
    expect(res.reason).toBe('no_executable_changed_lines')
  })

  it('shapes report payload and respects size limits', () => {
    const dummyFiles = Array.from({ length: 2500 }, (_, i) => ({
      path: `file_${i}.go`,
      executable: 10,
      covered: i % 2 === 0 ? 10 : 0,
      uncovered: i % 2 === 0 ? 0 : 10,
      pct: i % 2 === 0 ? 1.0 : 0.0,
      uncoveredRanges: [{ start: 1, end: 5 }]
    }))

    const report: CoverageReport = {
      source: 'measured',
      language: 'go',
      mode: 'set',
      totals: { stmts: 25000, covered: 12500, pct: 0.5 },
      diff: {
        changedExecutable: 10,
        covered: 5,
        uncovered: 5,
        diffCoverage: 0.5,
        files: [],
        excludedFiles: [],
        partial: false,
        modules: [],
        noTests: []
      },
      files: dummyFiles,
      truncated: false,
      totalCount: 2500,
      toolVersions: { go: 'go1.22' }
    }

    const shaped = shapeReportPayload(report)
    expect(shaped.totalCount).toBe(2500)
    expect(shaped.files.length).toBeLessThanOrEqual(2000)
    expect(shaped.truncated).toBe(true)
  })
})
