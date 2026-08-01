# TASK-ORCH-06: Fix relay-websocket Hardcoded Token

**Task ID:** TASK-ORCH-06  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** ORCH-013  
**Estimated effort:** Small  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/agent-connection-relay.ts`

**Current code (L26, L33):**
```typescript
const token = config.agentToken || 'relay-secret'  // ❌ hardcoded fallback

// ...
log.info(`Orca UI config → URL: ...?token=${token}`)  // ❌ token in logs
```

**Problems:**
1. **Security:** Any unauthenticated client can connect using `relay-secret` if admin forgets to set `ORCA_AGENT_TOKEN`
2. **Security:** Token logged to stdout/file — exposed in CI logs, monitoring systems, log aggregators
3. **Operational:** No clear error message → admin doesn't know WHY connection fails

---

## Implementation

### Replace the token setup block in `listenRelay()`

```typescript
export async function listenRelay(
  config: AgentConfig,
  tools:  ToolDefinition[],
  log:    AgentLogger,
): Promise<never> {
  // ORCH-013: Token is mandatory — no fallback allowed.
  const token = config.agentToken?.trim()
  if (!token) {
    log.error('FATAL: agentToken (ORCA_AGENT_TOKEN) is not set or is empty.')
    log.error('relay-websocket mode requires a shared secret for authentication.')
    log.error('Fix: on the Dev Server, run:')
    log.error('  export ORCA_AGENT_TOKEN=$(openssl rand -hex 32)')
    log.error('  node ~/orca-agent/agent.js')
    process.exit(1)
  }

  return new Promise<never>((_, reject) => {
    const wss = new WebSocketServer({ port: config.agentPort, path: '/orca-relay' })

    wss.once('listening', () => {
      log.info(`✅ Relay ready: ws://0.0.0.0:${config.agentPort}/orca-relay`)
      // ORCH-013: Never log the token
      log.info(`Orca UI → Dev Server settings: Type=relay-websocket, URL=ws://<host>:${config.agentPort}/orca-relay`)
      log.info(`Token: use the value of ORCA_AGENT_TOKEN from this machine`)
    })
    // ... rest unchanged
  })
}
```

---

## What NOT to change

- Authentication logic (`authenticate()` function) — keep as-is
- WebSocket server setup — keep as-is
- Connection handling — keep as-is

---

## Tests to update/add

File: `src/relay/__tests__/agent-connection-relay.test.ts`

The existing tests may fail because they set `agentToken: ''` in mock configs, which will now trigger `process.exit(1)`. Update tests that test the relay without a token to either:
1. Mock `process.exit`
2. Or provide a valid token in mock config

```typescript
// Update MOCK_CONFIG in tests:
const MOCK_CONFIG_WITH_TOKEN: AgentConfig = {
  ...MOCK_CONFIG,
  agentToken: 'test-secret-token',
}

// Test for missing token:
it('exits process when agentToken not set', () => {
  const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => { throw new Error('exit') })
  expect(() => listenRelay({ ...config, agentToken: '' }, [], log)).rejects.toThrow()
  exitSpy.mockRestore()
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-connection-relay

# Manual:
# 1. ORCA_AGENT_TOKEN="" node dist/relay.js → must log FATAL and exit(1)
# 2. ORCA_AGENT_TOKEN="abc" node dist/relay.js → must start listening
# 3. grep "token" relay.log → token must NOT appear in logs
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-connection-relay.ts: Đọc agentToken từ config.agentToken (không hardcode). Nếu token rỗng, log FATAL và không kết nối.  
**Tests:** Verified via config flow và ORCA_AGENT_TOKEN env var.  
