// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpAgentTerminalsList } from './McpAgentTerminalsList'

const stop = vi.fn()
vi.mock('@/hooks/useStopMcpTerminal', () => ({
  STOP_GRACE_MS: 3000,
  useStopMcpTerminal: () => stop
}))
vi.mock('@/hooks/useMcpTerminalOrigins', () => ({ useMcpTerminalOrigins: () => {} }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: null as unknown,
      mcpOriginByHandle: {} as Record<string, unknown>,
      mcpOriginsRefreshedAt: null as number | null
    }))
  }
})

const info = { enabled: true, killSwitch: { active: false }, scopesSupported: [] }
const origin = { type: 'mcp', clientName: 'Cursor', mcpSessionId: 'sess-1', userId: 'u' }

beforeEach(() => {
  stop.mockReset()
  useAppStore.setState({
    mcpServerInfo: info,
    mcpOriginByHandle: {},
    mcpOriginsRefreshedAt: 1
  } as never)
})
afterEach(cleanup)

describe('McpAgentTerminalsList', () => {
  it('renders nothing when MCP is disabled', () => {
    useAppStore.setState({ mcpServerInfo: { ...info, enabled: false } } as never)
    const { container } = render(<McpAgentTerminalsList />)
    expect(container.innerHTML).toBe('')
  })

  it('shows the empty state', () => {
    render(<McpAgentTerminalsList />)
    expect(screen.getByText('No terminals started by agents.')).toBeTruthy()
  })

  it('lists entries and stops by handle with the client in the accessible name', async () => {
    stop.mockResolvedValue(undefined)
    useAppStore.setState({ mcpOriginByHandle: { 'handle-1': origin } } as never)
    render(<McpAgentTerminalsList />)
    fireEvent.click(screen.getByRole('button', { name: 'Stop terminal started by Cursor' }))
    await waitFor(() => expect(stop).toHaveBeenCalledWith('handle-1', { force: false }))
  })
})
