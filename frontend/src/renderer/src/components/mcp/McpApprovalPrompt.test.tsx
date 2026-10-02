// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { McpApproval } from '../../../../shared/mcp-types'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { useAppStore } from '@/store'
import { McpApprovalPrompt } from './McpApprovalPrompt'

const call = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), info: vi.fn() }) }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  const { createMcpApprovalSlice } = await import('@/store/slices/mcp-approval-slice')
  return {
    useAppStore: create((...a: unknown[]) => ({
      ...(createMcpApprovalSlice as (...x: unknown[]) => object)(...a),
      mcpServerInfo: { killSwitch: { active: false } },
      refreshMcpServerInfo: vi.fn()
    }))
  }
})

const ap = (id: string, over: Partial<McpApproval> = {}): McpApproval => ({
  id,
  createdAt: new Date().toISOString(),
  expiresAt: new Date(Date.now() + 120_000).toISOString(),
  status: 'pending',
  tool: { name: 'terminal_send', title: 'Send to terminal', risk: 'exec' },
  clientName: 'Claude',
  sessionId: 'session-abcdef',
  argsPreview: { text: 'rm -rf /tmp/x', redacted: false },
  paramsHash: `hash-${id}`,
  ...over
})

const trustedClick = (el: HTMLElement): void => {
  const ev = new MouseEvent('click', { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'isTrusted', { value: true })
  act(() => {
    el.dispatchEvent(ev)
  })
}
const approveBtn = (): HTMLButtonElement =>
  screen.getByRole('button', { name: /^Approve / }) as HTMLButtonElement
const queue = (): string[] => useAppStore.getState().mcpApprovalQueue.map((q) => q.id)
const advance = async (ms: number): Promise<void> => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  call.mockReset()
  call.mockResolvedValue({ approvals: [] })
  useAppStore.getState().clearMcpApprovals()
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

const open = (...list: McpApproval[]): void => {
  act(() => {
    for (const a of list) {
      useAppStore.getState().enqueueMcpApproval(a, Date.now())
    }
  })
}

describe('McpApprovalPrompt', () => {
  it('never approves by itself, however long it stays open', async () => {
    open(ap('a'))
    render(<McpApprovalPrompt />)
    await advance(60_000)
    expect(call).not.toHaveBeenCalledWith('mcp.approval.decide', expect.anything())
  })

  it('starts focus on Deny and Enter does not decide', async () => {
    open(ap('a'))
    render(<McpApprovalPrompt />)
    await advance(10)
    expect(document.activeElement).toBe(screen.getByRole('button', { name: /^Deny / }))
    fireEvent.keyDown(document.activeElement as Element, { key: 'Enter' })
    expect(call).not.toHaveBeenCalledWith('mcp.approval.decide', expect.anything())
  })

  it('locks Approve for the risk delay and restarts it when the window regains focus', async () => {
    open(ap('a'))
    render(<McpApprovalPrompt />)
    expect(approveBtn().disabled).toBe(true)
    expect(approveBtn().textContent).toContain('Approve (2s)')
    await advance(1000)
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    await advance(1000)
    expect(approveBtn().disabled).toBe(true)
    await advance(700)
    expect(approveBtn().disabled).toBe(false)
  })

  it('does not run the lock while the window is unfocused', async () => {
    vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    open(ap('a'))
    render(<McpApprovalPrompt />)
    await advance(5000)
    expect(approveBtn().disabled).toBe(true)
  })

  it('sends approve with the server paramsHash only on a trusted click; synthetic clicks are ignored', async () => {
    open(ap('a'))
    render(<McpApprovalPrompt />)
    await advance(2100)
    fireEvent.click(approveBtn())
    expect(call).not.toHaveBeenCalled()
    call.mockResolvedValueOnce({})
    trustedClick(approveBtn())
    await advance(0)
    expect(call).toHaveBeenCalledWith('mcp.approval.decide', {
      approvalId: 'a',
      decision: 'approve',
      paramsHash: 'hash-a'
    })
    expect(queue()).toEqual([])
  })

  it('renders argsPreview as plain text, never as HTML, and shows hidden characters', async () => {
    open(ap('a', { argsPreview: { text: '<img src=x onerror=alert(1)>‮evil', redacted: true } }))
    const { container } = render(<McpApprovalPrompt />)
    const pre = screen.getByLabelText('Exact tool arguments')
    expect(pre.tagName).toBe('PRE')
    expect(pre.textContent).toContain('<img src=x onerror=alert(1)>')
    expect(pre.textContent).toContain('\\u{202E}')
    expect(document.body.querySelector('img')).toBeNull()
    expect(container.ownerDocument.querySelector('script')).toBeNull()
    expect(screen.getByText('Secrets hidden')).toBeTruthy()
  })

  it('HASH_MISMATCH keeps the request, shows the error, resyncs and never resends', async () => {
    open(ap('a'))
    render(<McpApprovalPrompt />)
    await advance(2100)
    call.mockImplementation((m: string) =>
      m === 'mcp.approval.decide'
        ? Promise.reject(new McpRpcError('MCP_APPROVAL_HASH_MISMATCH', 'x'))
        : Promise.resolve({ approvals: [ap('a', { paramsHash: 'new-hash' })] })
    )
    trustedClick(approveBtn())
    await advance(0)
    expect(screen.getByRole('alert').textContent).toContain('This request changed')
    expect(queue()).toEqual(['a'])
    expect(useAppStore.getState().mcpApprovalQueue[0].paramsHash).toBe('new-hash')
    expect(approveBtn().disabled).toBe(true)
    await advance(5000)
    expect(call.mock.calls.filter((c) => c[0] === 'mcp.approval.decide')).toHaveLength(1)
  })

  it.each(['MCP_APPROVAL_EXPIRED', 'MCP_APPROVAL_ALREADY_DECIDED', 'MCP_NOT_FOUND'] as const)(
    '%s removes the request from the queue',
    async (code) => {
      open(ap('a'))
      render(<McpApprovalPrompt />)
      await advance(2100)
      call.mockRejectedValueOnce(new McpRpcError(code, 'x'))
      trustedClick(approveBtn())
      await advance(0)
      expect(queue()).toEqual([])
    }
  )

  it('Deny sends deny and moves to the next queued approval with the lock restarted', async () => {
    open(ap('a'), ap('b', { createdAt: new Date(Date.now() + 1000).toISOString() }))
    render(<McpApprovalPrompt />)
    expect(screen.getByText('Request 1 of 2')).toBeTruthy()
    call.mockResolvedValueOnce({})
    fireEvent.click(screen.getByRole('button', { name: /^Deny / }))
    await advance(0)
    expect(call).toHaveBeenCalledWith('mcp.approval.decide', {
      approvalId: 'a',
      decision: 'deny',
      paramsHash: 'hash-a'
    })
    expect(queue()).toEqual(['b'])
    expect(approveBtn().disabled).toBe(true)
  })

  it('Decide later closes the prompt; a new approval reopens it', async () => {
    open(ap('a'))
    render(<McpApprovalPrompt />)
    fireEvent.click(screen.getByRole('button', { name: 'Decide later' }))
    expect(useAppStore.getState().mcpApprovalPromptOpen).toBe(false)
    expect(queue()).toEqual(['a'])
    open(ap('b', { createdAt: new Date(Date.now() + 1000).toISOString() }))
    expect(useAppStore.getState().mcpApprovalPromptOpen).toBe(true)
  })

  it('counts down, locks Approve at expiry and reconciles with the server once', async () => {
    open(ap('a', { tool: { name: 't', title: 'T', risk: 'read' } }))
    render(<McpApprovalPrompt />)
    expect(screen.getByText(/^1:5\d$|^2:00$/)).toBeTruthy()
    call.mockResolvedValue({ approvals: [ap('a')] })
    await advance(119_000)
    expect(screen.getByText('Expiring…')).toBeTruthy()
    expect(call.mock.calls.filter((c) => c[0] === 'mcp.approval.list')).toHaveLength(1)
    await advance(10_000)
    expect(call.mock.calls.filter((c) => c[0] === 'mcp.approval.list')).toHaveLength(1)
  })

  it('removes an expired request once the server no longer lists it', async () => {
    open(ap('a'))
    render(<McpApprovalPrompt />)
    call.mockResolvedValue({ approvals: [] })
    await advance(120_000)
    expect(queue()).toEqual([])
  })

  it('disables Approve while the kill switch is active', async () => {
    act(() => {
      useAppStore.setState({ mcpServerInfo: { killSwitch: { active: true } } } as never)
    })
    open(ap('a'))
    render(<McpApprovalPrompt />)
    await advance(3000)
    expect(approveBtn().disabled).toBe(true)
    expect(screen.getByText('MCP access is suspended')).toBeTruthy()
    act(() => {
      useAppStore.setState({ mcpServerInfo: { killSwitch: { active: false } } } as never)
    })
  })
})
