// src/relay/device-control-handler.ts
// device.attach/tap/gesture/button/rotate/shutdown — Android is real (drives
// `adb shell input`/`adb emu kill` via device-android-control.ts). iOS
// control stays an honest stub: the only real driving mechanism in
// backend/src/main/emulator (backends/ios-emulator-backend.ts) shells out to
// the third-party `serve-sim` binary, which needs a real Xcode install, a
// signed private-framework camera-injection dylib, and (per its own error
// text) a booted Simulator.app with a live framebuffer to attach to — none
// of which exists in this sandbox to verify against, and "looks right but
// wrong in a subtle way" here means real users tapping the wrong thing on a
// real device. See
// specs/emulator/bugs/missing-v1/tasks/TASK-EMU-010-device-control-handlers-port.md
// for the full scoping rationale.
//
// Wire params match what backend-go's infra-fleet-service actually sends
// (server_emulator_host.go's SendEmulator*/RotateEmulator/ShutdownEmulator
// handlers building the `device.*` params map from typed proto fields) —
// NOT the `points`/`name` shape specs/emulator/tdd/v1/02-device-rpc-catalog.md
// describes, which predates that handler and was never updated to match it.
// Real params: device.tap {sessionId, x, y}; device.gesture {sessionId,
// startX, startY, endX, endY, durationMs}; device.button {sessionId, button}
// (accepting `name` too, matching the doc, in case a future caller uses it);
// device.rotate {sessionId, orientation}; device.shutdown {sessionId}. All
// four also accept `deviceId` instead of `sessionId` for a caller that
// skipped device.attach.
import { execFile } from 'node:child_process'
import type { ExecFileFn, EmulatorDeviceRow } from './device-list-handler'
import { listDevices } from './device-list-handler'
import {
  androidButtonPress,
  androidRotate,
  androidShutdown,
  androidSwipe,
  androidTap
} from './device-android-control'
import {
  deviceSessionRegistry,
  type DevicePlatform,
  type DeviceSession,
  type DeviceSessionRegistry
} from './device-session-registry'

// Each method throws a typed error carrying JSON-RPC's -32601 (Method Not
// Found) so infra-fleet-service's existing domain.ErrAgentMethodNotFound
// detection (devserveragent.Client.Exec) already handles this correctly —
// callers get a permanent INFRA_EMULATOR_UNSUPPORTED, not a timeout.
export const DEVICE_METHOD_NOT_FOUND_CODE = -32601

export class DeviceMethodNotImplementedError extends Error {
  readonly code = DEVICE_METHOD_NOT_FOUND_CODE

  constructor(method: string) {
    super(
      `${method} not implemented yet for iOS — Android device control is real (adb); iOS needs the serve-sim helper (Xcode + a signed camera-injection framework), which cannot be verified in this environment. See specs/emulator/bugs/missing-v1/tasks/TASK-EMU-010-device-control-handlers-port.md`
    )
    this.name = 'DeviceMethodNotImplementedError'
  }
}

type Params = Record<string, unknown> | undefined

function stringParam(params: Params, key: string): string | undefined {
  const value = params?.[key]
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

function numberParam(params: Params, key: string): number | undefined {
  const value = params?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

async function findDeviceRow(
  execFileImpl: ExecFileFn,
  deviceId: string
): Promise<EmulatorDeviceRow | undefined> {
  const devices = await listDevices(execFileImpl)
  return devices.find((device) => device.id === deviceId)
}

async function resolveTargetDevice(
  params: Params,
  execFileImpl: ExecFileFn,
  registry: DeviceSessionRegistry
): Promise<DeviceSession> {
  const sessionId = stringParam(params, 'sessionId')
  if (sessionId) {
    const session = registry.get(sessionId)
    if (!session) {
      throw new Error(`Unknown sessionId "${sessionId}" — call device.attach first`)
    }
    return session
  }
  const deviceId = stringParam(params, 'deviceId')
  if (!deviceId) {
    throw new Error('device.* requires sessionId or deviceId')
  }
  const existing = registry.findByDeviceId(deviceId)
  if (existing) {
    return existing
  }
  const row = await findDeviceRow(execFileImpl, deviceId)
  if (!row) {
    throw new Error(`Device not found: ${deviceId}`)
  }
  return { sessionId: '', deviceId, platform: row.platform }
}

function requireAndroid(platform: DevicePlatform, method: string): void {
  if (platform !== 'android') {
    throw new DeviceMethodNotImplementedError(method)
  }
}

export async function attach(
  params?: Params,
  execFileImpl: ExecFileFn = execFile,
  registry: DeviceSessionRegistry = deviceSessionRegistry
): Promise<{ sessionId: string; deviceId: string; platform: DevicePlatform }> {
  const deviceId = stringParam(params, 'deviceId')
  if (!deviceId) {
    throw new Error('device.attach requires deviceId')
  }
  const row = await findDeviceRow(execFileImpl, deviceId)
  if (!row) {
    throw new Error(`Device not found: ${deviceId}`)
  }
  const existing = registry.findByDeviceId(deviceId)
  const session = existing ?? registry.attach(deviceId, row.platform)
  return { sessionId: session.sessionId, deviceId: session.deviceId, platform: session.platform }
}

export async function tap(
  params?: Params,
  execFileImpl: ExecFileFn = execFile,
  registry: DeviceSessionRegistry = deviceSessionRegistry
): Promise<Record<string, never>> {
  const { deviceId, platform } = await resolveTargetDevice(params, execFileImpl, registry)
  requireAndroid(platform, 'device.tap')
  const x = numberParam(params, 'x')
  const y = numberParam(params, 'y')
  if (x === undefined || y === undefined) {
    throw new Error('device.tap requires numeric x, y')
  }
  await androidTap(execFileImpl, deviceId, x, y)
  return {}
}

export async function gesture(
  params?: Params,
  execFileImpl: ExecFileFn = execFile,
  registry: DeviceSessionRegistry = deviceSessionRegistry
): Promise<Record<string, never>> {
  const { deviceId, platform } = await resolveTargetDevice(params, execFileImpl, registry)
  requireAndroid(platform, 'device.gesture')
  const startX = numberParam(params, 'startX')
  const startY = numberParam(params, 'startY')
  const endX = numberParam(params, 'endX')
  const endY = numberParam(params, 'endY')
  if (startX === undefined || startY === undefined || endX === undefined || endY === undefined) {
    throw new Error('device.gesture requires numeric startX, startY, endX, endY')
  }
  const durationMs = numberParam(params, 'durationMs')
  await androidSwipe(execFileImpl, deviceId, startX, startY, endX, endY, durationMs)
  return {}
}

export async function button(
  params?: Params,
  execFileImpl: ExecFileFn = execFile,
  registry: DeviceSessionRegistry = deviceSessionRegistry
): Promise<Record<string, never>> {
  const { deviceId, platform } = await resolveTargetDevice(params, execFileImpl, registry)
  requireAndroid(platform, 'device.button')
  // Real backend-go param key is `button` (SendEmulatorButtonRequest.button);
  // `name` also accepted — see the file-header note on the doc/code mismatch.
  const name = stringParam(params, 'button') ?? stringParam(params, 'name')
  if (!name) {
    throw new Error('device.button requires a button/name string')
  }
  await androidButtonPress(execFileImpl, deviceId, name)
  return {}
}

export async function rotate(
  params?: Params,
  execFileImpl: ExecFileFn = execFile,
  registry: DeviceSessionRegistry = deviceSessionRegistry
): Promise<Record<string, never>> {
  const { deviceId, platform } = await resolveTargetDevice(params, execFileImpl, registry)
  requireAndroid(platform, 'device.rotate')
  const orientation = stringParam(params, 'orientation')
  if (!orientation) {
    throw new Error('device.rotate requires a string orientation')
  }
  await androidRotate(execFileImpl, deviceId, orientation)
  return {}
}

export async function shutdown(
  params?: Params,
  execFileImpl: ExecFileFn = execFile,
  registry: DeviceSessionRegistry = deviceSessionRegistry
): Promise<Record<string, never>> {
  const { deviceId, platform } = await resolveTargetDevice(params, execFileImpl, registry)
  requireAndroid(platform, 'device.shutdown')
  await androidShutdown(execFileImpl, deviceId)
  registry.removeByDeviceId(deviceId)
  return {}
}
