// src/relay/device-android-discovery.ts
// Probes for a usable Android SDK on this host — fresh, self-contained
// implementation (not a port of backend/src/main/emulator/android/*, which
// is entangled with EmulatorBridge/backends; see
// specs/emulator/bugs/missing-v1/solutions/SOL-EMU-003-device-capability-and-list-handlers.md).
import { existsSync } from 'node:fs'
import { homedir, platform } from 'node:os'
import { join } from 'node:path'

export type AndroidSdkInfo = {
  sdkFound: boolean
  sdkPath?: string
  message: string
}

function defaultCandidatePaths(): string[] {
  const home = homedir()
  switch (platform()) {
    case 'darwin':
      return [join(home, 'Library', 'Android', 'sdk')]
    case 'win32':
      return [join(process.env['LOCALAPPDATA'] ?? join(home, 'AppData', 'Local'), 'Android', 'Sdk')]
    default:
      return [join(home, 'Android', 'Sdk')]
  }
}

// A valid SDK root has platform-tools/adb (or at least the platform-tools
// directory, for a fresh install that hasn't downloaded adb yet).
function isValidSdkRoot(path: string): boolean {
  const adbName = platform() === 'win32' ? 'adb.exe' : 'adb'
  return existsSync(join(path, 'platform-tools', adbName)) || existsSync(join(path, 'platform-tools'))
}

export function discoverAndroidSdk(overridePath?: string | null): AndroidSdkInfo {
  const candidates = [
    overridePath,
    process.env['ANDROID_HOME'],
    process.env['ANDROID_SDK_ROOT'],
    ...defaultCandidatePaths()
  ].filter((path): path is string => Boolean(path))

  for (const path of candidates) {
    if (isValidSdkRoot(path)) {
      return { sdkFound: true, sdkPath: path, message: `Found at ${path}` }
    }
  }
  return {
    sdkFound: false,
    message: 'Android SDK not found. Install Android Studio, then create a Virtual Device.'
  }
}
