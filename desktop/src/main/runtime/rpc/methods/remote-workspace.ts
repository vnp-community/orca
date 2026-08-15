import { z } from 'zod'
import { defineMethod, defineStreamingMethod, type RpcAnyMethod } from '../core'
import { requiredString } from '../schemas'
import type { Store } from '../../../persistence'
import type { WorkspaceSessionState } from '../../../../shared/types'
import {
  getRemoteWorkspaceClientId,
  getRemoteWorkspaceForTarget,
  listConnectedRemoteWorkspaceClients,
  listEnabledConnectedRemoteWorkspaceTargets,
  setRemoteWorkspaceForConnectedTargets
} from '../../../ipc/remote-workspace'
import { onRemoteWorkspaceChanged } from '../../../ipc/remote-workspace-change-bus'

// Why: monotonically increasing per-process counter, matching the
// notifications.subscribe pattern, avoids Date.now() collisions between
// near-simultaneous subscribe calls.
let remoteWorkspaceSubscriptionSeq = 0

const RemoteWorkspaceGetParams = z.object({
  targetId: requiredString('Missing targetId')
})

const RemoteWorkspaceSetForConnectedTargetsParams = z.object({
  session: z.unknown().optional(),
  hydratedTargetIds: z.array(z.string()).optional()
})

const RemoteWorkspaceListConnectedClientsParams = z
  .object({
    targetIds: z.array(z.string()).optional()
  })
  .nullable()
  .optional()

export const REMOTE_WORKSPACE_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'remoteWorkspace.get',
    params: RemoteWorkspaceGetParams,
    handler: async (params) => getRemoteWorkspaceForTarget(params)
  }),
  defineMethod({
    name: 'remoteWorkspace.setForConnectedTargets',
    params: RemoteWorkspaceSetForConnectedTargetsParams,
    handler: async (params, { runtime }) => {
      // Why: RuntimeStore only narrows the store's static type — the instance
      // backing it is always the real Store (see getRuntimeStoreForRpc).
      const store = runtime.getRuntimeStoreForRpc() as unknown as Store | null
      if (!store) {
        return []
      }
      return setRemoteWorkspaceForConnectedTargets(store, {
        session: params.session as WorkspaceSessionState | undefined,
        hydratedTargetIds: params.hydratedTargetIds
      })
    }
  }),
  defineMethod({
    name: 'remoteWorkspace.listEnabledConnectedTargets',
    params: null,
    handler: async () => listEnabledConnectedRemoteWorkspaceTargets()
  }),
  defineMethod({
    name: 'remoteWorkspace.listConnectedClients',
    params: RemoteWorkspaceListConnectedClientsParams,
    handler: async (params) => listConnectedRemoteWorkspaceClients(params ?? undefined)
  }),
  defineMethod({
    name: 'remoteWorkspace.clientId',
    params: null,
    handler: async () => getRemoteWorkspaceClientId()
  }),
  defineStreamingMethod({
    name: 'remoteWorkspace.subscribeChanged',
    params: null,
    handler: async (_params, { runtime, connectionId }, emit) => {
      await new Promise<void>((resolve) => {
        const unsubscribe = onRemoteWorkspaceChanged((event) => {
          emit(event)
        })

        const seq = ++remoteWorkspaceSubscriptionSeq
        const subscriptionId = `remote-workspace-${connectionId ?? 'inproc'}-${seq}`
        runtime.registerSubscriptionCleanup(
          subscriptionId,
          () => {
            unsubscribe()
            emit({ type: 'end' })
            resolve()
          },
          connectionId
        )

        emit({ type: 'ready', subscriptionId })
      })
    }
  })
]
