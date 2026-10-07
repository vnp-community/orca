import { JsonRpcRequest, JsonRpcResponse, makeError, makeNotifier } from './agent-rpc-dispatch'
import { AgentConfig } from './agent-config'
import { AgentLogger } from './agent-logger'
import { WireState } from 'orca-dev-agent-transport'
import WebSocket from 'ws'
import { CODEINTEL_METHODS, CodeIntelRequestContext, CodeIntelNotificationSink } from './codeintel-method-table'
import { setCodeIntelNotifier } from './codeintel-notification-sink'
import { toErrorPayload, CodeIntelError } from './codeintel-errors'
import { PerfCollector } from './codeintel-result-envelope'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

import { gateDisabled } from './codeintel/disabled-gate'
import { readRuntimeSwitches } from './codeintel/runtime-switches'

export async function dispatchCodeIntelRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  log: AgentLogger,
  ws: WebSocket,
  state: WireState
): Promise<JsonRpcResponse | null> {
  if (!rpc.method.startsWith('codeintel.')) {
    return null
  }

  const switches = readRuntimeSwitches(config?.toolEnv ?? process.env)
  const gated = gateDisabled(rpc, switches)
  if (gated) {
    return gated
  }

  const methodDef = CODEINTEL_METHODS[rpc.method]
  if (!methodDef) {
    return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Method not found: ${rpc.method}`)
  }

  try {
    const validatedParams = methodDef.validate(rpc.params)

    const ac = new AbortController()
    const timeoutId = setTimeout(() => ac.abort(), methodDef.timeoutMs)

    const sinkFn = makeNotifier(ws, state)
    setCodeIntelNotifier(sinkFn, ws)

    const notifier: CodeIntelNotificationSink = {
      notify: (method: string, params: any) => {
        sinkFn(method, params)
      }
    }

    const perf = new PerfCollector()
    perf.start()

    const ctx: CodeIntelRequestContext = {
      config,
      log,
      signal: ac.signal,
      deadline: Date.now() + methodDef.timeoutMs,
      notifier,
      perf
    }

    try {
      const result = await methodDef.handle(validatedParams, ctx)
      return { jsonrpc: '2.0', id: rpc.id, result }
    } finally {
      clearTimeout(timeoutId)
    }
  } catch (err: any) {
    if (!(err instanceof CodeIntelError)) {
      log.error(`Unhandled codeintel error: ${err.stack ?? err.message}`)
    }
    const payload = toErrorPayload(err)
    return makeError(rpc.id, payload.code, payload.message, payload.data)
  }
}
