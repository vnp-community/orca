import { describe, expect, it } from 'vitest'
import { parseAdbDevicesOutput, parseSimctlListOutput } from './device-list-handler'

describe('parseAdbDevicesOutput', () => {
  it('parses adb devices -l output into EmulatorDeviceRow[]', () => {
    const output = [
      'List of devices attached',
      'emulator-5554  device product:sdk_gphone64_arm64 model:sdk_gphone64_arm64 device:emu64a transport_id:1',
      ''
    ].join('\n')

    const rows = parseAdbDevicesOutput(output)
    expect(rows).toEqual([
      { id: 'emulator-5554', name: 'sdk_gphone64_arm64', platform: 'android', state: 'device' }
    ])
  })

  it('returns an empty list for the header-only case (no devices attached)', () => {
    expect(parseAdbDevicesOutput('List of devices attached\n')).toEqual([])
  })
})

describe('parseSimctlListOutput', () => {
  it('parses simctl list devices -j output, skipping unavailable devices', () => {
    const json = JSON.stringify({
      devices: {
        'com.apple.CoreSimulator.SimRuntime.iOS-17-4': [
          { udid: 'ABCD', name: 'iPhone 15', state: 'Booted', isAvailable: true },
          { udid: 'EFGH', name: 'iPhone 8', state: 'Shutdown', isAvailable: false }
        ]
      }
    })

    const rows = parseSimctlListOutput(json)
    expect(rows).toEqual([{ id: 'ABCD', name: 'iPhone 15', platform: 'ios', state: 'Booted' }])
  })
})
