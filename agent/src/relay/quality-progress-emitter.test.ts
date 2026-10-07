import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createProgressEmitter } from './quality-progress-emitter'

describe('quality-progress-emitter', () => {
  let deps: any

  beforeEach(() => {
    vi.useFakeTimers()
    deps = {
      emit: vi.fn(),
      now: vi.fn().mockReturnValue(1000),
      minIntervalMs: 1000,
      redact: (s: string) => s
    }
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('emits on stage change immediately and throttles same stage', () => {
    const em = createProgressEmitter(deps)
    const run = { runId: 'r1', workspaceRoot: '/repo', plan: { steps: [{}] }, journal: { steps: [] } } as any

    em.onStage(run, 'step:a', 1)
    expect(deps.emit).toHaveBeenCalledTimes(1)

    // same stage, no time passed -> throttled
    em.onStage(run, 'step:a', 1)
    expect(deps.emit).toHaveBeenCalledTimes(1)

    // different stage -> emit
    em.onStage(run, 'step:b', 2)
    expect(deps.emit).toHaveBeenCalledTimes(2)
  })

  it('percent is null when 0 steps', () => {
    const em = createProgressEmitter(deps)
    const run = { runId: 'r1', workspaceRoot: '/repo', plan: { steps: [] }, journal: { steps: [] } } as any

    em.onStage(run, 'step:a', 1)
    expect(deps.emit).toHaveBeenCalledWith('quality.progress', expect.objectContaining({ percent: null }))
  })

  it('percent is calculated correctly', () => {
    const em = createProgressEmitter(deps)
    const run = { runId: 'r1', workspaceRoot: '/repo', plan: { steps: [{}, {}, {}, {}] }, journal: { steps: [{ state: 'passed' }] } } as any

    em.onStage(run, 'step:a', 1)
    expect(deps.emit).toHaveBeenCalledWith('quality.progress', expect.objectContaining({ percent: 25 }))
  })

  it('emitFinished once', () => {
    const em = createProgressEmitter(deps)
    const run = { runId: 'r1', workspaceRoot: '/repo', state: 'completed', startedAt: 1, journal: { finishedAt: 2, steps: [{ id: 's1', state: 'passed' }] } } as any

    em.emitFinished(run)
    expect(deps.emit).toHaveBeenCalledWith('quality.finished', expect.objectContaining({
      status: 'completed',
      summary: 's1=passed'
    }))
  })
})
