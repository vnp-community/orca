import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'

// Why: unlike preflight.check, none of these 6 methods has a legitimate local
// (Orca-backend-container) fallback — "install the orca CLI" only makes sense
// on the machine that actually hosts a user's terminals. In server/web mode
// that machine is always a connected Dev Server, so devServerId is required
// rather than optional (contrast with preflight.ts's PreflightCheck schema).
const DevServerCliParams = z.object({
  devServerId: z.string().min(1)
})

const DevServerCliWslParams = z.object({
  devServerId: z.string().min(1),
  distro: z.string().min(1).nullable().optional()
})

const CLI_RELAY_TIMEOUT_MS = 30_000

function requireRelay(
  ctx: { devServerManager?: { getRelay(id: string): { call<T>(method: string, params: unknown, timeoutMs: number): Promise<T> } | null } },
  devServerId: string
) {
  if (!ctx.devServerManager) {
    throw new Error('Dev server manager is not available in this mode.')
  }
  const relay = ctx.devServerManager.getRelay(devServerId)
  if (!relay) {
    throw new Error(
      `Dev server '${devServerId}' relay is not connected. ` +
      `Connect to the dev server before managing the Orca CLI.`
    )
  }
  return relay
}

export const CLI_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'cli.getInstallStatus',
    params: DevServerCliParams,
    handler: async (params, ctx) => {
      const relay = requireRelay(ctx, params.devServerId)
      return relay.call<Record<string, unknown>>('cli.getInstallStatus', {}, CLI_RELAY_TIMEOUT_MS)
    }
  }),
  defineMethod({
    name: 'cli.install',
    params: DevServerCliParams,
    handler: async (params, ctx) => {
      const relay = requireRelay(ctx, params.devServerId)
      return relay.call<Record<string, unknown>>('cli.install', {}, CLI_RELAY_TIMEOUT_MS)
    }
  }),
  defineMethod({
    name: 'cli.remove',
    params: DevServerCliParams,
    handler: async (params, ctx) => {
      const relay = requireRelay(ctx, params.devServerId)
      return relay.call<Record<string, unknown>>('cli.remove', {}, CLI_RELAY_TIMEOUT_MS)
    }
  }),
  defineMethod({
    name: 'cli.getWslInstallStatus',
    params: DevServerCliWslParams,
    handler: async (params, ctx) => {
      const relay = requireRelay(ctx, params.devServerId)
      return relay.call<Record<string, unknown>>(
        'cli.getWslInstallStatus',
        { distro: params.distro },
        CLI_RELAY_TIMEOUT_MS
      )
    }
  }),
  defineMethod({
    name: 'cli.installWsl',
    params: DevServerCliWslParams,
    handler: async (params, ctx) => {
      const relay = requireRelay(ctx, params.devServerId)
      return relay.call<Record<string, unknown>>(
        'cli.installWsl',
        { distro: params.distro },
        CLI_RELAY_TIMEOUT_MS
      )
    }
  }),
  defineMethod({
    name: 'cli.removeWsl',
    params: DevServerCliWslParams,
    handler: async (params, ctx) => {
      const relay = requireRelay(ctx, params.devServerId)
      return relay.call<Record<string, unknown>>(
        'cli.removeWsl',
        { distro: params.distro },
        CLI_RELAY_TIMEOUT_MS
      )
    }
  })
]
