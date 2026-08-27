import type { GlobalSettings } from '../../../shared/types'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'
import type {
  RemoteWorkspaceChangedEvent,
  RemoteWorkspaceConnectedClient,
  RemoteWorkspacePatchResult,
  RemoteWorkspaceSnapshot
} from '../../../shared/remote-workspace-types'
import type { WorkspaceSessionState } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type RemoteWorkspaceSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

// Why: window.api.remoteWorkspace is absent on web/non-Electron builds
// (existing call sites already guard with `window.api.remoteWorkspace?.`) —
// preserve that no-op-safe behavior for the local branch.
export function getRemoteWorkspace(
  settings: RemoteWorkspaceSettings | null | undefined,
  args: { targetId: string }
): Promise<RemoteWorkspaceSnapshot | null> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.remoteWorkspace ? window.api.remoteWorkspace.get(args) : Promise.resolve(null)
  }
  return callRuntimeRpc<RemoteWorkspaceSnapshot | null>(target, 'remoteWorkspace.get', args)
}

export function setRemoteWorkspaceForConnectedTargets(
  settings: RemoteWorkspaceSettings | null | undefined,
  args: { session?: WorkspaceSessionState; hydratedTargetIds?: string[] }
): Promise<{ targetId: string; result: RemoteWorkspacePatchResult }[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.remoteWorkspace
      ? window.api.remoteWorkspace.setForConnectedTargets(args)
      : Promise.resolve([])
  }
  return callRuntimeRpc<{ targetId: string; result: RemoteWorkspacePatchResult }[]>(
    target,
    'remoteWorkspace.setForConnectedTargets',
    args
  )
}

export function listEnabledConnectedRemoteWorkspaceTargets(
  settings: RemoteWorkspaceSettings | null | undefined
): Promise<string[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.remoteWorkspace
      ? window.api.remoteWorkspace.listEnabledConnectedTargets()
      : Promise.resolve([])
  }
  return callRuntimeRpc<string[]>(target, 'remoteWorkspace.listEnabledConnectedTargets')
}

export function listConnectedRemoteWorkspaceClients(
  settings: RemoteWorkspaceSettings | null | undefined,
  args?: { targetIds?: string[] }
): Promise<{ targetId: string; clients: RemoteWorkspaceConnectedClient[] }[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.remoteWorkspace
      ? window.api.remoteWorkspace.listConnectedClients(args)
      : Promise.resolve([])
  }
  return callRuntimeRpc<{ targetId: string; clients: RemoteWorkspaceConnectedClient[] }[]>(
    target,
    'remoteWorkspace.listConnectedClients',
    args
  )
}

export function getRemoteWorkspaceClientId(
  settings: RemoteWorkspaceSettings | null | undefined
): Promise<string | null> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.remoteWorkspace ? window.api.remoteWorkspace.clientId() : Promise.resolve(null)
  }
  return callRuntimeRpc<string>(target, 'remoteWorkspace.clientId')
}

function isRemoteWorkspaceChangedEvent(value: unknown): value is RemoteWorkspaceChangedEvent {
  return (
    typeof value === 'object' &&
    value !== null &&
    'targetId' in value &&
    'snapshot' in value &&
    typeof (value as { targetId: unknown }).targetId === 'string'
  )
}

// Why: local mode keeps the existing always-on preload listener. An
// environment target subscribes to remoteWorkspace.subscribeChanged, which
// forwards the exact same RemoteWorkspaceChangedEvent objects
// handleRemoteWorkspaceNotification pushes locally (see
// desktop/src/main/ipc/remote-workspace-change-bus.ts).
export function subscribeToRemoteWorkspaceChanged(
  settings: RemoteWorkspaceSettings | null | undefined,
  callback: (event: RemoteWorkspaceChangedEvent) => void
): () => void {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    if (!window.api.remoteWorkspace) {
      return () => {}
    }
    return window.api.remoteWorkspace.onChanged(callback)
  }
  let handle: { unsubscribe: () => void } | null = null
  let cancelled = false
  window.api.runtimeEnvironments
    .subscribe(
      { selector: target.environmentId, method: 'remoteWorkspace.subscribeChanged' },
      {
        onResponse: (response) => {
          const payload = (response as RuntimeRpcResponse<unknown>).ok
            ? (response as { result: unknown }).result
            : undefined
          if (isRemoteWorkspaceChangedEvent(payload)) {
            callback(payload)
          }
        }
      }
    )
    .then((subscription) => {
      handle = subscription
      if (cancelled) {
        subscription.unsubscribe()
      }
    })
    .catch(() => {})
  return () => {
    cancelled = true
    handle?.unsubscribe()
  }
}
