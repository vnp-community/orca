import { describe, expect, it } from 'vitest'
import { DeviceSessionRegistry } from './device-session-registry'

describe('DeviceSessionRegistry', () => {
  it('attach creates a session with a unique sessionId', () => {
    const registry = new DeviceSessionRegistry()
    const a = registry.attach('device-1', 'android')
    const b = registry.attach('device-2', 'ios')
    expect(a.sessionId).not.toBe(b.sessionId)
    expect(a.deviceId).toBe('device-1')
    expect(a.platform).toBe('android')
    expect(b.platform).toBe('ios')
  })

  it('get resolves a session by sessionId', () => {
    const registry = new DeviceSessionRegistry()
    const session = registry.attach('device-1', 'android')
    expect(registry.get(session.sessionId)).toEqual(session)
    expect(registry.get('unknown-session-id')).toBeUndefined()
  })

  it('findByDeviceId resolves the session for an already-attached device', () => {
    const registry = new DeviceSessionRegistry()
    const session = registry.attach('device-1', 'android')
    expect(registry.findByDeviceId('device-1')).toEqual(session)
    expect(registry.findByDeviceId('device-2')).toBeUndefined()
  })

  it('removeByDeviceId clears every session for that device', () => {
    const registry = new DeviceSessionRegistry()
    const session = registry.attach('device-1', 'android')
    registry.removeByDeviceId('device-1')
    expect(registry.get(session.sessionId)).toBeUndefined()
    expect(registry.findByDeviceId('device-1')).toBeUndefined()
  })

  it('clear removes all sessions', () => {
    const registry = new DeviceSessionRegistry()
    registry.attach('device-1', 'android')
    registry.attach('device-2', 'ios')
    registry.clear()
    expect(registry.findByDeviceId('device-1')).toBeUndefined()
    expect(registry.findByDeviceId('device-2')).toBeUndefined()
  })
})
