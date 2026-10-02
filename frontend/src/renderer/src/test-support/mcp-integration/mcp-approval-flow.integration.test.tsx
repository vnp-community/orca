// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen } from '@testing-library/react'
import { useAppStore } from '@/store'
import { resetMcpStreamStateForTests } from '@/store/slices/mcp-slice'
import { useMcpSync } from '@/hooks/useMcpSync'
import McpGlobalLayer from '@/components/mcp/McpGlobalLayer'
import { createFakeMcpBackend, type FakeMcpBackend } from '../mcp-fake-backend'
import { makeMcpApproval } from '../mcp-fixtures'

vi.mock('@/store', async () => ({
  useAppStore: (await import('../mcp-test-store')).createMcpTestStore()
}))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), info: vi.fn() }) }))

function App(): React.JSX.Element {
  useMcpSync()
  return <McpGlobalLayer />
}

let backend: FakeMcpBackend
let uninstall: () => void

const advance = (ms: number): Promise<void> =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
// The prompt ignores synthetic clicks: only isTrusted user clicks can decide.
const trustedClick = (el: HTMLElement): void => {
  const ev = new MouseEvent('click', { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'isTrusted', { value: true })
  act(() => void el.dispatchEvent(ev))
}
const btn = (re: RegExp): HTMLButtonElement => screen.getByRole('button', { name: re })
const queue = (): string[] => useAppStore.getState().mcpApprovalQueue.map((a) => a.id)

async function mountAndRequest(over = {}) {
  render(<App />)
  await advance(0)
  let call!: ReturnType<FakeMcpBackend['requestToolCall']>
  act(() => {
    call = backend.requestToolCall(makeMcpApproval(over))
  })
  await advance(0)
  return call
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  resetMcpStreamStateForTests()
  backend = createFakeMcpBackend({ role: 'user' })
  uninstall = backend.install()
  useAppStore.getState().resetMcp()
  useAppStore.getState().clearMcpApprovals()
  useAppStore.setState({ currentUser: { id: 'u1', role: 'user' } } as never)
})
afterEach(() => {
  cleanup()
  resetMcpStreamStateForTests()
  uninstall()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('approval request -> prompt -> decide -> resolved (fake backend)', () => {
  it('shows the exact args, locks Approve, then sends the server hash and releases the agent', async () => {
    const call = await mountAndRequest()
    expect(backend.streamCount()).toBe(1)
    expect(screen.getByLabelText('Exact tool arguments').textContent).toContain('ls -la /tmp')
    expect(btn(/^Approve /).disabled).toBe(true)
    trustedClick(btn(/^Approve /))
    expect(backend.calls.some((c) => c.method === 'mcp.approval.decide')).toBe(false)

    await advance(2100)
    trustedClick(btn(/^Approve /))
    await advance(0)
    expect(backend.calls.find((c) => c.method === 'mcp.approval.decide')?.params).toEqual({
      approvalId: 'ap-1',
      decision: 'approve',
      paramsHash: 'hash-ap-1'
    })
    await expect(call.outcome).resolves.toEqual({ isError: false })
    expect(queue()).toEqual([])
    expect(screen.queryByLabelText('Exact tool arguments')).toBeNull()
  })

  it('deny makes the pending tools/call fail with isError', async () => {
    const call = await mountAndRequest()
    trustedClick(btn(/^Deny /))
    await advance(0)
    await expect(call.outcome).resolves.toEqual({ isError: true })
    expect(queue()).toEqual([])
  })

  it('never decides on its own', async () => {
    await mountAndRequest()
    await advance(60_000)
    expect(backend.calls.some((c) => c.method === 'mcp.approval.decide')).toBe(false)
  })

  it('MCP_APPROVAL_HASH_MISMATCH re-syncs the server version and needs a fresh click', async () => {
    const call = await mountAndRequest()
    backend.mutateApprovalHash('ap-1', 'hash-v2')
    await advance(2100)
    trustedClick(btn(/^Approve /))
    await advance(0)
    await advance(0)
    expect(screen.getByText('This request changed. Review it again.')).toBeTruthy()
    const decides = (): unknown[] =>
      backend.calls.filter((c) => c.method === 'mcp.approval.decide').map((c) => c.params)
    expect(decides()).toHaveLength(1)
    expect(useAppStore.getState().mcpApprovalQueue[0].paramsHash).toBe('hash-v2')
    await advance(2100)
    expect(decides()).toHaveLength(1)
    trustedClick(btn(/^Approve /))
    await advance(0)
    expect(decides()[1]).toMatchObject({ paramsHash: 'hash-v2' })
    await expect(call.outcome).resolves.toEqual({ isError: false })
  })

  it('removes the prompt when another device resolves the approval', async () => {
    await mountAndRequest()
    act(() => backend.emit({ type: 'approval.resolved', id: 'ap-1', status: 'approved' }))
    expect(queue()).toEqual([])
  })

  it('re-pulls pending approvals after the event stream reconnects', async () => {
    render(<App />)
    await advance(0)
    expect(backend.streamCount()).toBe(1)
    act(() => backend.closeStreams())
    // Created while the stream was down: only the re-list can find it.
    backend.requestToolCall(makeMcpApproval({ id: 'ap-missed', paramsHash: 'h-missed' }))
    expect(queue()).toEqual([])
    await advance(1100)
    expect(backend.streamCount()).toBe(1)
    expect(queue()).toEqual(['ap-missed'])
  })

  it('stays inert for a disabled tenant: no stream, no approval calls', async () => {
    backend.setInfo({ enabled: false })
    render(<App />)
    await advance(0)
    expect(backend.streamCount()).toBe(0)
    expect(backend.calls.map((c) => c.method)).not.toContain('mcp.approval.list')
    expect(useAppStore.getState().mcpApprovalQueue).toEqual([])
  })
})
