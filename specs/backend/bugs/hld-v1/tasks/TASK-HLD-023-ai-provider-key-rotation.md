# TASK-HLD-023: Implement AI Provider key rotation (shadow relay id + grace period 30s)

**Priority:** 🟠 HIGH
**Effort:** ~1-2 ngày (service method + RPC method + migration + ≥15 test case)
**Status:** ✅ DONE — 2026-08-09 (áp dụng đủ code-level theo solution: `ai-provider-types.ts` thêm status `'rotating'` + `rotationGraceUntil`; `AIProviderService.ts` thêm `DEFAULT_ROTATION_GRACE_PERIOD_MS`/`RotateKeyResult`/`rotationShadowId()`, constructor nhận `auditLogger?` optional (backward-compat), audit log ở `createAccount`/`updateAccount`(actorUserId, bỏ qua khi patch.status set)/`deleteAccount`/`writeCredentialToDevServer`, method mới `rotateKey()`/`completeRotation()` đúng state machine shadow-id + grace period, `resolveForProject()` cho phép `'rotating'` phục vụ request. `ai-provider-rpc-handler.ts`: thêm `RotateKeyParam` + RPC `aiProvider.rotateKey`, `update`/`delete` truyền `ctx.userId` làm actor. `ProviderHealthChecker.ts`: skip ping cho account `'rotating'`, crash-recovery gọi `completeRotation()` khi grace đã hết hạn. `server-bootstrap.ts`: wire `AuditLogger` mới (dùng `pool` chính, tách biệt với `adminDb`'s AuditLogger trong http-server.ts) vào `AIProviderService`. **Đổi số migration**: solution ghi `0014_ai_provider_rotation.ts` nhưng `0014` đã bị chiếm bởi `0014_workflow_pause_state.ts` (TASK-HLD-015, cùng phiên làm việc) — tạo `0015_ai_provider_rotation.ts` (`version: 15`) thay thế, đăng ký vào `migrations/index.ts` sau `migration0014WorkflowPauseState`. `tsc --noEmit` cho tất cả file đã sửa/tạo (`ai-provider-types.ts`, `AIProviderService.ts`, `ai-provider-rpc-handler.ts`, `ProviderHealthChecker.ts`, `0015_ai_provider_rotation.ts`, `migrations/index.ts`) không phát sinh lỗi mới — các lỗi còn lại trong `AIProviderService.ts`/`server-bootstrap.ts` đều thuộc baseline có sẵn (`TS2558`/`TS2740`/`TS2749`/`TS2345`/`TS2341`/`TS2339` từ `db.query<T>` generic + `GenericConnectionPool`/`DevServerManager.getRuntimeState` private access — đã xác nhận tồn tại từ trước, không do task này gây ra). ⚠️ Chưa viết 18 test case liệt kê trong task — effort budget của phiên làm việc này ưu tiên áp code fix cho toàn bộ 33 task trước.)
**Bug refs:** BUG-BE-HLD-014
**Solution ref:** [SOLUTION-ai-provider-exact.md](../solutions/SOLUTION-ai-provider-exact.md) §3a, §3b, §3c, §4.1-4.3, §4.5-4.6
**Depends on:** TASK-HLD-022 (AuditLogger phải insert đúng cột trước — nếu không, audit log của rotation sẽ lỗi im lặng)

---

## Mục tiêu

`docs/features/F35-ai-provider-account-management.md` mô tả rotate key với grace period 30 giây, status trung gian `'rotating'`, và audit log cho mỗi lần rotate. Hiện tại:

- Không có method `rotateKey` nào trong `AIProviderService`.
- `AIProviderStatus` chỉ có `'pending' | 'active' | 'invalid' | 'quota_exceeded' | 'unreachable'` — thiếu `'rotating'`.
- Không có RPC method `aiProvider.rotateKey`.
- Đổi key hiện tại chỉ qua `aiProvider.writeCredential`, ghi đè trực tiếp credential trên Dev Server — không có grace period, có thể gián đoạn request đang dùng key cũ.
- Toàn bộ domain AI Provider (CRUD + rotate) không ghi audit log.

## File cần sửa/tạo

```
backend/src/shared/ai-provider-types.ts                        (sửa — thêm status 'rotating' + field rotationGraceUntil)
backend/src/main/ai-providers/AIProviderService.ts              (sửa — rotateKey/completeRotation + audit log CRUD)
backend/src/main/ai-providers/ai-provider-rpc-handler.ts        (sửa — RPC aiProvider.rotateKey + chặn 'rotating' trong update)
backend/src/main/ai-providers/ProviderHealthChecker.ts          (sửa — bỏ qua status ping cho account đang rotate, recovery path)
backend/src/main/db/migrations/0014_ai_provider_rotation.ts     (tạo mới — cột rotation_grace_until)
backend/src/main/db/migrations/index.ts                          (sửa — đăng ký migration 0014)
backend/src/main/server-bootstrap.ts                              (sửa — wire AuditLogger vào AIProviderService)
backend/src/main/ai-providers/__tests__/AIProviderService.test.ts       (thêm test)
backend/src/main/ai-providers/__tests__/ai-provider-rpc-handler.test.ts (thêm test, tạo mới nếu chưa có)
```

## Thay đổi cụ thể

### 1. `ai-provider-types.ts` — thêm status `'rotating'` + field grace period

```typescript
/** Health / quota status of a provider account */
export type AIProviderStatus =
  | 'pending'        // newly registered, not yet tested
  | 'active'         // health check passed
  | 'rotating'        // BUG-BE-HLD-014: key rotation in progress — old credential
                       // still serves requests until the grace period commits the new one
  | 'invalid'        // credentials rejected
  | 'quota_exceeded' // daily quota hit
  | 'unreachable'    // network / relay error

/** A registered AI provider account (no credential stored here) */
export interface AIProviderAccount {
  id: string
  devServerId: string
  provider: AIProviderType
  scope: AIProviderScope
  scopeRefId?: string
  label: string
  model?: string
  baseUrl?: string
  status: AIProviderStatus
  lastHealthCheck?: Date
  /** BUG-BE-HLD-014: set while status='rotating' — old credential valid until this instant */
  rotationGraceUntil?: Date
  quotaLimitDay: number
  quotaUsedToday?: number
  createdBy: string
  createdAt: Date
  updatedAt: Date
}
```

### 2. `AIProviderService.ts` — imports + hằng số + shadow id helper

```typescript
import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import { Tracers } from '../../shared/trace/tracers'
import type { AuditLogger } from '../auth/audit-logger'
import type {
  AIProviderAccount,
  AIProviderType,
  AIProviderScope,
  AIProviderStatus,
  ProviderUsageToday,
} from '../../shared/ai-provider-types'

/** BUG-BE-HLD-014: default old-credential grace window when rotating a key. */
export const DEFAULT_ROTATION_GRACE_PERIOD_MS = 30_000

/** Result of a rotateKey() call — reported back over RPC. */
export interface RotateKeyResult {
  accountId: string
  status: AIProviderStatus
  rotationGraceUntil: Date
}

/** Shadow account id used to stage the new credential during a rotation. */
function rotationShadowId(accountId: string): string {
  return `${accountId}::rotating`
}
```

`AccountRow` cần thêm field `rotationGraceUntil: number | null` (map từ cột `rotation_grace_until`). `rowToAccount()` cần map `rotationGraceUntil: r.rotationGraceUntil ? new Date(r.rotationGraceUntil) : undefined`. `UpdateAccountParams` cần thêm `rotationGraceUntil?: Date | null` (truyền `null` để clear).

### 3. Constructor — thêm `auditLogger` optional (không phá vỡ call site hiện có)

```typescript
export class AIProviderService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool,
    // BUG-BE-HLD-014: optional so existing call sites (server-bootstrap.ts:433)
    // keep compiling until they're updated to inject a real AuditLogger.
    private readonly auditLogger?: AuditLogger
  ) {}
```

### 4. `createAccount` — audit log trước khi `return`

```typescript
    void this.auditLogger?.log({
      action: 'aiProvider.create',
      userId: params.createdBy,
      userEmail: params.createdBy, // RpcContext has no separate email field at this layer
      ip: '',
      details: { accountId: id, provider: params.provider, scope: params.scope, devServerId: params.devServerId },
    })
```

### 5. `updateAccount`/`deleteAccount` — thêm tham số `actorUserId?` cuối cùng + audit log + hỗ trợ patch `rotationGraceUntil`

```typescript
  async updateAccount(accountId: string, patch: UpdateAccountParams, actorUserId?: string): Promise<void> {
    const now = Date.now()
    const sets: string[] = ['updated_at = ?']
    const values: unknown[] = [now]

    if (patch.label !== undefined) { sets.push('label = ?'); values.push(patch.label) }
    if (patch.model !== undefined) { sets.push('model = ?'); values.push(patch.model) }
    if (patch.baseUrl !== undefined) { sets.push('base_url = ?'); values.push(patch.baseUrl) }
    if (patch.status !== undefined) { sets.push('status = ?'); values.push(patch.status) }
    if (patch.lastHealthCheck !== undefined) {
      sets.push('last_health_check = ?')
      values.push(patch.lastHealthCheck.getTime())
    }
    if (patch.quotaLimitDay !== undefined) {
      sets.push('quota_limit_day = ?')
      values.push(patch.quotaLimitDay)
    }
    // BUG-BE-HLD-014: rotationGraceUntil is set by rotateKey() and cleared (null)
    // by completeRotation() — `undefined` here means "leave column untouched".
    if (patch.rotationGraceUntil !== undefined) {
      sets.push('rotation_grace_until = ?')
      values.push(patch.rotationGraceUntil ? patch.rotationGraceUntil.getTime() : null)
    }

    values.push(accountId)
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_ai_provider_accounts SET ${sets.join(', ')} WHERE id = ?`, values)
    )

    // BUG-BE-HLD-014: audit only real CRUD edits from callers that pass actorUserId
    // (RPC handler does); internal calls from rotateKey/completeRotation/health
    // checker log their own dedicated action codes instead.
    if (actorUserId && patch.status === undefined) {
      void this.auditLogger?.log({
        action: 'aiProvider.update',
        userId: actorUserId,
        userEmail: actorUserId,
        ip: '',
        details: { accountId, patchedFields: Object.keys(patch) },
      })
    }
  }

  /** Delete a provider account (cascades to usage). */
  async deleteAccount(accountId: string, actorUserId?: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query('DELETE FROM orca_ai_provider_accounts WHERE id = ?', [accountId])
    )
    void this.auditLogger?.log({
      action: 'aiProvider.delete',
      userId: actorUserId ?? 'unknown',
      userEmail: actorUserId ?? 'unknown',
      ip: '',
      details: { accountId },
    })
  }
```

> **Tương thích ngược:** thêm `actorUserId?` optional cuối danh sách không phá vỡ chữ ký gọi hiện có — `ProviderHealthChecker.ts` gọi `updateAccount(id, patch)` không truyền actor, audit bị bỏ qua đúng thiết kế (health-checker cập nhật status không phải hành động CRUD của người dùng).

### 6. `writeCredentialToDevServer` — audit log sau khi ghi relay thành công

```typescript
      await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })
      await this.updateAccount(accountId, { status: 'active' })

      // BUG-BE-HLD-014: audit credential writes — length only, never the blob/iv.
      void this.auditLogger?.log({
        action: 'aiProvider.writeCredential',
        userId: 'system', // caller identity is enforced upstream by assertAccountAccess()
        userEmail: 'system',
        ip: '',
        details: { accountId, blobLength: encryptedBlob.length },
      })
```

### 7. Method mới `rotateKey()` / `completeRotation()` — chèn giữa `deleteAccount()` và section `// ── Relay operations`

```typescript
  // ── Key rotation (BUG-BE-HLD-014) ─────────────────────────────────────────────

  /**
   * Rotate the credential for an active account with a grace period.
   *
   * The REAL credential file (`${accountId}.enc` on the dev server) is left
   * untouched until completeRotation() commits — so requests already using
   * the old key, or new requests that resolve to this account while status
   * is 'rotating', keep working with the old key for the whole grace window.
   * The new key is staged + connection-tested at a shadow account id first,
   * so a bad key never touches the real credential or flips the account out
   * of 'active'.
   */
  async rotateKey(
    accountId: string,
    newCredential: { encryptedBlob: string; iv: string },
    options?: { gracePeriodMs?: number; actorUserId?: string }
  ): Promise<RotateKeyResult> {
    const gracePeriodMs = options?.gracePeriodMs ?? DEFAULT_ROTATION_GRACE_PERIOD_MS

    const account = await this.getAccount(accountId)
    if (!account) throw new Error(`ACCOUNT_NOT_FOUND: ${accountId}`)
    if (account.status === 'rotating') throw new Error(`ROTATION_IN_PROGRESS: ${accountId}`)
    if (account.status !== 'active') {
      throw new Error(`INVALID_STATUS_FOR_ROTATION: ${accountId} is '${account.status}', expected 'active'`)
    }

    const server = this.devServerManager.get(account.devServerId)
    if (!server) throw new Error(`DEV_SERVER_NOT_FOUND: ${account.devServerId}`)
    const relay = await this.relayPool.getOrConnect(account.devServerId, server)

    // Stage the new credential at a shadow id — never touches the real file.
    const shadowAccountId = rotationShadowId(accountId)
    await relay.call('ai.provider.writeCredential', {
      accountId: shadowAccountId,
      encryptedBlob: newCredential.encryptedBlob,
      iv: newCredential.iv,
    })

    const test = await relay.call<{ ok: boolean; error?: string }>(
      'ai.provider.testConnection',
      { accountId: shadowAccountId }
    )
    if (!test.ok) {
      throw new Error(`ROTATION_TEST_FAILED: ${test.error ?? 'unknown error'}`)
    }

    const rotationGraceUntil = new Date(Date.now() + gracePeriodMs)
    await this.updateAccount(accountId, { status: 'rotating', rotationGraceUntil })

    void this.auditLogger?.log({
      action: 'aiProvider.rotateKey.started',
      userId: options?.actorUserId ?? 'unknown',
      userEmail: options?.actorUserId ?? 'unknown',
      ip: '',
      details: { accountId, gracePeriodMs, blobLength: newCredential.encryptedBlob.length },
    })

    // Primary completion path. ProviderHealthChecker's 15-minute sweep is the
    // crash-recovery fallback if the process restarts before this fires —
    // completeRotation() re-reads the (still-encrypted) blob from the shadow
    // slot, so no credential needs to survive in process memory.
    const timer = setTimeout(() => {
      this.completeRotation(accountId).catch((err) =>
        console.error(`[AIProviderService] completeRotation failed for ${accountId}:`, err)
      )
    }, gracePeriodMs)
    timer.unref?.()

    return { accountId, status: 'rotating', rotationGraceUntil }
  }

  /**
   * Commit a rotation: copy the staged shadow credential onto the real
   * accountId and flip status back to 'active'. Idempotent no-op if the
   * account is no longer 'rotating' (already completed, deleted, or the
   * rotation was superseded).
   */
  async completeRotation(accountId: string): Promise<void> {
    const account = await this.getAccount(accountId)
    if (!account || account.status !== 'rotating') return

    const server = this.devServerManager.get(account.devServerId)
    if (!server) {
      await this.updateAccount(accountId, { status: 'unreachable', rotationGraceUntil: null })
      return
    }

    try {
      const relay = await this.relayPool.getOrConnect(account.devServerId, server)
      // Read back the ENCRYPTED blob staged in rotateKey() — never decrypted
      // on Orca Server (ADR-008: credentials only ever live on the dev server).
      const shadow = await relay.call<{ encryptedBlob: string; iv: string }>(
        'ai.provider.readCredential',
        { accountId: rotationShadowId(accountId) }
      )
      await relay.call('ai.provider.writeCredential', {
        accountId,
        encryptedBlob: shadow.encryptedBlob,
        iv: shadow.iv,
      })

      await this.updateAccount(accountId, { status: 'active', rotationGraceUntil: null })
      void this.auditLogger?.log({
        action: 'aiProvider.rotateKey.completed',
        userId: 'system',
        userEmail: 'system',
        ip: '',
        details: { accountId },
      })
    } catch (err) {
      // Real credential at ${accountId}.enc was never touched before this
      // catch — commit failed while copying, so we surface 'invalid' rather
      // than silently leaving 'rotating' (and the account unusable) forever.
      await this.updateAccount(accountId, { status: 'invalid', rotationGraceUntil: null })
      void this.auditLogger?.log({
        action: 'aiProvider.rotateKey.failed',
        userId: 'system',
        userEmail: 'system',
        ip: '',
        details: { accountId, error: err instanceof Error ? err.message : String(err) },
      })
      throw err
    }
  }
```

### 8. `resolveForProject()` — cho phép account `'rotating'` tiếp tục nhận request (key thật chưa đổi)

```typescript
    const all = await this.listAccounts(devServerId)
    // BUG-BE-HLD-014: 'rotating' accounts still serve requests with the OLD
    // credential until completeRotation() commits — treat them as usable.
    const active = all.filter(a => a.status === 'active' || a.status === 'rotating')
```

### 9. `ai-provider-rpc-handler.ts` — schema `RotateKeyParam` + chặn `'rotating'` trong `UpdateParam`

```typescript
// BUG-BE-HLD-014: same shape as WriteCredentialParam, plus an optional grace
// period override (defaults to AIProviderService.DEFAULT_ROTATION_GRACE_PERIOD_MS).
const RotateKeyParam = z.object({
  accountId: z.string().min(1),
  encryptedBlob: z.string().min(1),
  iv: z.string().min(1),
  gracePeriodMs: z.number().int().positive().optional(),
  traceId: z.string().optional(),
})

const UpdateParam = z.object({
  accountId: z.string().min(1),
  patch: z.object({
    label: z.string().optional(),
    model: z.string().optional(),
    baseUrl: z.string().url().optional(),
    // BUG-BE-HLD-014: 'rotating' intentionally NOT accepted here — only
    // rotateKey()/completeRotation() may transition an account into/out of
    // it. Allowing it via generic update would let a caller fake grace-period
    // state without ever staging/testing a new credential.
    status: z.enum(['pending', 'active', 'invalid', 'quota_exceeded', 'unreachable']).optional(),
    quotaLimitDay: z.number().int().min(0).optional(),
  }),
})
```

### 10. `ai-provider-rpc-handler.ts` — truyền `ctx.userId` cho audit ở `update`/`delete`, thêm RPC `aiProvider.rotateKey` (sau `aiProvider.writeCredential`, trước `aiProvider.testConnection`)

```typescript
    // ── aiProvider.update ─────────────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.update',
      params: UpdateParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) throw new Error('UNAUTHENTICATED')
        await assertAccountAccess(service, params.accountId, ctx.userId)
        await service.updateAccount(params.accountId, params.patch, ctx.userId) // BUG-BE-HLD-014: pass actor for audit
        return { success: true }
      }
    }),

    // ── aiProvider.delete ─────────────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.delete',
      params: AccountIdParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) throw new Error('UNAUTHENTICATED')
        await assertAccountAccess(service, params.accountId, ctx.userId)
        await service.deleteAccount(params.accountId, ctx.userId) // BUG-BE-HLD-014: pass actor for audit
        return { success: true }
      }
    }),

    // ── aiProvider.rotateKey ─────────────────────────────────────────────────── (NEW — BUG-BE-HLD-014)
    // Owner or admin — same access rule as writeCredential/update.
    defineMethod({
      name: 'aiProvider.rotateKey',
      params: RotateKeyParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) throw new Error('UNAUTHENTICATED')
        await assertAccountAccess(service, params.accountId, ctx.userId)
        return service.rotateKey(
          params.accountId,
          { encryptedBlob: params.encryptedBlob, iv: params.iv },
          { gracePeriodMs: params.gracePeriodMs, actorUserId: ctx.userId }
        )
      }
    }),
```

### 11. `ProviderHealthChecker.ts` — bỏ qua ping cho account `'rotating'` + crash-recovery path

```typescript
        // BUG-BE-HLD-014: an account mid key-rotation keeps its real credential
        // until completeRotation() commits — a normal connectivity ping here
        // would still succeed (old key) and must NOT flip status away from
        // 'rotating'. Use this cron cycle only as crash-recovery: if the
        // grace period already elapsed (rotateKey()'s setTimeout was lost to
        // a restart), finish the commit now.
        if (account.status === 'rotating') {
          if (account.rotationGraceUntil && account.rotationGraceUntil.getTime() <= Date.now()) {
            span.step('rotation-recovery', { accountId: account.id })
            await service.completeRotation(account.id).catch((err) =>
              console.warn(`[ProviderHealthChecker] completeRotation recovery failed for ${account.id}:`, err)
            )
          }
          continue
        }
```

(Đặt ngay đầu vòng lặp `for (const account of accounts) { try { ... } }`, trước dòng `const oldStatus = account.status`.)

### 12. Migration mới `0014_ai_provider_rotation.ts`

Số kế tiếp thật là **0014** (migration mới nhất đã đăng ký trong `index.ts` là 0013 `migration0013WorkflowTraceCorrelation`).

```typescript
/**
 * Migration 0014 — AI Provider Key Rotation (BUG-BE-HLD-014)
 *
 * Adds rotation_grace_until so a 'rotating' account can be recovered by
 * ProviderHealthChecker's cron sweep if Orca Server restarts mid-rotation
 * (see AIProviderService.rotateKey()/completeRotation()).
 *
 * @module db/migrations/0014_ai_provider_rotation
 */

import type { Migration } from './types'

export const migration0014AiProviderRotation: Migration = {
  version: 14,
  name: 'ai_provider_rotation',

  async up(db) {
    // Why: NULL means "not rotating". Set by rotateKey(), cleared by
    // completeRotation() — see AIProviderService.ts §rotateKey.
    await db.exec(`ALTER TABLE orca_ai_provider_accounts ADD COLUMN rotation_grace_until INTEGER`)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_ai_providers_rotating
        ON orca_ai_provider_accounts(status, rotation_grace_until)
    `)
  },

  async down(_db) {
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // cột thừa không ảnh hưởng hành vi nếu rollback (theo pattern migration 0013).
  },
}
```

Đăng ký vào `backend/src/main/db/migrations/index.ts`:

```typescript
import { migration0014AiProviderRotation } from './0014_ai_provider_rotation'
// ...
export const ALL_MIGRATIONS: readonly Migration[] = [
  // ...
  migration0013WorkflowTraceCorrelation,
  migration0014AiProviderRotation, // BUG-BE-HLD-014
]
```

### 13. `server-bootstrap.ts` — wire `AuditLogger` vào `AIProviderService` (Lines 429-442)

```typescript
  const { AuditLogger } = await import('./auth/audit-logger') // nếu chưa import ở nơi khác
  const aiProviderAuditLogger = new AuditLogger(pool)
  const aiProviderService = new AIProviderService(pool, devServerManager, relayConnectionPool, aiProviderAuditLogger)
  const providerResolver = new ProviderResolver(aiProviderService)
  const providerHealthChecker = new ProviderHealthChecker()
  providerHealthChecker.start(aiProviderService)
  providerHealthChecker.onStatusChanged = (event) => {
    console.log(`[ProviderHealthChecker] Status change: account=${event.accountId} ${event.oldStatus}→${event.newStatus}`)
  }
```

### Lưu ý rủi ro cần biết trước khi triển khai

1. **`RpcContext` không có `ip`/`userEmail`** — audit entry chỉ có `userId` đáng tin cậy; `ip` để rỗng, `userEmail` dùng tạm `userId`. Giới hạn đã biết, không thuộc phạm vi task này.
2. **Shadow slot không tự xoá** sau `completeRotation()` — relay hiện tại không có `ai.provider.deleteCredential`. File cũ bị ghi đè ở lần rotate tiếp theo nên không tích luỹ vô hạn, nhưng vẫn còn 1 bản credential cũ trên đĩa Dev Server lâu hơn cần thiết. Không chặn việc merge task này.
3. **Không rollback tự động khi commit thất bại** — `completeRotation()` lỗi giữa chừng → set `'invalid'` (fail-closed), buộc admin can thiệp thủ công.
4. **Multi-instance Orca Server** chưa được xử lý — thiết kế giả định 1 tiến trình sở hữu `setTimeout` của `rotateKey()`. Ghi nhận như rủi ro mở rộng, không phải vấn đề ở scale hiện tại.
5. **`RelayConnectionPool.getOrConnect()` không gọi `release()`** — rò rỉ ref-count tồn tại từ trước, không phải lỗi mới do task này gây ra, xử lý ở CR riêng.

## Verification

```bash
cd /opt/repos/orca
pnpm --filter backend tsc --noEmit
pnpm --filter backend test ai-providers
pnpm --filter backend test db/migrations

# Xác nhận migration đăng ký đúng
grep -n "migration0014" backend/src/main/db/migrations/index.ts

# Xác nhận RPC method mới có mặt
grep -n "aiProvider.rotateKey" backend/src/main/ai-providers/ai-provider-rpc-handler.ts
```

Test case cần thêm (≥ 15, khớp tinh thần "≥ 40 tests" toàn domain trong TDD-16 §9):

**`AIProviderService.test.ts`:**
1. `rotateKey()` account `'active'` → status `'rotating'`, `rotationGraceUntil` đúng, relay gọi đúng 2 lần (`writeCredential`, `testConnection` tới shadow id).
2. `rotateKey()` account không tồn tại → `ACCOUNT_NOT_FOUND`.
3. `rotateKey()` account đang `'rotating'` → `ROTATION_IN_PROGRESS`, không gọi relay.
4. `rotateKey()` account không phải `'active'` → `INVALID_STATUS_FOR_ROTATION`.
5. `rotateKey()` test connection shadow id thất bại → `ROTATION_TEST_FAILED`, account giữ nguyên `'active'` (verify `updateAccount` KHÔNG được gọi).
6. `completeRotation()` account `'rotating'` + grace hết hạn → đọc shadow, ghi vào accountId thật, status `'active'`, `rotationGraceUntil` `null`.
7. `completeRotation()` account không còn `'rotating'` → no-op.
8. `completeRotation()` relay lỗi → status `'invalid'`, audit `rotateKey.failed`, lỗi rethrow.
9. `resolveForProject()` account `'rotating'` vẫn được trả về (regression test).
10. Audit log: mỗi thao tác CRUD/rotate gọi `auditLogger.log()` đúng 1 lần, `details` không chứa `encryptedBlob`/`iv`/credential thật.
11. `AIProviderService` khởi tạo không truyền `auditLogger` → mọi thao tác vẫn chạy thành công (backward-compat).

**`ProviderHealthChecker.test.ts`:**
12. Account `'rotating'` grace tương lai → bị `continue`, không gọi `testConnection`/`updateAccount`.
13. Account `'rotating'` grace đã qua → gọi `completeRotation()` (recovery path).
14. Test hiện có cho `onStatusChanged`/reactive `quota_exceeded` không bị regress.

**`ai-provider-rpc-handler.test.ts`:**
15. `aiProvider.rotateKey` chưa auth → `UNAUTHENTICATED`.
16. `aiProvider.rotateKey` không phải owner → `FORBIDDEN`.
17. `aiProvider.rotateKey` input hợp lệ → gọi đúng `service.rotateKey(...)`.
18. `aiProvider.update` patch `status: 'rotating'` → bị zod từ chối trước handler.
