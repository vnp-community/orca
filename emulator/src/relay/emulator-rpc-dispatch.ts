// src/relay/emulator-rpc-dispatch.ts
// JSON-RPC 2.0 dispatch table for device.* — mirrors the shape of
// agent/src/relay/agent-rpc-dispatch.ts's dispatcher (dispatch(rpc) =>
// Promise<JsonRpcResponse>) WITHOUT importing anything from agent/ (see
// specs/emulator/tdd/v1/03-transport-reuse-analysis.md for why not yet).
import { getDeviceCapabilities } from './device-capabilities-handler'
import { getAvailability, listDevices } from './device-list-handler'
import {
  DeviceMethodNotImplementedError,
  attach,
  button,
  gesture,
  rotate,
  shutdown,
  tap
} from './device-control-handler'
import type { EmulatorLogger } from './emulator-logger'

export type JsonRpcId = string | number | null

export type JsonRpcRequest = {
  jsonrpc: '2.0'
  id: JsonRpcId
  method: string
  params?: Record<string, unknown>
}

export type JsonRpcResponse = {
  jsonrpc: '2.0'
  id: JsonRpcId
  result?: unknown
  error?: { code: number; message: string }
}

const METHOD_NOT_FOUND = -32601
const INTERNAL_ERROR = -32000

function ok(id: JsonRpcId, result: unknown): JsonRpcResponse {
  return { jsonrpc: '2.0', id, result }
}

function fail(id: JsonRpcId, code: number, message: string): JsonRpcResponse {
  return { jsonrpc: '2.0', id, error: { code, message } }
}

export type EmulatorRpcDispatcher = {
  dispatch(rpc: JsonRpcRequest): Promise<JsonRpcResponse>
}

export function createEmulatorRpcDispatcher(log: EmulatorLogger): EmulatorRpcDispatcher {
  return {
    async dispatch(rpc: JsonRpcRequest): Promise<JsonRpcResponse> {
      try {
        switch (rpc.method) {
          case 'device.capabilities':
            return ok(
              rpc.id,
              await getDeviceCapabilities((rpc.params?.['androidSdkPath'] as string | undefined) ?? null)
            )
          case 'device.list':
            return ok(rpc.id, { devices: await listDevices() })
          case 'device.availability':
            return ok(rpc.id, await getAvailability())
          case 'device.attach':
            return ok(rpc.id, await attach(rpc.params))
          case 'device.tap':
            return ok(rpc.id, await tap(rpc.params))
          case 'device.gesture':
            return ok(rpc.id, await gesture(rpc.params))
          case 'device.button':
            return ok(rpc.id, await button(rpc.params))
          case 'device.rotate':
            return ok(rpc.id, await rotate(rpc.params))
          case 'device.shutdown':
            return ok(rpc.id, await shutdown(rpc.params))
          default:
            return fail(rpc.id, METHOD_NOT_FOUND, `method not found: ${rpc.method}`)
        }
      } catch (error) {
        if (error instanceof DeviceMethodNotImplementedError) {
          return fail(rpc.id, error.code, error.message)
        }
        const message = error instanceof Error ? error.message : String(error)
        log.error(`dispatch failed for ${rpc.method}: ${message}`)
        return fail(rpc.id, INTERNAL_ERROR, message)
      }
    }
  }
}
