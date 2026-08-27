import type { RelayDispatcher, RequestContext } from './dispatcher'
import { detectListeningPorts } from './port-scan-handler'

type KillPortParams = {
  worktreeId?: string
  pid?: number
  port?: number
}

type KillPortResult = {
  ok: boolean
  reason?: string
}

export class PortKillHandler {
  constructor(dispatcher: RelayDispatcher) {
    dispatcher.onRequest('ports.kill', async (params, context: RequestContext): Promise<KillPortResult> => {
      const { pid, port } = params as KillPortParams
      if (typeof pid !== 'number' || !Number.isSafeInteger(pid) || pid <= 0) {
        return { ok: false, reason: 'invalid pid' }
      }
      if (typeof port !== 'number' || !Number.isSafeInteger(port) || port <= 0) {
        return { ok: false, reason: 'invalid port' }
      }

      // Validate the pid is actually one ports.detect would report before
      // killing anything — mirrors the old TS system's
      // workspace-port-ownership.ts killWorkspacePort validation shape,
      // ported to the agent side since the agent (not backend-go) is what
      // can see the remote process table.
      const owns = await this.pidOwnsPort(pid, port, context.signal)
      if (!owns) {
        return { ok: false, reason: `pid ${pid} is not listening on port ${port}` }
      }

      try {
        process.kill(pid, 'SIGTERM')
        return { ok: true }
      } catch (error) {
        return { ok: false, reason: error instanceof Error ? error.message : String(error) }
      }
    })
  }

  private async pidOwnsPort(pid: number, port: number, signal?: AbortSignal): Promise<boolean> {
    // Reuses port-scan-handler.ts's own detection logic (detectListeningPorts
    // — extracted as a free function precisely so this handler doesn't
    // duplicate /proc parsing or the Windows netstat/PowerShell path).
    const detected = await detectListeningPorts(signal)
    return detected.some((d) => d.port === port && d.pid === pid)
  }
}

type PortsKillJsonRpcResponse =
  | { jsonrpc: '2.0'; id: string | number | null; result: KillPortResult }
  | { jsonrpc: '2.0'; id: string | number | null; error: { code: number; message: string } }

/**
 * handlePortsKill is ports.kill's entry point on the live JSON-RPC switch
 * dispatcher (agent-rpc-dispatch.ts) — the actual wire path
 * infra-fleet-service's usecase.KillWorkspacePort relays to (TASK-SSH-04-02).
 * Mirrors handlePortsDetect's "same validation, wired twice" shape so
 * neither dispatcher mechanism drifts from the other.
 */
export async function handlePortsKill(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<PortsKillJsonRpcResponse> {
  const { pid, port } = params as KillPortParams
  if (typeof pid !== 'number' || !Number.isSafeInteger(pid) || pid <= 0) {
    return { jsonrpc: '2.0', id, result: { ok: false, reason: 'invalid pid' } }
  }
  if (typeof port !== 'number' || !Number.isSafeInteger(port) || port <= 0) {
    return { jsonrpc: '2.0', id, result: { ok: false, reason: 'invalid port' } }
  }

  const detected = await detectListeningPorts()
  const owns = detected.some((d) => d.port === port && d.pid === pid)
  if (!owns) {
    return { jsonrpc: '2.0', id, result: { ok: false, reason: `pid ${pid} is not listening on port ${port}` } }
  }

  try {
    process.kill(pid, 'SIGTERM')
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (error) {
    return {
      jsonrpc: '2.0',
      id,
      result: { ok: false, reason: error instanceof Error ? error.message : String(error) }
    }
  }
}
