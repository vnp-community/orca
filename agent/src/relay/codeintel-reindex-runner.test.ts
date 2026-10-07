import { describe, it, expect, vi, beforeEach } from 'vitest'
import { runReindexJob, cancelReindex, getRunningJobTail } from './codeintel-reindex-runner'
import { ReindexJob } from './codeintel-reindex-job'
import * as ToolRegistry from './agent-tool-registry'
import * as NotificationSink from './codeintel-notification-sink'
import * as GitnexusProbe from './gitnexus-index-probe'
import * as CodegraphProbe from './codegraph-index-probe'
import * as HeadCommit from './codeintel-head-commit'

vi.mock('./agent-tool-registry', async (importOriginal) => {
  const mod = await importOriginal<any>()
  return { ...mod, runToolCommand: vi.fn() }
})

vi.mock('./codeintel-notification-sink', () => ({
  emitCodeIntelNotification: vi.fn()
}))

vi.mock('./gitnexus-index-probe', () => ({
  gitnexusIndexProbe: vi.fn()
}))

vi.mock('./codegraph-index-probe', () => ({
  codegraphIndexProbe: vi.fn()
}))

vi.mock('./codeintel-head-commit', () => ({
  invalidateHeadCommit: vi.fn()
}))

describe('codeintel-reindex-runner', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(GitnexusProbe.gitnexusIndexProbe).mockResolvedValue({ state: 'ready' })
    vi.mocked(CodegraphProbe.codegraphIndexProbe).mockResolvedValue({ state: 'ready' })
    vi.mocked(ToolRegistry.runToolCommand).mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 })
  })

  it('runs job, emits progress, verifies and completes', async () => {
    const job: ReindexJob = {
      jobId: 'j1', state: 'queued', workspaceRoot: '/repo', repoRoot: '/repo', mode: 'full', tools: ['gitnexus'], trigger: 'manual', startedAt: 0, estimate: null, outcome: '', skipped: []
    }
    
    vi.mocked(ToolRegistry.runToolCommand).mockImplementation(async (binary, argv, opts: any) => {
      opts.onStdout(Buffer.from('Processing 10% \nDone 100% \n'))
      return { stdout: '', stderr: '', exitCode: 0 }
    })

    await runReindexJob(job, { toolEnv: {} })
    console.error(getRunningJobTail('j1'))
    
    expect(job.state).toBe('completed')
    expect(job.outcome).toBe('')
    expect(NotificationSink.emitCodeIntelNotification).toHaveBeenCalledWith('codeintel.indexChanged', { reason: 'reindex', workspaceRoot: '/repo' })
    expect(HeadCommit.invalidateHeadCommit).toHaveBeenCalledWith('/repo')
  })

  it('handles cancellation', async () => {
    const job: ReindexJob = {
      jobId: 'j2', state: 'queued', workspaceRoot: '/repo', repoRoot: '/repo', mode: 'full', tools: ['gitnexus'], trigger: 'manual', startedAt: 0, estimate: null, outcome: '', skipped: []
    }
    
    vi.mocked(ToolRegistry.runToolCommand).mockImplementation(async (binary, argv, opts: any) => {
      opts.signal?.addEventListener('abort', () => {
        // mock abort behavior
      })
      cancelReindex('j2')
      await new Promise(r => setTimeout(r, 10)) // let abort trigger
      return { stdout: '', stderr: '', exitCode: 124 }
    })

    await runReindexJob(job, { toolEnv: {} })
    expect(job.state).toBe('cancelled')
    expect(job.outcome).toBe('')
    expect(NotificationSink.emitCodeIntelNotification).not.toHaveBeenCalledWith('codeintel.indexChanged', expect.anything())
  })

  it('sets outcome index_not_updated if probe fails', async () => {
    const job: ReindexJob = {
      jobId: 'j3', state: 'queued', workspaceRoot: '/repo', repoRoot: '/repo', mode: 'full', tools: ['gitnexus'], trigger: 'manual', startedAt: 0, estimate: null, outcome: '', skipped: []
    }
    
    vi.mocked(GitnexusProbe.gitnexusIndexProbe).mockRejectedValue(new Error('probe failed'))

    await runReindexJob(job, { toolEnv: {} })
    
    expect(job.state).toBe('completed')
    expect(job.outcome).toBe('index_not_updated')
  })
})
