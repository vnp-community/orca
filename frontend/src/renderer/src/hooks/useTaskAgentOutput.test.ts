// @vitest-environment happy-dom
import { act, renderHook, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useTaskAgentOutput } from './useTaskAgentOutput'
import { resetTaskAgentOutputStoreForTests } from './task-agent-output-store'

vi.mock('../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

type SubscribeOptions = { onClose?: () => void }
const subscribeMock = vi.hoisted(() => vi.fn())
const targetMock = vi.hoisted(() => vi.fn())
vi.mock('../runtime/runtime-rpc-client', () => ({
  getActiveRuntimeTarget: targetMock,
  subscribeRuntimeStreamChannel: subscribeMock
}))

type Sub = {
  onEvent: (e: unknown) => void
  options: SubscribeOptions
  unsubscribe: ReturnType<typeof vi.fn>
}
let subs: Sub[] = []

function acceptSubscriptions(): void {
  subscribeMock.mockImplementation(
    async (
      _t: unknown,
      _m: string,
      _p: unknown,
      onEvent: (e: unknown) => void,
      options: SubscribeOptions
    ) => {
      const sub: Sub = { onEvent, options, unsubscribe: vi.fn() }
      subs.push(sub)
      return { ack: null, unsubscribe: sub.unsubscribe }
    }
  )
}

const frame = (stdout: string, extra: Record<string, unknown> = {}, stderr = ''): unknown => ({
  taskId: 't1',
  engine: 'direct_agent',
  eventType: 'agent_output_partial',
  payload: { origin_task_id: 't1', request_id: 'req:r:t1:1', stdout, stderr },
  ...extra
})

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  subs = []
  subscribeMock.mockReset()
  targetMock.mockReset()
  targetMock.mockReturnValue({ kind: 'environment', environmentId: 'env-1' })
  vi.stubGlobal('api', {})
  acceptSubscriptions()
})

afterEach(() => {
  cleanup()
  resetTaskAgentOutputStoreForTests()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('useTaskAgentOutput', () => {
  it('subscribes to task.activity.subscribe for the task and goes live on the ack', async () => {
    const { result } = renderHook(() => useTaskAgentOutput('t1'))
    expect(result.current.status).toBe('connecting')
    await flush()
    expect(subscribeMock).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'task.activity.subscribe',
      { taskId: 't1' },
      expect.any(Function),
      expect.objectContaining({ onClose: expect.any(Function) })
    )
    expect(result.current.status).toBe('live')
    expect(result.current.lines).toEqual([])
  })

  it('replaces the lines with each cumulative frame and keeps the run id', async () => {
    const { result } = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    act(() => subs[0]!.onEvent(frame('▸ Read a.py\n')))
    expect(result.current.lines.map((l) => l.text)).toEqual(['Read a.py'])
    act(() => subs[0]!.onEvent(frame('▸ Read a.py\n↳ ok · 2 lines\n■ Done\n', {}, 'warn')))
    expect(result.current.lines.map((l) => [l.kind, l.text])).toEqual([
      ['tool', 'Read a.py'],
      ['result', 'ok · 2 lines'],
      ['end', 'Done'],
      ['stderr', 'warn']
    ])
    expect(result.current.runId).toBe('req:r:t1:1')
    expect(result.current.lastFrameAt).not.toBeNull()
  })

  it('ignores other event types and malformed frames, and unwraps an args envelope', async () => {
    const { result } = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    act(() => {
      subs[0]!.onEvent(frame('» no', { eventType: 'status_changed' }))
      subs[0]!.onEvent(null)
      subs[0]!.onEvent('text')
      subs[0]!.onEvent({ eventType: 'agent_output_partial' })
    })
    expect(result.current.lines).toEqual([])
    act(() => subs[0]!.onEvent({ args: [frame('» wrapped')] }))
    expect(result.current.lines.map((l) => l.text)).toEqual(['wrapped'])
  })

  it('is unavailable on the desktop local target and never subscribes', async () => {
    targetMock.mockReturnValue({ kind: 'local' })
    const { result } = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    expect(result.current.status).toBe('unavailable')
    expect(subscribeMock).not.toHaveBeenCalled()
  })

  it('is unavailable, without retrying, when the gateway has no such channel', async () => {
    subscribeMock.mockRejectedValue(Object.assign(new Error('nope'), { code: 'method_not_found' }))
    const { result } = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    expect(result.current.status).toBe('unavailable')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(subscribeMock).toHaveBeenCalledTimes(1)
  })

  it('reconnects with backoff after the stream closes and keeps the lines already shown', async () => {
    const { result } = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    act(() => subs[0]!.onEvent(frame('» before the drop\n')))
    act(() => subs[0]!.options.onClose?.())
    expect(result.current.status).toBe('reconnecting')
    expect(result.current.lines).toHaveLength(1)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000)
    })
    expect(subs).toHaveLength(2)
    expect(result.current.status).toBe('live')
    act(() => subs[1]!.onEvent(frame('» before the drop\n» after\n')))
    expect(result.current.lines.map((l) => l.text)).toEqual(['before the drop', 'after'])
  })

  it('doubles the delay while attempts keep failing, then gives up', async () => {
    subscribeMock.mockRejectedValue(new Error('network'))
    const { result } = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    expect(result.current.status).toBe('reconnecting')
    expect(subscribeMock).toHaveBeenCalledTimes(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(999)
    })
    expect(subscribeMock).toHaveBeenCalledTimes(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(subscribeMock).toHaveBeenCalledTimes(2)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000)
    })
    expect(subscribeMock).toHaveBeenCalledTimes(3)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10 * 60_000)
    })
    expect(result.current.status).toBe('unavailable')
    expect(subscribeMock.mock.calls.length).toBeLessThanOrEqual(10)
  })

  it('keeps the stream after the view closes and replays the last output when it is opened again', async () => {
    const first = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    act(() => subs[0]!.onEvent(frame('» still running\n')))
    first.unmount()
    expect(subs[0]!.unsubscribe).not.toHaveBeenCalled()

    // A frame that arrives while no view is open is kept too.
    subs[0]!.onEvent(frame('» still running\n» while closed\n'))
    const second = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    expect(subscribeMock).toHaveBeenCalledTimes(1)
    expect(second.result.current.status).toBe('live')
    expect(second.result.current.lines.map((l) => l.text)).toEqual([
      'still running',
      'while closed'
    ])
  })

  it('shares one stream between two views of the same task', async () => {
    const a = renderHook(() => useTaskAgentOutput('t1'))
    const b = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    expect(subscribeMock).toHaveBeenCalledTimes(1)
    act(() => subs[0]!.onEvent(frame('» both\n')))
    expect(a.result.current.lines).toHaveLength(1)
    expect(b.result.current.lines).toHaveLength(1)
  })

  it('closes the oldest idle streams beyond the cap', async () => {
    for (let i = 0; i < 8; i++) {
      const view = renderHook(() => useTaskAgentOutput(`task-${i}`))
      await flush()
      view.unmount()
      vi.advanceTimersByTime(10)
    }
    expect(subs).toHaveLength(8)
    expect(subs.filter((s) => s.unsubscribe.mock.calls.length > 0)).toHaveLength(2)
    expect(subs[0]!.unsubscribe).toHaveBeenCalled()
    expect(subs[7]!.unsubscribe).not.toHaveBeenCalled()
  })

  it('does not reconnect a stream that closed while nobody was looking', async () => {
    const view = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    view.unmount()
    act(() => subs[0]!.options.onClose?.())
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(subscribeMock).toHaveBeenCalledTimes(1)
    const again = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    expect(subscribeMock).toHaveBeenCalledTimes(2)
    again.unmount()
  })

  it('stops a pending reconnect when the last view goes away', async () => {
    const view = renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    act(() => subs[0]!.options.onClose?.())
    view.unmount()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(subs).toHaveLength(1)
  })

  it('does not open a second stream when a view remounts before the first ack (React strict mode)', async () => {
    const first = renderHook(() => useTaskAgentOutput('t1'))
    first.unmount()
    renderHook(() => useTaskAgentOutput('t1'))
    await flush()
    expect(subscribeMock).toHaveBeenCalledTimes(1)
    expect(subs[0]!.unsubscribe).not.toHaveBeenCalled()
  })

  it('drops a subscription whose ack arrives after the stream was discarded', async () => {
    let release: (v: { ack: null; unsubscribe: () => void }) => void = () => {}
    const unsubscribe = vi.fn()
    subscribeMock.mockImplementation(() => new Promise((resolve) => (release = resolve)))
    renderHook(() => useTaskAgentOutput('t1'))
    resetTaskAgentOutputStoreForTests()
    await act(async () => {
      release({ ack: null, unsubscribe })
      await Promise.resolve()
    })
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('starts clean when the task changes', async () => {
    const { result, rerender } = renderHook(({ id }) => useTaskAgentOutput(id), {
      initialProps: { id: 't1' }
    })
    await flush()
    act(() => subs[0]!.onEvent(frame('» old task\n')))
    rerender({ id: 't2' })
    await flush()
    expect(result.current.lines).toEqual([])
    expect(subscribeMock).toHaveBeenLastCalledWith(
      expect.anything(),
      'task.activity.subscribe',
      { taskId: 't2' },
      expect.any(Function),
      expect.anything()
    )
  })

  it('does nothing without a task id', async () => {
    const { result } = renderHook(() => useTaskAgentOutput(null))
    await flush()
    expect(subscribeMock).not.toHaveBeenCalled()
    expect(result.current.lines).toEqual([])
  })
})
