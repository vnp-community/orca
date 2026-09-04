// src/relay/emulator-config.ts
// Config for the Orca Mobile Emulator Agent, read from env vars — mirrors
// agent/src/relay/agent-config.ts's convention (see
// specs/emulator/tdd/v1/04-deployment.md for the env var table).

export type EmulatorLogLevel = 'debug' | 'info' | 'warn' | 'error'

export type EmulatorConfig = {
  /** Gateway WebSocket URL — unused until TASK-EMU-006 wires the real transport. */
  backendUrl?: string
  /** Registration token from the pairing flow (F28, --kind=mobile-emulator). */
  agentToken?: string
  logLevel: EmulatorLogLevel
  /** Overrides auto-discovery in device-android-discovery.ts. */
  androidSdkPath?: string
}

function isLogLevel(value: string): value is EmulatorLogLevel {
  return value === 'debug' || value === 'info' || value === 'warn' || value === 'error'
}

export function loadEmulatorConfig(env: NodeJS.ProcessEnv = process.env): EmulatorConfig {
  const rawLevel = env['ORCA_EMULATOR_LOG_LEVEL'] ?? 'info'
  return {
    backendUrl: env['ORCA_BACKEND_URL'],
    agentToken: env['ORCA_AGENT_TOKEN'],
    logLevel: isLogLevel(rawLevel) ? rawLevel : 'info',
    androidSdkPath: env['ORCA_ANDROID_SDK_PATH']
  }
}
