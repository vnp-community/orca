import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import type { PersistedState } from '../../../../shared/types'
import { getActiveOnboardingStore } from '../../../ipc/onboarding-ipc'

const SetGitHub = z.object({ cache: z.unknown() })

// Why: ports desktop/src/main/runtime/rpc/methods/cache.ts. Desktop reuses a
// dedicated getCacheStoreForRpc() singleton set by registerSettingsHandlers;
// backend has no ipc/settings.ts equivalent, so this reuses
// getActiveOnboardingStore() instead — same single Store instance
// server-bootstrap.ts constructs (see telemetry.ts's identical reuse for the
// precedent), just under a name that predates this cache use.
export const CACHE_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'cache.getGitHub',
    params: null,
    handler: () => {
      const store = getActiveOnboardingStore()
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
      const store = getActiveOnboardingStore()
      if (!store) {
        throw new Error('runtime_unavailable')
      }
      store.setGitHubCache(params.cache as PersistedState['githubCache'])
    }
  })
]
