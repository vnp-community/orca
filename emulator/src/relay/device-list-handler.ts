// src/relay/device-list-handler.ts
// Backs `device.list` and `device.availability`. Field names
// ({id, name, platform, state}) match decodeEmulatorDevices in
// backend-go/services/infra-fleet-service/internal/usecase/emulator_relay.go
// verbatim — see specs/emulator/tdd/v1/02-device-rpc-catalog.md.
import { execFile } from 'node:child_process'
import { discoverAndroidSdk } from './device-android-discovery'
import { discoverIosToolchain } from './device-ios-discovery'
import { getDeviceCapabilities } from './device-capabilities-handler'

export type EmulatorDeviceRow = {
  id: string
  name: string
  platform: 'android' | 'ios'
  state: string
}

export type ExecFileFn = typeof execFile

function execFileAsync(execFileImpl: ExecFileFn, cmd: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    execFileImpl(cmd, args, { timeout: 8000, maxBuffer: 4 * 1024 * 1024 }, (error, stdout) => {
      if (error) {
        reject(error)
        return
      }
      resolve(stdout.toString())
    })
  })
}

// `adb devices -l` output (after the header line) looks like:
//   emulator-5554  device product:sdk_gphone64_arm64 model:sdk_gphone64_arm64 device:emu64a transport_id:1
export function parseAdbDevicesOutput(output: string): EmulatorDeviceRow[] {
  return output
    .split('\n')
    .slice(1)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [id, stateToken, ...rest] = line.split(/\s+/)
      const modelMatch = rest.join(' ').match(/model:(\S+)/)
      return {
        id: id ?? 'unknown',
        name: modelMatch?.[1] ?? id ?? 'unknown',
        platform: 'android' as const,
        state: stateToken ?? 'unknown'
      }
    })
}

async function listAndroidDevices(execFileImpl: ExecFileFn): Promise<EmulatorDeviceRow[]> {
  if (!discoverAndroidSdk().sdkFound) return []
  try {
    const output = await execFileAsync(execFileImpl, 'adb', ['devices', '-l'])
    return parseAdbDevicesOutput(output)
  } catch {
    return []
  }
}

type SimctlDevice = { udid: string; name: string; state: string; isAvailable?: boolean }
type SimctlListOutput = { devices: Record<string, SimctlDevice[]> }

export function parseSimctlListOutput(json: string): EmulatorDeviceRow[] {
  const parsed = JSON.parse(json) as SimctlListOutput
  const rows: EmulatorDeviceRow[] = []
  for (const runtimeDevices of Object.values(parsed.devices)) {
    for (const device of runtimeDevices) {
      if (device.isAvailable === false) continue
      rows.push({ id: device.udid, name: device.name, platform: 'ios', state: device.state })
    }
  }
  return rows
}

async function listIosDevices(execFileImpl: ExecFileFn): Promise<EmulatorDeviceRow[]> {
  const ios = await discoverIosToolchain(execFileImpl)
  if (!ios.simctlOk) return []
  try {
    const output = await execFileAsync(execFileImpl, 'xcrun', ['simctl', 'list', 'devices', '-j'])
    return parseSimctlListOutput(output)
  } catch {
    return []
  }
}

export async function listDevices(execFileImpl: ExecFileFn = execFile): Promise<EmulatorDeviceRow[]> {
  const [android, ios] = await Promise.all([listAndroidDevices(execFileImpl), listIosDevices(execFileImpl)])
  return [...android, ...ios]
}

export type AvailabilityResult = { available: boolean; reason?: string }

export async function getAvailability(execFileImpl: ExecFileFn = execFile): Promise<AvailabilityResult> {
  const devices = await listDevices(execFileImpl)
  if (devices.length > 0) {
    return { available: true }
  }
  const caps = await getDeviceCapabilities()
  if (caps.android.sdkFound || caps.ios.simctlOk) {
    return { available: false, reason: 'No devices found. Create an Android Virtual Device or iOS Simulator.' }
  }
  return { available: false, reason: caps.ios.message ?? caps.android.message }
}
