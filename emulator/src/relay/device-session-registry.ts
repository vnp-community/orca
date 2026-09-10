// src/relay/device-session-registry.ts
// Maps sessionId -> {deviceId, platform} for the lifetime of this agent
// process. `device.attach` creates an entry; `device.tap/gesture/button/
// rotate/shutdown` resolve one by sessionId (or by deviceId directly, for a
// caller that skipped attach). Deliberately in-memory only, no persistence —
// mirrors backend/src/main/emulator/emulator-session-registry.ts in spirit
// (one active mapping per key) but far smaller: this agent drives control
// only, it owns no stream/helper process lifecycle to track.
import { randomUUID } from 'node:crypto'
import type { EmulatorDeviceRow } from './device-list-handler'

export type DevicePlatform = EmulatorDeviceRow['platform']

export type DeviceSession = {
  sessionId: string
  deviceId: string
  platform: DevicePlatform
}

export class DeviceSessionRegistry {
  private readonly sessions = new Map<string, DeviceSession>()

  attach(deviceId: string, platform: DevicePlatform): DeviceSession {
    const session: DeviceSession = { sessionId: randomUUID(), deviceId, platform }
    this.sessions.set(session.sessionId, session)
    return session
  }

  get(sessionId: string): DeviceSession | undefined {
    return this.sessions.get(sessionId)
  }

  findByDeviceId(deviceId: string): DeviceSession | undefined {
    for (const session of this.sessions.values()) {
      if (session.deviceId === deviceId) {
        return session
      }
    }
    return undefined
  }

  removeByDeviceId(deviceId: string): void {
    for (const [sessionId, session] of this.sessions.entries()) {
      if (session.deviceId === deviceId) {
        this.sessions.delete(sessionId)
      }
    }
  }

  clear(): void {
    this.sessions.clear()
  }
}

// Module-level singleton — device.* dispatch handles one agent process per
// host, so sessions live as long as the process (same lifetime the JSON-RPC
// dispatcher itself assumes for e.g. keepalive/handshake state).
export const deviceSessionRegistry = new DeviceSessionRegistry()
