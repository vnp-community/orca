# TASK-02: Create src/relay/agent-logger.ts

**Phase:** 1  
**SOL Ref:** SOL-03  
**Estimated time:** 15m  
**Precondition:** Không có  

---

## Tạo file mới: `src/relay/agent-logger.ts`

```typescript
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
```

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-logger" || echo "No errors for agent-logger"
```

## Definition of Done

- [x] `src/relay/agent-logger.ts` created
- [x] `pnpm run typecheck:node` passes (no new errors)
