# T18 — Tạo ai-credential-contract.ts + ai-provider-types-shared.ts [NEW FILE STRATEGY]

**Phase:** 3 (parallel với T15)  
**Effort:** ~30 minutes  
**Depends on:** T03 (shared types đã verify)  
**Solution ref:** [conflict-analysis-tdd-v6.md §C4, §C6](../../../../../conflict-analysis-tdd-v6.md)  
**TDD ref:** TDD-16 (AI Provider Mgmt)  
**⚠️ Conflict Resolution:** C4 (credential contract) + C6 (type re-export)

---

## ⚠️ QUAN TRỌNG — Quy tắc bất biến

```
❌ KHÔNG sửa: src/relay/agent-credential-store.ts   (Agent owns)
❌ KHÔNG sửa: src/relay/ai-provider-handler.ts      (Backend owns — 2-tier intentional)
❌ KHÔNG sửa: src/renderer/src/types/ai-provider-types.ts  (v5 file — GIỮ NGUYÊN)
✅ TẠO MỚI:  src/shared/ai-credential-contract.ts
✅ TẠO MỚI:  src/renderer/src/types/ai-provider-types-shared.ts
```

## Mục tiêu

1. `ai-credential-contract.ts` [NEW] — shared interface giữa Agent tier và Backend tier (C4)
2. `ai-provider-types-shared.ts` [NEW] — re-export từ `src/shared/` cho components mới (C6)

**Effort: ~30 min (đơn giản — chỉ type definitions)**

---

## Files Cần Đọc Trước

1. `src/relay/agent-credential-store.ts` — xem `healthCheck` return shape
2. `src/relay/ai-provider-handler.ts` — xem `healthCheck` return shape (phải match)
3. `src/shared/ai-provider-types.ts` — xem types hiện có để re-export
4. `src/renderer/src/types/ai-provider-types.ts` — xem types đang dùng ở renderer

---

## File 1: `src/shared/ai-credential-contract.ts` [NEW]

```typescript
/**
 * ai-credential-contract.ts — Shared interface contract for credential operations
 *
 * Đồng bộ API shape giữa:
 *   - src/relay/agent-credential-store.ts  (Agent tier — Dev Server)
 *   - src/relay/ai-provider-handler.ts     (Backend tier — Orca Server)
 *
 * Hai files trên implement RIÊNG (2-tier intentional) nhưng cần
 * cùng return shape để Frontend không bị type mismatch.
 *
 * @module shared/ai-credential-contract
 */

/**
 * Kết quả đọc credential từ bất kỳ tier nào.
 */
export interface CredentialReadResult {
  encryptedBlob: string
  iv:            string
  updatedAt:     string
}

/**
 * Kết quả kiểm tra sức khỏe AI provider connection.
 * Cả agent-credential-store.ts và ai-provider-handler.ts phải trả về shape này.
 */
export interface HealthCheckResult {
  ok:        boolean
  latencyMs: number
  error?:    string
}

/**
 * Params để write/update credential.
 */
export interface CredentialWriteParams {
  accountId:     string
  encryptedBlob: string
  iv:            string
}

/**
 * Params để thực hiện health check.
 */
export interface HealthCheckParams {
  accountId: string
  /** Timeout tính bằng ms. Default: 5000 */
  timeoutMs?: number
}
```

---

## File 2: `src/renderer/src/types/ai-provider-types-shared.ts` [NEW]

```typescript
/**
 * ai-provider-types-shared.ts — Re-export từ src/shared/ (Conflict C6 resolution)
 *
 * Cho components MỚI import từ đây để lấy types chính xác từ shared source-of-truth.
 * Components CŨ vẫn dùng ai-provider-types.ts (không đụng file đó).
 *
 * Pattern:
 *   // Component cũ (không đổi):
 *   import { AIProviderType } from '../types/ai-provider-types'
 *
 *   // Component mới (dùng file này):
 *   import { AIProviderType } from '../types/ai-provider-types-shared'
 *
 * @module renderer/types/ai-provider-types-shared
 */

// Re-export tất cả từ shared source of truth:
export type {
  AIProviderType,
  AIProviderAccount,
  AIProviderStatus,
  AIProviderScope,
} from '../../../../shared/ai-provider-types'

// Re-export shared credential contract:
export type {
  CredentialReadResult,
  HealthCheckResult,
  CredentialWriteParams,
  HealthCheckParams,
} from '../../../../shared/ai-credential-contract'
```

---

## Bước Verify

```bash
# TypeScript compile:
npx tsc --noEmit

# Verify exports đúng:
grep -n "export" src/shared/ai-credential-contract.ts
grep -n "export" src/renderer/src/types/ai-provider-types-shared.ts

# Verify file cũ không bị chỉnh:
git diff src/renderer/src/types/ai-provider-types.ts    # phải empty
git diff src/relay/agent-credential-store.ts             # phải empty
git diff src/relay/ai-provider-handler.ts                # phải empty
```

---

## Acceptance Criteria

- [x] `src/shared/ai-credential-contract.ts` tồn tại với 4 interfaces ✅ (CredentialReadResult, HealthCheckResult, CredentialWriteParams, HealthCheckParams)
- [x] `src/renderer/src/types/ai-provider-types-shared.ts` tồn tại, re-export từ shared ✅
- [x] **`src/renderer/src/types/ai-provider-types.ts` GIỮ NGUYÊN** — `git diff` empty ✅
- [x] `npx tsc --noEmit` → 0 errors ✅
- [x] Có thể import: `import type { HealthCheckResult } from 'src/shared/ai-credential-contract'` ✅
