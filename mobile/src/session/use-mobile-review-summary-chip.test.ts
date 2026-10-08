// @vitest-environment happy-dom
import { createElement, act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ConnectionState, RpcResponse } from '../transport/types'
import { useMobileReviewSummaryChip } from './use-mobile-review-summary-chip'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

type Input = Parameters<typeof useMobileReviewSummaryChip>[0]
type Result = ReturnType<typeof useMobileReviewSummaryChip>

const ok = (result: unknown): RpcResponse => ({
  id: 'x',
  ok: true,
  result,
  _meta: { runtimeId: 'r' }
})
const READY = {
  available: true,
  risk: { level: 'HIGH', reasons: [] },
  findings: {
    totalOpen: 2,
    bySeverity: { error: 0, warning: 2, info: 0 },
    items: [],
    truncated: false
  }
}

const roots: { unmount: () => void }[] = []

async function mount(input: Input) {
  const ref: { current: Result | undefined } = { current: undefined }
  function Probe({ value }: { value: Input }) {
    ref.current = useMobileReviewSummaryChip(value)
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

function base(
  result: unknown = READY,
  over: Partial<Input> = {}
): Input & { send: ReturnType<typeof vi.fn> } {
  const send = vi.fn(async () => ok(result))
  return {
    client: { sendRequest: send } as never,
    connState: 'connected',
    worktreeId: 'w1',
    enabled: true,
    send,
    ...over
  }
}

afterEach(async () => {
  await act(async () => roots.splice(0).forEach((r) => r.unmount()))
})

describe('useMobileReviewSummaryChip', () => {
  it('probes once and exposes a chip when the summary is available', async () => {
    const input = base()
    const { ref, rerender } = await mount(input)
    expect(ref.current).toMatchObject({ label: 'Review summary', tone: 'danger' })
    await rerender({ ...input })
    expect(input.send).toHaveBeenCalledTimes(1)
    expect(input.send).toHaveBeenCalledWith('codeIntel.reviewSummary', { worktree: 'id:w1' })
  })

  it('hides the chip when unavailable and does not probe while disabled or disconnected', async () => {
    const unavailable = base({ available: false, reason: 'flag_off' })
    expect((await mount(unavailable)).ref.current).toBeNull()
    const disabled = base(READY, { enabled: false })
    expect((await mount(disabled)).ref.current).toBeNull()
    const offline = base(READY, { connState: 'disconnected' as ConnectionState })
    expect((await mount(offline)).ref.current).toBeNull()
    expect(disabled.send).not.toHaveBeenCalled()
    expect(offline.send).not.toHaveBeenCalled()
  })

  it('re-probes after reconnecting and ignores a response for a previous worktree', async () => {
    const resolvers: ((r: RpcResponse) => void)[] = []
    const send = vi.fn(() => new Promise<RpcResponse>((res) => resolvers.push(res)))
    const input = base(READY, { client: { sendRequest: send } as never })
    const { ref, rerender } = await mount(input)
    await rerender({ ...input, worktreeId: 'w2' })
    expect(send).toHaveBeenCalledTimes(2)
    await act(async () => resolvers[0]!(ok(READY)))
    expect(ref.current).toBeNull()
    await act(async () =>
      resolvers[1]!(ok({ ...READY, risk: { level: 'LOW', reasons: [] }, findings: undefined }))
    )
    expect(ref.current?.tone).toBe('neutral')
    await rerender({ ...input, worktreeId: 'w2', connState: 'disconnected' })
    await rerender({ ...input, worktreeId: 'w2', connState: 'connected' })
    expect(send).toHaveBeenCalledTimes(3)
  })
})
