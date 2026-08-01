# TASK-ORCH-01: Fix resolveAgentSpec — Remove --no-cache, Add codex/opencode/ollama

**Task ID:** TASK-ORCH-01  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** ORCH-012, ORCH-004  
**Estimated effort:** Small (1 function replace)  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

File: `src/relay/agent-spawner.ts`

**Current code (L81-89):**
```typescript
export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) {
    return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
  }
  if (modelId.startsWith('gemini')) {
    return { binary: 'gemini', args: ['--stream'] }
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}
```

**Problems:**
1. `--no-cache` is not a valid Claude CLI flag → spawn fails (ORCH-012)
2. Missing `codex` (OpenAI), `opencode`, `ollama` (local) support (ORCH-004)
3. Return type `{ binary, args }` is insufficient — need `apiKeyEnvVar` to know which env var to inject

---

## Implementation

### Step 1: Add `AgentBinarySpec` interface (before `resolveAgentSpec`)

```typescript
// Add BEFORE the resolveAgentSpec function:
export interface AgentBinarySpec {
  readonly binary:         string
  readonly baseArgs:       string[]
  readonly apiKeyEnvVar:   string | null  // null = no API key needed
  readonly localInference?: boolean
}
```

### Step 2: Replace `resolveAgentSpec` function

```typescript
const AGENT_SPECS: AgentBinarySpec[] = [
  { binary: 'claude',   baseArgs: ['--output-format', 'stream-json', '--verbose'], apiKeyEnvVar: 'ANTHROPIC_API_KEY' },
  { binary: 'codex',    baseArgs: [],                                              apiKeyEnvVar: 'OPENAI_API_KEY' },
  { binary: 'gemini',   baseArgs: ['--stream'],                                    apiKeyEnvVar: 'GEMINI_API_KEY' },
  { binary: 'opencode', baseArgs: [],                                              apiKeyEnvVar: null },
  { binary: 'ollama',   baseArgs: [],                                              apiKeyEnvVar: null, localInference: true },
]

const MODEL_PREFIX_MAP: Array<[prefix: string, specIndex: number]> = [
  ['claude',   0],
  ['gpt-',     1],
  ['codex',    1],
  ['gemini',   2],
  ['opencode', 3],
  ['ollama-',  4],
]

export function resolveAgentSpec(modelId: string): AgentBinarySpec {
  for (const [prefix, idx] of MODEL_PREFIX_MAP) {
    if (modelId.startsWith(prefix)) return AGENT_SPECS[idx]
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}. Supported prefixes: claude, gpt-, codex, gemini, opencode, ollama-`)
}
```

### Step 3: Add `buildAgentArgs` helper (after `resolveAgentSpec`)

```typescript
function buildAgentArgs(spec: AgentBinarySpec, req: AgentSpawnRequest): string[] {
  if (spec.binary === 'claude' && req.resumeId) {
    return ['--resume', req.resumeId]
  }
  if (spec.binary === 'codex' && req.resumeId) {
    return ['--session-file', `~/.codex/${req.resumeId}.json`]
  }
  return [...spec.baseArgs]
}
```

### Step 4: Update `AgentSpawnRequest` interface to add `resumeId`

```typescript
export interface AgentSpawnRequest {
  taskId:    string
  userId:    string
  modelId:   string
  accountId: string
  cwd?:      string
  resumeId?: string  // ORCH-009: optional resume session ID
}
```

---

## Verification

```bash
# Type check
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-spawner

# Unit test
npx vitest run src/relay/__tests__/sub-agent-spawner.test.ts
```

**Expected behavior:**
- `resolveAgentSpec('claude')` → `{ binary: 'claude', baseArgs: ['--output-format', 'stream-json', '--verbose'], apiKeyEnvVar: 'ANTHROPIC_API_KEY' }`
- `resolveAgentSpec('claude-opus-4')` → same (prefix match)
- `resolveAgentSpec('codex')` → `{ binary: 'codex', baseArgs: [], apiKeyEnvVar: 'OPENAI_API_KEY' }`
- `resolveAgentSpec('gpt-4o')` → `{ binary: 'codex', ... }` (gpt- prefix)
- `resolveAgentSpec('opencode')` → `{ binary: 'opencode', apiKeyEnvVar: null }`
- `resolveAgentSpec('ollama-llama3')` → `{ binary: 'ollama', localInference: true }`
- `resolveAgentSpec('unknown')` → throws Error

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-spawner.ts: AgentBinarySpec với buildArgs() fn, AGENT_SPECS map, resolveAgentSpec trả undefined (không throw) cho unknown model. Thêm codex/opencode/ollama support.  
**Tests:** agent-spawner.test.ts: 45/45 tests pass. resolveAgentSpec, buildArgs, prefix matching.  
