import { describe, it, expect } from 'vitest'
import path from 'node:path'
import {
  parseVitestCoverageFinal,
  preflightVitestCoverage,
  COVERAGE_TS_PROFILE
} from './quality-coverage-vitest-report'

describe('parseVitestCoverageFinal', () => {
  it('parses valid coverage-final json with statements inside workspace', () => {
    const workspaceRoot = '/app'
    const fixture = JSON.stringify({
      '/app/src/index.ts': {
        path: '/app/src/index.ts',
        statementMap: {
          '0': { start: { line: 1, column: 0 }, end: { line: 1, column: 20 } },
          '1': { start: { line: 2, column: 0 }, end: { line: 3, column: 10 } }
        },
        s: {
          '0': 5,
          '1': 0
        }
      }
    })

    const blocks = parseVitestCoverageFinal(fixture, workspaceRoot)
    expect(blocks).toHaveLength(2)
    expect(blocks[0]).toEqual({
      file: 'src/index.ts',
      startLine: 1,
      startCol: 0,
      endLine: 1,
      endCol: 20,
      numStmt: 1,
      count: 5
    })
    expect(blocks[1].count).toBe(0)
  })

  it('skips files outside workspaceRoot', () => {
    const workspaceRoot = '/app'
    const fixture = JSON.stringify({
      '/other/external.ts': {
        path: '/other/external.ts',
        statementMap: {
          '0': { start: { line: 1, column: 0 }, end: { line: 1, column: 10 } }
        },
        s: { '0': 1 }
      }
    })

    const blocks = parseVitestCoverageFinal(fixture, workspaceRoot)
    expect(blocks).toHaveLength(0)
  })

  it('handles invalid json gracefully by returning empty list', () => {
    const blocks = parseVitestCoverageFinal('invalid json', '/app')
    expect(blocks).toEqual([])
  })
})

describe('preflightVitestCoverage', () => {
  it('reports missing provider when node_modules/@vitest/coverage-v8 is absent', () => {
    const res = preflightVitestCoverage('/non/existent/pkg')
    expect(res.ready).toBe(false)
    expect(res.reason).toBe('coverage_provider_missing')
  })
})

describe('COVERAGE_TS_PROFILE', () => {
  it('has enabled set to false pending approval', () => {
    expect(COVERAGE_TS_PROFILE.enabled).toBe(false)
    expect(COVERAGE_TS_PROFILE.id).toBe('coverage-ts')
    expect(COVERAGE_TS_PROFILE.argv).toContain('--coverage.enabled')
  })
})
