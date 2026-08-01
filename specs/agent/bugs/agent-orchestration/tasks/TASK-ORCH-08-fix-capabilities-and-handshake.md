# TASK-ORCH-08: Fix agent-session capabilities + agent-ws handshake

**Task ID:** TASK-ORCH-08  
**Priority:** 🔴 HIGH  
**Bugs fixed:** AWS-001  
**Estimated effort:** Small  
**Dependencies:** TASK-ORCH-05 (cleanupAllPtys import)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/agent-session.ts`

**Current code (L67):**
```typescript
capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees'] as const,
```

**Problem (AWS-001):** Missing `'pty'` capability in handshake message. Orca Server checks `capabilities` to determine if it can send PTY-related commands (`pty.spawn`, `pty.data`, etc.). Without `'pty'`, Orca Server assumes the relay cannot handle PTY operations and disables terminal functionality.

---

## Implementation

### Fix 1: Add `'pty'` to capabilities in `agent-session.ts`

```typescript
// OLD:
capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees'] as const,

// NEW:
capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees', 'pty'] as const,
```

---

## Verification

### Check handshake method name (AWS-001 secondary fix)

Before committing, run:
```bash
grep -n "AGENT_HANDSHAKE_METHOD\s*=" src/shared/agent-wire-protocol.ts
grep -rn "agent.handshake\|agent.hello\|AGENT_HANDSHAKE" src/main/dev-server/ | head -10
```

If the constant value and the server expectation differ → also fix `AGENT_HANDSHAKE_METHOD` value in `src/shared/agent-wire-protocol.ts`.

### TypeScript check
```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-session
```

### Manual verification
1. Start relay agent
2. Check handshake message in Orca Server logs — should include `"pty"` in capabilities array
3. Confirm Orca Server terminal pane can open (green terminal icon)

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-rpc-dispatch.ts: Handshake response bao gồm capabilities list. ws-handshake.ts: devServerId field thêm vào WsHandshakeInfo.  
**Tests:** Type checks pass. Handshake flow verified.  
