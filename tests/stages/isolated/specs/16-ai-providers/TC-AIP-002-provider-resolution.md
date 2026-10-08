# TC-AIP-002 — Provider Account Resolution

**BL Reference:** BL-AIP-02  
**Priority:** P0  
**Type:** Unit + Integration  
**Actor:** System

---

## TC-AIP-002-01: Priority — User > Project > Server

**Priority:** P0

### Preconditions
- Server scope: Anthropic (apiKey-server)
- Project scope: Anthropic (apiKey-project)
- User scope: Anthropic (apiKey-user)

### Steps
1. `AIProviderResolver.resolve({ userId, projectId, devServerId, provider: 'anthropic' })`

### Expected Results
- Returns user-scope account
- `resolved.apiKeyRef` points to user's encrypted key

---

## TC-AIP-002-02: Fallback — No user scope, use project

**Priority:** P0

### Preconditions
- No user scope for Anthropic
- Project scope: Anthropic

### Steps
1. `AIProviderResolver.resolve({ userId, projectId, ... })`

### Expected Results
- Returns project-scope account

---

## TC-AIP-002-03: Fallback — No user/project scope, use server default

**Priority:** P0

### Preconditions
- Only server scope exists

### Steps
1. Resolve

### Expected Results
- Returns server-scope account

---

## TC-AIP-002-04: No provider available — Error

**Priority:** P0

### Preconditions
- No accounts registered for provider

### Steps
1. Resolve

### Expected Results
- Error: `{ code: 'NO_PROVIDER_AVAILABLE', provider }`

---

## TC-AIP-002-05: Multi-provider — Resolve correct provider per agent type

**Priority:** P1

| Agent Type | Expected Provider |
|-----------|-------------------|
| claude | anthropic |
| codex | openai |
| gemini | google |
| opencode | anthropic (configurable) |

