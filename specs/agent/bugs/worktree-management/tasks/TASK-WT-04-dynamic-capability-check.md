# TASK-WT-04: Dynamic Capability Check — git + pty Availability at Startup

**Task ID:** TASK-WT-04  
**Priority:** 🟡 MEDIUM  
**Bugs fixed:** WT-Issue-2 (capability validation at startup)  
**Estimated effort:** Medium (add buildCapabilities() function)  
**Dependencies:** TASK-ORCH-08 (static capabilities fix already done)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/agent-session.ts`

**Current approach:** Capabilities are hardcoded as a static array in the handshake message:
```typescript
capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees', 'pty'] as const,
```

**Problem:** The static list doesn't reflect reality:
- If `git` is not installed → `'worktrees'` and `'git.exec'` should NOT be advertised
- If `node-pty` native module fails to load → `'pty'` should NOT be advertised
- Advertising unsupported capabilities → Orca Server sends requests that always fail

---

## Implementation

### Step 1: Add helper check functions

Add before `createSession()` or `sendHandshake()` in `agent-session.ts`:

```typescript
import { access, constants } from 'node:fs/promises'
import { join } from 'node:path'
import { execFile } from 'node:child_process'

/**
 * checkGitAvailable — Check if git binary is accessible.
 * Returns true if 'git --version' succeeds within 3 seconds.
 */
async function checkGitAvailable(toolPath: string): Promise<boolean> {
  // Quick path: check in toolPath dirs first
  const dirs = (toolPath || process.env.PATH || '').split(':')
  for (const dir of dirs) {
    try {
      await access(join(dir, 'git'), constants.X_OK)
      return true
    } catch { /* continue */ }
  }
  // Fallback: try running git
  return new Promise((resolve) => {
    const child = execFile('git', ['--version'], { timeout: 3000 })
    child.on('close', (code) => resolve(code === 0))
    child.on('error', () => resolve(false))
  })
}

/**
 * checkPtyAvailable — Check if node-pty native module loads successfully.
 * Returns true if require('node-pty') does not throw.
 */
async function checkPtyAvailable(): Promise<boolean> {
  try {
    await import('node-pty')
    return true
  } catch {
    return false
  }
}

/**
 * buildCapabilities — Dynamically build the capabilities list.
 * Only advertise capabilities that are actually available.
 */
async function buildCapabilities(config: AgentConfig): Promise<readonly string[]> {
  const caps: string[] = [
    'fs',            // always available
    'preflight',     // always available
    'ai.providers',  // always available
    'agent.spawn',   // always available (spawn uses node-pty check below)
    'agent.exec',    // always available
    'agent.sendInput',
    'agent.kill',
  ]

  // Git capabilities: only if git is installed
  const hasGit = await checkGitAvailable(config.toolPath ?? '')
  if (hasGit) {
    caps.push('git', 'git.exec', 'git.execStream')
    caps.push('worktrees', 'git.worktree.list', 'git.worktree.add', 'git.worktree.remove')
  }

  // PTY capabilities: only if node-pty native module loads
  const hasPty = await checkPtyAvailable()
  if (hasPty) {
    caps.push('pty', 'pty.create', 'pty.write', 'pty.resize', 'pty.destroy', 'pty.scrollback')
  }

  return caps as readonly string[]
}
```

### Step 2: Use `buildCapabilities()` in `sendHandshake()`

Find `sendHandshake()` in `agent-session.ts` and update to use async capabilities:

```typescript
// BEFORE (approximate):
async function sendHandshake(): Promise<void> {
  const rpc = {
    jsonrpc: '2.0',
    id: 1,
    method: AGENT_HANDSHAKE_METHOD,
    params: {
      agentVersion: '5.0.0',
      platform:     process.platform,
      arch:         process.arch,
      nodeVersion:  process.version,
      capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees', 'pty'] as const,
      ...(config.agentToken ? { agentToken: config.agentToken } : {}),
      devServerId:  config.devServerId,
    },
  }
  // ...send rpc...
}

// AFTER:
async function sendHandshake(): Promise<void> {
  // WT-Issue-2: Dynamic capability detection
  const capabilities = await buildCapabilities(config)

  const rpc = {
    jsonrpc: '2.0',
    id: 1,
    method: AGENT_HANDSHAKE_METHOD,
    params: {
      agentVersion: '5.0.0',
      platform:     process.platform,
      arch:         process.arch,
      nodeVersion:  process.version,
      capabilities,                               // ← dynamic
      ...(config.agentToken ? { agentToken: config.agentToken } : {}),
      devServerId:  config.devServerId,
    },
  }
  // ...send rpc unchanged...
}
```

---

## Log Output

After this fix, startup logs should show detected capabilities:

```
[agent-session] Detected capabilities:
  ✅ git  (git 2.43.0)
  ✅ pty  (node-pty 1.0.0)
  ✅ ai.providers
  capabilities=[fs, preflight, ai.providers, agent.spawn, agent.exec, ..., git, worktrees, pty]
```

Add this logging in `buildCapabilities()`:

```typescript
async function buildCapabilities(config: AgentConfig, log: AgentLogger): Promise<readonly string[]> {
  // ... same as above ...
  const hasGit = await checkGitAvailable(config.toolPath ?? '')
  log.info(`capability check: git=${hasGit}`)
  const hasPty = await checkPtyAvailable()
  log.info(`capability check: pty=${hasPty}`)
  // ...
  log.info(`capabilities: [${caps.join(', ')}]`)
  return caps as readonly string[]
}
```

---

## Fallback Behavior

If `buildCapabilities()` throws (network timeout, permission error), fall back to static capabilities:

```typescript
let capabilities: readonly string[]
try {
  capabilities = await Promise.race([
    buildCapabilities(config, log),
    new Promise<readonly string[]>((_, reject) =>
      setTimeout(() => reject(new Error('capability check timeout')), 5000)
    ),
  ])
} catch (err) {
  log.warn(`buildCapabilities failed: ${err} — using static fallback`)
  capabilities = ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees', 'pty'] as const
}
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-session

# Test: start relay without git in PATH
PATH=/usr/bin node dist/relay.js
# Expected in log: "capability check: git=false"
# Handshake should NOT include 'worktrees' or 'git.exec'

# Test: start relay normally
# Expected in log: "capability check: git=true, pty=true"
```

---

## ⏸ Deferred Notes

**Decision:** agent-session.ts capabilities hiện tại là static. Dynamic check phức tạp và risk thấp. Defer sang Phase 3 backlog.  
**Risk:** Low — không ảnh hưởng luồng chính  

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-session.ts: buildCapabilities() async, checkGitAvailable() (fs.access + execFile fallback), checkPtyAvailable() (dynamic import). createSession() nhận optional _prebuiltCapabilities param cho testing. Handshake dùng dynamic list với 5s timeout fallback về static.
**Tests:** agent-session.test.ts: 17/17 tests pass. Thêm "handshake params include capabilities array" + "dynamic buildCapabilities runs when no prebuilt caps provided".
