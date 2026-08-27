import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import type { PersistedState } from '../../../../shared/types'
import { getCacheStoreForRpc } from '../../../ipc/settings'

const SetGitHub = z.object({ cache: z.unknown() })

// Why: reuses the exact Store instance and getGitHubCache/setGitHubCache
// methods desktop/src/main/ipc/settings.ts's `cache:getGitHub` /
// `cache:setGitHub` ipcMain handlers call, via the module-level ref that
// registerSettingsHandlers populates at startup.
export const CACHE_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'cache.getGitHub',
    params: null,
    handler: () => {
      const store = getCacheStoreForRpc()
      if (!store) {
        throw new Error('runtime_unavailable')
      }
      return store.getGitHubCache()
    }
  }),
  defineMethod({
    name: 'cache.setGitHub',
    params: SetGitHub,
    handler: (params) => {
      const store = getCacheStoreForRpc()
      if (!store) {
        throw new Error('runtime_unavailable')
      }
      store.setGitHubCache(params.cache as PersistedState['githubCache'])
    }
  })
]
