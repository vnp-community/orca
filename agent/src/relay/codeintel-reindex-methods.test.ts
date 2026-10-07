import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  handleReindex,
  handleReindexStatus,
  handleReindexCancel,
  handleWatch,
  validateReindex,
  validateReindexStatus,
  validateReindexCancel,
  validateWatch
} from './codeintel-reindex-methods'
import { activeJobs, clearReindexStateForTest, ReindexJob } from './codeintel-reindex-job'
import * as RepoRes from './codeintel-repo-resolution'
import * as Runner from './codeintel-reindex-runner'
import * as Watcher from './codeintel-index-watcher'
import { CodeIntelError } from './codeintel-errors'

vi.mock('./codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn().mockResolvedValue({
    toplevel: '/repo',
    workspaceRoot: '/repo',
    gitNexusRegistryPath: '/repo/.gitnexus'
  })
}))

vi.mock('./codeintel-reindex-runner', () => ({
  runReindexJob: vi.fn().mockResolvedValue(undefined),
  cancelReindex: vi.fn().mockResolvedValue(true)
}))

vi.mock('./codeintel-index-watcher', () => ({
  enableWatch: vi.fn(),
  disableWatch: vi.fn(),
  cleanupCodeIntelWatchers: vi.fn(),
  getWatchingRoots: vi.fn().mockReturnValue(['/repo'])
}))

describe('codeintel-reindex-methods', () => {
  const ctx: any = {
    config: {},
    log: { info: vi.fn(), error: vi.fn(), debug: vi.fn() }
  }

  beforeEach(() => {
    vi.clearAllMocks()
    clearReindexStateForTest()
  })

  describe('validations', () => {
    it('rejects unknown params in reindex', () => {
      expect(() => validateReindex({ workspaceRoot: '/repo', unknown: true })).toThrowError(CodeIntelError)
    })
    it('rejects missing workspaceRoot in reindexStatus', () => {
      expect(() => validateReindexStatus({})).toThrowError(CodeIntelError)
    })
    it('rejects missing jobId in reindexCancel', () => {
      expect(() => validateReindexCancel({ workspaceRoot: '/repo' })).toThrowError(CodeIntelError)
    })
    it('rejects non-boolean enabled in watch', () => {
      expect(() => validateWatch({ workspaceRoot: '/repo', enabled: 'yes' })).toThrowError(CodeIntelError)
    })
  })

  describe('handleReindex', () => {
    it('starts reindex and returns job object without envelope', async () => {
      const res = await handleReindex({ workspaceRoot: '/repo', tools: ['gitnexus'] }, ctx)
      expect(res.jobId).toBeDefined()
      expect(res.state).toBe('queued')
      expect(res.tools).toContain('gitnexus')
      expect(res.sources).toBeUndefined() // No envelope
    })
  })

  describe('handleReindexStatus', () => {
    it('returns job: null if no job exists', async () => {
      const res = await handleReindexStatus({ workspaceRoot: '/repo' }, ctx)
      expect(res).toEqual({ job: null })
    })

    it('returns active job by jobId', async () => {
      const job: ReindexJob = {
        jobId: 'job-123',
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
      activeJobs.set('job-123', job)

      const res = await handleReindexStatus({ workspaceRoot: '/repo', jobId: 'job-123' }, ctx)
      expect(res.job).toBe(job)
    })
  })

  describe('handleReindexCancel', () => {
    it('cancels job idempotently', async () => {
      const res = await handleReindexCancel({ workspaceRoot: '/repo', jobId: 'job-123' }, ctx)
      expect(res.jobId).toBe('job-123')
      expect(res.state).toBe('cancelled')
      expect(Runner.cancelReindex).toHaveBeenCalledWith('job-123', expect.anything())
    })
  })

  describe('handleWatch', () => {
    it('enables watching', async () => {
      const res = await handleWatch({ workspaceRoot: '/repo', enabled: true }, ctx)
      expect(res.enabled).toBe(true)
      expect(Watcher.enableWatch).toHaveBeenCalled()
      expect(res.watching).toEqual(['/repo'])
    })

    it('disables watching', async () => {
      const res = await handleWatch({ workspaceRoot: '/repo', enabled: false }, ctx)
      expect(res.enabled).toBe(false)
      expect(Watcher.disableWatch).toHaveBeenCalledWith('/repo')
    })
  })
})
