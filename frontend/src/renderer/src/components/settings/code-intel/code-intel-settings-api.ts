/**
 * code-intel-settings-api.ts — FE-CV-TASK-073-06
 *
 * Adapter between CodeIntelSettingsCard and the tenant-scoped settings RPCs. After each read the
 * shared support state is refreshed, because the Review tab (the only other poller) may be
 * closed while the switches change.
 *
 * @module components/settings/code-intel/code-intel-settings-api
 */

import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'
import { getCodeIntelClient } from '../../../runtime/code-intel-client'
import { CODE_INTEL_RPC_METHODS } from '../../../../../shared/code-intel-rpc-methods'
import { mapSettingsResult } from '../../../hooks/useCodeIntelSupport'
import type { CodeIntelSettings } from '../../../../../shared/code-intel-types'
import type { CodeIntelSettingsApi } from './CodeIntelSettingsCard'

async function callTenant(method: string, params: Record<string, unknown>): Promise<CodeIntelSettings> {
  const state = useAppStore.getState()
  // Tenant-scoped methods ignore the worktree; the environment only picks which server to ask.
  const res = await getCodeIntelClient().call('', method, params, {
    environmentId: getRuntimeEnvironmentIdForWorktree(state, state.activeWorktreeId)
  })
  if (!res.ok) {
    const code = res.error.code
    throw new Error(code ? `${code}: ${res.error.message}` : res.error.message)
  }
  return res.result as CodeIntelSettings
}

export function createCodeIntelSettingsApi(): CodeIntelSettingsApi {
  return {
    get: async () => {
      const settings = await callTenant(CODE_INTEL_RPC_METHODS.SETTINGS_GET, {})
      const store = useAppStore.getState()
      store.setCodeIntelSupportState(mapSettingsResult(settings, store.codeIntelSupportState))
      return settings
    },
    set: (patch) => callTenant(CODE_INTEL_RPC_METHODS.SETTINGS_SET, patch)
  }
}
