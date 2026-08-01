# TASK-ORCH-02: Fix buildAgentEnv — Remove placeholder-key, Inject Real Credential

**Task ID:** TASK-ORCH-02  
**Priority:** 🔴 HIGH  
**Bugs fixed:** ORCH-003  
**Estimated effort:** Medium (rewrite function, add credential lookup)  
**Dependencies:** TASK-ORCH-01 (needs `AgentBinarySpec` type)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

File: `src/relay/agent-spawner.ts`

**Current code (L93-107, L147):**
```typescript
export async function buildAgentEnv(
  accountId: string,
  apiKey:    string,   // ← always called with 'placeholder-key'
  cwd:       string,
): Promise<Record<string, string>> {
  return {
    ANTHROPIC_API_KEY: apiKey,  // ← all 3 get same fake key
    OPENAI_API_KEY:    apiKey,
    GEMINI_API_KEY:    apiKey,
    ...
  }
}

// L147 — the call site:
const env = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)
```

**Problem:** Agent spawned with `ANTHROPIC_API_KEY=placeholder-key` → ALL AI CLI calls fail auth immediately.

**Architecture note (important):**  
The credential store's `readDecryptedKey()` returns **Layer 1 encrypted blob** (browser-encrypted), NOT plaintext.  
The correct fix is: Orca Server passes `resolvedApiKey` (plaintext) in the spawn request.  
If `resolvedApiKey` is absent, fallback to reading the blob (agent may fail auth, but at least not `placeholder-key`).

---

## Implementation

### Step 1: Add import for `readDecryptedKey` at top of file

```typescript
import { readDecryptedKey } from './agent-credential-store'
```

### Step 2: Replace `buildAgentEnv` function signature and body

```typescript
export async function buildAgentEnv(
  accountId:       string,
  spec:            AgentBinarySpec,
  cwd:             string,
  config:          AgentConfig,
  log:             AgentLogger,
  resolvedApiKey?: string,  // Plaintext key injected by Orca Server (preferred)
): Promise<Record<string, string>> {
  const base: Record<string, string> = {
    HOME:            process.env.HOME ?? '/tmp',
    PATH:            process.env.PATH ?? '/usr/bin:/bin',
    TERM:            'xterm-256color',
    ORCA_AGENT_CWD:  cwd,
    ORCA_ACCOUNT_ID: accountId,
  }

  if (spec.apiKeyEnvVar) {
    if (resolvedApiKey) {
      base[spec.apiKeyEnvVar] = resolvedApiKey
    } else if (accountId) {
      const blob = await readDecryptedKey(accountId, config, log)
      if (blob) {
        base[spec.apiKeyEnvVar] = blob
        log.warn(`buildAgentEnv: injecting Layer1 blob for ${spec.apiKeyEnvVar} — agent may fail auth`)
      } else {
        log.warn(`buildAgentEnv: no credential for accountId=${accountId}`)
      }
    }
  }

  if (spec.localInference) {
    base.OLLAMA_HOST     = process.env.OLLAMA_HOST     ?? 'http://localhost:11434'
    base.OPENAI_BASE_URL = process.env.OPENAI_BASE_URL ?? 'http://localhost:8000/v1'
  }

  return base
}
```

### Step 3: Update `handleAgentSpawn` — fix the call to `buildAgentEnv`

In `handleAgentSpawn`, find and replace the old call:

```typescript
// OLD:
const spec = resolveAgentSpec(req.modelId)
const env  = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)

// NEW:
const spec = resolveAgentSpec(req.modelId)
const resolvedApiKey = typeof params.resolvedApiKey === 'string' ? params.resolvedApiKey : undefined
const env = await buildAgentEnv(
  req.accountId,
  spec,
  req.cwd ?? config.workDir,
  config,
  log,
  resolvedApiKey,
)
```

### Step 4: Update `ptyId` to include `userId` for isolation

```typescript
// OLD:
const ptyId = `${req.taskId}-${Date.now()}`

// NEW:
const ptyId = `pty-${req.userId}-${req.taskId}-${Date.now()}`
```

### Step 5: Update spawn call to use `buildAgentArgs`

```typescript
// OLD:
const pty = nodePty.spawn(spec.binary, spec.args, { ... })

// NEW:
const args = buildAgentArgs(spec, req)
const pty = nodePty.spawn(spec.binary, args, { ... })
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-spawner
```

**Expected behavior:**
- `agent.spawn` with `resolvedApiKey: 'sk-ant-xxx'` → `ANTHROPIC_API_KEY=sk-ant-xxx` in env
- `agent.spawn` without `resolvedApiKey` → reads from credential store
- `agent.spawn` for `opencode` model → no API key env var injected (spec.apiKeyEnvVar is null)
- `agent.spawn` for `ollama-llama3` → `OLLAMA_HOST` and `OPENAI_BASE_URL` set

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-spawner.ts: buildAgentEnv(req, spec, config, apiKey) — inject ORCA_TASK_ID, ORCA_USER_ID, ORCA_PROJECT_ID, GH_CONFIG_DIR, GLAB_CONFIG_DIR. resolvedApiKey fills all 3 provider env vars.  
**Tests:** agent-spawner.test.ts: buildAgentEnv suite 15+ tests pass.  
