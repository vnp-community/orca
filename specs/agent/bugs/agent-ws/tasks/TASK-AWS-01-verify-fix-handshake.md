# TASK-AWS-01: Verify + Fix Handshake Method Name + Capabilities

**Task ID:** TASK-AWS-01  
**Priority:** 🟡 MEDIUM  
**Bugs fixed:** AWS-001  
**Estimated effort:** Small (investigate first, then 1 constant change)  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**Files:**
- `src/shared/agent-wire-protocol.ts` — `AGENT_HANDSHAKE_METHOD` constant
- `src/relay/agent-session.ts` — uses constant, sets capabilities
- `src/main/dev-server/ws-handshake.ts` — server-side expectation (READ FIRST)

**Bug:** HLD BL-AWS-02 defines handshake as `{ type: 'agent.handshake' }`, but `agent-session.ts` sends `{ method: AGENT_HANDSHAKE_METHOD }` where `AGENT_HANDSHAKE_METHOD = 'agent.hello'`.

This is ONLY a bug if the server side expects `'agent.handshake'` literally.

---

## Step 1: Investigate Server Side (MANDATORY FIRST)

```bash
grep -n "agent.handshake\|agent.hello\|AGENT_HANDSHAKE" \
  src/main/dev-server/ws-handshake.ts \
  src/main/dev-server/agent-ws-server.ts 2>/dev/null | head -20
```

### If server expects `'agent.hello'` (Scenario A — NOT a bug):

```typescript
// Only fix capabilities list — do NOT change AGENT_HANDSHAKE_METHOD
// in src/relay/agent-session.ts, update capabilities:
capabilities: [
  'fs', 'git', 'preflight', 'ai.providers',
  'agent.spawn', 'worktrees', 'pty',          // ← 'pty' already added by TASK-ORCH-08
  'agent.exec',                                // ← ADD: now that TG-001 is fixed
  'git.worktree.list', 'git.worktree.add',     // ← ADD: now that WT-01 is fixed
] as const,
```

### If server expects `'agent.handshake'` (Scenario B — fix constant):

```typescript
// src/shared/agent-wire-protocol.ts:
// BEFORE:
export const AGENT_HANDSHAKE_METHOD = 'agent.hello'
// AFTER:
export const AGENT_HANDSHAKE_METHOD = 'agent.handshake'
```

> No other changes needed — `agent-session.ts` already uses the constant.

---

## Step 2: Update Capabilities (regardless of Scenario A or B)

The capabilities list in `src/relay/agent-session.ts` should reflect ALL registered RPC methods.

After all other tasks are completed, update to:

```typescript
capabilities: [
  // Core capabilities:
  'fs',              // fs.readDir, fs.readFile, etc.
  'git',             // git.exec, git.execStream
  'preflight',       // preflight checks
  // Agent operations:
  'agent.spawn',     // spawn interactive AI agent (PTY)
  'agent.exec',      // non-interactive execution (task graph)
  'agent.sendInput', // send input to running agent
  'agent.kill',      // kill agent PTY
  // Worktree support:
  'worktrees',                // generic worktree flag
  'git.worktree.list',        // list worktrees
  'git.worktree.add',         // create worktree
  // PTY support:
  'pty',             // PTY terminal management
  // Provider support:
  'ai.providers',    // AI credential store + health check
] as const,
```

---

## Step 3: Verify handshake flow end-to-end

```bash
# TypeScript check:
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep -E "agent-session|agent-wire-protocol"

# Start the relay:
ORCA_AGENT_TOKEN=test123 node dist/relay.js

# In Orca Server logs, look for:
# "Handshake OK: sessionId=..." (success)
# "Handshake failed: ..." (method mismatch → Scenario B confirmed)
```

---

## Decision Log

Record the findings from investigation here (fill in after Step 1):

| Question | Answer |
|----------|--------|
| `ws-handshake.ts` expects which method name? | [ ] `agent.hello` / [ ] `agent.handshake` |
| Is this Scenario A or B? | [ ] A (match) / [ ] B (mismatch) |
| Did we change `AGENT_HANDSHAKE_METHOD`? | [ ] No / [ ] Yes → old value: ___ |

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep -E "agent-session|agent-wire-protocol"
```

**Manual check:**
```bash
# In relay log after connection:
grep "Handshake" relay.log
# Expected: "Handshake OK" — not "Handshake failed"
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** Verified: method='agent.handshake' là đúng theo HLD v5. ws-session-router.ts và relay-handshake.ts sử dụng đúng method name. WsHandshakeInfo.devServerId thêm optional field.  
**Tests:** Handshake flow: relay-handshake → ws-session-router → agent-ws-server verified.  
