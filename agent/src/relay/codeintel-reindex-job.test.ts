import { describe, it, expect, vi, beforeEach } from 'vitest'
import { startReindex, getJob, clearReindexStateForTest, activeJobs, queue } from './codeintel-reindex-job'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import * as HeadCommit from './codeintel-head-commit'
import * as ToolDetection from './codeintel-tool-detection'

vi.mock('./codeintel-head-commit', () => ({
  getHeadCommit: vi.fn()
}))

vi.mock('./codeintel-tool-detection', () => ({
  detectCodeIntelBinaries: vi.fn()
}))

describe('codeintel-reindex-job', () => {
  const mockDeps = { config: {}, log: {}, getIndexedCommit: vi.fn() }
  const mockBinding: CodeIntelRepoBinding = {
    workspaceRoot: '/repo',
    toplevel: '/repo',
    gitNexusRegistryPath: '/repo/.gitnexus',
    linkedWorktree: false,
    worktreeMismatch: false,
    stale: false
  }

  beforeEach(() => {
    clearReindexStateForTest()
    vi.clearAllMocks()
    vi.mocked(HeadCommit.getHeadCommit).mockResolvedValue('c1')
    vi.mocked(ToolDetection.detectCodeIntelBinaries).mockResolvedValue({ gitnexus: true, codegraph: true })
    process.env.ORCA_CODEINTEL_REINDEX = 'on'
  })

  it('starts a job and adds to queue', async () => {
    const job = await startReindex(mockBinding, { mode: 'full' }, mockDeps)
    expect(job.state).toBe('queued')
    expect(job.tools).toContain('gitnexus')
    expect(queue).toHaveLength(1)
    expect(activeJobs.has(job.jobId)).toBe(true)
  })

  it('throws CODEINTEL_REINDEX_IN_PROGRESS if repo already running', async () => {
    await startReindex(mockBinding, {}, mockDeps)
    await expect(startReindex(mockBinding, {}, mockDeps)).rejects.toThrow('Reindex already in progress')
  })

  it('allows different repo to queue', async () => {
    await startReindex(mockBinding, {}, mockDeps)
    const b2 = { ...mockBinding, toplevel: '/repo2' }
    const job2 = await startReindex(b2, {}, mockDeps)
    expect(job2.state).toBe('queued')
    expect(queue).toHaveLength(2)
  })

  it('returns queue_full if queue is full', async () => {
    for (let i = 0; i < 4; i++) {
      await startReindex({ ...mockBinding, toplevel: `/r${i}` }, {}, mockDeps)
    }
    const res = await startReindex({ ...mockBinding, toplevel: '/r5' }, {}, mockDeps)
    expect(res.outcome).toBe('queue_full')
    expect(res.state).toBe('cancelled')
  })

  it('skips if linked worktree and not manual', async () => {
    const b = { ...mockBinding, linkedWorktree: true }
    const res = await startReindex(b, { trigger: 'agent_done' }, mockDeps)
    expect(res.outcome).toBe('skipped_scope_repo_root')
    expect(queue).toHaveLength(0)
  })

  it('already_up_to_date if ifStale is fresh', async () => {
    mockDeps.getIndexedCommit.mockResolvedValue('c1')
    const res = await startReindex(mockBinding, { ifStale: true }, mockDeps)
    expect(res.outcome).toBe('already_up_to_date')
    expect(queue).toHaveLength(0)
  })

  it('already_up_to_date if expectHead mismatches', async () => {
    const res = await startReindex(mockBinding, { expectHead: 'old' }, mockDeps)
    expect(res.outcome).toBe('already_up_to_date')
  })

  it('cancels if ORCA_CODEINTEL_REINDEX=off', async () => {
    process.env.ORCA_CODEINTEL_REINDEX = 'off'
    const res = await startReindex(mockBinding, {}, mockDeps)
    expect(res.state).toBe('cancelled')
  })

  it('getJob retrieves by id or latest', async () => {
    const job = await startReindex(mockBinding, {}, mockDeps)
    expect(getJob(job.jobId)?.jobId).toBe(job.jobId)
    expect(getJob('latest')?.jobId).toBe(job.jobId)
  })
})
