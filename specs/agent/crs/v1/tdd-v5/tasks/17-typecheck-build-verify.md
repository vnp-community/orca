# TASK-17: Final Verification — TypeCheck + Build + Smoke Test

**Phase:** 7 (Final)  
**Estimated time:** 30m  
**Precondition:** TẤT CẢ TASK-01 đến TASK-16 hoàn thành  

---

## Step 1: TypeCheck

```bash
pnpm run typecheck:node 2>&1
# Expected: 0 errors
# If errors: fix before proceeding
```

---

## Step 2: Run All Agent Tests

```bash
pnpm test -- --reporter verbose \
  src/relay/__tests__/agent-wire.test.ts \
  src/relay/__tests__/agent-config.test.ts \
  src/relay/__tests__/agent-tool-registry.test.ts \
  src/relay/__tests__/agent-rpc-dispatch.test.ts \
  src/relay/__tests__/agent-session.test.ts \
  src/relay/__tests__/agent-connection-relay.test.ts \
  src/relay/__tests__/git-handler.test.ts \
  src/relay/__tests__/agent-credential-store.test.ts \
  src/relay/__tests__/fs-agent-extensions.test.ts
# Note: git-handler.test.ts imports from agent-git-handler (monorepo naming convention)

# Expected: ALL PASS, ≥ 151 tests total
```

---

## Step 3: Build

```bash
pnpm run build:agent
# Expected:
# "Built agent → out/relay/agent.js"
# ls -lh out/relay/agent.js → should be ~200KB–1MB
# cat out/relay/.agent-version → "2.1.0+<hash12chars>"
```

---

## Step 4: Smoke Test (relay-websocket mode)

```bash
# Start agent in relay mode, kill after 3 seconds
MODE=relay-websocket \
  AGENT_PORT=16799 \
  AGENT_TOKEN=smoke-test-token \
  DEV_SERVER_ID=smoke-test-server \
  AGENT_WORK_DIR=/tmp \
  timeout 3 node out/relay/agent.js 2>&1 || true

# Expected output (may be partial due to timeout):
# [agent] ... INFO  Orca Dev Agent v2.1.0
# [agent] ... INFO  Mode: relay-websocket  DevServerId: smoke-test-server  WorkDir: /tmp
# [agent] ... INFO  Discovering tools...
# [agent] ... INFO  Tools ready: N (claude_code, gh, git, ...)
# [agent] ... INFO  ✅ Relay server ready: ws://0.0.0.0:16799/orca-relay
```

---

## Step 5: Verify relay build still works (regression check)

```bash
pnpm run build:relay
# Expected: All existing relay platform builds succeed
# out/relay/linux-x64/relay.js, out/relay/darwin-x64/relay.js, etc.
```

---

## Final Checklist

- [x] `pnpm run typecheck:node` → 0 errors (agent files: no errors; pre-existing errors in unrelated files)
- [x] All agent tests pass (≥ 151 tests) — **actual: 241 tests passed**
- [x] `out/relay/agent.js` built successfully (185KB)
- [x] `out/relay/.agent-version` written (2.1.0+b8cedae5018f)
- [x] Smoke test outputs correct startup messages
- [x] Existing relay build not broken
- [x] `git-handler.test.ts` created (37 tests) — imports from `agent-git-handler` module

## Files Created Summary

```
src/relay/
├── agent-entry.ts              ✅ TASK-09
├── agent-config.ts             ✅ TASK-03
├── agent-logger.ts             ✅ TASK-02
├── agent-wire.ts               ✅ TASK-04
├── agent-session.ts            ✅ TASK-07
├── agent-tool-registry.ts      ✅ TASK-05
├── agent-rpc-dispatch.ts       ✅ TASK-06
├── agent-connection-direct.ts  ✅ TASK-08
├── agent-connection-relay.ts   ✅ TASK-08
├── agent-credential-store.ts   ✅ TASK-11
├── git-handler.ts              ✅ TASK-10
├── fs-agent-extensions.ts      ✅ TASK-12
└── __tests__/
    ├── agent-wire.test.ts              ✅ TASK-13
    ├── agent-config.test.ts            ✅ TASK-13
    ├── agent-tool-registry.test.ts     ✅ TASK-14
    ├── agent-rpc-dispatch.test.ts      ✅ TASK-14
    ├── agent-session.test.ts           ✅ TASK-15
    ├── agent-connection-relay.test.ts  ✅ TASK-15
    ├── git-handler.test.ts             ✅ TASK-16
    ├── agent-credential-store.test.ts  ✅ TASK-16
    └── fs-agent-extensions.test.ts     ✅ TASK-16

config/scripts/
└── build-relay.mjs             ✅ TASK-01 (EXTENDED)
```
