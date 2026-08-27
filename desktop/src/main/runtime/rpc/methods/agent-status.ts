// Why: RPC-channel counterpart to the desktop-only 'agentStatus:*' ipcMain
// handlers (see ipc/agent-hooks.ts / registerAgentHookHandlers). Only the
// request/response methods are covered here — getSnapshot, inferInterrupt,
// getMigrationUnsupportedSnapshot, drop, dropByTabPrefix. The push events
// ('agentStatus:set'/'clear'/'migrationUnsupported'/'migrationUnsupportedClear')
// are intentionally NOT ported: AgentHookServer.setListener()/
// setPaneStatusClearListener() are single-slot callbacks already claimed by
// desktop/src/main/index.ts for the real desktop UI, so adding a second RPC
// subscriber would silently break that live wiring. Converting
// AgentHookServer to a multi-listener emitter is a separate follow-up.
import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import { agentHookServer, isValidPaneKey } from '../../../agent-hooks/server'
import { enrichAgentStatusIpcPayload } from '../../../ipc/agent-hooks'
import {
  clearMigrationUnsupportedPtysByTabPrefix,
  clearMigrationUnsupportedPtysForPaneKey,
  getMigrationUnsupportedPtySnapshot
} from '../../../agent-hooks/migration-unsupported-pty-state'
import { isValidTerminalTabId } from '../../../../shared/terminal-tab-id'
import type { AgentInterruptInferenceRequest } from '../../../../shared/agent-interrupt-intent'

// Why: mirrors MAX_AGENT_STATUS_DROP_TAB_ID_LENGTH in ipc/agent-hooks.ts —
// that constant is private to the ipcMain handler module, so this is a
// duplicated literal rather than a shared export to avoid widening that
// module's public surface for one bound value.
const MAX_AGENT_STATUS_DROP_TAB_ID_LENGTH = 160

const AgentStatusDropParams = z.object({
  paneKey: z.string().refine(isValidPaneKey, 'Invalid paneKey')
})

const AgentStatusDropByTabPrefixParams = z.object({
  tabId: z
    .string()
    .max(MAX_AGENT_STATUS_DROP_TAB_ID_LENGTH)
    .refine((value) => value.trim() === value && isValidTerminalTabId(value), 'Invalid tabId')
})

const AgentInterruptInferenceParams = z.object({
  paneKey: z.string(),
  baselineUpdatedAt: z.number(),
  baselineStateStartedAt: z.number(),
  baselinePrompt: z.string(),
  baselineAgentType: z.string().optional(),
  intent: z.enum(['plain-escape', 'ctrl-c']),
  inputCount: z.number().optional()
})

export const AGENT_STATUS_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'agentStatus.getSnapshot',
    params: null,
    // Why: the renderer pulls this after workspace hydration so startup
    // cannot lose replayed statuses while its local store is still empty —
    // same reasoning as the ipcMain 'agentStatus:getSnapshot' handler.
    handler: (_params, { runtime }) =>
      agentHookServer.getStatusSnapshot().map((entry) => enrichAgentStatusIpcPayload(entry, runtime))
  }),
  defineMethod({
    name: 'agentStatus.inferInterrupt',
    params: AgentInterruptInferenceParams,
    // Why: mirrors the ipcMain 'agentStatus:inferInterrupt' handler's cast —
    // the request shape is validated above; AgentInterruptInferenceRequest's
    // `baselineAgentType: AgentType | undefined` is a required key typed to
    // allow undefined, which zod's `.optional()` (an optional key) doesn't
    // structurally match 1:1.
    handler: (params) => agentHookServer.inferInterrupt(params as AgentInterruptInferenceRequest)
  }),
  defineMethod({
    name: 'agentStatus.getMigrationUnsupportedSnapshot',
    params: null,
    handler: () => getMigrationUnsupportedPtySnapshot()
  }),
  // Why: fire-and-forget on the ipcMain side (ipcRenderer.send, no reply) —
  // kept as a request/response RPC method here since the generic RPC
  // channel has no send-only primitive, but the handler still swallows
  // errors the same way the ipcMain listener does so a bad call can't crash
  // the connection.
  defineMethod({
    name: 'agentStatus.drop',
    params: AgentStatusDropParams,
    handler: (params) => {
      try {
        // Why: dropStatusEntry (not clearPaneState) is correct here — the
        // user is dismissing a status row, not tearing down a PTY.
        agentHookServer.dropStatusEntry(params.paneKey)
        clearMigrationUnsupportedPtysForPaneKey(params.paneKey)
      } catch (err) {
        console.warn('[agent-status rpc] dropStatusEntry failed:', err)
      }
      return { dropped: true }
    }
  }),
  defineMethod({
    name: 'agentStatus.dropByTabPrefix',
    params: AgentStatusDropByTabPrefixParams,
    handler: (params) => {
      try {
        agentHookServer.dropStatusEntriesByTabPrefix(params.tabId)
        clearMigrationUnsupportedPtysByTabPrefix(params.tabId)
      } catch (err) {
        console.warn('[agent-status rpc] dropStatusEntriesByTabPrefix failed:', err)
      }
      return { dropped: true }
    }
  })
]
