import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import { OptionalString, requiredString } from '../schemas'
import type { Store } from '../../../persistence'
import type { LocalhostWorktreeLabelResult } from '../../../../shared/localhost-worktree-labels'
import { registerLocalhostWorktreeLabelRoute } from '../../../ipc/localhost-worktree-labels'

const LocalhostWorktreeLabelRouteParams = z.object({
  targetUrl: requiredString('Missing targetUrl'),
  projectName: requiredString('Missing projectName'),
  worktreeName: requiredString('Missing worktreeName'),
  worktreePath: OptionalString.nullable().optional(),
  repoId: OptionalString.nullable().optional(),
  worktreeId: OptionalString.nullable().optional()
})

export const LOCALHOST_WORKTREE_LABEL_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'localhostWorktreeLabels.register',
    params: LocalhostWorktreeLabelRouteParams,
    handler: async (params, { runtime }): Promise<LocalhostWorktreeLabelResult> => {
      // Why: RuntimeStore only narrows the store's static type — the instance
      // backing it is always the real Store (see getRuntimeStoreForRpc).
      const store = runtime.getRuntimeStoreForRpc() as unknown as Store | null
      if (!store) {
        throw new Error('runtime_unavailable')
      }
      return registerLocalhostWorktreeLabelRoute(store, params)
    }
  })
]
