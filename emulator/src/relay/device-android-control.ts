// src/relay/device-android-control.ts
// Real Android device control via `adb shell input` — tap/swipe/keyevent —
// plus `adb emu kill` to power off an AVD. Fresh, self-contained
// implementation (same posture as device-android-discovery.ts): not a
// literal port of backend/src/main/emulator/android/android-input-commands.ts,
// which is entangled with EmulatorBridge's AndroidCommandRunner/AndroidSdkPaths
// plumbing this package doesn't have. Coordinates are raw device pixels (the
// wire params are `int32` — see backend-go's infrafleet.proto
// SendEmulatorTapRequest/SendEmulatorGestureRequest — not the 0..1 normalized
// space the *local* desktop EmulatorBridge/serve-sim path uses), so no
// `wm size` lookup is needed before a tap/swipe like the backend/ version does.
import type { ExecFileFn } from './device-list-handler'

export type AndroidCommandResult = { stdout: string; stderr: string; code: number }

function runAdb(execFileImpl: ExecFileFn, args: readonly string[]): Promise<AndroidCommandResult> {
  return new Promise((resolve) => {
    execFileImpl(
      'adb',
      [...args],
      { timeout: 15_000, maxBuffer: 4 * 1024 * 1024 },
      (error, stdout, stderr) => {
        const exitCode =
          error && typeof (error as { code?: unknown }).code === 'number'
            ? (error as { code: number }).code
            : error
              ? 1
              : 0
        resolve({
          stdout: stdout?.toString() ?? '',
          stderr: stderr?.toString() ?? '',
          code: exitCode
        })
      }
    )
  })
}

function ensureAdbOk(result: AndroidCommandResult, label: string): void {
  if (result.code !== 0) {
    throw new Error(`${label} failed: ${(result.stderr || result.stdout).trim() || 'unknown error'}`)
  }
}

export function androidShellArgs(serial: string, command: readonly string[]): string[] {
  return ['-s', serial, 'shell', ...command]
}

export function androidTapArgs(serial: string, x: number, y: number): string[] {
  return androidShellArgs(serial, ['input', 'tap', String(Math.round(x)), String(Math.round(y))])
}

const DEFAULT_SWIPE_DURATION_MS = 300

export function androidSwipeArgs(
  serial: string,
  startX: number,
  startY: number,
  endX: number,
  endY: number,
  durationMs?: number
): string[] {
  const duration = Math.max(1, Math.round(durationMs ?? DEFAULT_SWIPE_DURATION_MS))
  return androidShellArgs(serial, [
    'input',
    'swipe',
    String(Math.round(startX)),
    String(Math.round(startY)),
    String(Math.round(endX)),
    String(Math.round(endY)),
    String(duration)
  ])
}

// Android KeyEvent KEYCODE_* values, including the common aliases agents use
// — same vocabulary as backend/src/main/emulator/android/android-input-mapping.ts
// so a frontend/CLI caller written against either control path works unchanged.
const BUTTON_KEYCODES: Record<string, number> = {
  home: 3,
  back: 4,
  recents: 187,
  app_switch: 187,
  recent: 187,
  overview: 187,
  power: 26,
  lock: 26,
  volume_up: 24,
  volup: 24,
  volume_down: 25,
  voldown: 25
}

export function androidButtonKeycode(name: string): number {
  const keycode = BUTTON_KEYCODES[name]
  if (keycode === undefined) {
    throw new Error(`Unknown Android hardware button: ${name}`)
  }
  return keycode
}

export function androidButtonArgs(serial: string, name: string): string[] {
  return androidShellArgs(serial, ['input', 'keyevent', String(androidButtonKeycode(name))])
}

// `orientation` matches the frontend's rotate control vocabulary
// (use-emulator-pane-controls.ts): 'portrait' | 'landscape_left' |
// 'landscape_right' | 'portrait_upside_down'. Maps 1:1 to Android's
// Surface.ROTATION_* constants used by `settings put system user_rotation`.
function orientationToRotation(orientation: string): number {
  switch (orientation) {
    case 'landscape_left':
      return 1
    case 'portrait_upside_down':
      return 2
    case 'landscape_right':
      return 3
    default:
      return 0
  }
}

export function androidRotateArgs(serial: string, orientation: string): [string[], string[]] {
  return [
    androidShellArgs(serial, ['settings', 'put', 'system', 'accelerometer_rotation', '0']),
    androidShellArgs(serial, [
      'settings',
      'put',
      'system',
      'user_rotation',
      String(orientationToRotation(orientation))
    ])
  ]
}

export function androidShutdownArgs(serial: string): string[] {
  return ['-s', serial, 'emu', 'kill']
}

export async function androidTap(execFileImpl: ExecFileFn, serial: string, x: number, y: number): Promise<void> {
  ensureAdbOk(await runAdb(execFileImpl, androidTapArgs(serial, x, y)), 'adb tap')
}

export async function androidSwipe(
  execFileImpl: ExecFileFn,
  serial: string,
  startX: number,
  startY: number,
  endX: number,
  endY: number,
  durationMs?: number
): Promise<void> {
  ensureAdbOk(
    await runAdb(execFileImpl, androidSwipeArgs(serial, startX, startY, endX, endY, durationMs)),
    'adb swipe'
  )
}

export async function androidButtonPress(
  execFileImpl: ExecFileFn,
  serial: string,
  name: string
): Promise<void> {
  ensureAdbOk(await runAdb(execFileImpl, androidButtonArgs(serial, name)), 'adb button')
}

export async function androidRotate(
  execFileImpl: ExecFileFn,
  serial: string,
  orientation: string
): Promise<void> {
  const [disableAutoRotateArgs, setRotationArgs] = androidRotateArgs(serial, orientation)
  ensureAdbOk(await runAdb(execFileImpl, disableAutoRotateArgs), 'adb rotate')
  ensureAdbOk(await runAdb(execFileImpl, setRotationArgs), 'adb rotate')
}

export async function androidShutdown(execFileImpl: ExecFileFn, serial: string): Promise<void> {
  ensureAdbOk(await runAdb(execFileImpl, androidShutdownArgs(serial)), 'adb emu kill')
}
