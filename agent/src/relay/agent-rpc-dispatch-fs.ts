// src/relay/agent-rpc-dispatch-fs.ts
// fs.* RPC methods — split out of agent-rpc-dispatch.ts's giant switch to
// keep each file under the oxlint max-lines budget.

import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { WireState } from 'orca-dev-agent-transport'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError, makeNotifier } from './agent-rpc-dispatch'

export async function dispatchFsRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  ws: WebSocket,
  state: WireState
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── v5.0: fs.readDir ─────────────────────────────────────────────────────
    case 'fs.readDir': {
      try {
        const { handleFsReadDir } = await import('./fs-agent-extensions')
        return (await handleFsReadDir(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.readDir unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.readFile ────────────────────────────────────────────────────
    case 'fs.readFile': {
      try {
        const { handleFsReadFile } = await import('./fs-agent-extensions')
        return (await handleFsReadFile(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.readFile unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.grep ────────────────────────────────────────────────────────
    case 'fs.grep': {
      try {
        const { handleFsGrep } = await import('./fs-agent-extensions')
        return (await handleFsGrep(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.grep unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.stat ────────────────────────────────────────────────────────
    case 'fs.stat': {
      try {
        const { handleFsStat } = await import('./fs-agent-extensions')
        return (await handleFsStat(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.stat unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.glob ────────────────────────────────────────────────────────
    case 'fs.glob': {
      try {
        const { handleFsGlob } = await import('./fs-agent-extensions')
        return (await handleFsGlob(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.glob unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.writeFile ───────────────────────────────────────────────────
    case 'fs.writeFile': {
      try {
        const { handleFsWriteFile } = await import('./fs-agent-extensions')
        return (await handleFsWriteFile(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.writeFile unavailable: ${msg}`)
      }
    }

    // ─── fs.copyFile (web/server-mode DevServerFilePickerDialog support) ────
    // Backs backend/src/main/runtime/rpc/methods/dev-server.ts's devServer.copyFile
    // — see agent-shell-handler.ts for the full rationale.
    case 'fs.copyFile': {
      try {
        const { handleFsCopyFile } = await import('./agent-shell-handler')
        return (await handleFsCopyFile(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.copyFile unavailable: ${msg}`)
      }
    }
    // ─── end fs.copyFile ─────────────────────────────────────────────────────

    // ── fs.mkdir ─────────────────────────────────────────────────────────────
    // Creates a directory (recursive) on the agent's filesystem.
    case 'fs.mkdir': {
      try {
        const { handleFsMkdir } = await import('./fs-agent-extensions')
        return (await handleFsMkdir(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.mkdir unavailable: ${msg}`)
      }
    }

    // ── fs.rmdir ─────────────────────────────────────────────────────────────
    // Removes an empty directory on the agent's filesystem.
    case 'fs.rmdir': {
      try {
        const { handleFsRmdir } = await import('./fs-agent-extensions')
        return (await handleFsRmdir(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.rmdir unavailable: ${msg}`)
      }
    }

    // ── fs.listDirectory ─────────────────────────────────────────────────────
    // Called by backend/src/main/ipc/repo-remote-ipc.ts's
    // repo.listRemoteDirectory/repo.scanRemote, { path, includeGitStatus? }.
    // Previously Part-B-only; see specs/agent/api/gaps-and-findings.md #5.
    case 'fs.listDirectory': {
      try {
        const { handleFsListDirectory } = await import('./fs-agent-directory-browse')
        return (await handleFsListDirectory(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.listDirectory unavailable: ${msg}`)
      }
    }

    // ── fs.watch ─────────────────────────────────────────────────────────────
    // Starts pushing `fs.changed` notifications for a path. Idempotent/refcounted.
    case 'fs.watch': {
      try {
        const { handleFsWatch } = await import('./fs-agent-extensions')
        return (await handleFsWatch(
          rpc.id,
          rpc.params ?? {},
          config,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.watch unavailable: ${msg}`)
      }
    }

    // ── fs.unwatch ───────────────────────────────────────────────────────────
    case 'fs.unwatch': {
      try {
        const { handleFsUnwatch } = await import('./fs-agent-extensions')
        return (await handleFsUnwatch(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.unwatch unavailable: ${msg}`)
      }
    }

    default:
      return null
  }
}
