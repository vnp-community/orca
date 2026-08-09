import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { requiredString } from '../schemas'

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
      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected. ` +
          `Connect to the dev server first.`
        )
      }

      const args = ['auth', 'login']
      if (params.host) {
        args.push('--hostname', params.host)
      }

      // FIX BUG-BE-HLD-005: forward the authenticated user so the Agent can
      // namespace GH_CONFIG_DIR per user (see external-api-connector.ts buildGhEnv).
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'gh',
        args,
        env: {},
        userId: ctx.userId,
        cols: 120,
        rows: 30
      })

      return { ptyId, devServerId: params.devServerId }
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
      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected.`
        )
      }

      const args = params.host
        ? ['auth', 'logout', '--hostname', params.host]
        : ['auth', 'logout']

      // FIX BUG-BE-HLD-005: same per-user GH_CONFIG_DIR namespacing as startAuthLogin.
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'gh',
        args,
        env: {},
        userId: ctx.userId,
        cols: 80,
        rows: 10
      })

      return { ptyId, devServerId: params.devServerId }
    }
  })
]
