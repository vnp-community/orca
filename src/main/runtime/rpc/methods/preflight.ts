import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import {
  detectRemoteAgents,
  detectRemoteWindowsTerminalCapabilities,
  detectInstalledAgentsWithShellPathHydration,
  refreshShellPathAndDetectAgents,
  runPreflightCheck
} from '../../../ipc/preflight'

// Why: In Web mode (ORCA_MULTI_USER=1), preflight.check routes to the relay on the
// target Dev Server instead of running locally on the Orca Server container.
// The optional devServerId disambiguates between local and remote execution.
const PreflightCheck = z.object({
  force: z.boolean().optional(),
  devServerId: z.string().optional()
})
const PreflightDetectRemoteAgents = z.object({
  connectionId: z.string().min(1)
})
const PreflightDetectRemoteWindowsTerminalCapabilities = z.object({
  connectionId: z.string().min(1)
})

export const PREFLIGHT_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'preflight.check',
    params: PreflightCheck,
    handler: async (params, ctx) => {
      // Web mode: proxy CLI check to the relay running on the Dev Server.
      // The relay's preflight.check runs gh/glab/git on the actual dev machine.
      if (params.devServerId && ctx.devServerManager) {
        const relay = ctx.devServerManager.getRelay(params.devServerId)
        if (!relay) {
          throw new Error(
            `Dev server '${params.devServerId}' relay is not connected. ` +
            `Connect to the dev server before running preflight check.`
          )
        }
        // Delegate the full CLI check to the relay.
        // The relay's PreflightHandler.checkFullPreflight() returns
        // platform + gh + glab + git status.
        const result = await relay.call<Record<string, unknown>>(
          'preflight.check',
          {},
          30_000
        )
        return result
      }

      // Local mode: run preflight check on the Orca Server host (Electron or local dev).
      return runPreflightCheck(params.force)
    }
  }),
  defineMethod({
    name: 'preflight.detectAgents',
    params: null,
    handler: async () => detectInstalledAgentsWithShellPathHydration()
  }),
  defineMethod({
    name: 'preflight.detectRemoteAgents',
    params: PreflightDetectRemoteAgents,
    handler: async (params) => detectRemoteAgents(params)
  }),
  defineMethod({
    name: 'preflight.detectRemoteWindowsTerminalCapabilities',
    params: PreflightDetectRemoteWindowsTerminalCapabilities,
    handler: async (params) => detectRemoteWindowsTerminalCapabilities(params)
  }),
  defineMethod({
    name: 'preflight.refreshAgents',
    params: null,
    handler: async () => refreshShellPathAndDetectAgents()
  })
]
