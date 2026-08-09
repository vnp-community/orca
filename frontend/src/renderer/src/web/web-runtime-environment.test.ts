// @vitest-environment happy-dom
import { describe, expect, it, beforeEach } from 'vitest'
import {
  saveStoredWebRuntimeEnvironment,
  readStoredWebRuntimeEnvironment,
  clearStoredWebRuntimeEnvironment,
  type StoredWebRuntimeEnvironment
} from './web-runtime-environment'
import { __resetSessionXorKeyForTests } from './web-runtime-environment-crypto'

function buildEnvironment(deviceToken: string): StoredWebRuntimeEnvironment {
  return {
    id: 'env-1',
    name: 'Test Orca Server',
    createdAt: 1,
    updatedAt: 1,
    lastUsedAt: null,
    runtimeId: null,
    preferredEndpointId: 'ws-env-1',
    endpoints: [
      {
        id: 'ws-env-1',
        kind: 'websocket',
        label: 'WebSocket',
        endpoint: 'wss://orca.example.com/ws',
        deviceToken,
        publicKeyB64: 'fake-public-key'
      }
    ]
  }
}

describe('web-runtime-environment — deviceToken storage (BUG-FE-HLD-001)', () => {
  beforeEach(() => {
    window.localStorage.clear()
    __resetSessionXorKeyForTests()
  })

  it('does not write the plaintext deviceToken to localStorage', () => {
    saveStoredWebRuntimeEnvironment(buildEnvironment('super-secret-device-token'))
    const raw = window.localStorage.getItem('orca.web.runtimeEnvironment.v1')
    expect(raw).not.toBeNull()
    expect(raw).not.toContain('super-secret-device-token')
  })

  it('round-trips the deviceToken back to plaintext when read in the same session', () => {
    saveStoredWebRuntimeEnvironment(buildEnvironment('super-secret-device-token'))
    const read = readStoredWebRuntimeEnvironment()
    expect(read?.endpoints[0]?.deviceToken).toBe('super-secret-device-token')
  })

  it('forces re-pairing (clears storage, returns null) when the session key is lost', () => {
    saveStoredWebRuntimeEnvironment(buildEnvironment('super-secret-device-token'))
    __resetSessionXorKeyForTests() // simulate reload — in-memory key is gone
    const read = readStoredWebRuntimeEnvironment()
    expect(read).toBeNull()
    expect(window.localStorage.getItem('orca.web.runtimeEnvironment.v1')).toBeNull()
  })

  it('handles an empty deviceToken (session-auth environment) without wrapping', () => {
    saveStoredWebRuntimeEnvironment(buildEnvironment(''))
    const read = readStoredWebRuntimeEnvironment()
    expect(read?.endpoints[0]?.deviceToken).toBe('')
  })

  it('clearStoredWebRuntimeEnvironment removes the stored entry', () => {
    saveStoredWebRuntimeEnvironment(buildEnvironment('token'))
    clearStoredWebRuntimeEnvironment()
    expect(readStoredWebRuntimeEnvironment()).toBeNull()
  })
})
