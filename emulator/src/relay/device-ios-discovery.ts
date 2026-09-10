// src/relay/device-ios-discovery.ts
// Probes for a usable iOS Simulator toolchain (Xcode command line tools) —
// darwin-only, matches backend/src/main/emulator/emulator-availability.ts's
// simctl check but without the EmulatorBridge dependency.
import { execFile } from 'node:child_process'
import { platform } from 'node:os'

export type IosToolchainInfo = {
  simctlOk: boolean
  message?: string
}

export type ExecFileFn = typeof execFile

function findSimctl(execFileImpl: ExecFileFn): Promise<boolean> {
  return new Promise((resolve) => {
    execFileImpl('xcrun', ['-find', 'simctl'], { timeout: 5000 }, (error) => {
      resolve(!error)
    })
  })
}

export async function discoverIosToolchain(
  execFileImpl: ExecFileFn = execFile,
  hostPlatform: NodeJS.Platform = platform()
): Promise<IosToolchainInfo> {
  if (hostPlatform !== 'darwin') {
    return { simctlOk: false, message: 'iOS Simulator requires macOS.' }
  }
  const found = await findSimctl(execFileImpl)
  if (!found) {
    return {
      simctlOk: false,
      message: 'Xcode command line tools not found. Install Xcode, then run `xcode-select --install`.'
    }
  }
  return { simctlOk: true }
}
