// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/react'
import { useAppStore } from '@/store'
import { useMcpTerminalOrigins } from './useMcpTerminalOrigins'

const poller = vi.hoisted(() => ({ start: vi.fn(), stop: vi.fn() }))
vi.mock('@/lib/mcp-terminal-origin-poller', () => ({
  startMcpTerminalOriginPolling: (...a: unknown[]) => {
    poller.start(...a)
    return poller.stop
  }
}))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: null as unknown,
      settings: null as unknown,
      clearMcpTerminalOrigins: vi.fn(),
      setMcpTerminalOrigins: vi.fn()
    }))
  }
})

function Probe(): null {
  useMcpTerminalOrigins()
  return null
}
const enable = (enabled: boolean): void =>
  useAppStore.setState({
    mcpServerInfo: { enabled, killSwitch: { active: false }, scopesSupported: [] }
  } as never)

beforeEach(() => {
  poller.start.mockReset()
  poller.stop.mockReset()
  enable(true)
})
afterEach(cleanup)

describe('useMcpTerminalOrigins', () => {
  it('does not poll when MCP is disabled and clears stale origins', () => {
    enable(false)
    render(<Probe />)
    expect(poller.start).not.toHaveBeenCalled()
    expect(
      (useAppStore.getState() as never as { clearMcpTerminalOrigins: ReturnType<typeof vi.fn> })
        .clearMcpTerminalOrigins
    ).toHaveBeenCalled()
  })

  it('shares one poller across consumers and stops it after the last unmount', () => {
    const a = render(<Probe />)
    const b = render(<Probe />)
    expect(poller.start).toHaveBeenCalledTimes(1)
    a.unmount()
    expect(poller.stop).not.toHaveBeenCalled()
    b.unmount()
    expect(poller.stop).toHaveBeenCalledTimes(1)
  })
})
