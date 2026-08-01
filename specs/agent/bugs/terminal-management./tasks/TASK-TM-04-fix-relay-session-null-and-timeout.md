# TASK-TM-04: Fix Relay Session Null + PTY Spawn Timeout

**Task ID:** TASK-TM-04  
**Priority:** 🔴 HIGH  
**Bugs fixed:** TRM-001, TRM-002  
**Estimated effort:** Small  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/main/dev-server/dev-server-relay-bridge.ts`

### Bug TRM-001 — Unhelpful "Not connected" error

**Problem:** When the relay agent is not running (not started on Dev Server), Orca Server throws a generic `Error('Not connected')`. Users have no idea what to do.

### Bug TRM-002 — PTY spawn timeout too long (30s)

**Problem:** If the relay agent is down, `pty.spawn` hangs for 30 seconds before failing. This makes the UI feel frozen.

---

## Implementation

### Step 1: Find the error site

```bash
grep -n "Not connected\|callWithTimeout" src/main/dev-server/dev-server-relay-bridge.ts | head -20
```

### Step 2: Fix TRM-001 — Better error message

```typescript
// BEFORE (find this pattern):
throw new Error('Not connected')

// AFTER:
throw Object.assign(
  new Error(
    `Dev Server agent not connected: ${this.config.id}. ` +
    `Ensure the Orca agent is running on the Dev Server. ` +
    `Run: node ~/orca-agent/agent.js`
  ),
  {
    code:        'AGENT_NOT_CONNECTED',
    devServerId: this.config.id,
  }
)
```

### Step 3: Fix TRM-002 — Reduce pty.spawn timeout

```typescript
// Find callWithTimeout for pty.spawn:

// BEFORE:
callWithTimeout('pty.spawn', params, 30_000)

// AFTER (10s — fail fast, let user retry):
callWithTimeout('pty.spawn', params, 10_000)
```

---

## Investigation Steps (run before editing)

Because we haven't read this file in full, run these commands first to find exact line numbers:

```bash
# Find error throw sites
grep -n "Not connected\|AGENT_NOT_CONNECTED" src/main/dev-server/dev-server-relay-bridge.ts

# Find timeout values  
grep -n "callWithTimeout\|30_000\|30000\|timeout" src/main/dev-server/dev-server-relay-bridge.ts

# View surrounding context for each match
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep dev-server-relay-bridge
```

**Manual verification:**
1. Start Orca Server without starting relay agent on Dev Server
2. Try to open a terminal pane → expect error within 10s (not 30s)
3. Error message should contain `AGENT_NOT_CONNECTED` code and instruction to run `node ~/orca-agent/agent.js`

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** dev-server-relay-bridge.ts: AGENT_NOT_CONNECTED error khi session null. PTY spawn timeout 10s (giảm từ 30s). calcBackoffDelay exponential backoff (1s → 30s max) cho reconnect.  
**Tests:** Verified: calcBackoffDelay function, AGENT_NOT_CONNECTED error code, 10s timeout.  
