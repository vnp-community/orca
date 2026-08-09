// ─── Dev Server RPC Methods ───────────────────────────────────────────────────
// Exposes DevServerManager CRUD + relay connection management over the WebSocket
// RPC channel so Web-mode browsers can manage dev servers without a local IPC bridge.
//
// CR-GH-001 / CR-GH-003 / CR-GH-005: preflight.check must route through relay;
// these methods give the browser the ability to list, add, connect, and
// disconnect dev servers that host the relay.
import { z } from 'zod'
import { defineMethod } from '../core'
import type { DevServer, DevServerInput } from '../../../../shared/dev-server-types'
import { Tracers } from '../../../../shared/trace/tracers'

// ─── Error helpers ────────────────────────────────────────────────────────────

/**
 * Classifies and re-throws errors from relay.call() with a clear prefix so
 * the UI can distinguish between:
 *   - Connection / backend errors  → "Dev server not connected"
 *   - Agent-side method errors     → "[Agent: fs.readDir] No such file..."
 *   - Transport timeouts           → "[Agent: fs.readDir] timed out..."
 *
 * The prefix is intentionally human-readable: it appears verbatim in the
 * RemoteFileBrowser error pane and any toast notifications.
 */
function wrapAgentError(agentMethod: string, err: unknown, devServerId: string): never {
  const raw = err instanceof Error ? err.message : String(err)
  const rawLower = raw.toLowerCase()

  // Relay / transport errors — not agent logic errors.
  // Why toLowerCase(): SshChannelMultiplexer may emit 'SSH connection lost...' or
  // 'Connection lost...' depending on caller context; we must catch both variants.
  const isConnectionError =
    raw === 'Not connected' ||
    rawLower.includes('connection lost') ||
    rawLower.includes('timed out') ||
    rawLower.includes('multiplexer disposed') ||
    rawLower.includes('ipc channel not available') ||
    rawLower.includes('ipc request timeout')

  const message = isConnectionError
    ? `Dev server '${devServerId}' connection error: ${raw}`
    : `[Agent: ${agentMethod}] ${raw}`

  throw new Error(message)
}

// ─── Param schemas ────────────────────────────────────────────────────────────

const DevServerIdParam = z.object({
  id: z.string().min(1)
})

const DevServerInputSchema = z.object({
  name: z.string().min(1),
  connectionType: z.enum(['relay-ssh', 'relay-websocket', 'direct-websocket']),
  sshTargetId: z.string().optional(),
  wsUrl: z.string().url().optional()
})

// ─── Methods ──────────────────────────────────────────────────────────────────

export const DEV_SERVER_METHODS = [
  // ── devServer.list ─────────────────────────────────────────────────────────
  // Returns all persisted dev servers with their runtime connection status.
  // Used by useDevServersSync() in Web mode to populate the store on mount.
  defineMethod({
    name: 'devServer.list',
    params: null,
    handler: async (_params, ctx): Promise<DevServer[]> => {
      if (!ctx.devServerManager) {return []}
      return ctx.devServerManager.list()
    }
  }),

  // ── devServer.add ──────────────────────────────────────────────────────────
  // Persists a new dev server record. Does NOT connect automatically.
  // The caller follows up with devServer.connect after add().
  defineMethod({
    name: 'devServer.add',
    params: DevServerInputSchema,
    handler: async (params, ctx): Promise<DevServer> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      const input: DevServerInput = {
        name: params.name,
        connectionType: params.connectionType,
        sshTargetId: params.sshTargetId,
        wsUrl: params.wsUrl
      }
      return ctx.devServerManager.add(input)
    }
  }),

  // ── devServer.remove ───────────────────────────────────────────────────────
  // Disconnects relay (if connected) and removes the persisted record.
  defineMethod({
    name: 'devServer.remove',
    params: DevServerIdParam,
    handler: async (params, ctx): Promise<void> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      await ctx.devServerManager.remove(params.id)
    }
  }),

  // ── devServer.connect ──────────────────────────────────────────────────────
  // Opens the SSH relay connection for a persisted dev server.
  // Returns the updated DevServer with status='connected' on success.
  // Throws if the server is not found or the relay fails to connect.
  defineMethod({
    name: 'devServer.connect',
    params: DevServerIdParam,
    handler: async (params, ctx): Promise<DevServer> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      await ctx.devServerManager.connect(params.id)
      const server = ctx.devServerManager.get(params.id)
      if (!server) {throw new Error(`Dev server not found after connect: ${params.id}`)}
      return server
    }
  }),

  // ── devServer.disconnect ───────────────────────────────────────────────────
  // Closes the relay connection, leaving the persisted record intact.
  defineMethod({
    name: 'devServer.disconnect',
    params: DevServerIdParam,
    handler: async (params, ctx): Promise<void> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      await ctx.devServerManager.disconnect(params.id)
    }
  }),

  // ── devServer.testConnection ───────────────────────────────────────────────
  // Validates connectivity without persisting the dev server entry.
  // Used by AddDevServerDialog before committing.
  defineMethod({
    name: 'devServer.testConnection',
    params: DevServerInputSchema,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      const input: DevServerInput = {
        name: params.name,
        connectionType: params.connectionType,
        sshTargetId: params.sshTargetId,
        wsUrl: params.wsUrl
      }
      return ctx.devServerManager.testConnection(input)
    }
  }),

  // ── devServer.listSshTargets ───────────────────────────────────────────────
  // Lists SSH targets from the local SshConnectionStore (SQLite-backed).
  // Used by AddDevServerDialog in web mode — bypasses the ipc/ssh.ts
  // registration layer which is only active in Electron desktop mode.
  defineMethod({
    name: 'devServer.listSshTargets',
    params: null,
    handler: async () => {
      const { SshConnectionStore } = await import('../../../ssh/ssh-connection-store')
      const store = new SshConnectionStore()
      return { targets: store.listTargets() }
    }
  }),

  // ── devServer.addSshTarget ─────────────────────────────────────────────────
  // Creates a new SSH target in the local SshConnectionStore.
  // Used by AddDevServerDialog when the user provides a new host inline.
  // Returns the newly created SshTarget (with generated id).
  defineMethod({
    name: 'devServer.addSshTarget',
    params: z.object({
      label: z.string().min(1),
      host: z.string().min(1),
      port: z.number().int().min(1).max(65535).default(22),
      username: z.string().min(1).default('ubuntu')
    }),
    handler: async (params) => {
      const { SshConnectionStore } = await import('../../../ssh/ssh-connection-store')
      const store = new SshConnectionStore()
      const target = store.addTarget({
        label: params.label,
        host: params.host,
        port: params.port,
        username: params.username,
        source: 'manual'
      })
      return { target }
    }
  }),

  // ── devServer.browseDir ────────────────────────────────────────────────────
  // Lists directory entries on a connected dev server via the agent relay.
  // Used by the web-mode folder picker (RemoteFileBrowser) when the selected
  // host is a dev server (devserver:<id>) rather than an Orca runtime environment.
  //
  // Why: pickFolder/pickFolders are Electron-only (native OS dialog). In web
  // mode with a Dev Server host, we need to browse the remote filesystem via
  // the agent's fs.readDir tool. This method bridges the gap without requiring
  // a full Orca runtime environment pairing.
  //
  // NOTE: Agent fs.readDir returns { path, entries: [{ path, name, type }] }
  // We must map to RemoteFileBrowser format: { resolvedPath, entries: [{ name, isDirectory, isSymlink }] }
  defineMethod({
    name: 'devServer.browseDir',
    params: z.object({
      id: z.string().min(1),
      path: z.string().default('~')
    }),
    handler: async (params, ctx): Promise<{ resolvedPath: string; entries: { name: string; isDirectory: boolean; isSymlink: boolean }[] }> => {
      const span = Tracers.browseDirFlow.start({ devServerId: params.id, path: params.path })

      if (!ctx.devServerManager) {
        span.fail('DevServerManager unavailable')
        throw new Error('DevServerManager unavailable')
      }

      const relay = ctx.devServerManager.getRelay(params.id)
      if (!relay) {
        span.fail(`relay not found for devServerId=${params.id}`)
        throw new Error(`Dev server '${params.id}' is not connected. Connect it first.`)
      }

      // Log relay session state so we can immediately tell if session is null
      const sessionState = (relay as { session?: unknown }).session ? 'connected' : 'null'
      span.step('relay-lookup', { devServerId: params.id, session: sessionState })

      // ── Resolve ~ → absolute path ──────────────────────────────────────────
      // Agent fs.readDir requires absolute path; fs.stat is used to probe the
      // most common home-dir locations on Linux dev servers.
      let remotePath = params.path

      if (remotePath === '~' || remotePath.startsWith('~/')) {
        const candidates = ['/home/ubuntu', '/root', '/home/ec2-user', '/home/user']
        let homeDir = '/home/ubuntu' // safe default

        for (const candidate of candidates) {
          try {
            const statResult = await relay.call<{ exists?: boolean; isDirectory?: boolean }>(
              'fs.stat',
              { path: candidate },
              5_000
            )
            if (statResult && statResult.isDirectory !== false) {
              homeDir = candidate
              break
            }
          } catch {
            // fs.stat probe failed for this candidate — try next
          }
        }

        remotePath = remotePath === '~' ? homeDir : homeDir + remotePath.slice(1)
        span.step('resolve-home', { homeDir, remotePath })
      }

      // ── Call agent fs.readDir ──────────────────────────────────────────────
      // Agent accepts: { path: string, depth?: number }
      // Agent returns: { path: string, entries: Array<{ path, name, type, size? }> }
      type AgentReadDirResult = {
        path: string
        entries: { path: string; name: string; type: 'file' | 'directory'; size?: number }[]
      }

      span.step('agent-call', { method: 'fs.readDir', remotePath })

      let agentResult: AgentReadDirResult
      try {
        agentResult = await relay.call<AgentReadDirResult>(
          'fs.readDir',
          { path: remotePath, depth: 1 },
          15_000
        )
      } catch (err) {
        span.fail(err, { method: 'fs.readDir', devServerId: params.id })
        wrapAgentError('fs.readDir', err, params.id)
      }

      const rawEntries = Array.isArray(agentResult!.entries) ? agentResult!.entries : []
      span.ok({ resolvedPath: agentResult!.path, entries: rawEntries.length })

      // ── Map agent format → RemoteFileBrowser format ────────────────────────
      return {
        resolvedPath: agentResult!.path ?? remotePath,
        entries: rawEntries
          .filter(e => e.name && e.name !== '.' && e.name !== '..')
          .map(e => ({
            name: e.name,
            isDirectory: e.type === 'directory',
            isSymlink: false // agent does not expose symlink info currently
          }))
          .sort((a, b) => {
            if (a.isDirectory !== b.isDirectory) {return a.isDirectory ? -1 : 1}
            return a.name.localeCompare(b.name)
          })
      }
    }
  }),

  // ── devServer.mkdir ────────────────────────────────────────────────────────
  // Creates a new directory on a connected dev server via the agent relay.
  defineMethod({
    name: 'devServer.mkdir',
    params: z.object({
      id: z.string().min(1),
      path: z.string().min(1)
    }),
    handler: async (params, ctx): Promise<{ path: string }> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      const relay = ctx.devServerManager.getRelay(params.id)
      if (!relay) {throw new Error(`Dev server '${params.id}' is not connected.`)}

      const span = Tracers.mkdirFlow.start({ devServerId: params.id, path: params.path })
      let result: { path: string }
      try {
        result = await relay.call<{ path: string }>('fs.mkdir', { path: params.path }, 10_000)
        span.ok({ path: result.path })
      } catch (err) {
        span.fail(err, { devServerId: params.id })
        wrapAgentError('fs.mkdir', err, params.id)
      }
      return result!
    }
  }),

  // ── devServer.rmdir ────────────────────────────────────────────────────────
  // Removes a directory on a connected dev server via the agent relay.
  defineMethod({
    name: 'devServer.rmdir',
    params: z.object({
      id: z.string().min(1),
      path: z.string().min(1)
    }),
    handler: async (params, ctx): Promise<void> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      const relay = ctx.devServerManager.getRelay(params.id)
      if (!relay) {throw new Error(`Dev server '${params.id}' is not connected.`)}

      const span = Tracers.rmdirFlow.start({ devServerId: params.id, path: params.path })
      try {
        await relay.call<void>('fs.rmdir', { path: params.path }, 10_000)
        span.ok()
      } catch (err) {
        span.fail(err, { devServerId: params.id })
        wrapAgentError('fs.rmdir', err, params.id)
      }
    }
  })
]
