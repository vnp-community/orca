// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpApprovalsTab } from './McpApprovalsTab'

const call = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  const { createMcpApprovalSlice } = await import('@/store/slices/mcp-approval-slice')
  return {
    useAppStore: create((...a: unknown[]) => ({
      ...(createMcpApprovalSlice as (...x: unknown[]) => object)(...a)
    }))
  }
})

const ap = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  createdAt: '2026-10-01T10:00:00Z',
  expiresAt: '2026-10-01T10:02:00Z',
  status: 'approved',
  tool: { name: `tool_${id}`, title: `Tool ${id}`, risk: 'exec' },
  clientName: 'Claude',
  sessionId: 's',
  argsPreview: { text: '<b>bold</b> secret', redacted: false },
  paramsHash: 'h',
  ...over
})

beforeEach(() => {
  call.mockReset()
  scrollSpy.mockReset()
  Element.prototype.scrollIntoView = scrollSpy
  useAppStore.getState().clearMcpApprovals()
})
const scrollSpy = vi.fn()
afterEach(cleanup)

describe('McpApprovalsTab', () => {
  it('shows pending from the slice; Review focuses it but never decides', () => {
    call.mockResolvedValue({ approvals: [] })
    act(() => useAppStore.getState().enqueueMcpApproval(ap('p', { status: 'pending' }) as never, 0))
    act(() => useAppStore.getState().setMcpApprovalPromptOpen(false))
    render(<McpApprovalsTab />)
    fireEvent.click(screen.getByRole('button', { name: 'Review' }))
    expect(useAppStore.getState().mcpApprovalPromptOpen).toBe(true)
    expect(call).not.toHaveBeenCalledWith('mcp.approval.decide', expect.anything())
  })

  it('history paginates by cursor, hides pending rows and keeps arguments collapsed', async () => {
    call.mockResolvedValueOnce({
      approvals: [ap('1'), ap('x', { status: 'pending' })],
      nextCursor: 'c'
    })
    render(<McpApprovalsTab />)
    fireEvent.click(screen.getByRole('radio', { name: 'History' }))
    await screen.findByText('Tool 1')
    expect(screen.queryByText('Tool x')).toBeNull()
    expect(screen.queryByText(/secret/)).toBeNull()
    call.mockResolvedValueOnce({ approvals: [ap('1'), ap('2')] })
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    await screen.findByText('Tool 2')
    expect(call).toHaveBeenLastCalledWith('mcp.approval.list', {
      status: 'all',
      limit: 50,
      cursor: 'c'
    })
    expect(screen.getAllByText('Tool 1')).toHaveLength(1)
    fireEvent.click(screen.getAllByRole('button', { name: 'Show arguments' })[0])
    expect(screen.getByText('<b>bold</b> secret').tagName).toBe('PRE')
    expect(document.querySelector('b')).toBeNull()
  })

  it('deep link: scrolls to and highlights the row, then consumes the id', async () => {
    call.mockResolvedValue({ approvals: [ap('7')] })
    act(() => useAppStore.getState().setMcpApprovalFocusId('7'))
    render(<McpApprovalsTab />)
    const row = await screen.findByTestId('mcp-approval-7')
    await waitFor(() => expect(row.className).toContain('ring-2'))
    expect(scrollSpy).toHaveBeenCalled()
    expect(useAppStore.getState().mcpApprovalFocusId).toBeNull()
  })

  it('deep link to an unknown id shows a quiet note', async () => {
    call.mockResolvedValue({ approvals: [] })
    act(() => useAppStore.getState().setMcpApprovalFocusId('nope'))
    render(<McpApprovalsTab />)
    await screen.findByText('This request is no longer available')
  })

  it('shows an error with retry for history', async () => {
    call.mockRejectedValue(new Error('down'))
    render(<McpApprovalsTab />)
    fireEvent.click(screen.getByRole('radio', { name: 'History' }))
    await screen.findByText('down')
  })

  it('deep link to a history row switches to History; a later id re-focuses while mounted', async () => {
    call.mockResolvedValue({ approvals: [ap('h1'), ap('h2')] })
    render(<McpApprovalsTab />)
    await waitFor(() => expect(call).toHaveBeenCalled())
    act(() => useAppStore.getState().setMcpApprovalFocusId('h1'))
    const row1 = await screen.findByTestId('mcp-approval-h1')
    await waitFor(() => expect(row1.className).toContain('ring-2'))
    expect(screen.getByRole('radio', { name: 'History' }).getAttribute('aria-checked')).toBe('true')
    expect(useAppStore.getState().mcpApprovalFocusId).toBeNull()
    act(() => useAppStore.getState().setMcpApprovalFocusId('h2'))
    const row2 = await screen.findByTestId('mcp-approval-h2')
    await waitFor(() => expect(row2.className).toContain('ring-2'))
    expect(row1.className).not.toContain('ring-2')
    expect(useAppStore.getState().mcpApprovalFocusId).toBeNull()
  })

  it('a found id clears the "unavailable" note left by an earlier unknown id', async () => {
    call.mockResolvedValue({ approvals: [ap('h1')] })
    render(<McpApprovalsTab />)
    await waitFor(() => expect(call).toHaveBeenCalled())
    act(() => useAppStore.getState().setMcpApprovalFocusId('nope'))
    await screen.findByText('This request is no longer available')
    expect(useAppStore.getState().mcpApprovalFocusId).toBeNull()
    act(() => useAppStore.getState().setMcpApprovalFocusId('h1'))
    await screen.findByTestId('mcp-approval-h1')
    expect(screen.queryByText('This request is no longer available')).toBeNull()
  })

  it('waits for history to load before resolving the id, and a pending id wins', async () => {
    let release!: (v: unknown) => void
    call.mockReturnValue(new Promise((r) => (release = r)))
    act(() => useAppStore.getState().enqueueMcpApproval(ap('p', { status: 'pending' }) as never, 0))
    act(() => useAppStore.getState().setMcpApprovalFocusId('p'))
    render(<McpApprovalsTab />)
    expect(useAppStore.getState().mcpApprovalFocusId).toBe('p')
    await act(async () => release({ approvals: [] }))
    await waitFor(() => expect(useAppStore.getState().mcpApprovalFocusId).toBeNull())
    expect(screen.getByRole('radio', { name: /Pending/ }).getAttribute('aria-checked')).toBe('true')
    expect(screen.queryByText('This request is no longer available')).toBeNull()
  })
})
