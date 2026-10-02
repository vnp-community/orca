// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpOriginBadge } from './McpOriginBadge'
import { McpOriginStopMenuItem } from './McpOriginStopMenuItem'
import { McpTabOriginBadge } from './McpTabOriginBadge'

const confirm = vi.fn()
const stop = vi.fn()
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('@/hooks/useStopMcpTerminal', () => ({ useStopMcpTerminal: () => stop }))
vi.mock('@/hooks/useMcpTerminalOrigins', () => ({ useMcpTerminalOrigins: () => {} }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenuSeparator: () => <hr />,
  DropdownMenuItem: ({
    children,
    disabled,
    onSelect
  }: {
    children?: React.ReactNode
    disabled?: boolean
    onSelect?: () => void
  }) => (
    <button type="button" disabled={disabled} onClick={() => onSelect?.()}>
      {children}
    </button>
  )
}))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      ptyIdsByTabId: {} as Record<string, string[]>,
      mcpOriginByHandle: {} as Record<string, unknown>
    }))
  }
})

const origin = { type: 'mcp', clientName: 'Claude Code', mcpSessionId: 's1', userId: 'u1' } as const

beforeEach(() => {
  confirm.mockReset()
  stop.mockReset()
  useAppStore.setState({
    ptyIdsByTabId: { t1: ['remote:env@@h1'], t2: ['p2'] },
    mcpOriginByHandle: {}
  })
})
afterEach(cleanup)

describe('McpOriginBadge', () => {
  it('shows icon plus text and an accessible name', () => {
    render(<McpOriginBadge origin={origin} />)
    const badge = screen.getByTestId('mcp-origin-badge')
    expect(badge.textContent).toBe('Created by agent Claude Code')
    expect(badge.getAttribute('aria-label')).toBe('Created by agent Claude Code')
  })

  it('compact mode keeps the aria-label but drops the text', () => {
    render(<McpOriginBadge origin={origin} compact />)
    const badge = screen.getByTestId('mcp-origin-badge')
    expect(badge.textContent).toBe('')
    expect(badge.getAttribute('aria-label')).toBe('Created by agent Claude Code')
  })

  it('renders a hostile client name as plain text', () => {
    const { container } = render(
      <McpOriginBadge origin={{ ...origin, clientName: '<img src=x onerror=alert(1)>' }} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByTestId('mcp-origin-badge').textContent).toContain('<img src=x')
  })
})

describe('McpTabOriginBadge (absent-origin regression)', () => {
  it('renders nothing when the tab has no MCP origin', () => {
    const { container } = render(<McpTabOriginBadge tabId="t1" />)
    expect(container.innerHTML).toBe('')
  })

  it('renders nothing for a tab whose pty is not in the origin map', () => {
    useAppStore.setState({ mcpOriginByHandle: { h1: origin } })
    const { container } = render(<McpTabOriginBadge tabId="t2" />)
    expect(container.innerHTML).toBe('')
  })

  it('shows the badge when the tab pty matches an origin', () => {
    useAppStore.setState({ mcpOriginByHandle: { h1: origin } })
    render(<McpTabOriginBadge tabId="t1" />)
    expect(screen.getByTestId('mcp-origin-badge')).toBeTruthy()
  })
})

describe('McpOriginStopMenuItem', () => {
  it('renders nothing and never needs the confirmation provider when origin is absent', () => {
    const { container } = render(<McpOriginStopMenuItem tabId="t1" />)
    expect(container.innerHTML).toBe('')
  })

  it('stops by handle only after confirmation', async () => {
    useAppStore.setState({ mcpOriginByHandle: { h1: origin } })
    stop.mockResolvedValue(undefined)
    confirm.mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    render(<McpOriginStopMenuItem tabId="t1" />)
    const item = screen.getByRole('button', { name: 'Stop agent-created process' })
    fireEvent.click(item)
    await waitFor(() => expect(confirm).toHaveBeenCalledTimes(1))
    expect(stop).not.toHaveBeenCalled()
    fireEvent.click(item)
    await waitFor(() => expect(stop).toHaveBeenCalledWith('h1'))
  })
})
