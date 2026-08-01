# BL-AIP-02 — Provider Account Resolution cho Agent/Workflow

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-AIP-02 |
| **Tên** | Provider Account Resolution |
| **Domain** | AI Provider Management |
| **Actor** | System (auto-resolve khi spawn agent/step) |
| **Priority** | P0 |

---

## Mô tả

Khi agent hoặc workflow step cần một AI provider, hệ thống tự động resolve account phù hợp nhất dựa trên context (server, project, user, scope priority) và availability.

---

## Resolution Algorithm

```typescript
async function resolveProviderAccount(
  ctx: ExecutionContext
): Promise<AIProviderAccount> {

  const { devServerId, projectId, userId, providerSpec } = ctx

  // Case 1: Explicit account ID
  if (providerSpec.accountId) {
    return validateAndReturn(providerSpec.accountId, devServerId)
  }

  // Case 2: Scope-qualified string "server:anthropic-default"
  // "project:vnp-blc:openai", "user:personal-ollama"
  if (providerSpec.scopedRef) {
    return resolveByRef(providerSpec.scopedRef, ctx)
  }

  // Case 3: Model-based auto-select
  const provider = detectProviderFromModel(providerSpec.model)
  // e.g. 'claude-opus-4-5' → 'anthropic'

  // Priority cascade (highest → lowest):
  const candidates = await db.aiProviderAccounts.findAll({
    devServerId,
    provider,
    status: 'healthy',
    orderBy: [
      // 1. User-scope (most specific)
      "CASE WHEN scope='user' AND user_id=? THEN 0 ELSE 99 END",
      // 2. Project-scope
      "CASE WHEN scope='project' AND project_id=? THEN 1 ELSE 99 END",
      // 3. Server default
      "CASE WHEN scope='server' AND is_default=1 THEN 2 ELSE 99 END",
      // 4. Any server-scope
      "CASE WHEN scope='server' THEN 3 ELSE 99 END",
    ]
  }, [userId, projectId])

  if (candidates.length === 0) {
    throw new WorkflowError(
      `No AI provider account for '${provider}' on server '${devServerId}'. ` +
      `Please configure one in Settings → Dev Server → AI Providers.`
    )
  }

  const selected = candidates[0]

  // Validate model is in account's allowed models
  if (providerSpec.model && !selected.models.includes(providerSpec.model)) {
    throw new WorkflowError(
      `Model '${providerSpec.model}' not allowed by account '${selected.name}'`
    )
  }

  return selected
}
```

---

## Supported Scope Ref Formats

```
"server:anthropic-default"          → default Anthropic account on devServer
"server:<accountId>"                → specific account by ID
"project:<projectId>:<provider>"    → project-scope provider
"user:<provider>"                   → user's personal account
"fleet:tag:<tag>:<provider>"        → any server with tag (load balance)
{ model: "claude-opus-4-5" }        → auto-detect provider, then cascade
{ provider: "openai", model: "..." }→ specific provider + model
```

---

## Model → Provider Detection

```typescript
const MODEL_PROVIDER_MAP: Record<string, AIProvider> = {
  'claude-*':     'anthropic',
  'gpt-*':        'openai',
  'o1-*':         'openai',
  'o3-*':         'openai',
  'gemini-*':     'google',
  'llama*':       'ollama',
  'mistral*':     'ollama',
  'codellama*':   'ollama',
  'qwen*':        'ollama',
}

function detectProviderFromModel(model: string): AIProvider {
  for (const [pattern, provider] of Object.entries(MODEL_PROVIDER_MAP)) {
    if (minimatch(model, pattern)) return provider
  }
  throw new Error(`Unknown model '${model}'`)
}
```

---

## Tiêu chí chấp nhận

- [ ] Resolve theo explicit accountId
- [ ] Resolve theo scoped ref string
- [ ] Resolve theo model auto-detect + priority cascade
- [ ] Priority: user > project > server-default > server-any
- [ ] Filter by status='healthy' (exclude quota_exceeded, invalid_key)
- [ ] Validate model in account.models list
- [ ] Clear error message khi không tìm thấy account
