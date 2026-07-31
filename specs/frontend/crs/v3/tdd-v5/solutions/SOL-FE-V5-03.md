# SOL-FE-V5-03: AI Provider Admin UI

**TDD Ref:** [TDD-FE-13](../../../tdd/13-ai-provider-ui.md)  
**Feature:** F35 | **ADR:** ADR-008 | **HLD:** C3.11a  
**Status:** ✅ DONE — Implemented via TASK-V5-06, TASK-V5-07, TASK-V5-08  
**Security:** API keys NEVER stored plaintext — SubtleCrypto encrypt in browser

---

## 1. Files Cần Tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/store/slices/ai-provider.ts` | Zustand Slice | accounts, usageByAccount |
| `src/renderer/src/components/ai-provider/ProviderList.tsx` | Component | Table + filters |
| `src/renderer/src/components/ai-provider/ProviderForm.tsx` | Component | Add/Edit dialog |
| `src/renderer/src/components/ai-provider/CredentialInput.tsx` | Component | Encrypted key input |
| `src/renderer/src/components/ai-provider/HealthStatusBadge.tsx` | Component | Status indicator |
| `src/renderer/src/components/ai-provider/UsageChart.tsx` | Component | Token quota progress bar |
| `src/renderer/src/components/ai-provider/ProviderTypeSelector.tsx` | Component | Provider type picker |
| `src/renderer/src/components/ai-provider/ScopeSelector.tsx` | Component | Server/Project/User scope |
| `src/renderer/src/hooks/useAIProviders.ts` | Hook | Fetch + CRUD + test |
| `src/renderer/src/lib/credential-crypto.ts` | Utility | SubtleCrypto encrypt/decrypt |

---

## 2. Files Cần Sửa (Additive)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/index.ts` | Register `createAIProviderSlice` |
| `src/renderer/src/components/admin/AdminApp.tsx` | Thêm route `/admin/ai-providers` → `ProviderList` |

---

## 3. AI Provider Slice

```typescript
// src/renderer/src/store/slices/ai-provider.ts

export type AIProviderAccount = {
  id:             string
  provider:       AIProviderType          // 'anthropic' | 'openai' | 'gemini' | 'azure' | 'bedrock' | 'ollama' | 'vllm'
  label:          string
  model:          string
  baseUrl?:       string                  // Ollama/vLLM
  scope:          AIProviderScope         // 'server' | 'project' | 'user'
  scopeRefId:     string                  // serverId, projectId, or userId
  devServerId:    string
  status:         AIProviderStatus        // 'active' | 'pending' | 'invalid' | 'quota_exceeded' | 'unreachable'
  quotaLimitDay:  number                  // 0 = unlimited
  createdAt:      number
}

export type AIProviderUsage = {
  accountId: string
  tokens:    number
  requests:  number
  date:      string                       // YYYY-MM-DD
}

export type AIProviderSlice = {
  accounts:          AIProviderAccount[]
  usageByAccount:    Record<string, AIProviderUsage>
  isLoadingAccounts: boolean

  setAccounts(accounts: AIProviderAccount[]): void
  updateAccountStatus(id: string, status: AIProviderStatus): void
  addAccount(account: AIProviderAccount): void
  removeAccount(id: string): void
  setUsage(accountId: string, usage: AIProviderUsage): void
  setLoadingAccounts(v: boolean): void
}
```

---

## 4. Credential Crypto Utility

```typescript
// src/renderer/src/lib/credential-crypto.ts
// CRITICAL: API keys NEVER logged, NEVER in state plaintext after encryption

/**
 * Derives a session key from the session token (available in AuthSlice).
 * Key is ephemeral — tied to browser session.
 */
async function deriveSessionKey(sessionToken: string): Promise<CryptoKey> {
  const keyMaterial = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(sessionToken),
    { name: 'PBKDF2' },
    false,
    ['deriveKey']
  )
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt: new TextEncoder().encode('orca-cred-v1'), iterations: 100_000, hash: 'SHA-256' },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  )
}

/**
 * Encrypt API key with session-derived AES-GCM key.
 * Returns base64(iv) + ':' + base64(ciphertext)
 */
export async function encryptCredential(
  plaintext: string,
  sessionToken: string
): Promise<{ encryptedBlob: string; iv: string }> {
  const key = await deriveSessionKey(sessionToken)
  const iv  = crypto.getRandomValues(new Uint8Array(16))
  const encrypted = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    new TextEncoder().encode(plaintext)
  )
  return {
    encryptedBlob: btoa(String.fromCharCode(...new Uint8Array(encrypted))),
    iv:            btoa(String.fromCharCode(...iv)),
  }
}
```

> **Security note:** Session token used as key derivation input — key changes on logout.
> Server receives only `encryptedBlob + iv`, decrypts using its own shared secret with the agent.

---

## 5. CredentialInput Component — Security Critical

```typescript
// src/renderer/src/components/ai-provider/CredentialInput.tsx
// See TDD-FE-13 section 3 for full implementation

// Key design decisions:
// 1. rawValue state cleared IMMEDIATELY after encryption (setRawValue(''))
// 2. onEncrypted callback only called with encrypted blob — never plaintext
// 3. isEncrypting state prevents submit during async encrypt
// 4. autoComplete="off" prevents browser password manager saving
// 5. spellCheck={false} prevents cloud spell-check sending key chars
// 6. Ollama → no credential input rendered (no key needed)
```

---

## 6. ProviderList — Mount Point

```typescript
// Admin SPA: /admin/ai-providers
// Route trong AdminApp.tsx:
<Route path="/ai-providers" element={<ProviderList />} />
```

---

## 7. useAIProviders Hook

```typescript
// src/renderer/src/hooks/useAIProviders.ts
// Xem TDD-FE-13 section 5

// Extensions so với TDD:
// - testConnection() shows toast on result
// - refresh() sets isLoadingAccounts during fetch
// - Auto-refresh mỗi 60s (pooling usage stats)
```

---

## 8. RPC Methods

| Method | Params | Return |
|--------|--------|--------|
| `aiProvider.list` | `{ devServerId? }` | `AIProviderAccount[]` |
| `aiProvider.create` | `{ provider, label, model, ... }` | `AIProviderAccount` |
| `aiProvider.update` | `{ accountId, ...patch }` | `AIProviderAccount` |
| `aiProvider.delete` | `{ accountId }` | `void` |
| `aiProvider.writeCredential` | `{ accountId, encryptedBlob, iv }` | `void` |
| `aiProvider.testConnection` | `{ accountId }` | `{ ok, latencyMs, error? }` |
| `aiProvider.getUsage` | `{ accountId, date? }` | `AIProviderUsage` |

---

## 9. Test Plan

```
src/renderer/src/components/ai-provider/__tests__/
├── ProviderList.test.tsx          (5 tests)
│   ├── renders all account rows
│   ├── filters by devServerId
│   ├── filters by scope
│   ├── filters by status
│   └── "Add Account" button opens ProviderForm
├── ProviderForm.test.tsx          (6 tests)
│   ├── creates account — calls aiProvider.create
│   ├── updates account — calls aiProvider.update
│   ├── calls aiProvider.writeCredential when credential encrypted
│   ├── does NOT call writeCredential when credential unchanged
│   ├── Ollama provider hides base URL input
│   └── cancel button closes form
├── CredentialInput.test.tsx       (7 tests)
│   ├── shows label for anthropic provider
│   ├── type ≥10 chars → encrypts in browser
│   ├── type <10 chars → NOT encrypted yet
│   ├── after encryption → rawValue cleared from state
│   ├── shows 🔒 icon when encrypted
│   ├── calls onEncrypted with encryptedBlob + iv
│   └── ollama provider → input not rendered
├── HealthStatusBadge.test.tsx     (4 tests)
│   ├── active → green CheckCircle
│   ├── invalid → red XCircle
│   ├── quota_exceeded → orange AlertTriangle
│   └── unreachable → gray WifiOff
└── hooks/__tests__/useAIProviders.test.ts  (5 tests)
    ├── fetches accounts on mount
    ├── devServerId filter applied to fetch
    ├── testConnection ok → status 'active' in store
    ├── testConnection fail → status 'invalid' in store
    └── refresh re-fetches accounts
```

**Target:** ≥ 27 tests

---

## 10. Security Checklist

- [ ] `type="password"` trên API key input
- [ ] `autoComplete="off"` + `spellCheck={false}`
- [ ] rawValue cleared immediately sau encrypt
- [ ] encryptedBlob + iv gửi lên server — không phải plaintext
- [ ] Không log `AGENT_TOKEN` hoặc API key
- [ ] `ollama` provider → không có credential input
- [ ] Session key derived từ session token (ephemeral)
