// src/relay/emulator-logger.ts
import type { EmulatorLogLevel } from './emulator-config'

export type EmulatorLogger = {
  debug(message: string): void
  info(message: string): void
  warn(message: string): void
  error(message: string): void
}

const LEVEL_ORDER: Record<EmulatorLogLevel, number> = { debug: 0, info: 1, warn: 2, error: 3 }

// Why: stdout is reserved for JSON-RPC responses in stdio debug mode
// (emulator-entry.ts) — all log output goes to stderr so piping stdout into
// a JSON parser never sees a stray log line.
export function createEmulatorLogger(level: EmulatorLogLevel): EmulatorLogger {
  const threshold = LEVEL_ORDER[level]
  const write = (lvl: EmulatorLogLevel, message: string): void => {
    if (LEVEL_ORDER[lvl] < threshold) return
    process.stderr.write(`[${new Date().toISOString()}] [${lvl}] ${message}\n`)
  }
  return {
    debug: (m) => write('debug', m),
    info: (m) => write('info', m),
    warn: (m) => write('warn', m),
    error: (m) => write('error', m)
  }
}
