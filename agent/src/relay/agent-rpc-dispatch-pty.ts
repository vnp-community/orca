// src/relay/agent-rpc-dispatch-pty.ts
// pty.* RPC methods — split out of agent-rpc-dispatch.ts's giant switch to
// keep each file under the oxlint max-lines budget.
//
// Why all seven pty.* cases below pass makeNotifier(ws, state) (not just
// create/attach): PTYs now live in the detached pty-daemon process
// (pty-daemon-client.ts), which can push pty.data/pty.exit/pty.replay for
// ANY live PTY at any time, independent of which request last arrived —
// every dispatch call rebinds the client's "current notify" to the live
// WebSocket connection so a push always reaches it.

import type WebSocket from 'ws'
import type { AgentLogger } from './agent-logger'
import type { WireState } from 'orca-dev-agent-transport'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError, makeNotifier } from './agent-rpc-dispatch'

export async function dispatchPtyRpc(
  rpc: JsonRpcRequest,
  log: AgentLogger,
  ws: WebSocket,
  state: WireState
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── v5.0: pty.create ─────────────────────────────────────────────────────
    // TM-001/TM-006: Create a PTY session in agent mode.
    // Params: { cwd, cols?, rows?, env?, shellOverride? }
    // Returns: { id, cols, rows, cwd, shell }
    case 'pty.create': {
      try {
        const { handlePtyCreate } = await import('./pty-daemon-client')
        return (await handlePtyCreate(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.create unavailable: ${msg}`)
      }
    }

    // ── pty.attach ───────────────────────────────────────────────────────────
    // Reattach to a PTY that survived a WebSocket disconnect (grace period)
    // or an agent process restart (the pty-daemon process survives it).
    case 'pty.attach': {
      try {
        const { handlePtyAttach } = await import('./pty-daemon-client')
        return (await handlePtyAttach(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.attach unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.write ──────────────────────────────────────────────────────
    // Send input data to PTY stdin.
    // Params: { id, data }
    case 'pty.write': {
      try {
        const { handlePtyWrite } = await import('./pty-daemon-client')
        return (await handlePtyWrite(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.write unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.resize ─────────────────────────────────────────────────────
    // Resize PTY terminal window.
    // Params: { id, cols, rows }
    case 'pty.resize': {
      try {
        const { handlePtyResize } = await import('./pty-daemon-client')
        return (await handlePtyResize(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.resize unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.destroy ────────────────────────────────────────────────────
    // Close and cleanup a PTY session.
    // Params: { id, graceful? }
    case 'pty.destroy': {
      try {
        const { handlePtyDestroy } = await import('./pty-daemon-client')
        return (await handlePtyDestroy(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.destroy unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.scrollback ─────────────────────────────────────────────────
    // Get scrollback buffer content.
    // Params: { id, lines? }
    case 'pty.scrollback': {
      try {
        const { handlePtyScrollback } = await import('./pty-daemon-client')
        return (await handlePtyScrollback(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.scrollback unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.sendSignal ─────────────────────────────────────────────────
    // Send a signal to the PTY process (SIGTERM, SIGKILL, SIGINT, etc.).
    // Params: { id, signal }
    case 'pty.sendSignal': {
      try {
        const { handlePtySendSignal } = await import('./pty-daemon-client')
        return (await handlePtySendSignal(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.sendSignal unavailable: ${msg}`)
      }
    }

    // ── pty.listProcesses ────────────────────────────────────────────────────
    // Enumerate every PTY this daemon currently tracks. Backend's
    // DevServerPtyProvider.listProcesses() uses this so its liveness sweep can
    // detect a Dev-Server-hosted PTY that died without any client noticing
    // (BUG-FE-PTY-001) — previously there was no agent-wide enumeration RPC.
    case 'pty.listProcesses': {
      try {
        const { handlePtyListProcesses } = await import('./pty-daemon-client')
        return (await handlePtyListProcesses(
          rpc.id,
          rpc.params ?? {},
          log,
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `pty.listProcesses unavailable: ${msg}`
        )
      }
    }

    default:
      return null
  }
}
