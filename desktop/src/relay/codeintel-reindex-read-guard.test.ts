import { describe, it, expect, vi, beforeEach } from 'vitest'
import { assertReadable } from './codeintel-reindex-read-guard'
import { activeJobs, clearReindexStateForTest, ReindexJob } from './codeintel-reindex-job'
import * as SqliteReader from './codegraph-sqlite-reader'
import { CodeIntelError } from './codeintel-errors'

vi.mock('./codegraph-sqlite-reader', () => ({
  closeCodeGraphDb: vi.fn()
}))

describe('codeintel-reindex-read-guard', () => {
  const binding: any = {
    toplevel: '/repo',
    mainCheckoutRoot: '/repo'
  }

  beforeEach(() => {
    vi.clearAllMocks()
    clearReindexStateForTest()
  })

  it('allows reading when no reindex job is active', () => {
    expect(() => assertReadable(binding, 'gitnexus')).not.toThrow()
    expect(() => assertReadable(binding, 'codegraph')).not.toThrow()
  })

  it('throws REINDEX_IN_PROGRESS for gitnexus when a gitnexus job is active', () => {
    const job: ReindexJob = {
      jobId: 'job-1',
      state: 'running',
      workspaceRoot: '/repo',
      repoRoot: '/repo',
      mode: 'full',
      tools: ['gitnexus'],
      trigger: 'manual',
      startedAt: Date.now(),
      estimate: null,
      outcome: '',
      skipped: []
    }
    activeJobs.set('job-1', job)

    expect(() => assertReadable(binding, 'gitnexus')).toThrowError(CodeIntelError)
    try {
      assertReadable(binding, 'gitnexus')
    } catch (err: any) {
      expect(err.code).toBe('CODEINTEL_REINDEX_IN_PROGRESS')
      expect(err.data.jobId).toBe('job-1')
    }
  })

  it('does not throw for gitnexus when only codegraph job is running', () => {
    const job: ReindexJob = {
      jobId: 'job-cg',
      state: 'running',
      workspaceRoot: '/repo',
      repoRoot: '/repo',
      mode: 'full',
      tools: ['codegraph'],
      trigger: 'manual',
      startedAt: Date.now(),
      estimate: null,
      outcome: '',
      skipped: []
    }
    activeJobs.set('job-cg', job)

    expect(() => assertReadable(binding, 'gitnexus')).not.toThrow()
  })

  it('closes sqlite connection when codegraph reindex job is active', () => {
    const job: ReindexJob = {
      jobId: 'job-cg',
      state: 'running',
      workspaceRoot: '/repo',
      repoRoot: '/repo',
      mode: 'full',
      tools: ['codegraph'],
      trigger: 'manual',
      startedAt: Date.now(),
      estimate: null,
      outcome: '',
      skipped: []
    }
    activeJobs.set('job-cg', job)

    assertReadable(binding, 'codegraph')
    expect(SqliteReader.closeCodeGraphDb).toHaveBeenCalledWith('/repo')
  })
})
