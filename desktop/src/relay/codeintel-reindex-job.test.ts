import { describe, it, expect, vi, beforeEach } from 'vitest'
import { startReindex, getJob, clearReindexStateForTest, activeJobs, queue, validateReindexParams } from './codeintel-reindex-job'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import * as HeadCommit from './codeintel-head-commit'
import * as ToolDetection from './codeintel-tool-detection'

vi.mock('./codeintel-head-commit', () => ({
  getHeadCommit: vi.fn()
}))

vi.mock('./codeintel-tool-detection', () => ({
  detectCodeIntelBinaries: vi.fn()
}))

describe('codeintel-reindex-job validateReindexParams', () => {
  it('throws CODEINTEL_INVALID_PARAMS on invalid keys', () => {
    expect(() => validateReindexParams({ workspaceRoot: '/foo', tiers: ['sync'] })).toThrow(/Unknown parameter: tiers/)
    expect(() => validateReindexParams({ workspaceRoot: '/foo', invalidKey: true })).toThrow(/Unknown parameter: invalidKey/)
  })

  it('validates trigger values', () => {
    expect(() => validateReindexParams({ workspaceRoot: '/foo', trigger: 'invalid' })).toThrow(/Invalid trigger/)
    expect(() => validateReindexParams({ workspaceRoot: '/foo', trigger: 'manual' })).not.toThrow()
    expect(() => validateReindexParams({ workspaceRoot: '/foo', trigger: 'agent_done' })).not.toThrow()
    expect(() => validateReindexParams({ workspaceRoot: '/foo', trigger: 'head_change' })).not.toThrow()
  })

  it('validates expectHead', () => {
    expect(() => validateReindexParams({ workspaceRoot: '/foo', expectHead: 'invalid' })).toThrow(/Invalid expectHead format/)
    expect(() => validateReindexParams({ workspaceRoot: '/foo', expectHead: '1234567890abcdef' })).not.toThrow()
  })
})

describe('codeintel-reindex-job startReindex', () => {
  const mockDeps = { config: {}, log: {}, getIndexedCommit: vi.fn(), checkFreshness: vi.fn() }
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

  it('returns queue_full if queue is full', async () => {
    for (let i = 0; i < 4; i++) {
      await startReindex({ ...mockBinding, toplevel: `/r${i}` }, {}, mockDeps)
    }
    const res = await startReindex({ ...mockBinding, toplevel: '/r5' }, {}, mockDeps)
    expect(res.outcome).toBe('queue_full')
    expect(res.state).toBe('cancelled')
  })

  it('throws PATH_NOT_ALLOWED if linked worktree and manual trigger', async () => {
    const b = { ...mockBinding, linkedWorktree: true }
    await expect(startReindex(b, { trigger: 'manual' }, mockDeps)).rejects.toThrow(/Cannot manually reindex linked worktree/)
  })

  it('skips if linked worktree and agent_done trigger', async () => {
    const b = { ...mockBinding, linkedWorktree: true }
    const res = await startReindex(b, { trigger: 'agent_done' }, mockDeps)
    expect(res.outcome).toBe('skipped_scope_repo_root')
    expect(res.skipped).toEqual([
      { tool: 'gitnexus', reason: 'index_root_is_main_checkout' },
      { tool: 'codegraph', reason: 'index_root_is_main_checkout' }
    ])
    expect(queue).toHaveLength(0)
  })

  it('already_up_to_date if ifStale is fresh via checkFreshness', async () => {
    mockDeps.checkFreshness.mockResolvedValue(true)
    const res = await startReindex(mockBinding, { ifStale: true }, mockDeps)
    expect(res.outcome).toBe('already_up_to_date')
    expect(queue).toHaveLength(0)
  })

  it('superseded if expectHead mismatches', async () => {
    const res = await startReindex(mockBinding, { expectHead: '1234567890abcdef' }, mockDeps)
    expect(res.outcome).toBe('superseded')
    expect(res.state).toBe('completed')
  })
})
