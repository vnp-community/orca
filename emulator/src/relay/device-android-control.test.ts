import { describe, expect, it } from 'vitest'
import {
  androidButtonArgs,
  androidButtonKeycode,
  androidRotateArgs,
  androidShellArgs,
  androidShutdownArgs,
  androidSwipeArgs,
  androidTapArgs
} from './device-android-control'

describe('androidTapArgs', () => {
  it('builds `adb -s <serial> shell input tap x y`, rounding fractional coordinates', () => {
    expect(androidTapArgs('emulator-5554', 100.4, 200.6)).toEqual([
      '-s',
      'emulator-5554',
      'shell',
      'input',
      'tap',
      '100',
      '201'
    ])
  })
})

describe('androidSwipeArgs', () => {
  it('builds `adb -s <serial> shell input swipe startX startY endX endY duration`', () => {
    expect(androidSwipeArgs('emulator-5554', 10, 20, 300, 400, 500)).toEqual([
      '-s',
      'emulator-5554',
      'shell',
      'input',
      'swipe',
      '10',
      '20',
      '300',
      '400',
      '500'
    ])
  })

  it('defaults duration to 300ms when not given', () => {
    expect(androidSwipeArgs('emulator-5554', 0, 0, 10, 10)).toEqual([
      '-s',
      'emulator-5554',
      'shell',
      'input',
      'swipe',
      '0',
      '0',
      '10',
      '10',
      '300'
    ])
  })
})

describe('androidButtonKeycode', () => {
  it('maps canonical names to Android KeyEvent codes', () => {
    expect(androidButtonKeycode('home')).toBe(3)
    expect(androidButtonKeycode('back')).toBe(4)
    expect(androidButtonKeycode('recents')).toBe(187)
    expect(androidButtonKeycode('power')).toBe(26)
    expect(androidButtonKeycode('volume_up')).toBe(24)
    expect(androidButtonKeycode('volume_down')).toBe(25)
  })

  it('accepts common aliases', () => {
    expect(androidButtonKeycode('app_switch')).toBe(187)
    expect(androidButtonKeycode('lock')).toBe(26)
    expect(androidButtonKeycode('volup')).toBe(24)
    expect(androidButtonKeycode('voldown')).toBe(25)
  })

  it('throws for an unknown button name', () => {
    expect(() => androidButtonKeycode('bogus')).toThrow('Unknown Android hardware button: bogus')
  })
})

describe('androidButtonArgs', () => {
  it('builds `adb -s <serial> shell input keyevent <code>`', () => {
    expect(androidButtonArgs('emulator-5554', 'back')).toEqual([
      '-s',
      'emulator-5554',
      'shell',
      'input',
      'keyevent',
      '4'
    ])
  })
})

describe('androidRotateArgs', () => {
  it('maps orientation names to Surface.ROTATION_* values', () => {
    expect(androidRotateArgs('emulator-5554', 'portrait')[1].at(-1)).toBe('0')
    expect(androidRotateArgs('emulator-5554', 'landscape_left')[1].at(-1)).toBe('1')
    expect(androidRotateArgs('emulator-5554', 'portrait_upside_down')[1].at(-1)).toBe('2')
    expect(androidRotateArgs('emulator-5554', 'landscape_right')[1].at(-1)).toBe('3')
  })

  it('always disables auto-rotate before setting the fixed rotation', () => {
    const [disableAutoRotate, setRotation] = androidRotateArgs('emulator-5554', 'portrait')
    expect(disableAutoRotate).toEqual([
      '-s',
      'emulator-5554',
      'shell',
      'settings',
      'put',
      'system',
      'accelerometer_rotation',
      '0'
    ])
    expect(setRotation).toEqual([
      '-s',
      'emulator-5554',
      'shell',
      'settings',
      'put',
      'system',
      'user_rotation',
      '0'
    ])
  })
})

describe('androidShutdownArgs', () => {
  it('builds `adb -s <serial> emu kill`', () => {
    expect(androidShutdownArgs('emulator-5554')).toEqual(['-s', 'emulator-5554', 'emu', 'kill'])
  })
})

describe('androidShellArgs', () => {
  it('prefixes a shell command with `-s <serial> shell`', () => {
    expect(androidShellArgs('emulator-5554', ['input', 'text', 'hi'])).toEqual([
      '-s',
      'emulator-5554',
      'shell',
      'input',
      'text',
      'hi'
    ])
  })
})
