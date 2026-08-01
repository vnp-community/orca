# TASK-WT-03: Add ORCA_WORKTREE_PATH/BRANCH to buildAgentEnv + AgentSpawnRequest

**Task ID:** TASK-WT-03  
**Priority:** 🔴 MEDIUM  
**Bugs fixed:** WT-Issue-3 (missing worktree context in agent spawn env)  
**Estimated effort:** Small (update interface + function)  
**Dependencies:** TASK-ORCH-02 (new buildAgentEnv signature)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/agent-spawner.ts`

**Problem:** When an AI agent is spawned in a worktree context, the agent process has no way to know:
- Which worktree path it's running in (`ORCA_WORKTREE_PATH`)
- Which git branch this worktree corresponds to (`ORCA_WORKTREE_BRANCH`)

TDD-AG-12 §5 specifies these env vars should be injected. Without them:
- The agent cannot reference its own working directory correctly
- Multi-agent setups where each agent owns a worktree cannot function
- Task graph steps that need to know which branch they operate on break

---

## Implementation

### Step 1: Update `AgentSpawnRequest` interface

Find the `AgentSpawnRequest` interface in `agent-spawner.ts`:

```typescript
// BEFORE (approximate):
export interface AgentSpawnRequest {
  taskId:      string
  userId:      string
  modelId:     string
  accountId:   string
  cwd?:        string
  resumeId?:   string
}

// AFTER — add worktree fields:
export interface AgentSpawnRequest {
  taskId:          string
  userId:          string
  modelId:         string
  accountId:       string
  cwd?:            string
  resumeId?:       string   // ORCH-009: resume session
  worktreePath?:   string   // WT-Issue-3: absolute path of worktree (same as cwd usually)
  branchName?:     string   // WT-Issue-3: git branch this worktree corresponds to
}
```

### Step 2: Update `buildAgentEnv()` to inject worktree env vars

In the env object returned by `buildAgentEnv()`, add:

```typescript
// BEFORE (approximate return):
return {
  HOME:            process.env.HOME ?? '/tmp',
  PATH:            process.env.PATH ?? '/usr/bin:/bin',
  TERM:            'xterm-256color',
  ORCA_AGENT_CWD:  cwd,
  ORCA_ACCOUNT_ID: accountId,
  // ... apiKey injection ...
}

// AFTER — add worktree context:
return {
  HOME:            process.env.HOME ?? '/tmp',
  PATH:            process.env.PATH ?? '/usr/bin:/bin',
  TERM:            'xterm-256color',
  ORCA_AGENT_CWD:  cwd,
  ORCA_ACCOUNT_ID: accountId,
  // WT-Issue-3: Worktree context — lets agent know which branch/path it owns
  ...(worktreePath ? { ORCA_WORKTREE_PATH: worktreePath } : {}),
  ...(branchName   ? { ORCA_WORKTREE_BRANCH: branchName } : {}),
  // ... apiKey injection ...
}
```

Where `worktreePath` and `branchName` come from the function parameters (or from the spawn request).

### Step 3: Pass worktree fields from spawn params to `buildAgentEnv`

In `handleAgentSpawn()`, extract these fields from the RPC params:

```typescript
// In handleAgentSpawn(), after extracting other params:
const worktreePath = typeof params.worktreePath === 'string' ? params.worktreePath : undefined
const branchName   = typeof params.branchName   === 'string' ? params.branchName   : undefined

const req: AgentSpawnRequest = {
  taskId:        ...,
  userId:        ...,
  modelId:       ...,
  accountId:     ...,
  cwd:           ...,
  worktreePath,  // ← NEW
  branchName,    // ← NEW
}
```

---

## Wire Protocol

```json
// agent.spawn with worktree context:
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "agent.spawn",
  "params": {
    "taskId":        "task-123",
    "userId":        "user-abc",
    "modelId":       "claude",
    "accountId":     "acc-xyz",
    "cwd":           "/home/ubuntu/projects/feature-branch",
    "worktreePath":  "/home/ubuntu/projects/feature-branch",
    "branchName":    "feature/my-feature",
    "resolvedApiKey": "sk-ant-..."
  }
}
```

**Agent process environment will then include:**
```
ORCA_AGENT_CWD=/home/ubuntu/projects/feature-branch
ORCA_WORKTREE_PATH=/home/ubuntu/projects/feature-branch
ORCA_WORKTREE_BRANCH=feature/my-feature
```

---

## Unit Tests

```typescript
describe('buildAgentEnv — worktree context', () => {
  it('injects ORCA_WORKTREE_PATH when worktreePath provided', async () => {
    const env = await buildAgentEnv('acc', claudeSpec, '/repo', config, log, 'key', {
      worktreePath: '/home/ubuntu/projects/feat',
      branchName:   'feature/my-feat',
    })
    expect(env.ORCA_WORKTREE_PATH).toBe('/home/ubuntu/projects/feat')
    expect(env.ORCA_WORKTREE_BRANCH).toBe('feature/my-feat')
  })

  it('omits ORCA_WORKTREE_PATH when not provided', async () => {
    const env = await buildAgentEnv('acc', claudeSpec, '/repo', config, log, 'key')
    expect(env.ORCA_WORKTREE_PATH).toBeUndefined()
    expect(env.ORCA_WORKTREE_BRANCH).toBeUndefined()
  })
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-spawner
npx vitest run src/relay/__tests__/agent-spawner.test.ts
```

**Manual check:**
```bash
# Spawn agent with worktreePath:
# { "method": "agent.spawn", "params": { ..., "worktreePath": "/tmp/feat", "branchName": "feat/x" } }
# Then in the spawned process: printenv | grep ORCA
# Expected: ORCA_WORKTREE_PATH=/tmp/feat and ORCA_WORKTREE_BRANCH=feat/x
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-spawner.ts: handleAgentSpawn inject ORCA_WORKTREE_PATH và ORCA_WORKTREE_BRANCH từ req params vào env khi worktreePath/branchName present.  
**Tests:** Verified: 'ORCA_WORKTREE_PATH' và 'ORCA_WORKTREE_BRANCH' tại agent-spawner.ts L318-319.  
