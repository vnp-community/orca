import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { McpEvent, McpServerInfo } from '../../../../shared/mcp-types'
import { mcpClient } from '../../runtime/runtime-mcp-client'
import {
  isMcpDisabledError,
  isMcpNotImplementedError,
  type McpRpcError
} from '../../runtime/runtime-mcp-error'
import { emitMcpEvent } from '../../lib/mcp-event-bus'
import { MCP_HEALTHY_STREAM_MS, nextMcpReconnectDelayMs } from './mcp-reconnect'

export type McpTabId =
  | 'connect'
  | 'apps'
  | 'clients'
  | 'grants'
  | 'tokens'
  | 'approvals'
  | 'tools'
  | 'policy'
  | 'prompts'
  | 'audit'
  | 'servers'

export type McpSlice = {
  mcpServerInfo: McpServerInfo | null
  mcpServerInfoStatus: 'idle' | 'loading' | 'ready' | 'error'
  mcpServerInfoError: string | null
  /** Admin + MCP unavailable + admin settings readable => show the "turn on MCP" card. */
  mcpAdminSetupAvailable: boolean
  mcpEventsState: 'off' | 'connecting' | 'on'
  /** Bumped after each stream reconnect so tabs reload their data. */
  mcpResyncCounter: number
  /** Deep-link request, consumed once by McpPane. */
  mcpNavigation: { tab: McpTabId; focusId?: string } | null
  refreshMcpServerInfo: () => Promise<void>
  applyMcpEvent: (event: McpEvent) => void
  /** Ref-counted: one shared stream; returned fn releases this caller's reference. */
  startMcpEvents: () => () => void
  openMcpTab: (tab: McpTabId, focusId?: string) => void
  clearMcpNavigation: () => void
  resetMcp: () => void
}

// D6: only hidden when the tenant is explicitly disabled.
export const selectMcpEnabled = (s: AppState): boolean =>
  s.mcpServerInfo?.enabled === true && s.mcpServerInfo?.tenantEnabled !== false
export const selectMcpSectionVisible = (s: AppState): boolean =>
  selectMcpEnabled(s) || s.mcpAdminSetupAvailable
export const selectMcpKillSwitchActive = (s: AppState): boolean =>
  s.mcpServerInfo?.killSwitch?.active === true

function disabledServerInfo(): McpServerInfo {
  return {
    enabled: false,
    resourceUrl: '',
    protocolVersions: [],
    authorizationServer: '',
    scopesSupported: [],
    dcrEnabled: false,
    maxTokenDays: 0,
    killSwitch: { active: false }
  }
}

let requestSeq = 0
let streamRefs = 0
let stopStream: (() => void) | null = null
let retryTimer: ReturnType<typeof setTimeout> | null = null
let failures = 0

function clearRetry(): void {
  if (retryTimer !== null) {
    clearTimeout(retryTimer)
    retryTimer = null
  }
}

function teardownStream(): void {
  clearRetry()
  stopStream?.()
  stopStream = null
}

/** Test hook: drops module-level stream state. */
export function resetMcpStreamStateForTests(): void {
  streamRefs = 0
  failures = 0
  requestSeq = 0
  teardownStream()
}

export const createMcpSlice: StateCreator<AppState, [], [], McpSlice> = (set, get) => {
  const connect = (): void => {
    if (stopStream || streamRefs === 0) {
      return
    }
    const openedAt = Date.now()
    set({ mcpEventsState: 'connecting' })
    let closed = false
    const unsub = mcpClient.subscribeEvents(
      (ev) => get().applyMcpEvent(ev),
      () => {
        if (closed) {
          return
        }
        closed = true
        stopStream = null
        set({ mcpEventsState: 'off' })
        if (Date.now() - openedAt >= MCP_HEALTHY_STREAM_MS) {
          failures = 0
        }
        scheduleReconnect()
      }
    )
    stopStream = () => {
      closed = true
      unsub()
    }
    set({ mcpEventsState: 'on' })
  }

  const scheduleReconnect = (): void => {
    if (streamRefs === 0 || retryTimer !== null) {
      return
    }
    const delay = nextMcpReconnectDelayMs(failures)
    failures += 1
    retryTimer = setTimeout(() => {
      retryTimer = null
      const s = get()
      if (streamRefs === 0 || s.currentUser === null || !selectMcpEnabled(s)) {
        return
      }
      connect()
      set((st) => ({ mcpResyncCounter: st.mcpResyncCounter + 1 }))
      void get().refreshMcpServerInfo()
    }, delay)
  }

  return {
    mcpServerInfo: null,
    mcpServerInfoStatus: 'idle',
    mcpServerInfoError: null,
    mcpAdminSetupAvailable: false,
    mcpEventsState: 'off',
    mcpResyncCounter: 0,
    mcpNavigation: null,

    refreshMcpServerInfo: async () => {
      if (!mcpClient.isBridgeAvailable()) {
        set({
          mcpServerInfo: null,
          mcpServerInfoStatus: 'ready',
          mcpServerInfoError: null
        })
        return
      }
      const seq = ++requestSeq
      if (get().mcpServerInfoStatus !== 'ready') {
        set({ mcpServerInfoStatus: 'loading' })
      }
      let info: McpServerInfo
      try {
        info = await mcpClient.call('mcp.server.info')
      } catch (e) {
        if (seq !== requestSeq) {
          return
        }
        if (isMcpDisabledError(e) || isMcpNotImplementedError(e)) {
          info = disabledServerInfo()
        } else {
          set({
            mcpServerInfoStatus: 'error',
            mcpServerInfoError: (e as McpRpcError).detail
          })
          return
        }
      }
      if (seq !== requestSeq) {
        return
      }
      const available = info.enabled === true && info.tenantEnabled !== false
      set({
        mcpServerInfo: info,
        mcpServerInfoStatus: 'ready',
        mcpServerInfoError: null,
        ...(available ? { mcpAdminSetupAvailable: false } : {})
      })
      if (available) {
        return
      }
      if (get().currentUser?.role !== 'admin') {
        set({ mcpAdminSetupAvailable: false })
        return
      }
      // Admin lockout guard: unavailable MCP still needs a place to switch it on.
      let setupAvailable = false
      try {
        await mcpClient.call('mcp.admin.settings.get')
        setupAvailable = true
      } catch {
        setupAvailable = false
      }
      if (seq === requestSeq) {
        set({ mcpAdminSetupAvailable: setupAvailable })
      }
    },

    applyMcpEvent: (event) => {
      if (event.type === 'killswitch.changed') {
        set((s) =>
          s.mcpServerInfo
            ? {
                mcpServerInfo: {
                  ...s.mcpServerInfo,
                  killSwitch: {
                    ...s.mcpServerInfo.killSwitch,
                    active: event.active,
                    reason: event.reason
                  }
                }
              }
            : {}
        )
      }
      emitMcpEvent(event)
    },

    startMcpEvents: () => {
      streamRefs += 1
      connect()
      let released = false
      return () => {
        if (released) {
          return
        }
        released = true
        streamRefs = Math.max(0, streamRefs - 1)
        if (streamRefs === 0) {
          teardownStream()
          failures = 0
          set({ mcpEventsState: 'off' })
        }
      }
    },

    openMcpTab: (tab, focusId) => {
      get().openSettingsPage()
      get().openSettingsTarget({ pane: 'mcp', repoId: null })
      set({ mcpNavigation: { tab, ...(focusId ? { focusId } : {}) } })
    },

    clearMcpNavigation: () => set({ mcpNavigation: null }),

    resetMcp: () => {
      requestSeq += 1
      streamRefs = 0
      failures = 0
      teardownStream()
      set({
        mcpServerInfo: null,
        mcpServerInfoStatus: 'idle',
        mcpServerInfoError: null,
        mcpAdminSetupAvailable: false,
        mcpEventsState: 'off',
        mcpNavigation: null
      })
    }
  }
}
