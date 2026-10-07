import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleQualityCoverage } from './quality-coverage-handler'
import { saveCoverageReport } from './quality-coverage-collector'
import { getQualityRunManager } from './quality-run-manager'

describe('quality-coverage-handler', () => {
  const mockCtx = {
    config: {},
    log: { info: vi.fn(), warn: vi.fn(), error: vi.fn() }
  }

  it('throws CODEINTEL_RUN_NOT_FOUND when run does not exist', async () => {
    await expect(
      handleQualityCoverage({ workspaceRoot: '/tmp', runId: 'qr_unknown' }, mockCtx)
    ).rejects.toThrowError(/Run not found/)
  })

  it('returns coverage report when present', async () => {
    saveCoverageReport('/tmp', 'qr_valid', {
      source: 'measured',
      language: 'go',
      mode: 'set',
      totals: { stmts: 10, covered: 8, pct: 0.8 },
      diff: {
        changedExecutable: 10,
        covered: 8,
        uncovered: 2,
        diffCoverage: 0.8,
        files: [],
        excludedFiles: [],
        partial: false,
        modules: [],
        noTests: []
      },
      files: [],
      truncated: false,
      totalCount: 0,
      toolVersions: { go: 'go1.22' }
    })

    const res = await handleQualityCoverage({ workspaceRoot: '/tmp', runId: 'qr_valid' }, mockCtx)
    expect(res.available).toBe(true)
    expect(res.runId).toBe('qr_valid')
    expect(res.report?.totals.pct).toBe(0.8)
  })
})
