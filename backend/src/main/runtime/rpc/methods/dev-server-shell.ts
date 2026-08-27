// ─── Dev Server shell-parity RPC methods ─────────────────────────────────────
// Split out of dev-server.ts (which was already at the max-lines budget) to
// keep additions self-contained. Web/server-mode counterparts of desktop's
// shell.pathExists/shell.readFile(dataUrl helper)/shell.copyFile — desktop
// operates on its own local filesystem, but in server/web mode the paths
// involved live on the connected Dev Server, so these relay to the agent
// instead (fs.stat / fs.readFile / fs.copyFile).
//
// Used by frontend/src/renderer/src/runtime/runtime-dev-server-shell-client.ts,
// itself consumed by DevServerFilePickerDialog callers (the web/server-mode
// replacement for the desktop-only shell.pick* native OS dialogs).
import { z } from 'zod'
import { defineMethod } from '../core'
import { Tracers } from '../../../../shared/trace/tracers'
import { wrapAgentError } from './dev-server-relay-error'

export const DEV_SERVER_SHELL_METHODS = [
  // ── devServer.pathExists ───────────────────────────────────────────────────
  defineMethod({
    name: 'devServer.pathExists',
    params: z.object({
      id: z.string().min(1),
      path: z.string().min(1)
    }),
    handler: async (params, ctx): Promise<boolean> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      const relay = ctx.devServerManager.getRelay(params.id)
      if (!relay) {throw new Error(`Dev server '${params.id}' is not connected.`)}

      const span = Tracers.pathExistsFlow.start({ devServerId: params.id, path: params.path })
      try {
        await relay.call<unknown>('fs.stat', { path: params.path }, 10_000)
        span.ok({ exists: true })
        return true
      } catch (err) {
        // fs.stat reports a clear "Not found: <path>" message for a missing
        // path — anything else is a real relay/agent failure and must not be
        // silently read as "does not exist".
        const message = err instanceof Error ? err.message : String(err)
        if (message.includes('Not found:')) {
          span.ok({ exists: false })
          return false
        }
        span.fail(err, { devServerId: params.id })
        wrapAgentError('fs.stat', err, params.id)
      }
    }
  }),

  // ── devServer.readFile ─────────────────────────────────────────────────────
  // Used to build the { dataUrl, fileName } result DevServerFilePickerDialog's
  // repo-icon caller needs — the picker itself only returns a path, so the
  // caller reads the picked file's bytes through this method afterward.
  defineMethod({
    name: 'devServer.readFile',
    params: z.object({
      id: z.string().min(1),
      path: z.string().min(1)
    }),
    handler: async (
      params,
      ctx
    ): Promise<{ content: string; encoding: 'utf-8' | 'base64'; isBinary: boolean }> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      const relay = ctx.devServerManager.getRelay(params.id)
      if (!relay) {throw new Error(`Dev server '${params.id}' is not connected.`)}

      const span = Tracers.readFileFlow.start({ devServerId: params.id, path: params.path })
      let result: { content: string; encoding: 'utf-8' | 'base64'; isBinary: boolean; path: string }
      try {
        result = await relay.call('fs.readFile', { path: params.path }, 15_000)
        span.ok({ path: result.path, bytes: result.content.length })
      } catch (err) {
        span.fail(err, { devServerId: params.id })
        wrapAgentError('fs.readFile', err, params.id)
      }
      return { content: result!.content, encoding: result!.encoding, isBinary: result!.isBinary }
    }
  }),

  // ── devServer.copyFile ─────────────────────────────────────────────────────
  // Web/server-mode counterpart of desktop's shell.copyFile (copyPickedFile in
  // desktop/src/main/ipc/shell.ts) — e.g. copying a DevServerFilePickerDialog-
  // picked image next to the markdown file that references it.
  defineMethod({
    name: 'devServer.copyFile',
    params: z.object({
      id: z.string().min(1),
      srcPath: z.string().min(1),
      destPath: z.string().min(1)
    }),
    handler: async (params, ctx): Promise<{ path: string }> => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      const relay = ctx.devServerManager.getRelay(params.id)
      if (!relay) {throw new Error(`Dev server '${params.id}' is not connected.`)}

      const span = Tracers.copyFileFlow.start({
        devServerId: params.id,
        srcPath: params.srcPath,
        destPath: params.destPath
      })
      let result: { ok: boolean; path: string }
      try {
        result = await relay.call<{ ok: boolean; path: string }>(
          'fs.copyFile',
          { srcPath: params.srcPath, destPath: params.destPath },
          15_000
        )
        span.ok({ path: result.path })
      } catch (err) {
        span.fail(err, { devServerId: params.id })
        wrapAgentError('fs.copyFile', err, params.id)
      }
      return { path: result!.path }
    }
  })
]
