// @vitest-environment happy-dom
import { createElement, act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ConnectionState, RpcResponse } from '../transport/types'
import { useMobileReviewSummaryController } from './use-mobile-review-summary-controller'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

type Input = Parameters<typeof useMobileReviewSummaryController>[0]
type Result = ReturnType<typeof useMobileReviewSummaryController>

function ok(result: unknown): RpcResponse {
  return { id: 'x', ok: true, result, _meta: { runtimeId: 'r' } }
}

const finding = (over: Record<string, unknown> = {}) => ({
  key: 'k1',
  kind: 'quality',
  severity: 'warning',
  title: 't',
  summary: 's',
  origin: 'quality',
  filePath: 'src/a.ts',
  startLine: 3,
  inChangedFiles: true,
  ...over
})

const roots: { unmount: () => void }[] = []

async function mount(input: Input) {
  const ref: { current: Result | null } = { current: null }
  function Probe({ value }: { value: Input }) {
    ref.current = useMobileReviewSummaryController(value)
    return null
  }
  const root = createRoot(document.createElement('div'))
  roots.push(root)
  await act(async () => root.render(createElement(Probe, { value: input })))
  return {
    ref,
    rerender: (next: Input) => act(async () => root.render(createElement(Probe, { value: next })))
  }
}

function base(over: Partial<Input> = {}): Input {
  return {
    client: { sendRequest: vi.fn(async () => ok({ available: true })) } as never,
    connState: 'connected',
    hostId: 'h1',
    worktreeId: 'repo::/tmp/wt',
    name: 'wt',
    onNavigate: vi.fn(),
    onReconnect: vi.fn(),
    ...over
  }
}

afterEach(async () => {
  await act(async () => roots.splice(0).forEach((r) => r.unmount()))
})

describe('useMobileReviewSummaryController', () => {
  it('loads once on entry (no polling) and exposes ready state', async () => {
    const input = base()
    const { ref } = await mount(input)
    expect(ref.current?.screenState.kind).toBe('ready')
    expect(
      (input.client as never as { sendRequest: ReturnType<typeof vi.fn> }).sendRequest
    ).toHaveBeenCalledTimes(1)
  })

  it('waits for desktop when not connected and does not request', async () => {
    const input = base({ connState: 'connecting' as ConnectionState })
    const { ref } = await mount(input)
    expect(ref.current?.screenState).toEqual({ kind: 'error', message: 'Waiting for desktop...' })
    expect(
      (input.client as never as { sendRequest: ReturnType<typeof vi.fn> }).sendRequest
    ).not.toHaveBeenCalled()
  })

  it('calls onReconnect automatically once when the link drops', async () => {
    const input = base()
    const { rerender } = await mount(input)
    expect(input.onReconnect).not.toHaveBeenCalled()
    await rerender({ ...input, connState: 'disconnected' })
    expect(input.onReconnect).toHaveBeenCalledTimes(1)
    expect(input.onReconnect).toHaveBeenCalledWith('h1')
    await rerender({ ...input, connState: 'reconnecting' })
    expect(input.onReconnect).toHaveBeenCalledTimes(1)
  })

  it('manual reconnect delegates to onReconnect with the host id', async () => {
    const input = base()
    const { ref } = await mount(input)
    await act(async () => void ref.current?.reconnect())
    expect(input.onReconnect).toHaveBeenCalledWith('h1')
  })

  it('refresh reloads and toggles refreshing back to false', async () => {
    const input = base()
    const { ref } = await mount(input)
    await act(async () => ref.current?.refresh())
    expect(ref.current?.refreshing).toBe(false)
    expect(
      (input.client as never as { sendRequest: ReturnType<typeof vi.fn> }).sendRequest
    ).toHaveBeenCalledTimes(2)
  })

  it('stale response from an older request never overwrites a newer one', async () => {
    const resolvers: ((r: RpcResponse) => void)[] = []
    const sendRequest = vi.fn(() => new Promise<RpcResponse>((res) => resolvers.push(res)))
    const input = base({ client: { sendRequest } as never })
    const { ref } = await mount(input)
    let refreshing: Promise<void> | undefined
    await act(async () => {
      refreshing = ref.current?.refresh()
    })
    expect(resolvers).toHaveLength(2)
    await act(async () => {
      resolvers[1]!(ok({ available: false, reason: 'flag_off' }))
      await refreshing
    })
    expect(ref.current?.screenState.kind).toBe('unavailable')
    await act(async () => resolvers[0]!(ok({ available: true })))
    expect(ref.current?.screenState.kind).toBe('unavailable')
  })

  it('toggleExpanded opens then closes the same key', async () => {
    const { ref } = await mount(base())
    await act(async () => ref.current?.toggleExpanded('a'))
    expect(ref.current?.expandedKey).toBe('a')
    await act(async () => ref.current?.toggleExpanded('a'))
    expect(ref.current?.expandedKey).toBeNull()
  })

  it('openFileDiff navigates with the branch area only for changed files', async () => {
    const input = base()
    const { ref } = await mount(input)
    await act(async () => ref.current?.openFileDiff(finding({ inChangedFiles: false }) as never))
    expect(input.onNavigate).not.toHaveBeenCalled()
    await act(async () => ref.current?.openFileDiff(finding() as never))
    expect(input.onNavigate).toHaveBeenCalledTimes(1)
    const route = String((input.onNavigate as ReturnType<typeof vi.fn>).mock.calls[0]![0])
    expect(route).toContain('branch')
    expect(route).toContain(encodeURIComponent('src/a.ts'))
  })
})
