import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createRunManager, RunPlan } from './quality-run-manager'

describe('quality-run-manager', () => {
  let deps: any

  beforeEach(() => {
    vi.useFakeTimers()
    deps = {
      executeStep: vi.fn().mockImplementation(async () => new Promise(r => setTimeout(() => r({ kind: 'success', exitCode: 0, durationMs: 10 }), 100))),
      journal: { write: vi.fn(), trim: vi.fn() },
      emit: vi.fn(),
      now: vi.fn().mockReturnValue(1000),
      fingerprintOf: vi.fn().mockResolvedValue('fp1'),
      tmpDir: '/tmp'
    }
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('runs sequentially', async () => {
    const mgr = createRunManager(deps)
    const plan: RunPlan = {
      workspaceRoot: '/repo',
      steps: [
        { id: 's1', parser: 'foo' } as any,
        { id: 's2', parser: 'foo' } as any
      ]
    }

    const { runId } = await mgr.submit(plan)
    expect(mgr.getStatus('/repo', runId)?.state).toBe('queued')
    
    await vi.advanceTimersByTimeAsync(1) // let pump run
    
    // now it should be running
    expect(mgr.getStatus('/repo', runId)?.state).toBe('running')
    
    // wait for completion
    await vi.advanceTimersByTimeAsync(500)
    
    expect(mgr.getStatus('/repo', runId)).toBeNull() // run completed and removed from active
    expect(deps.journal.write).toHaveBeenCalled()
    expect(deps.executeStep).toHaveBeenCalledTimes(2)
  })

  it('locks worktree', async () => {
    const mgr = createRunManager(deps)
    const plan: RunPlan = { workspaceRoot: '/repo', steps: [{ id: 's1' } as any] }

    deps.executeStep.mockImplementation(async () => {
      return new Promise(r => setTimeout(() => r({ kind: 'success', exitCode: 0, durationMs: 500 }), 500))
    })

    await mgr.submit(plan)
    
    await expect(mgr.submit(plan)).rejects.toThrow(/Worktree is busy/)
  })

  it('handles queue_full', async () => {
    const mgr = createRunManager(deps)
    
    deps.executeStep.mockImplementation(async () => {
      return new Promise(r => setTimeout(() => r({ kind: 'success', exitCode: 0, durationMs: 500 }), 500))
    })

    await mgr.submit({ workspaceRoot: '/repo1', steps: [] })
    await mgr.submit({ workspaceRoot: '/repo2', steps: [] })
    await mgr.submit({ workspaceRoot: '/repo3', steps: [] })
    await mgr.submit({ workspaceRoot: '/repo4', steps: [] })
    
    await expect(mgr.submit({ workspaceRoot: '/repo5', steps: [] })).rejects.toThrow(/Queue is full/)
  })

  it('cancels queued and running jobs', async () => {
    const mgr = createRunManager(deps)
    const plan1: RunPlan = { workspaceRoot: '/repo1', steps: [{ id: 's1' } as any] }
    const plan2: RunPlan = { workspaceRoot: '/repo2', steps: [{ id: 's1' } as any] }

    deps.executeStep.mockImplementation(async (step: any, ctx: any) => {
      return new Promise(r => {
        ctx.signal.addEventListener('abort', () => r({ kind: 'cancelled' }))
      })
    })

    const r1 = await mgr.submit(plan1)
    const r2 = await mgr.submit(plan2)
    
    await vi.advanceTimersByTimeAsync(1) // start 1

    expect(mgr.getStatus('/repo1', r1.runId)?.state).toBe('running')
    expect(mgr.getStatus('/repo2', r2.runId)?.state).toBe('queued')

    // Cancel queued
    mgr.cancel('/repo2', r2.runId)
    expect(mgr.getStatus('/repo2', r2.runId)).toBeNull() // removed

    // Cancel running
    mgr.cancel('/repo1', r1.runId)
    expect(mgr.getStatus('/repo1', r1.runId)?.state).toBe('cancelling')
    
    await vi.advanceTimersByTimeAsync(1) // allow abort to process
    await vi.advanceTimersByTimeAsync(1)
    
    expect(mgr.getStatus('/repo1', r1.runId)).toBeNull() // completed as cancelled
  })

  it('handles timeout', async () => {
    const mgr = createRunManager(deps)
    const plan1: RunPlan = { workspaceRoot: '/repo1', steps: [{ id: 's1' } as any] }

    deps.executeStep.mockImplementation(async (step: any, ctx: any) => {
      return new Promise(r => {
        ctx.signal.addEventListener('abort', () => r({ kind: 'cancelled' }))
      })
    })

    const { runId } = await mgr.submit(plan1)
    await vi.advanceTimersByTimeAsync(1)

    expect(mgr.getStatus('/repo1', runId)?.state).toBe('running')

    // advance beyond ORCA_QUALITY_RUN_TIMEOUT_MS
    await vi.advanceTimersByTimeAsync(2700000 + 100)
    await vi.advanceTimersByTimeAsync(1)
    await vi.advanceTimersByTimeAsync(1)

    expect(mgr.getStatus('/repo1', runId)).toBeNull() // completed as timeout
  })
})
