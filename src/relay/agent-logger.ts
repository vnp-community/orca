// src/relay/agent-logger.ts
// Lightweight structured logger for the Orca Dev Agent process.
// Uses console.log/warn/error — output captured by systemd journald on dev server.

export interface AgentLogger {
  info(...args: unknown[]): void
  warn(...args: unknown[]): void
  error(...args: unknown[]): void
  debug(...args: unknown[]): void
}

/**
 * Create a logger that prefixes all messages with [agent] and ISO timestamp.
 * @param level - 'debug' enables verbose output; other values suppress debug lines
 */
export function createAgentLogger(level: string): AgentLogger {
  const ts = (): string => new Date().toISOString()
  return {
    info:  (...a: unknown[]) => console.log(`[agent] ${ts()} INFO  ${a.join(' ')}`),
    warn:  (...a: unknown[]) => console.warn(`[agent] ${ts()} WARN  ${a.join(' ')}`),
    error: (...a: unknown[]) => console.error(`[agent] ${ts()} ERROR ${a.join(' ')}`),
    debug: (...a: unknown[]) => {
      if (level === 'debug') console.log(`[agent] ${ts()} DEBUG ${a.join(' ')}`)
    },
  }
}
