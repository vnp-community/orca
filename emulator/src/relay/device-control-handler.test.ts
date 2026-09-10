import { describe, expect, it, vi, beforeEach } from 'vitest'
import { attach, button, gesture, rotate, shutdown, tap } from './device-control-handler'
import { DeviceSessionRegistry } from './device-session-registry'
import type { ExecFileFn } from './device-list-handler'

// device.attach resolves the device's platform via listDevices(), which
// gates the Android branch on discoverAndroidSdk()'s node:fs probe — mock it
// the same way device-android-discovery.test.ts does so attach() sees a
// "found" SDK regardless of what's actually installed on this host.
const existsSyncMock = vi.fn<(path: string) => boolean>()
vi.mock('node:fs', () => ({
  existsSync: (path: string) => existsSyncMock(path)
}))

type Call = { cmd: string; args: string[] }

function fakeAdbExecFile(options: { devicesOutput?: string; fail?: boolean } = {}): {
  execFileImpl: ExecFileFn
  calls: Call[]
} {
  const calls: Call[] = []
  const execFileImpl = ((
    cmd: string,
    args: string[],
    _opts: unknown,
    callback: (error: Error | null, stdout?: string, stderr?: string) => void
  ) => {
    calls.push({ cmd, args })
    if (cmd === 'adb' && args[0] === 'devices') {
      callback(
        null,
        options.devicesOutput ??
          'List of devices attached\nemulator-5554\tdevice product:sdk_gphone64_arm64 model:sdk_gphone64_arm64 device:emu64a transport_id:1\n'
      )
      return
    }
    if (cmd === 'xcrun') {
      // Real host is non-darwin in CI, so device-ios-discovery short-circuits
      // before ever calling execFileImpl for this — kept only as a safety net.
      callback(new Error('not on darwin'))
      return
    }
    if (options.fail) {
      callback(Object.assign(new Error('adb failed'), { code: 1 }), '', 'permission denied')
      return
    }
    callback(null, '', '')
  }) as unknown as ExecFileFn
  return { execFileImpl, calls }
}

function withAndroidSession(options: { fail?: boolean } = {}): {
  execFileImpl: ExecFileFn
  calls: Call[]
  registry: DeviceSessionRegistry
  sessionId: string
} {
  const { execFileImpl, calls } = fakeAdbExecFile(options)
  const registry = new DeviceSessionRegistry()
  const session = registry.attach('emulator-5554', 'android')
  return { execFileImpl, calls, registry, sessionId: session.sessionId }
}

describe('device-control-handler', () => {
  beforeEach(() => {
    existsSyncMock.mockReset()
    existsSyncMock.mockReturnValue(true)
  })

  describe('attach', () => {
    it('creates a session for a known android device and records its platform', async () => {
      const { execFileImpl } = fakeAdbExecFile()
      const registry = new DeviceSessionRegistry()
      const result = await attach({ deviceId: 'emulator-5554' }, execFileImpl, registry)
      expect(result.deviceId).toBe('emulator-5554')
      expect(result.platform).toBe('android')
      expect(result.sessionId).toBeTruthy()
      expect(registry.get(result.sessionId)).toBeDefined()
    })

    it('reuses the existing session on a second attach to the same device', async () => {
      const { execFileImpl } = fakeAdbExecFile()
      const registry = new DeviceSessionRegistry()
      const first = await attach({ deviceId: 'emulator-5554' }, execFileImpl, registry)
      const second = await attach({ deviceId: 'emulator-5554' }, execFileImpl, registry)
      expect(second.sessionId).toBe(first.sessionId)
    })

    it('throws for an unknown device', async () => {
      const { execFileImpl } = fakeAdbExecFile({ devicesOutput: 'List of devices attached\n' })
      const registry = new DeviceSessionRegistry()
      await expect(attach({ deviceId: 'nope' }, execFileImpl, registry)).rejects.toThrow(
        'Device not found'
      )
    })

    it('throws when deviceId is missing', async () => {
      const { execFileImpl } = fakeAdbExecFile()
      const registry = new DeviceSessionRegistry()
      await expect(attach({}, execFileImpl, registry)).rejects.toThrow(
        'device.attach requires deviceId'
      )
    })
  })

  describe('tap', () => {
    it('sends `adb shell input tap` at the given coordinates', async () => {
      const { execFileImpl, calls, registry, sessionId } = withAndroidSession()
      await tap({ sessionId, x: 100, y: 200 }, execFileImpl, registry)
      const tapCall = calls.find((c) => c.args.includes('tap'))
      expect(tapCall?.args).toEqual(['-s', 'emulator-5554', 'shell', 'input', 'tap', '100', '200'])
    })

    it('resolves the target device directly by deviceId when no session exists', async () => {
      const { execFileImpl, calls } = fakeAdbExecFile()
      const registry = new DeviceSessionRegistry()
      await tap({ deviceId: 'emulator-5554', x: 5, y: 6 }, execFileImpl, registry)
      const tapCall = calls.find((c) => c.args.includes('tap'))
      expect(tapCall?.args).toEqual(['-s', 'emulator-5554', 'shell', 'input', 'tap', '5', '6'])
    })

    it('throws a clear error when neither sessionId nor deviceId is given', async () => {
      const { execFileImpl } = fakeAdbExecFile()
      const registry = new DeviceSessionRegistry()
      await expect(tap({ x: 1, y: 1 }, execFileImpl, registry)).rejects.toThrow(
        'sessionId or deviceId'
      )
    })

    it('throws when x/y are missing or non-numeric', async () => {
      const { execFileImpl, registry, sessionId } = withAndroidSession()
      await expect(tap({ sessionId }, execFileImpl, registry)).rejects.toThrow(
        'requires numeric x, y'
      )
    })

    it('propagates a real adb failure instead of swallowing it', async () => {
      const { execFileImpl, registry, sessionId } = withAndroidSession({ fail: true })
      await expect(tap({ sessionId, x: 1, y: 1 }, execFileImpl, registry)).rejects.toThrow(
        'adb tap failed: permission denied'
      )
    })

    it('throws DeviceMethodNotImplementedError (-32601) for an iOS session', async () => {
      const registry = new DeviceSessionRegistry()
      const session = registry.attach('ABCD-1234', 'ios')
      const { execFileImpl } = fakeAdbExecFile()
      await expect(
        tap({ sessionId: session.sessionId, x: 1, y: 1 }, execFileImpl, registry)
      ).rejects.toMatchObject({ code: -32601, name: 'DeviceMethodNotImplementedError' })
    })
  })

  describe('gesture', () => {
    it('sends `adb shell input swipe` between the start/end points', async () => {
      const { execFileImpl, calls, registry, sessionId } = withAndroidSession()
      await gesture(
        { sessionId, startX: 10, startY: 20, endX: 30, endY: 40, durationMs: 500 },
        execFileImpl,
        registry
      )
      const swipeCall = calls.find((c) => c.args.includes('swipe'))
      expect(swipeCall?.args).toEqual([
        '-s',
        'emulator-5554',
        'shell',
        'input',
        'swipe',
        '10',
        '20',
        '30',
        '40',
        '500'
      ])
    })

    it('throws when any coordinate is missing', async () => {
      const { execFileImpl, registry, sessionId } = withAndroidSession()
      await expect(
        gesture({ sessionId, startX: 10, startY: 20, endX: 30 }, execFileImpl, registry)
      ).rejects.toThrow('requires numeric startX, startY, endX, endY')
    })
  })

  describe('button', () => {
    it('sends the correct keyevent code for the real wire key `button`', async () => {
      const { execFileImpl, calls, registry, sessionId } = withAndroidSession()
      await button({ sessionId, button: 'back' }, execFileImpl, registry)
      const keyeventCall = calls.find((c) => c.args.includes('keyevent'))
      expect(keyeventCall?.args).toEqual([
        '-s',
        'emulator-5554',
        'shell',
        'input',
        'keyevent',
        '4'
      ])
    })

    it('accepts `name` as a fallback for the doc-described shape', async () => {
      const { execFileImpl, calls, registry, sessionId } = withAndroidSession()
      await button({ sessionId, name: 'home' }, execFileImpl, registry)
      const keyeventCall = calls.find((c) => c.args.includes('keyevent'))
      expect(keyeventCall?.args).toEqual([
        '-s',
        'emulator-5554',
        'shell',
        'input',
        'keyevent',
        '3'
      ])
    })

    it('throws for an unknown button name', async () => {
      const { execFileImpl, registry, sessionId } = withAndroidSession()
      await expect(
        button({ sessionId, button: 'bogus' }, execFileImpl, registry)
      ).rejects.toThrow('Unknown Android hardware button: bogus')
    })
  })

  describe('rotate', () => {
    it('sets accelerometer_rotation off then user_rotation to the mapped value', async () => {
      const { execFileImpl, calls, registry, sessionId } = withAndroidSession()
      await rotate({ sessionId, orientation: 'landscape_left' }, execFileImpl, registry)
      const settingsCalls = calls.filter((c) => c.args.includes('settings'))
      expect(settingsCalls).toHaveLength(2)
      expect(settingsCalls[0]?.args).toEqual([
        '-s',
        'emulator-5554',
        'shell',
        'settings',
        'put',
        'system',
        'accelerometer_rotation',
        '0'
      ])
      expect(settingsCalls[1]?.args).toEqual([
        '-s',
        'emulator-5554',
        'shell',
        'settings',
        'put',
        'system',
        'user_rotation',
        '1'
      ])
    })
  })

  describe('shutdown', () => {
    it('runs `adb emu kill` and clears the session', async () => {
      const { execFileImpl, calls, registry, sessionId } = withAndroidSession()
      await shutdown({ sessionId }, execFileImpl, registry)
      const killCall = calls.find((c) => c.args.includes('kill'))
      expect(killCall?.args).toEqual(['-s', 'emulator-5554', 'emu', 'kill'])
      expect(registry.get(sessionId)).toBeUndefined()
    })
  })
})
