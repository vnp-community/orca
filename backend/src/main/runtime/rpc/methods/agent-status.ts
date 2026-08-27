// Why: unlike cli.* (which relays to the Dev Server Agent because "install a
// wrapper script" only makes sense on the machine that hosts the terminal),
// agentStatus.* is a DIRECT port, not a devServerId relay. Backend already
// runs its own `agentHookServer` singleton (backend/src/main/agent-hooks/
// server.ts is byte-identical to desktop's) that is kept live by:
//   - native/local hook POSTs to its loopback HTTP server,
//   - ssh-relay-session.ts forwarding `agent.hook` envelopes into
//     agentHookServer.ingestRemote() for SSH-connected hosts,
//   - wsl-hook-relay-manager.ts doing the same for WSL guests.
// So the cache this file reads from is already populated per-paneKey across
// every connection type above — there is no per-devServerId split to relay
// to, so no `devServerId` param and no agent-side handler file for this
// namespace (contrast with cli.ts / agent-cli-handler.ts).
//
// KNOWN GAP (found during this port, not fixed here — out of scope): Dev
// Server Agent connections (agent.js over WebSocket, DevServerRelayBridge)
// do NOT feed this cache today. dev-server-manager.ts's bridge.onNotification
// re-emits every agent notification as a generic 'devServer:notification'
// event, but nothing subscribes to it to call agentHookServer.ingestRemote()
// the way ssh-relay-session.ts does for SSH. agent/src/relay/agent-hook-
// server.ts (RelayAgentHookServer) does hold its own per-agent last-status
// cache and forwards events via `forward()`, but nothing on the backend side
// consumes that forward for WS-connected Dev Servers. Until that notification
// wiring exists, agentStatus.getSnapshot will not show hook-driven status for
// panes hosted by a WS-connected Dev Server Agent (SSH- and WSL-hosted panes
// are unaffected). Fixing that is a separate, larger change (new
// dev-server-manager.ts notification subscriber), not part of this RPC-
// surface port.
import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import { agentHookServer, isValidPaneKey } from '../../../agent-hooks/server'
import {
  clearMigrationUnsupportedPtysByTabPrefix,
  clearMigrationUnsupportedPtysForPaneKey,
  getMigrationUnsupportedPtySnapshot
} from '../../../agent-hooks/migration-unsupported-pty-state'
import { isValidTerminalTabId } from '../../../../shared/terminal-tab-id'
import type { AgentInterruptInferenceRequest } from '../../../../shared/agent-interrupt-intent'
import type { AgentStatusIpcPayload } from '../../../../shared/agent-status-types'
import type { RpcContext } from '../core'

// Why: mirrors MAX_AGENT_STATUS_DROP_TAB_ID_LENGTH in desktop's
// ipc/agent-hooks.ts / rpc/methods/agent-status.ts — duplicated literal
// rather than a shared export, same reasoning as the desktop port.
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

// Why: ports desktop's ipc/agent-hooks.ts's enrichAgentStatusIpcPayload.
// Backend has no ipc/agent-hooks.ts equivalent (no ipcMain surface at all),
// so this is inlined here instead of extracted to a new ipc/ file for one
// caller. getAgentStatusOrchestrationContextForPaneKey resolves through
// getAgentStatusOrchestrationContextForHandle, which
// orca-runtime-terminal-agent-status.ts deliberately, unconditionally returns
// undefined from post-ADR-021 (its own caller chain wasn't verified safe to
// cascade async through) — so `orchestration` here is always omitted in
// server mode today. That is the same already-accepted degradation, not a
// new one introduced by this file.
function enrichAgentStatusIpcPayload(
  data: AgentStatusIpcPayload,
  runtime: RpcContext['runtime']
): AgentStatusIpcPayload {
  const terminalHandle = runtime.getAgentStatusTerminalHandleForPaneKey(data.paneKey)
  const orchestration = runtime.getAgentStatusOrchestrationContextForPaneKey(data.paneKey)
  return {
    ...data,
    ...(terminalHandle ? { terminalHandle } : {}),
    ...(orchestration ? { orchestration } : {})
  }
}

export const AGENT_STATUS_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'agentStatus.getSnapshot',
    params: null,
    // Why: the frontend pulls this after workspace hydration so startup
    // cannot lose replayed statuses while its local store is still empty —
    // same reasoning as desktop's ipcMain 'agentStatus:getSnapshot' handler.
    handler: (_params, { runtime }) =>
      agentHookServer.getStatusSnapshot().map((entry) => enrichAgentStatusIpcPayload(entry, runtime))
  }),
  defineMethod({
    name: 'agentStatus.inferInterrupt',
    params: AgentInterruptInferenceParams,
    // Why: mirrors desktop's ipcMain/rpc cast — the request shape is
    // validated above; AgentInterruptInferenceRequest's
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
