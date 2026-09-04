import { describe, expect, it } from 'vitest'
import { createEmulatorRpcDispatcher, type JsonRpcRequest } from './emulator-rpc-dispatch'
import { createEmulatorLogger } from './emulator-logger'

const log = createEmulatorLogger('error')
const dispatcher = createEmulatorRpcDispatcher(log)

function req(method: string, params?: Record<string, unknown>): JsonRpcRequest {
  return { jsonrpc: '2.0', id: 1, method, params }
}

describe('createEmulatorRpcDispatcher', () => {
  it('answers device.capabilities with a real probe result', async () => {
    const response = await dispatcher.dispatch(req('device.capabilities'))
    expect(response.error).toBeUndefined()
    expect(response.result).toMatchObject({ platform: expect.any(String) })
  })

  it('answers device.list with a devices array even with no devices connected', async () => {
    const response = await dispatcher.dispatch(req('device.list'))
    expect(response.error).toBeUndefined()
    expect(Array.isArray((response.result as { devices: unknown[] }).devices)).toBe(true)
  })

  it('answers device.tap with a real (not method-not-found) error when no device/session is given', async () => {
    // TASK-EMU-010: device.tap is ported (real adb) now — missing a target
    // device is a real validation error, not the old honest-stub -32601.
    const response = await dispatcher.dispatch(req('device.tap', { x: 1, y: 2 }))
    expect(response.result).toBeUndefined()
    expect(response.error?.code).not.toBe(-32601)
    expect(response.error?.message).toContain('sessionId or deviceId')
  })

  it('returns method-not-found for an unknown method', async () => {
    const response = await dispatcher.dispatch(req('device.bogus'))
    expect(response.error?.code).toBe(-32601)
  })
})
