// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render } from '@testing-library/react'
import { useMcpSync } from './useMcpSync'
import { useAppStore } from '@/store'
import { dispatchPushDeepLink, parsePushDeepLink } from '../web/web-push-deep-link'

const openMcpTab = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { isBridgeAvailable: () => true }
}))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      currentUser: { id: 'u' },
      mcpServerInfoStatus: 'idle',
      persistedUIReady: false,
      mcpServerInfo: null,
      mcpAdminSetupAvailable: false,
      resetMcp: vi.fn(),
      refreshMcpServerInfo: vi.fn(),
      startMcpEvents: () => () => {},
      openMcpTab: (...a: unknown[]) => openMcpTab(...a)
    }))
  }
})

function App(): null {
  useMcpSync()
  return null
}
const setStore = (p: Record<string, unknown>): void => act(() => useAppStore.setState(p as never))
const visibleInfo = { enabled: true, tenantEnabled: true }

beforeEach(() => {
  openMcpTab.mockReset()
  useAppStore.setState({
    currentUser: { id: 'u' },
    mcpServerInfoStatus: 'idle',
    persistedUIReady: false,
    mcpServerInfo: null,
    mcpAdminSetupAvailable: false
  } as never)
  window.history.replaceState(null, '', '/?section=mcp&tab=approvals&approval=a1')
})
afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
})

describe('useMcpSync cold-start deep link', () => {
  it('waits for persistedUIReady and server info, then opens once and clears the URL', () => {
    render(<App />)
    expect(openMcpTab).not.toHaveBeenCalled()
    setStore({ mcpServerInfo: visibleInfo, mcpServerInfoStatus: 'ready' })
    expect(openMcpTab).not.toHaveBeenCalled()
    expect(window.location.search).toContain('section=mcp')
    setStore({ persistedUIReady: true })
    expect(openMcpTab).toHaveBeenCalledTimes(1)
    expect(openMcpTab).toHaveBeenCalledWith('approvals', 'a1')
    expect(window.location.search).toBe('')
    setStore({ mcpServerInfoStatus: 'loading' })
    setStore({ mcpServerInfoStatus: 'ready' })
    expect(openMcpTab).toHaveBeenCalledTimes(1)
  })

  it('opens immediately when everything is already ready on mount', () => {
    useAppStore.setState({
      persistedUIReady: true,
      mcpServerInfo: visibleInfo,
      mcpServerInfoStatus: 'ready'
    } as never)
    render(<App />)
    expect(openMcpTab).toHaveBeenCalledWith('approvals', 'a1')
    expect(window.location.search).toBe('')
  })

  it('drops the link without opening when MCP is hidden and info is ready', () => {
    useAppStore.setState({ persistedUIReady: true, mcpServerInfoStatus: 'ready' } as never)
    render(<App />)
    expect(openMcpTab).not.toHaveBeenCalled()
    expect(window.location.search).toBe('')
  })

  it('does nothing when the URL has no MCP link', () => {
    window.history.replaceState(null, '', '/')
    useAppStore.setState({
      persistedUIReady: true,
      mcpServerInfo: visibleInfo,
      mcpServerInfoStatus: 'ready'
    } as never)
    render(<App />)
    expect(openMcpTab).not.toHaveBeenCalled()
  })

  it('handles a pushed link after mount and stops handling after unmount', () => {
    window.history.replaceState(null, '', '/')
    useAppStore.setState({
      persistedUIReady: true,
      mcpServerInfo: visibleInfo,
      mcpServerInfoStatus: 'ready'
    } as never)
    const { unmount } = render(<App />)
    const link = parsePushDeepLink('/?section=mcp&tab=tokens&token=t9', 'http://x')!
    act(() => dispatchPushDeepLink(link))
    expect(openMcpTab).toHaveBeenCalledWith('tokens', 't9')
    unmount()
    openMcpTab.mockReset()
    act(() => dispatchPushDeepLink(link))
    expect(openMcpTab).not.toHaveBeenCalled()
  })
})
