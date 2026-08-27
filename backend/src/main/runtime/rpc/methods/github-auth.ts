import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { requiredString } from '../schemas'
import { getRemotePtyProvider } from '../../../ipc/pty'
import { buildPosixShellCommand } from '../../../../shared/posix-shell-quote'

// Why: GitHub CLI auth login runs interactively in a PTY (Device Flow).
// Spawning it on the remote Dev Server relay lets the user authenticate
// against GitHub while all credential files stay on their dev machine
// (in the Linux user home isolated by DevServerProvisioner).
const StartAuthLoginParams = z.object({
  devServerId: requiredString('devServerId is required'),
  host: z.string().optional()          // For GitHub Enterprise instances
})

const RevokeAuthParams = z.object({
  devServerId: requiredString('devServerId is required'),
  host: z.string().optional()
})

export const GITHUB_AUTH_METHODS: RpcMethod[] = [
  /**
   * Spawn `gh auth login` in a remote PTY on the Dev Server relay.
   * Returns a ptyId that the client subscribes to for terminal output.
   *
   * The gh CLI will print a Device Code that the user enters in their browser
   * to authorize the Orca app. Credentials are stored in ~/.config/gh on the
   * dev machine, isolated per Linux user by DevServerProvisioner.
   */
  defineMethod({
    name: 'github.startAuthLogin',
    params: StartAuthLoginParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error(
          'github.startAuthLogin requires Web Server mode (devServerManager not available)'
        )
      }
      // Why routed through the IPtyProvider registry instead of a raw
      // relay.call('pty.spawn', ...): 'pty.spawn' only exists on relay-ssh
      // dev servers — direct-websocket/relay-websocket (the default
      // connection mode) only register 'pty.create', which this bypassed
      // entirely, throwing MethodNotFound. getRemotePtyProvider() resolves
      // to whichever provider (SshPtyProvider/DevServerPtyProvider) the
      // connection type actually supports, and both now forward `command`/
      // `commandDelivery`/`userId` correctly. See
      // specs/agent/api/gaps-and-findings.md #5.
      const provider = getRemotePtyProvider(params.devServerId)
      if (!provider) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected. ` +
          `Connect to the dev server first.`
        )
      }

      // FIX BUG-BE-HLD-005: forward the authenticated user so the Agent can
      // namespace GH_CONFIG_DIR per user (see external-api-connector.ts buildGhEnv).
      const result = await provider.spawn({
        cols: 120,
        rows: 30,
        command: buildPosixShellCommand([
          'gh', 'auth', 'login',
          ...(params.host ? ['--hostname', params.host] : [])
        ]),
        commandDelivery: 'provider',
        ...(ctx.userId ? { userId: ctx.userId } : {})
      })

      return { ptyId: result.id, devServerId: params.devServerId }
    }
  }),

  /**
   * Spawn `gh auth logout` in a remote PTY on the Dev Server relay.
   * Returns a ptyId for the client to observe the logout output.
   */
  defineMethod({
    name: 'github.revokeAuth',
    params: RevokeAuthParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error(
          'github.revokeAuth requires Web Server mode (devServerManager not available)'
        )
      }
      const provider = getRemotePtyProvider(params.devServerId)
      if (!provider) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected.`
        )
      }

      // FIX BUG-BE-HLD-005: same per-user GH_CONFIG_DIR namespacing as startAuthLogin.
      const result = await provider.spawn({
        cols: 80,
        rows: 10,
        command: buildPosixShellCommand([
          'gh', 'auth', 'logout',
          ...(params.host ? ['--hostname', params.host] : [])
        ]),
        commandDelivery: 'provider',
        ...(ctx.userId ? { userId: ctx.userId } : {})
      })

      return { ptyId: result.id, devServerId: params.devServerId }
    }
  })
]
