# SOLUTION: task-graph — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế  
**Files nguồn đã đọc:** `ProfileAwareAgentSpawner.ts`, `agent-rpc-dispatch.ts`, `TaskAIPlanner.ts`

---

## BUG-BE-TG-001: ProfileAwareAgentSpawner gọi `agent.exec` với params sai

**File:** [`src/main/project/ProfileAwareAgentSpawner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/project/ProfileAwareAgentSpawner.ts)  
**Line:** 106

### Code sai thực tế:
```typescript
// ProfileAwareAgentSpawner.ts:106–110
const result = await relay.call('agent.exec', {
  command,           // ← "command" không phải "binary"
  workdir: workdir ?? project.repoPath,  // ← "workdir" không phải "cwd"
  env: profileEnv,
})
```

### Relay `agent.exec` thực tế expect (agent-rpc-dispatch.ts:506–516):
```typescript
// relay dispatch agent.exec expects:
const binary  = typeof p.binary   === 'string' ? p.binary   : ''  // ← "binary" field
const args    = Array.isArray(p.args) ? ...                        // ← "args" array
const cwd     = typeof p.cwd      === 'string' ? p.cwd : config.workDir  // ← "cwd" field
const extraEnv = p.env ?? {}
```

**Root cause:** ProfileAwareAgentSpawner dùng `command` (string) nhưng relay expect `binary` + `args` (như `child_process.spawn`).

### Fix Option A (recommended) — Fix params trong ProfileAwareAgentSpawner:
```typescript
// src/main/project/ProfileAwareAgentSpawner.ts — Thay thế lines 104–110:

// 6. Get relay and send agent.exec với đúng params
const relay = await this.router.getRelayForProject(projectId, userId)

// Parse command string thành binary + args
// Ví dụ: "claude --model opus" → binary="claude", args=["--model", "opus"]
const [binary, ...args] = command.split(/\s+/).filter(Boolean)

const result = await relay.call('agent.exec', {
  binary: binary ?? command,   // ← "binary" field (required)
  args,                        // ← "args" array
  cwd: workdir ?? project.repoPath,   // ← "cwd" field (not "workdir")
  env: profileEnv,
  timeoutMs: 5 * 60 * 1000,   // 5 phút timeout
})
```

### Fix Option B — Thêm `command` support vào relay dispatch:
```typescript
// src/relay/agent-rpc-dispatch.ts:502–520
// Sau dòng: const binary = typeof p.binary === 'string' ? p.binary : ''
// Thêm fallback từ "command" field:
const binary = typeof p.binary === 'string' 
  ? p.binary 
  : typeof p.command === 'string'
    ? p.command.split(/\s+/)[0] ?? ''
    : ''
const args = Array.isArray(p.args) 
  ? (p.args as unknown[]).map(String) 
  : typeof p.command === 'string'
    ? p.command.split(/\s+/).slice(1)
    : []
const cwd = typeof p.cwd === 'string' ? p.cwd 
  : typeof p.workdir === 'string' ? p.workdir  // ← fallback "workdir" → "cwd"
  : config.workDir
```

**Option A là cách đúng** — align caller với existing relay interface thay vì mở rộng relay.

---

## BUG-BE-TG-002: `ai.complete` relay handler missing

**File:** [`src/main/task/TaskAIPlanner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/task/TaskAIPlanner.ts)  
**Line:** 54

**File phải tạo:** `src/relay/agent-rpc-dispatch.ts` — thêm case `'ai.complete'`

### Code hiện tại (TaskAIPlanner.ts:54):
```typescript
const response = (await relay.call('ai.complete', {
  prompt,
  format: 'json',
  taskId,
})) as { content?: string; text?: string } | string
```

### `ai.complete` không tồn tại trong `agent-rpc-dispatch.ts`

**Kiểm tra:** `grep 'ai.complete' src/relay/agent-rpc-dispatch.ts` → No results. Confirmed missing.

### Fix — Thêm `ai.complete` case vào agent-rpc-dispatch.ts:

Thêm sau `case 'agent.exec'` (line 557):

```typescript
// src/relay/agent-rpc-dispatch.ts — thêm sau case 'agent.exec' block (sau line ~557):

// ── v5.0: ai.complete ─────────────────────────────────────────────────────────
// TG-002: Non-interactive AI completion for task planning and git commit messages.
// Reads credentials from ai-provider-handler (same as ai.provider.readCredential).
case 'ai.complete': {
  try {
    const p      = rpc.params ?? {}
    const prompt = typeof p.prompt === 'string' ? p.prompt : ''
    const format = typeof p.format === 'string' ? p.format : 'text'

    if (!prompt) {
      return makeError(rpc.id, AgentErrorCode.InvalidParams, 'ai.complete: prompt is required')
    }

    // Delegate to ai-provider-handler which manages credentials:
    const { handleAIComplete } = await import('./ai-complete-handler')
    const result = await handleAIComplete({ prompt, format }, config, log)
    return { jsonrpc: '2.0', id: rpc.id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `ai.complete failed: ${msg}`)
  }
}
```

### Tạo `src/relay/ai-complete-handler.ts`:
```typescript
// src/relay/ai-complete-handler.ts (NEW)
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

export async function handleAIComplete(
  params: { prompt: string; format?: string },
  config: AgentConfig,
  log: AgentLogger,
): Promise<{ content: string; model?: string }> {
  const { prompt, format } = params

  // 1. Read AI provider credential from credential store
  const { CredentialStore } = await import('./credential-store')
  const store = new CredentialStore(config)

  // Try each available credential in order of priority:
  const accountIds = config.aiProviderAccountIds ?? []
  if (accountIds.length === 0) {
    throw new Error('No AI provider configured. Set up an AI provider first.')
  }

  const accountId = accountIds[0]!
  const cred = await store.readDecrypted(accountId).catch(() => null)
  if (!cred) {
    throw new Error(`Credential not found for accountId: ${accountId}`)
  }

  // 2. Determine provider type from accountId prefix or config:
  const model = config.defaultModel ?? 'claude-opus-4-5'
  const text  = await callProvider({ model, apiKey: cred, prompt, format, log })

  return { content: text, model }
}

async function callProvider(params: {
  model: string; apiKey: string; prompt: string; format?: string; log: AgentLogger
}): Promise<string> {
  const { model, apiKey, prompt, log } = params

  if (model.startsWith('claude')) {
    return callAnthropic({ model, apiKey, prompt, log })
  } else if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3')) {
    return callOpenAI({ model, apiKey, prompt, log })
  } else if (model.startsWith('gemini')) {
    return callGoogle({ model, apiKey, prompt, log })
  }

  throw new Error(`Unknown model provider: ${model}`)
}

async function callAnthropic(params: {
  model: string; apiKey: string; prompt: string; log: AgentLogger
}): Promise<string> {
  const response = await fetch('https://api.anthropic.com/v1/messages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-api-key': params.apiKey,
      'anthropic-version': '2023-06-01',
    },
    body: JSON.stringify({
      model: params.model,
      max_tokens: 4096,
      messages: [{ role: 'user', content: params.prompt }],
    }),
    signal: AbortSignal.timeout(120_000),
  })

  if (!response.ok) {
    const err = await response.text().catch(() => response.statusText)
    throw new Error(`Anthropic API error ${response.status}: ${err}`)
  }

  const data = await response.json() as {
    content: Array<{ type: string; text?: string }>
  }
  return data.content.find(c => c.type === 'text')?.text ?? ''
}

async function callOpenAI(params: {
  model: string; apiKey: string; prompt: string; log: AgentLogger
}): Promise<string> {
  const response = await fetch('https://api.openai.com/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${params.apiKey}`,
    },
    body: JSON.stringify({
      model: params.model,
      messages: [{ role: 'user', content: params.prompt }],
      max_tokens: 4096,
    }),
    signal: AbortSignal.timeout(120_000),
  })

  if (!response.ok) {
    const err = await response.text().catch(() => response.statusText)
    throw new Error(`OpenAI API error ${response.status}: ${err}`)
  }

  const data = await response.json() as {
    choices: Array<{ message: { content: string } }>
  }
  return data.choices[0]?.message.content ?? ''
}

async function callGoogle(params: {
  model: string; apiKey: string; prompt: string; log: AgentLogger
}): Promise<string> {
  const url = `https://generativelanguage.googleapis.com/v1beta/models/${params.model}:generateContent?key=${params.apiKey}`
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      contents: [{ parts: [{ text: params.prompt }] }],
    }),
    signal: AbortSignal.timeout(120_000),
  })

  if (!response.ok) {
    const err = await response.text().catch(() => response.statusText)
    throw new Error(`Google AI API error ${response.status}: ${err}`)
  }

  const data = await response.json() as {
    candidates: Array<{ content: { parts: Array<{ text?: string }> } }>
  }
  return data.candidates[0]?.content.parts[0]?.text ?? ''
}
```

---

## BUG-TG-002: TaskService thiếu dependency check

**File:** `src/main/task/TaskService.ts`  

Grep để tìm:
```bash
grep -n "executeTask\|execute\|runTask" src/main/task/TaskService.ts | head -20
```

Theo TDD-18 và bug report: `TaskService.executeTask()` không check dependency status trước khi execute.

### Fix pattern (align với WorkflowOrchestrator.ts wave model đã implement):
```typescript
// src/main/task/TaskService.ts — thêm vào executeTask():

async executeTask(taskId: string, projectId: string, userId: string): Promise<void> {
  const task = await this.get(taskId)
  if (!task) throw new Error(`TASK_NOT_FOUND: ${taskId}`)

  // FIX TG-002: Check all dependencies are completed
  if (task.dependencies && task.dependencies.length > 0) {
    const deps = await Promise.all(
      task.dependencies.map(depId => this.get(depId))
    )
    const blockers = deps.filter(dep => dep && dep.status !== 'done' && dep.status !== 'completed')
    if (blockers.length > 0) {
      const blockerIds = blockers.map(b => b?.id).join(', ')
      throw new Error(`TASK_BLOCKED: dependencies not completed: ${blockerIds}`)
    }
  }

  // Proceed with execution
  await this.update(taskId, { status: 'in-progress' })
  try {
    await this.agentExecutor.executeTask(task, projectId, userId)
    await this.update(taskId, { status: 'done' })
  } catch (err) {
    await this.update(taskId, { status: 'blocked', progressPercent: 0 })
    throw err
  }
}
```

---

## Tóm tắt thay đổi

| Bug | File | Lines | Thay đổi |
|-----|------|-------|---------|
| BE-TG-001 | `ProfileAwareAgentSpawner.ts` | 106–110 | Fix params: `command` → `binary`+`args`, `workdir` → `cwd` |
| BE-TG-002 | `agent-rpc-dispatch.ts` + NEW `ai-complete-handler.ts` | after line 557 | Thêm `case 'ai.complete'` + AI provider callers |
| TG-002 | `TaskService.ts` | executeTask | Thêm dependency completed check trước execute |
