# SOL-FE-V6-003: AI Provider Admin UI (TDD-FE-13)

**Solution ID:** SOL-FE-V6-003
**TDD Ref:** [TDD-FE-13](../../../../tdd/v5/13-ai-provider-ui.md)
**Feature:** F35 | **ADR:** ADR-008 | **HLD Ref:** C3.11a
**Date:** 2026-07-30
**Status:** ✅ COMPLETED — 2026-07-30

---

## 1. Phan tich code hien co

### 1.1 Da ton tai (KHONG viet lai)

| File | Size | Nhan xet |
|------|------|---------|
| `components/ai-provider/ProviderList.tsx` | 5854 bytes | Co san — day du theo TDD |
| `components/ai-provider/ProviderForm.tsx` | 5320 bytes | Co san — can kiem tra credential flow |
| `components/ai-provider/CredentialInput.tsx` | 3412 bytes | Co san — CRITICAL: kiem tra khong log raw value |
| `components/ai-provider/HealthStatusBadge.tsx` | 1324 bytes | Co san — day du |
| `components/ai-provider/UsageChart.tsx` | 1432 bytes | Co san — day du |
| `store/slices/ai-provider-slice.ts` | 2391 bytes | Co san — day du 7 actions |
| `hooks/useAIProviders.ts` | 2279 bytes | Co san — can kiem tra RPC method names |

### 1.2 Chua ton tai / Can kiem tra

| File | Nhan xet |
|------|---------|
| `types/ai-provider-types.ts` | Can kiem tra co day du types khong |
| Test files trong `__tests__/` | Can tao moi |

---

## 2. CRITICAL: Credential Security Verification

**Day la diem quan trong nhat cua TDD-FE-13 theo ADR-008:**

**Kiem tra CredentialInput.tsx phai dam bao:**

```typescript
// 1. Input type="password" — KHONG cho browser autofill/luu
// 2. Raw value duoc clear ngay sau khi encrypt thanh cong
// 3. KHONG log credential value bat ky cho (console.log, Sentry)
// 4. SubtleCrypto encrypt TRUOC khi gui len server

// Pattern can verify trong CredentialInput.tsx:
const handleChange = async (value: string) => {
  setRawValue(value)  // temporary — se duoc clear
  
  if (value.length > 10) {
    setIsEncrypting(true)
    try {
      // SubtleCrypto encrypt
      const iv = crypto.getRandomValues(new Uint8Array(16))
      // ... encrypt logic ...
      onEncrypted(encryptedBlob, ivB64)
      setIsEncrypted(true)
    } finally {
      setIsEncrypting(false)
      setRawValue('')  // CLEAR raw value sau khi encrypt
      // KHONG console.log value bat ky dau
    }
  }
}
```

**Neu CredentialInput.tsx chua implement dung:**

Gap co the co:
- Raw value chua duoc clear sau encrypt
- Missing `deriveSessionKey()` function
- Missing `autoComplete="off"` tren input

---

## 3. Giai phap — useAIProviders Hook Verification

**Can kiem tra RPC method names trong `hooks/useAIProviders.ts`:**

```typescript
// TDD-FE-13 spec cac RPC methods:
'aiProvider.list'            // lay danh sach accounts
'aiProvider.create'          // tao moi account
'aiProvider.update'          // cap nhat account
'aiProvider.testConnection'  // test ket noi
'aiProvider.writeCredential' // ghi credential (qua relay)
'aiProvider.delete'          // xoa account

// Kiem tra trong useAIProviders.ts co dung method names khong
// Vi du - neu dang dung 'ai-providers.list' thi can doi thanh 'aiProvider.list'
```

**Gap can xu ly:** RPC method naming convention co the sai (dash vs camelCase).
Theo HLD C4.9: dung `aiProvider.*` (camelCase)

---

## 4. Giai phap — ProviderList Filtering

**ProviderList.tsx (5854 bytes) — Kiem tra filtering logic:**

```typescript
// TDD-FE-13 yeu cau 3 filters:
// 1. filterServer: devServerId
// 2. filterScope: 'server' | 'project' | 'user'
// 3. filterStatus: 'healthy' | 'quota_exceeded' | 'invalid' | 'unreachable'

// Dam bao devServers list duoc load tu store:
const servers = useAppStore(s => s.devServers)  // lay tu dev-servers slice

// Dam bao filterServer options duoc populate tu actual servers list
```

---

## 5. Giai phap — Admin SPA Integration

**File can boi sung:** `src/renderer/src/components/admin/AdminApp.tsx`

```typescript
// MODIFY (additive): Them route /admin/ai-providers
import { ProviderList } from '../ai-provider/ProviderList'

// Trong Router:
<Route path="/ai-providers" element={<ProviderList />} />

// Them menu item trong admin sidebar navigation
```

---

## 6. Type Definitions Verification

**File can kiem tra:** `src/renderer/src/types/ai-provider-types.ts`

Phai co day du:

```typescript
export type AIProviderType = 
  'anthropic' | 'openai' | 'gemini' | 'azure-openai' | 'bedrock' | 'ollama' | 'vllm'

export type AIProviderScope = 'server' | 'project' | 'user'

export type AIProviderStatus = 'healthy' | 'quota_exceeded' | 'invalid' | 'unreachable'

export interface AIProviderAccount {
  id: string
  devServerId: string
  label: string
  provider: AIProviderType
  scope: AIProviderScope
  scopeId?: string      // projectId hoac userId
  model: string
  endpoint?: string
  status: AIProviderStatus
  lastTestedAt?: Date
  createdAt: Date
  usageToday?: number
  usageLimit?: number
}

export interface AIProviderUsage {
  accountId: string
  tokens: number
  date: string
}
```

---

## 7. Test Plan

**Target:** >= 25 tests | **Dac biet:** CredentialInput phai test raw value KHONG ton tai sau encrypt

```
src/renderer/src/components/ai-provider/__tests__/
├── ProviderList.test.tsx            (5+ tests)
│   ├── renders account rows with provider, label, server
│   ├── filters by server (devServerId match)
│   ├── filters by scope (server/project/user)
│   ├── filters by status (healthy/invalid/quota_exceeded)
│   └── Add Account button opens ProviderForm
├── ProviderForm.test.tsx            (5+ tests)
│   ├── renders in create mode (no account prop)
│   ├── renders in edit mode (existing account prop)
│   ├── calls aiProvider.create on save (new)
│   ├── calls aiProvider.update on save (existing)
│   ├── calls aiProvider.writeCredential when credential encrypted
│   └── does NOT call writeCredential when credential unchanged
├── CredentialInput.test.tsx         (6+ tests) [CRITICAL TESTS]
│   ├── renders input type="password"
│   ├── value < 10 chars: NOT encrypted yet
│   ├── value >= 10 chars: SubtleCrypto encrypt called
│   ├── after encryption: raw value CLEARED from state (state = '')
│   ├── after encryption: shows lock icon
│   └── ollama provider: input NOT rendered (no credential needed)
├── HealthStatusBadge.test.tsx       (3+ tests)
│   ├── healthy => green CheckCircle
│   ├── invalid => red XCircle
│   └── quota_exceeded => orange AlertTriangle
└── hooks/__tests__/useAIProviders.test.ts (5+ tests)
    ├── fetches accounts on mount via aiProvider.list
    ├── filters by devServerId when provided
    ├── testConnection ok => status updated to 'healthy'
    ├── testConnection fail => status updated to 'invalid'
    └── refresh() re-fetches accounts
```

---

## 8. Phu thuoc va Thu tu

**Prerequisite:** Khong co (doc lap hoat dong)

**Cai dat sau khi implement:**
- Admin SPA se co `/admin/ai-providers` page
- Settings panel co the include `ProviderList` trong tab "AI Providers"
- `useAIProviders` co the duoc dung trong `AgentPanel` de hien thi provider hien tai
