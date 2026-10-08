import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  BACKOFF_INITIAL_MS,
  BACKOFF_MAX_MS,
  HEALTHY_DURATION_MS,
  resetCodeIntelStreams,
  retainCodeIntelStream
} from './code-intel-stream-reconnect'
import type { CodeIntelStreamDeps } from './code-intel-stream-reconnect'
import type { CodeIntelSubscribeCallbacks } from '../../../../shared/code-intel-bridge'
import { publishCodeIntelEvent, resetCodeIntelEventBus, subscribeCodeIntelEvents } from '../../lib/code-intel-event-bus'

function makeDeps() {
  const opened: CodeIntelSubscribeCallbacks[] = []
  const unsubs: ReturnType<typeof vi.fn>[] = []
  const deps = {
    subscribe: vi.fn((_env: string | null, cb: CodeIntelSubscribeCallbacks) => {
      opened.push(cb)
      const u = vi.fn()
      unsubs.push(u)
      return u
    }),
    invalidateWorktree: vi.fn(),
    triggerResync: vi.fn(),
    setEventsState: vi.fn()
  } satisfies CodeIntelStreamDeps
  return { deps, opened, unsubs }
}

beforeEach(() => {
  vi.useFakeTimers()
  resetCodeIntelStreams()
  resetCodeIntelEventBus()
})
afterEach(() => {
  resetCodeIntelStreams()
  vi.useRealTimers()
})

describe('retainCodeIntelStream ref-count', () => {
  it('opens one stream per environment and closes it when the last ref releases', () => {
    const { deps, unsubs } = makeDeps()
    const a = retainCodeIntelStream('env-1', deps)
    const b = retainCodeIntelStream('env-1', deps)
    expect(deps.subscribe).toHaveBeenCalledTimes(1)
    a()
    expect(unsubs[0]).not.toHaveBeenCalled()
    b()
    expect(unsubs[0]).toHaveBeenCalledTimes(1)
    expect(deps.setEventsState).toHaveBeenLastCalledWith('idle')
  })

  it('release is idempotent', () => {
    const { deps, unsubs } = makeDeps()
    const a = retainCodeIntelStream('env-1', deps)
    const b = retainCodeIntelStream('env-1', deps)
    a()
    a()
    expect(unsubs[0]).not.toHaveBeenCalled()
    b()
    expect(unsubs[0]).toHaveBeenCalledTimes(1)
  })

  it('separate environments get separate streams', () => {
    const { deps } = makeDeps()
    retainCodeIntelStream('env-1', deps)
    retainCodeIntelStream('env-2', deps)
    expect(deps.subscribe).toHaveBeenCalledTimes(2)
  })
})

describe('events', () => {
  it('plain changed invalidates the cache but does not resync', () => {
    const { deps, opened } = makeDeps()
    retainCodeIntelStream('env-1', deps)
    opened[0].onEvent({ event: 'changed', worktreeId: 'wt-1', reason: 'commit', resync: false })
    expect(deps.invalidateWorktree).toHaveBeenCalledWith('wt-1')
    expect(deps.triggerResync).not.toHaveBeenCalled()
  })

  it('changed{resync:true} also bumps the resync counter', () => {
    const { deps, opened } = makeDeps()
    retainCodeIntelStream('env-1', deps)
    opened[0].onEvent({ event: 'changed', worktreeId: 'wt-1', reason: 'unknown', resync: true })
    expect(deps.triggerResync).toHaveBeenCalledTimes(1)
  })

  it('publishes to the bus; reindexProgress keeps percent null; quality events only hit the bus', () => {
    const { deps, opened } = makeDeps()
    const seen: unknown[] = []
    subscribeCodeIntelEvents((e) => seen.push(e))
    retainCodeIntelStream('env-1', deps)
    opened[0].onEvent({ event: 'reindexProgress', worktreeId: 'w', percent: null, running: true })
    opened[0].onEvent({ event: 'qualityProgress', worktreeId: 'w', runId: 'r', percent: null, phase: 'collect' })
    expect(seen[0]).toMatchObject({ event: 'reindexProgress', percent: null })
    expect(seen[1]).toMatchObject({ event: 'qualityProgress', runId: 'r' })
    expect(deps.invalidateWorktree).not.toHaveBeenCalled()
  })

  it('one failing bus listener does not stop others', () => {
    const good = vi.fn()
    subscribeCodeIntelEvents(() => {
      throw new Error('bad listener')
    })
    subscribeCodeIntelEvents(good)
    publishCodeIntelEvent({ event: 'gateChanged', worktreeId: 'w' })
    expect(good).toHaveBeenCalledTimes(1)
  })
})

describe('reconnect backoff', () => {
  it('reconnects after 1s, then 2s, 4s ... capped at 30s, and resyncs once per reconnect', () => {
    const { deps, opened } = makeDeps()
    retainCodeIntelStream('env-1', deps)

    opened[0].onClose()
    vi.advanceTimersByTime(BACKOFF_INITIAL_MS - 1)
    expect(deps.subscribe).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(1)
    expect(deps.subscribe).toHaveBeenCalledTimes(2)
    expect(deps.triggerResync).toHaveBeenCalledTimes(1)

    opened[1].onClose()
    vi.advanceTimersByTime(2_000)
    expect(deps.subscribe).toHaveBeenCalledTimes(3)

    let delay = 4_000
    for (let i = 2; i < 10; i++) {
      opened[i].onClose()
      vi.advanceTimersByTime(Math.min(delay, BACKOFF_MAX_MS))
      delay *= 2
    }
    // After many failures the wait is capped at BACKOFF_MAX_MS
    const before = deps.subscribe.mock.calls.length
    opened.at(-1)!.onClose()
    vi.advanceTimersByTime(BACKOFF_MAX_MS - 1)
    expect(deps.subscribe).toHaveBeenCalledTimes(before)
    vi.advanceTimersByTime(1)
    expect(deps.subscribe).toHaveBeenCalledTimes(before + 1)
  })

  it('resets the backoff after 30s of healthy connection', () => {
    const { deps, opened } = makeDeps()
    retainCodeIntelStream('env-1', deps)
    opened[0].onClose()
    vi.advanceTimersByTime(1_000) // reconnect #1, next backoff is 2s
    vi.advanceTimersByTime(HEALTHY_DURATION_MS) // healthy -> reset to 1s
    opened[1].onClose()
    vi.advanceTimersByTime(BACKOFF_INITIAL_MS)
    expect(deps.subscribe).toHaveBeenCalledTimes(3)
  })

  it('does not reconnect after release and leaves no pending timers', () => {
    const { deps, opened } = makeDeps()
    const release = retainCodeIntelStream('env-1', deps)
    opened[0].onClose()
    release()
    expect(vi.getTimerCount()).toBe(0)
    vi.advanceTimersByTime(60_000)
    expect(deps.subscribe).toHaveBeenCalledTimes(1)
  })

  it('reset clears timers', () => {
    const { deps, opened } = makeDeps()
    retainCodeIntelStream('env-1', deps)
    opened[0].onClose()
    resetCodeIntelStreams()
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('unsupported transport', () => {
  it('local target (null) falls back to polling and keeps no timers', () => {
    const { deps, opened } = makeDeps()
    retainCodeIntelStream(null, deps)
    opened[0].onUnsupported()
    expect(deps.setEventsState).toHaveBeenLastCalledWith('polling')
    expect(vi.getTimerCount()).toBe(0)
  })
})
