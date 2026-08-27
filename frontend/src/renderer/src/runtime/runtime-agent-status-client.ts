// Why: mirrors runtime-git-client.ts's local-vs-environment routing for the
// 5 request/response agentStatus RPC methods (see
// desktop/src/main/runtime/rpc/methods/agent-status.ts). The push events
// (onSet/onClear/onMigrationUnsupported/onMigrationUnsupportedClear) are
// intentionally NOT wrapped here — they stay on window.api.agentStatus.on*
// exactly as-is; AgentHookServer's single-slot listener has no RPC-side
// push equivalent yet (see agent-status.ts's file header for why).
import type {
  AgentStatusIpcPayload,
  MigrationUnsupportedPtyEntry
} from '../../../shared/agent-status-types'
import type { AgentInterruptInferenceRequest } from '../../../shared/agent-interrupt-intent'
import { callRuntimeRpc, type RuntimeClientTarget } from './runtime-rpc-client'

const LOCAL_TARGET: RuntimeClientTarget = { kind: 'local' }

export async function getRuntimeAgentStatusSnapshot(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<AgentStatusIpcPayload[]> {
  if (target.kind === 'local') {
    return window.api.agentStatus.getSnapshot()
  }
  return callRuntimeRpc<AgentStatusIpcPayload[]>(target, 'agentStatus.getSnapshot')
}

export async function inferRuntimeAgentStatusInterrupt(
  request: AgentInterruptInferenceRequest,
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<boolean> {
  if (target.kind === 'local') {
    return window.api.agentStatus.inferInterrupt(request)
  }
  return callRuntimeRpc<boolean>(target, 'agentStatus.inferInterrupt', request)
}

export async function getRuntimeAgentStatusMigrationUnsupportedSnapshot(
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<MigrationUnsupportedPtyEntry[]> {
  if (target.kind === 'local') {
    return window.api.agentStatus.getMigrationUnsupportedSnapshot()
  }
  return callRuntimeRpc<MigrationUnsupportedPtyEntry[]>(
    target,
    'agentStatus.getMigrationUnsupportedSnapshot'
  )
}

export async function dropRuntimeAgentStatus(
  paneKey: string,
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<void> {
  if (target.kind === 'local') {
    window.api.agentStatus.drop(paneKey)
    return
  }
  await callRuntimeRpc<{ dropped: boolean }>(target, 'agentStatus.drop', { paneKey })
}

export async function dropRuntimeAgentStatusByTabPrefix(
  tabId: string,
  target: RuntimeClientTarget = LOCAL_TARGET
): Promise<void> {
  if (target.kind === 'local') {
    window.api.agentStatus.dropByTabPrefix(tabId)
    return
  }
  await callRuntimeRpc<{ dropped: boolean }>(target, 'agentStatus.dropByTabPrefix', { tabId })
}
