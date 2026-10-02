import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { create } from 'zustand'
import type { McpServerInfo } from '../../../../shared/mcp-types'
import { parseMcpError } from '../../runtime/runtime-mcp-error'
import { subscribeMcpEvents } from '../../lib/mcp-event-bus'
import type { AppState } from '../types'
import {
  createMcpSlice,
  resetMcpStreamStateForTests,
  selectMcpEnabled,
  selectMcpKillSwitchActive,
  selectMcpSectionVisible,
  type McpSlice
} from './mcp-slice'

const client = vi.hoisted(() => ({
  call: vi.fn(),
  subscribeEvents: vi.fn(),
  isBridgeAvailable: vi.fn(() => true)
}))
vi.mock('../../runtime/runtime-mcp-client', () => ({ mcpClient: client }))

function info(over: Partial<McpServerInfo> = {}): McpServerInfo {
  return {
    enabled: true,
    resourceUrl: 'u',
    protocolVersions: [],
    authorizationServer: 'a',
    scopesSupported: [],
    dcrEnabled: false,
    maxTokenDays: 30,
    killSwitch: { active: false },
    ...over
  }
}

function makeStore(role: 'admin' | 'developer' = 'developer') {
  const openSettingsPage = vi.fn()
  const openSettingsTarget = vi.fn()
  const store = create<McpSlice & Record<string, unknown>>()((...a) => ({
    ...createMcpSlice(...(a as unknown as Parameters<typeof createMcpSlice>)),
    currentUser: { id: 'u', role },
    openSettingsPage,
    openSettingsTarget
  }))
  return { store, openSettingsPage, openSettingsTarget }
}
const sel = <T>(s: McpSlice, f: (a: AppState) => T): T => f(s as unknown as AppState)

beforeEach(() => {
  vi.useFakeTimers()
  client.call.mockReset()
  client.subscribeEvents.mockReset()
  client.isBridgeAvailable.mockReturnValue(true)
  resetMcpStreamStateForTests()
})
afterEach(() => {
  resetMcpStreamStateForTests()
  vi.useRealTimers()
})

describe('mcp-slice selectors', () => {
  it('honors enabled && tenantEnabled', () => {
    const { store } = makeStore()
    expect(sel(store.getState(), selectMcpEnabled)).toBe(false)
    store.setState({ mcpServerInfo: info({ tenantEnabled: false }) })
    expect(sel(store.getState(), selectMcpEnabled)).toBe(false)
    store.setState({ mcpServerInfo: info() })
    expect(sel(store.getState(), selectMcpEnabled)).toBe(true)
    store.setState({ mcpServerInfo: info({ killSwitch: { active: true } }) })
    expect(sel(store.getState(), selectMcpKillSwitchActive)).toBe(true)
    store.setState({
      mcpServerInfo: info({ enabled: false }),
      mcpAdminSetupAvailable: true
    })
    expect(sel(store.getState(), selectMcpSectionVisible)).toBe(true)
  })
})

describe('refreshMcpServerInfo', () => {
  it('stores info', async () => {
    const { store } = makeStore()
    client.call.mockResolvedValue(info())
    await store.getState().refreshMcpServerInfo()
    expect(store.getState().mcpServerInfoStatus).toBe('ready')
    expect(store.getState().mcpServerInfo?.enabled).toBe(true)
  })

  it('treats MCP_DISABLED and not-implemented as disabled without error', async () => {
    const { store } = makeStore()
    client.call.mockRejectedValueOnce(parseMcpError(new Error('MCP_DISABLED: off')))
    await store.getState().refreshMcpServerInfo()
    expect(store.getState().mcpServerInfo?.enabled).toBe(false)
    expect(store.getState().mcpServerInfoStatus).toBe('ready')
    client.call.mockRejectedValueOnce(new Error('channel "mcp.server.info" is not yet implemented'))
    await store.getState().refreshMcpServerInfo()
    expect(store.getState().mcpServerInfo?.enabled).toBe(false)
    expect(store.getState().mcpServerInfoError).toBeNull()
  })

  it('keeps old info on other errors', async () => {
    const { store } = makeStore()
    client.call.mockResolvedValueOnce(info())
    await store.getState().refreshMcpServerInfo()
    client.call.mockRejectedValueOnce(parseMcpError(new Error('boom')))
    await store.getState().refreshMcpServerInfo()
    expect(store.getState().mcpServerInfoStatus).toBe('error')
    expect(store.getState().mcpServerInfoError).toBe('boom')
    expect(store.getState().mcpServerInfo?.enabled).toBe(true)
  })

  it('applies only the latest response', async () => {
    const { store } = makeStore()
    let resolveFirst!: (v: McpServerInfo) => void
    client.call.mockImplementationOnce(() => new Promise((r) => (resolveFirst = r)))
    client.call.mockResolvedValueOnce(info({ maxTokenDays: 2 }))
    const first = store.getState().refreshMcpServerInfo()
    await store.getState().refreshMcpServerInfo()
    resolveFirst(info({ maxTokenDays: 1 }))
    await first
    expect(store.getState().mcpServerInfo?.maxTokenDays).toBe(2)
  })

  it('offers admin setup when disabled and settings are readable', async () => {
    const { store } = makeStore('admin')
    client.call.mockImplementation(async (m: string) =>
      m === 'mcp.server.info' ? info({ tenantEnabled: false }) : { enabled: false }
    )
    await store.getState().refreshMcpServerInfo()
    expect(store.getState().mcpAdminSetupAvailable).toBe(true)
  })

  it('no admin setup for regular users or when settings.get fails', async () => {
    const dev = makeStore('developer')
    client.call.mockResolvedValue(info({ enabled: false }))
    await dev.store.getState().refreshMcpServerInfo()
    expect(dev.store.getState().mcpAdminSetupAvailable).toBe(false)
    const adm = makeStore('admin')
    client.call.mockImplementation(async (m: string) => {
      if (m === 'mcp.server.info') {
        return info({ enabled: false })
      }
      throw parseMcpError(new Error('MCP_DISABLED: off'))
    })
    await adm.store.getState().refreshMcpServerInfo()
    expect(adm.store.getState().mcpAdminSetupAvailable).toBe(false)
  })

  it('without a bridge: ready with null info', async () => {
    const { store } = makeStore()
    client.isBridgeAvailable.mockReturnValue(false)
    await store.getState().refreshMcpServerInfo()
    expect(store.getState().mcpServerInfo).toBeNull()
    expect(store.getState().mcpServerInfoStatus).toBe('ready')
  })
})

describe('events', () => {
  it('killswitch.changed updates info and fans out', () => {
    const { store } = makeStore()
    store.setState({ mcpServerInfo: info() })
    const seen = vi.fn()
    const off = subscribeMcpEvents(seen)
    store.getState().applyMcpEvent({ type: 'killswitch.changed', active: true, reason: 'r' })
    expect(store.getState().mcpServerInfo?.killSwitch).toMatchObject({
      active: true,
      reason: 'r'
    })
    expect(seen).toHaveBeenCalledTimes(1)
    off()
  })

  it('startMcpEvents is ref-counted and opens one stream', () => {
    const { store } = makeStore()
    const teardown = vi.fn()
    client.subscribeEvents.mockReturnValue(teardown)
    const stopA = store.getState().startMcpEvents()
    const stopB = store.getState().startMcpEvents()
    expect(client.subscribeEvents).toHaveBeenCalledTimes(1)
    expect(store.getState().mcpEventsState).toBe('on')
    stopA()
    stopA()
    expect(teardown).not.toHaveBeenCalled()
    stopB()
    expect(teardown).toHaveBeenCalledTimes(1)
    expect(store.getState().mcpEventsState).toBe('off')
  })

  it('reconnects with backoff after close, bumps resync counter, refreshes info', async () => {
    const { store } = makeStore()
    store.setState({ mcpServerInfo: info() })
    client.call.mockResolvedValue(info())
    const closers: (() => void)[] = []
    client.subscribeEvents.mockImplementation((_e: unknown, onClose: () => void) => {
      closers.push(onClose)
      return vi.fn()
    })
    store.getState().startMcpEvents()
    closers[0]()
    expect(store.getState().mcpEventsState).toBe('off')
    await vi.advanceTimersByTimeAsync(999)
    expect(client.subscribeEvents).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(client.subscribeEvents).toHaveBeenCalledTimes(2)
    expect(store.getState().mcpResyncCounter).toBe(1)
    closers[1]()
    await vi.advanceTimersByTimeAsync(1999)
    expect(client.subscribeEvents).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1)
    expect(client.subscribeEvents).toHaveBeenCalledTimes(3)
  })

  it('resetMcp stops the stream and clears state', () => {
    const { store } = makeStore()
    const teardown = vi.fn()
    client.subscribeEvents.mockReturnValue(teardown)
    store.setState({ mcpServerInfo: info() })
    store.getState().startMcpEvents()
    store.getState().resetMcp()
    expect(teardown).toHaveBeenCalled()
    expect(store.getState().mcpServerInfo).toBeNull()
  })
})

describe('navigation', () => {
  it('openMcpTab opens Settings at the mcp pane and records the target', () => {
    const { store, openSettingsPage, openSettingsTarget } = makeStore()
    store.getState().openMcpTab('approvals', 'a1')
    expect(openSettingsPage).toHaveBeenCalled()
    expect(openSettingsTarget).toHaveBeenCalledWith({
      pane: 'mcp',
      repoId: null
    })
    expect(store.getState().mcpNavigation).toEqual({
      tab: 'approvals',
      focusId: 'a1'
    })
    store.getState().clearMcpNavigation()
    expect(store.getState().mcpNavigation).toBeNull()
  })
})
