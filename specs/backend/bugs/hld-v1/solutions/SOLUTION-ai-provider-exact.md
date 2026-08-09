# SOLUTION: BUG-BE-HLD-014 & BUG-BE-HLD-015 — AI Provider key rotation + quota 80% alert

**Source-verified:** ✅ Dựa trên source code thực tế
**Files nguồn đã đọc:** `backend/src/shared/ai-provider-types.ts`, `backend/src/main/ai-providers/AIProviderService.ts`, `backend/src/main/ai-providers/ProviderHealthChecker.ts`, `backend/src/main/ai-providers/ai-provider-rpc-handler.ts`, `backend/src/main/db/migrations/0008_ai_providers.ts`, `backend/src/main/db/migrations/index.ts`, `backend/src/main/auth/audit-logger.ts`, `backend/src/main/db/migrations/0005_add_auth_schema.ts`, `backend/src/main/dev-server/relay-connection-pool.ts`, `backend/src/main/dev-server/dev-server-relay-bridge.ts`, `desktop/src/relay/ai-provider-handler.ts`, `backend/src/main/server-bootstrap.ts`, `specs/backend/tdd/v5/16-ai-provider-management.md`

---

## 1. Tóm tắt 2 bug

### BUG-BE-HLD-014 — Key rotation (grace period, status `'rotating'`, audit log) không tồn tại

`docs/features/F35-ai-provider-account-management.md` mô tả rotate key với grace period 30 giây, status trung gian `'rotating'`, và audit log cho mỗi lần rotate. Thực tế:

- Không có method `rotateKey` nào trong `AIProviderService`.
- `AIProviderStatus` (`ai-provider-types.ts:29-34`) chỉ có `'pending' | 'active' | 'invalid' | 'quota_exceeded' | 'unreachable'` — thiếu `'rotating'`.
- 9 RPC method thật trong `ai-provider-rpc-handler.ts` (`list, create, get, update, delete, writeCredential, testConnection, getUsageToday, resolve`) không có `rotateKey`.
- Đổi key hiện tại chỉ có thể qua `aiProvider.writeCredential`, **ghi đè trực tiếp** credential trên Dev Server — không có grace period, có thể làm gián đoạn request đang dùng key cũ.
- Toàn bộ domain AI Provider (CRUD + rotate) **không ghi audit log** — grep `audit_log`/`auditLog` trong `backend/src/main/ai-providers/`: 0 kết quả (đã xác nhận lại).

### BUG-BE-HLD-015 — Cảnh báo quota 80% không tồn tại, chỉ phát hiện SAU khi đã vượt

`docs/features/F35-ai-provider-account-management.md` yêu cầu cảnh báo sớm ở ngưỡng 80% quota. Thực tế: `ProviderHealthChecker.runCheck()` (`ProviderHealthChecker.ts:98-105`) chỉ set `'quota_exceeded'` khi provider **trả lỗi chứa chuỗi `"quota"`** (`result.error?.toLowerCase().includes('quota')`) — đây là phát hiện phản ứng (reactive), sau khi request đã bị chặn, không phải cảnh báo chủ động dựa trên số liệu đã có sẵn trong bảng `orca_provider_usage` (migration 0008). Không có logic nào đọc `tokens_used` rồi so với `quota_limit_day`.

---

## 2. Vị trí code hiện tại có vấn đề (trích nguyên văn)

### 2.1. `AIProviderStatus` thiếu `'rotating'`

**File:** `/opt/repos/orca/backend/src/shared/ai-provider-types.ts` — **Lines:** 28-34

```typescript
/** Health / quota status of a provider account */
export type AIProviderStatus =
  | 'pending'        // newly registered, not yet tested
  | 'active'         // health check passed
  | 'invalid'        // credentials rejected
  | 'quota_exceeded' // daily quota hit
  | 'unreachable'    // network / relay error
```

### 2.2. `AIProviderService` không có `rotateKey`, ghi đè trực tiếp qua `writeCredentialToDevServer`

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 224-271

```typescript
  // ── Relay operations ─────────────────────────────────────────────────────────

  /**
   * Write an encrypted credential to the dev server via relay.
   * NEVER stores the credential on Orca Server.
   */
  async writeCredentialToDevServer(
    accountId: string,
    encryptedBlob: string,
    iv: string,
    traceId?: string // optional — forwarded from ai-provider-rpc-handler.ts when FE sends one
  ): Promise<void> {
    ...
      await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })

      // FIX TASK-AIP-001: Update status pending → active after successful credential write.
      // Without this, resolveForProject() returns no candidates (it filters for status='active')
      // and all AI features fail silently.
      await this.updateAccount(accountId, { status: 'active' })
```

`writeCredentialToDevServer` ghi đè thẳng file credential trên Dev Server (`${accountId}.enc`, xem `desktop/src/relay/ai-provider-handler.ts:41`) — không có bước giữ song song key cũ, không grace period, không audit log. Đây cũng chính là method mà một client muốn "đổi key" buộc phải gọi.

### 2.3. Không có RPC `rotateKey`

**File:** `/opt/repos/orca/backend/src/main/ai-providers/ai-provider-rpc-handler.ts` — **Lines:** 82-195 (toàn bộ mảng trả về của `createAIProviderMethods`)

9 phần tử: `aiProvider.list, create, get, update, delete, writeCredential, testConnection, getUsageToday, resolve`. Không có `aiProvider.rotateKey`.

### 2.4. `ProviderHealthChecker` chỉ phát hiện quota_exceeded phản ứng, không cảnh báo sớm 80%

**File:** `/opt/repos/orca/backend/src/main/ai-providers/ProviderHealthChecker.ts` — **Lines:** 92-141

```typescript
    for (const account of accounts) {
      try {
        const oldStatus = account.status
        span.step('ping-account', { accountId: account.id, provider: account.provider })
        const result = await service.testConnection(account.id)

        let newStatus: 'active' | 'quota_exceeded' | 'invalid'
        if (result.ok) {
          newStatus = 'active'
        } else if (result.error?.toLowerCase().includes('quota')) {
          newStatus = 'quota_exceeded'
        } else {
          newStatus = 'invalid'
        }
        ...
        const checkedAt = new Date()
        await service.updateAccount(account.id, {
          status: newStatus,
          lastHealthCheck: checkedAt,
        })
        ...
```

Không có bất kỳ lời gọi nào tới `service.getUsageToday(account.id)` trong file này (đã grep xác nhận `getUsageToday` chỉ được gọi từ `ai-provider-rpc-handler.ts:174`, không phải từ `ProviderHealthChecker.ts`). `account.quotaLimitDay` (có sẵn trên object `AIProviderAccount` trả về từ `service.getAllAccounts()`) không được dùng ở đâu trong file này.

### 2.5. Bảng `orca_provider_usage` đã có đủ dữ liệu cần

**File:** `/opt/repos/orca/backend/src/main/db/migrations/0008_ai_providers.ts` — **Lines:** 42-57

```typescript
    // ── orca_provider_usage ───────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_provider_usage (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        account_id  TEXT    NOT NULL REFERENCES orca_ai_provider_accounts(id) ON DELETE CASCADE,
        date        TEXT    NOT NULL,
        tokens_used INTEGER NOT NULL DEFAULT 0,
        requests    INTEGER NOT NULL DEFAULT 0,
        cost_usd    REAL    NOT NULL DEFAULT 0,
        UNIQUE(account_id, date)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_provider_usage_date
        ON orca_provider_usage(account_id, date DESC)
    `)
```

Cột thật là `tokens_used`, `requests`, `cost_usd`, khoá theo `(account_id, date)`. Cột quota nằm ở bảng account, tên thật là `quota_limit_day` (`0008_ai_providers.ts:31`), map sang field TypeScript `quotaLimitDay` (`AIProviderService.ts:39`, `80`, `204-207`) — **không phải** `quotaLimitDay` trên bảng usage như tên gọi tắt trong ticket dễ gây nhầm; `quotaLimitDay` nằm trên `orca_ai_provider_accounts`, còn usage đã dùng nằm trên `orca_provider_usage.tokens_used`.

### 2.6. `AuditLogger` thật — API và một cảnh báo quan trọng về schema

**File:** `/opt/repos/orca/backend/src/main/auth/audit-logger.ts` — **Lines:** 33-64

```typescript
export class AuditLogger {
  constructor(private readonly pool: IConnectionPool) {}

  async log(entry: AuditEntry): Promise<void> {
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_audit_log
           (action, user_id, user_email, ip, user_agent, details_json, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [entry.action, entry.userId, entry.userEmail, entry.ip, entry.userAgent ?? '', JSON.stringify(entry.details ?? {}), now]
      )
    ).catch((err: unknown) => {
      console.error('[AuditLogger] Write failed (non-fatal):', err)
    })
  }
}
```

`AuditEntry` = `{ action, userId, userEmail, ip, userAgent?, details? }`. **Cảnh báo (đã xác nhận bằng cách đọc migration 0005):** bảng `orca_audit_log` thật (`backend/src/main/db/migrations/0005_add_auth_schema.ts:62-70`) có các cột `id, created_at, user_id, user_email, action, detail, ip_address` — **không có cột `ip`, `user_agent`, `details_json`** mà câu INSERT ở trên tham chiếu. Đây là một bug tồn tại độc lập trong `AuditLogger` (không thuộc phạm vi BUG-BE-HLD-014/015) — gọi `AuditLogger.log()` ở trạng thái hiện tại sẽ ném lỗi SQL "no such column: ip" khi thực thi thật. Giải pháp này **vẫn dùng `AuditLogger`/`AuditEntry` đúng theo API hiện có** như ticket yêu cầu ("dùng lại AuditLogger"), nhưng lưu ý rủi ro thực thi này ở mục 6 — nó cần được vá (đổi câu SQL trong `audit-logger.ts` cho khớp cột thật, hoặc ALTER bảng `orca_audit_log`) trước khi audit log của AI Provider có thể chạy được trong production.

---

## 3. Thiết kế giải pháp

### 3a. Thêm status `'rotating'` — vòng đời & state machine

`'rotating'` là trạng thái trung gian: **credential thật của account (accountId) không bị đổi cho tới khi grace period kết thúc**, nên request đang dùng key cũ luôn an toàn — thay vì "giữ 2 key cùng hoạt động trên cùng 1 slot" (Dev Server relay hiện chỉ lưu 1 file `${accountId}.enc` mỗi account, xem `desktop/src/relay/ai-provider-handler.ts:41`), thiết kế này **staging key mới vào một "shadow" accountId** (`"${accountId}::rotating"`) trên cùng Dev Server, dùng lại nguyên vẹn 2 relay method đã có (`ai.provider.writeCredential`, `ai.provider.testConnection`, `ai.provider.readCredential`) — không cần sửa relay/Dev Server.

State machine:

```
active ──rotateKey()──► rotating ──(grace period hết hạn, commit OK)──► active
                              │
                              └──(commit relay lỗi)──► invalid
active ──rotateKey()──► [test credential mới thất bại] ──► ném lỗi, account GIỮ NGUYÊN 'active' (không chuyển trạng thái)
rotating ──rotateKey() lần nữa──► ném lỗi ROTATION_IN_PROGRESS (chỉ 1 rotation tại 1 thời điểm/account)
```

- Chỉ account đang `'active'` mới được rotate (không rotate từ `'pending'/'invalid'/'quota_exceeded'/'unreachable'` — phải test-connection/re-active trước).
- Trong lúc `'rotating'`: `resolveForProject()` vẫn coi account là khả dụng (patch filter `status === 'active' || status === 'rotating'`), vì credential thật chưa đổi — request mới vẫn resolve về đúng account và dùng key cũ bình thường.
- `ProviderHealthChecker` (chạy mỗi 15 phút) **không được ghi đè status** của account đang `'rotating'` bằng kết quả ping thông thường (nếu không sẽ phá vỡ state machine) — thay vào đó nó dùng chính chu kỳ 15 phút này làm cơ chế **khôi phục sau crash**: nếu `rotationGraceUntil` đã trôi qua mà `completeRotation()` chưa kịp chạy (do Orca Server restart giữa chừng làm mất `setTimeout` trong bộ nhớ), sweep sẽ tự gọi lại `completeRotation()`.
- Lỗi rotate thất bại (test connection với key mới fail) → ném lỗi ngay tại `rotateKey()`, account **không đổi trạng thái**, không có gì bị ghi đè — an toàn để retry.
- Lỗi ở bước commit (hết grace period, ghi key mới vào accountId thật thất bại) → account chuyển `'invalid'` (không tự động rollback về key cũ vì lúc này chưa rõ file thật đã bị ghi 1 phần hay chưa — cần admin can thiệp thủ công, xem mục 6).

### 3b. `rotateKey(accountId, newCredential)` với grace period

- Grace period mặc định **30 giây** (khớp `docs/features/F35`/TDD-16 §8), cấu hình được qua tham số `gracePeriodMs` (per-call override, không cần thêm biến môi trường mới).
- Cơ chế "giữ cả 2 key song song": key **cũ** tiếp tục phục vụ request bình thường tại `${accountId}.enc` trên Dev Server (không đụng tới); key **mới** được ghi + test tại shadow slot `${accountId}::rotating.enc`. Sau grace period, `completeRotation()` đọc lại blob đã mã hoá từ shadow slot (`ai.provider.readCredential` — trả về `encryptedBlob`/`iv` **vẫn ở dạng mã hoá**, chưa từng giải mã trên Orca Server, giữ đúng ADR-008) rồi ghi đè vào `${accountId}.enc` thật — hoàn tất chuyển đổi.
- An toàn khi có request đang dùng key cũ: vì key thật không bị đổi cho tới bước commit cuối cùng, mọi request đang chạy dở (agent đã đọc key cũ để gọi provider) không bị gián đoạn; request mới trong lúc `'rotating'` vẫn resolve về account này và cũng dùng key cũ cho tới khi commit xong.
- `completeRotation()` được lên lịch bằng `setTimeout` ngay trong `rotateKey()`, và có đường khôi phục dự phòng qua `ProviderHealthChecker` (mục 3a) nếu tiến trình Orca Server restart giữa chừng — không phụ thuộc credential nào giữ trong bộ nhớ tiến trình, vì `completeRotation()` tự đọc lại từ shadow slot trên Dev Server.

### 3c. RPC method `aiProvider.rotateKey`

Theo đúng convention hiện có (`defineMethod`, zod schema riêng theo tên tham số, kiểm tra `ctx.userId` rồi `assertAccountAccess` — owner hoặc admin, giống `aiProvider.update`/`aiProvider.writeCredential`):

- Input: `{ accountId, encryptedBlob, iv, gracePeriodMs?, traceId? }` (cùng shape với `WriteCredentialParam`, cộng `gracePeriodMs` optional).
- Output: `{ accountId, status: 'rotating', rotationGraceUntil }`.
- `aiProvider.update` **không** được phép set `status: 'rotating'` thủ công qua patch — enum zod của `UpdateParam` cố tình không liệt kê `'rotating'`, chỉ `rotateKey`/`completeRotation` nội bộ mới được chuyển vào/ra trạng thái này (toàn vẹn state machine).

### 3d. Audit log cho CRUD + rotate

Dùng lại `AuditLogger`/`AuditEntry` y nguyên API đã đọc ở mục 2.6. `AIProviderService` nhận thêm `auditLogger?: AuditLogger` qua constructor (tham số optional thứ 4 — **không phá vỡ** lời gọi hiện tại ở `server-bootstrap.ts:433`, chỉ audit log sẽ bị bỏ qua nếu không truyền, cho tới khi call site được cập nhật — xem mục 6).

| Action code | Khi nào | metadata (`details`) — KHÔNG bao giờ chứa credential |
|---|---|---|
| `aiProvider.create` | `createAccount()` thành công | `{ accountId, provider, scope, scopeRefId, devServerId, label }` |
| `aiProvider.update` | `updateAccount()` | `{ accountId, patchedFields: Object.keys(patch) }` (chỉ tên field đổi, không giá trị nhạy cảm) |
| `aiProvider.delete` | `deleteAccount()` | `{ accountId }` |
| `aiProvider.writeCredential` | `writeCredentialToDevServer()` thành công | `{ accountId, blobLength }` — **không** `encryptedBlob`/`iv` |
| `aiProvider.rotateKey.started` | `rotateKey()` set `'rotating'` | `{ accountId, gracePeriodMs, blobLength }` |
| `aiProvider.rotateKey.completed` | `completeRotation()` thành công | `{ accountId }` |
| `aiProvider.rotateKey.failed` | `completeRotation()` commit lỗi | `{ accountId, error }` |

`actor` = `ctx.userId` từ RPC context (field `userId?: string` có sẵn trên `RpcContext`, `backend/src/main/runtime/rpc/core.ts:87`). **Giới hạn đã xác nhận:** `RpcContext` không có field `ip`/`userEmail` (khác với `auth-router.ts` là HTTP layer có `req.ip`) — nên với domain AI Provider, `ip` được ghi `''` và `userEmail` dùng tạm `userId` (không có email riêng ở tầng RPC này). Đây là giới hạn hiện tại của `RpcContext`, không phải lỗi của giải pháp này — nêu rõ ở mục 6.

### 3e. Cảnh báo quota 80%

Trong `ProviderHealthChecker.runCheck()` — vòng lặp đã có sẵn chạy mỗi account mỗi 15 phút — thêm bước: nếu `account.quotaLimitDay > 0`, gọi `service.getUsageToday(account.id)` (đã tồn tại, `AIProviderService.ts:326-337`, đọc đúng `orca_provider_usage.tokens_used`), tính `ratio = usage.tokens / account.quotaLimitDay`. Nếu `ratio >= 0.8` → phát cảnh báo qua **callback pattern giống hệt `onStatusChanged`** đã có (`ProviderHealthChecker.ts:45`, được wire ở `server-bootstrap.ts:439-442` thành `console.log` + TODO WS broadcast/webhook) — thêm `onQuotaWarning: ((event: ProviderQuotaWarning) => void) | null`. Có debounce 1 lần/account/ngày (Map nội bộ theo `accountId → date đã cảnh báo`) để tránh spam mỗi 15 phút khi vẫn ở trên ngưỡng.

---

## 4. Code cụ thể

### 4.1. `ai-provider-types.ts` — thêm `'rotating'` + field quan sát grace period

**File:** `/opt/repos/orca/backend/src/shared/ai-provider-types.ts` — **Lines:** 28-60 (thay thế đoạn tương ứng)

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
  /** Which dev server holds the encrypted credential */
  devServerId: string
  provider: AIProviderType
  scope: AIProviderScope
  /** projectId (scope='project') or userId (scope='user'); null for scope='server' */
  scopeRefId?: string
  /** Human-readable label, e.g. "Team Claude 3.5" */
  label: string
  /** Optional default model override */
  model?: string
  /** Optional base URL override (Ollama, vLLM, Azure) */
  baseUrl?: string
  status: AIProviderStatus
  lastHealthCheck?: Date
  /** BUG-BE-HLD-014: set while status='rotating' — old credential valid until this instant */
  rotationGraceUntil?: Date
  /** Daily token quota (0 = unlimited) */
  quotaLimitDay: number
  /** Tokens used today — populated on demand from orca_provider_usage */
  quotaUsedToday?: number
  createdBy: string
  createdAt: Date
  updatedAt: Date
}
```

### 4.2. `AIProviderService.ts` — `rotateKey` / `completeRotation` + CRUD audit log

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 14-25 (imports — thay thế)

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

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 27-43 (mở rộng `AccountRow`)

```typescript
/** Raw DB row from orca_ai_provider_accounts */
interface AccountRow {
  id: string
  devServerId: string
  provider: string
  scope: string
  scopeRefId: string | null
  label: string
  model: string | null
  baseUrl: string | null
  status: string
  lastHealthCheck: number | null
  /** BUG-BE-HLD-014: NULL unless status='rotating' (migration 0014, see §4.5) */
  rotationGraceUntil: number | null
  quotaLimitDay: number
  createdBy: string
  createdAt: number
  updatedAt: number
}
```

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 58-66 (mở rộng `UpdateAccountParams`)

```typescript
/** Partial update payload */
export interface UpdateAccountParams {
  label?: string
  model?: string
  baseUrl?: string
  status?: AIProviderStatus
  lastHealthCheck?: Date
  quotaLimitDay?: number
  /** BUG-BE-HLD-014: pass `null` to clear once rotation completes/fails */
  rotationGraceUntil?: Date | null
}
```

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 68-85 (`rowToAccount` — thêm mapping)

```typescript
function rowToAccount(r: AccountRow): AIProviderAccount {
  return {
    id: r.id,
    devServerId: r.devServerId,
    provider: r.provider as AIProviderType,
    scope: r.scope as AIProviderScope,
    scopeRefId: r.scopeRefId ?? undefined,
    label: r.label,
    model: r.model ?? undefined,
    baseUrl: r.baseUrl ?? undefined,
    status: r.status as AIProviderStatus,
    lastHealthCheck: r.lastHealthCheck ? new Date(r.lastHealthCheck) : undefined,
    rotationGraceUntil: r.rotationGraceUntil ? new Date(r.rotationGraceUntil) : undefined,
    quotaLimitDay: r.quotaLimitDay,
    createdBy: r.createdBy,
    createdAt: new Date(r.createdAt),
    updatedAt: new Date(r.updatedAt),
  }
}
```

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 87-92 (constructor — thêm `auditLogger` optional)

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

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 97-139 (`createAccount` — thêm audit log ở cuối, trước `return`)

```typescript
  /** Create a new provider account. Returns the created account. */
  async createAccount(params: CreateAccountParams): Promise<AIProviderAccount> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_ai_provider_accounts
           (id, dev_server_id, provider, scope, scope_ref_id, label, model, base_url,
            status, last_health_check, quota_limit_day, created_by, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id, params.devServerId, params.provider, params.scope, params.scopeRefId ?? null,
          params.label, params.model ?? null, params.baseUrl ?? null, 'pending', null,
          params.quotaLimitDay ?? 0, params.createdBy, now, now,
        ]
      )
    )

    // BUG-BE-HLD-014: audit trail for account creation. Never blocks the caller.
    void this.auditLogger?.log({
      action: 'aiProvider.create',
      userId: params.createdBy,
      userEmail: params.createdBy, // RpcContext has no separate email field at this layer
      ip: '',
      details: { accountId: id, provider: params.provider, scope: params.scope, devServerId: params.devServerId },
    })

    return {
      id,
      devServerId: params.devServerId,
      provider: params.provider,
      scope: params.scope,
      scopeRefId: params.scopeRefId,
      label: params.label,
      model: params.model,
      baseUrl: params.baseUrl,
      status: 'pending',
      quotaLimitDay: params.quotaLimitDay ?? 0,
      createdBy: params.createdBy,
      createdAt: new Date(now),
      updatedAt: new Date(now),
    }
  }
```

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 190-220 (`updateAccount`/`deleteAccount` — thêm `rotationGraceUntil` vào patch + audit log; cần `actorUserId` truyền từ RPC layer vì các method này hiện không biết ai gọi)

```typescript
  /** Update a provider account (partial patch). */
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
    // checker log their own dedicated action codes instead (see §4.3/§4.4).
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

> **Lưu ý tương thích ngược:** thêm tham số `actorUserId?` (optional, cuối danh sách) vào `updateAccount`/`deleteAccount` không phá vỡ chữ ký gọi hiện có (`ProviderHealthChecker.ts` gọi `updateAccount(id, patch)` mà không truyền actor — audit bị bỏ qua đúng như thiết kế ở điều kiện `if (actorUserId && ...)`, vì health-checker cập nhật status không phải hành động CRUD của người dùng).

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 228-271 (`writeCredentialToDevServer` — thêm audit log sau khi ghi thành công)

```typescript
      span.step('agent-call', { method: 'ai.provider.writeCredential', accountId })
      await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })

      // FIX TASK-AIP-001: Update status pending → active after successful credential write.
      await this.updateAccount(accountId, { status: 'active' })

      // BUG-BE-HLD-014: audit credential writes — length only, never the blob/iv.
      void this.auditLogger?.log({
        action: 'aiProvider.writeCredential',
        userId: 'system', // caller identity is enforced upstream by assertAccountAccess()
        userEmail: 'system',
        ip: '',
        details: { accountId, blobLength: encryptedBlob.length },
      })

      span.ok({ accountId, status: 'active' })
```

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 220-222 (chèn 2 method mới **giữa** `deleteAccount()` và section `// ── Relay operations`)

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

**File:** `/opt/repos/orca/backend/src/main/ai-providers/AIProviderService.ts` — **Lines:** 352-353 (`resolveForProject` — cho phép account đang rotate tiếp tục nhận request, vì key thật chưa đổi)

```typescript
    const all = await this.listAccounts(devServerId)
    // BUG-BE-HLD-014: 'rotating' accounts still serve requests with the OLD
    // credential until completeRotation() commits — treat them as usable.
    const active = all.filter(a => a.status === 'active' || a.status === 'rotating')
```

### 4.3. `ai-provider-rpc-handler.ts` — RPC `aiProvider.rotateKey`

**File:** `/opt/repos/orca/backend/src/main/ai-providers/ai-provider-rpc-handler.ts` — **Lines:** 57-62 (thêm schema mới sau `WriteCredentialParam`)

```typescript
const WriteCredentialParam = z.object({
  accountId: z.string().min(1),
  encryptedBlob: z.string().min(1),
  iv: z.string().min(1),
  traceId: z.string().optional(), // CR-TRACE-000 §3.3 — WS RPC row
})

// BUG-BE-HLD-014: same shape as WriteCredentialParam, plus an optional grace
// period override (defaults to AIProviderService.DEFAULT_ROTATION_GRACE_PERIOD_MS).
const RotateKeyParam = z.object({
  accountId: z.string().min(1),
  encryptedBlob: z.string().min(1),
  iv: z.string().min(1),
  gracePeriodMs: z.number().int().positive().optional(),
  traceId: z.string().optional(),
})
```

**File:** `/opt/repos/orca/backend/src/main/ai-providers/ai-provider-rpc-handler.ts` — **Lines:** 46-55 (`UpdateParam` — chú thích rõ vì sao không cho set `'rotating'` thủ công)

```typescript
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

**File:** `/opt/repos/orca/backend/src/main/ai-providers/ai-provider-rpc-handler.ts` — **Lines:** 113-124 (`aiProvider.update` — truyền `ctx.userId` cho audit) và **Lines:** 126-137 (`aiProvider.delete` — tương tự), rồi chèn method mới **sau** `aiProvider.writeCredential` (trước `aiProvider.testConnection`, dòng 156-158)

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

    // ── aiProvider.writeCredential ────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.writeCredential',
      params: WriteCredentialParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) throw new Error('UNAUTHENTICATED')
        await assertAccountAccess(service, params.accountId, ctx.userId)
        await service.writeCredentialToDevServer(
          params.accountId, params.encryptedBlob, params.iv, params.traceId
        )
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

    // ── aiProvider.testConnection ──────────────────────────────────────────────
```

### 4.4. `ProviderHealthChecker.ts` — bỏ qua status ping cho account đang rotate + cảnh báo quota 80%

**File:** `/opt/repos/orca/backend/src/main/ai-providers/ProviderHealthChecker.ts` — **Lines:** 24-45 (thêm hằng số + event type + callback mới)

```typescript
const HEALTH_CHECK_INTERVAL_MS = 15 * 60 * 1000 // 15 minutes
const QUOTA_ALERT_THRESHOLD_RATIO = 0.8 // BUG-BE-HLD-015: warn at 80% of quotaLimitDay

// ── Status change event ───────────────────────────────────────────────────────

export interface ProviderStatusChange {
  accountId:  string
  oldStatus:  string
  newStatus:  string
  checkedAt:  Date
}

// BUG-BE-HLD-015: emitted the first time an account crosses 80% of its daily quota.
export interface ProviderQuotaWarning {
  accountId:    string
  tokensUsed:   number
  quotaLimitDay: number
  ratio:        number
  checkedAt:    Date
}

// ── ProviderHealthChecker ─────────────────────────────────────────────────────

export class ProviderHealthChecker {
  private timer: ReturnType<typeof setInterval> | null = null
  // BUG-BE-HLD-015: debounce — one warning per account per calendar day so the
  // 15-minute cron doesn't re-alert every cycle while still above threshold.
  private readonly quotaWarnedOn = new Map<string, string>() // accountId -> 'YYYY-MM-DD'

  onStatusChanged: ((event: ProviderStatusChange) => void) | null = null

  /**
   * BUG-BE-HLD-015: optional callback for early quota warnings.
   * Wire this in server-bootstrap next to onStatusChanged:
   *   checker.onQuotaWarning = (e) => { wsServer.broadcast('provider:quotaWarning', e); sendWebhook(e) }
   */
  onQuotaWarning: ((event: ProviderQuotaWarning) => void) | null = null
```

**File:** `/opt/repos/orca/backend/src/main/ai-providers/ProviderHealthChecker.ts` — **Lines:** 92-139 (thay thế toàn bộ thân vòng lặp `for (const account of accounts)`)

```typescript
    for (const account of accounts) {
      try {
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

        const oldStatus = account.status
        span.step('ping-account', { accountId: account.id, provider: account.provider })
        const result = await service.testConnection(account.id)

        let newStatus: 'active' | 'quota_exceeded' | 'invalid'
        if (result.ok) {
          newStatus = 'active'
        } else if (result.error?.toLowerCase().includes('quota')) {
          newStatus = 'quota_exceeded'
        } else {
          newStatus = 'invalid'
        }
        span.step('ping-result', {
          accountId: account.id, ok: result.ok, latencyMs: result.latencyMs, newStatus,
        })

        const checkedAt = new Date()
        await service.updateAccount(account.id, { status: newStatus, lastHealthCheck: checkedAt })

        if (newStatus === 'active') activeCount++
        else if (newStatus === 'quota_exceeded') quotaExceededCount++
        else invalidCount++

        if (oldStatus !== newStatus && this.onStatusChanged) {
          console.log(`[ProviderHealthChecker] Account ${account.id}: ${oldStatus} → ${newStatus}`)
          this.onStatusChanged({ accountId: account.id, oldStatus, newStatus, checkedAt })
        }

        // BUG-BE-HLD-015: proactive 80% quota warning, independent of the
        // reactive quota_exceeded status above (which only fires once the
        // provider itself has already started rejecting requests).
        if (account.quotaLimitDay > 0) {
          const usage = await service.getUsageToday(account.id)
          const ratio = usage.tokens / account.quotaLimitDay
          const today = checkedAt.toISOString().slice(0, 10)
          if (ratio >= QUOTA_ALERT_THRESHOLD_RATIO && this.quotaWarnedOn.get(account.id) !== today) {
            this.quotaWarnedOn.set(account.id, today)
            span.step('quota-warning', { accountId: account.id, ratio, tokensUsed: usage.tokens })
            this.onQuotaWarning?.({
              accountId: account.id,
              tokensUsed: usage.tokens,
              quotaLimitDay: account.quotaLimitDay,
              ratio,
              checkedAt,
            })
          } else if (ratio < QUOTA_ALERT_THRESHOLD_RATIO) {
            this.quotaWarnedOn.delete(account.id) // usage dropped (new day) — allow re-alert later
          }
        }
      } catch (err) {
        errorCount++
        console.warn(`[ProviderHealthChecker] Failed to check account ${account.id}:`, err)
      }
    }
```

### 4.5. Migration mới đề xuất — `0014_ai_provider_rotation.ts` (KHÔNG sửa 0008; chỉ mô tả, không tạo file thật)

**Lưu ý:** đã kiểm tra `backend/src/main/db/migrations/index.ts` — migration mới nhất đã đăng ký là **0013** (`migration0013WorkflowTraceCorrelation`), nên số kế tiếp thật là **0014**, không phải `0009` (0009 đã bị `migration0009Workflows` chiếm).

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

Và đăng ký vào `backend/src/main/db/migrations/index.ts` (chỉ mô tả, không sửa thật):

```typescript
import { migration0014AiProviderRotation } from './0014_ai_provider_rotation'
// ...
export const ALL_MIGRATIONS: readonly Migration[] = [
  // ...
  migration0013WorkflowTraceCorrelation,
  migration0014AiProviderRotation, // BUG-BE-HLD-014
]
```

### 4.6. `server-bootstrap.ts` — cập nhật call site (chỉ mô tả thay đổi cần thiết, không sửa thật)

**File:** `/opt/repos/orca/backend/src/main/server-bootstrap.ts` — **Lines:** 429-442

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
  // BUG-BE-HLD-015: wire quota warnings the same way status changes are wired.
  providerHealthChecker.onQuotaWarning = (event) => {
    console.warn(
      `[ProviderHealthChecker] Quota warning: account=${event.accountId} ` +
      `${event.tokensUsed}/${event.quotaLimitDay} (${Math.round(event.ratio * 100)}%)`
    )
    // TODO: extend with rpcServer.broadcast('provider:quotaWarning', event) and webhook call
  }
```

---

## 5. Ghi chú test cần bổ sung

`backend/src/main/ai-providers/__tests__/`:

- **`AIProviderService.test.ts`**
  - `rotateKey()`: account `'active'` → status chuyển `'rotating'`, `rotationGraceUntil` được set đúng `Date.now() + gracePeriodMs`; relay được gọi đúng 2 lần (`writeCredential` tới shadow id, `testConnection` tới shadow id) — mock `relayPool.getOrConnect().call`.
  - `rotateKey()`: account không tồn tại → ném `ACCOUNT_NOT_FOUND`.
  - `rotateKey()`: account đang `'rotating'` → ném `ROTATION_IN_PROGRESS`, không gọi relay.
  - `rotateKey()`: account `'pending'/'invalid'/'quota_exceeded'/'unreachable'` → ném `INVALID_STATUS_FOR_ROTATION`.
  - `rotateKey()`: test connection ở shadow id thất bại → ném `ROTATION_TEST_FAILED`, account **giữ nguyên** `'active'` (verify `updateAccount` KHÔNG được gọi).
  - `completeRotation()`: account `'rotating'` + grace hết hạn → đọc shadow credential, ghi vào accountId thật, status → `'active'`, `rotationGraceUntil` → `null`.
  - `completeRotation()`: account không còn `'rotating'` (đã completed/deleted) → no-op, không gọi relay.
  - `completeRotation()`: relay lỗi ở bước ghi/đọc → status → `'invalid'`, audit `aiProvider.rotateKey.failed` được ghi, lỗi được rethrow.
  - `resolveForProject()`: account `'rotating'` vẫn được trả về (không còn `null`/bị loại) — regression test cho patch filter §4.2.
  - Audit log: `createAccount`/`updateAccount(..., actorUserId)`/`deleteAccount(..., actorUserId)`/`writeCredentialToDevServer`/`rotateKey`/`completeRotation` mỗi cái gọi `auditLogger.log()` đúng 1 lần với `action` tương ứng, và **không** field nào trong `details` chứa `encryptedBlob`/`iv`/credential thật (assert bằng cách kiểm tra `JSON.stringify(details)` không chứa giá trị mock của blob).
  - `AIProviderService` khởi tạo **không truyền** `auditLogger` (backward-compat) → mọi thao tác CRUD/rotate vẫn chạy thành công, không throw.

- **`ProviderHealthChecker.test.ts`**
  - Account `'rotating'` với `rotationGraceUntil` trong tương lai → bị `continue`, KHÔNG gọi `testConnection`/`updateAccount` với status mới (status giữ nguyên `'rotating'`).
  - Account `'rotating'` với `rotationGraceUntil` đã qua (mô phỏng restart giữa chừng) → gọi `service.completeRotation(accountId)` — recovery path.
  - `quotaLimitDay > 0` và `tokens/quotaLimitDay >= 0.8` → `onQuotaWarning` được gọi đúng 1 lần với `ratio` chính xác.
  - Gọi `runCheck()` 2 lần liên tiếp trong cùng ngày, vẫn trên ngưỡng 80% → `onQuotaWarning` chỉ được gọi ở lần đầu (debounce theo ngày).
  - `quotaLimitDay === 0` (unlimited) → không bao giờ gọi `onQuotaWarning` dù usage lớn.
  - Usage tụt xuống dưới 80% sau khi đã cảnh báo (ví dụ sang ngày mới, `getUsageToday` reset) → debounce map được xoá, sẵn sàng cảnh báo lại nếu vượt ngưỡng lần nữa.
  - Test hiện có cho `onStatusChanged`/reactive `quota_exceeded` (dựa theo lỗi provider) không được regress bởi thay đổi ở §4.4.

- **`ai-provider-rpc-handler.test.ts`** (nếu file test cho RPC layer tồn tại, hoặc tạo mới theo pattern các RPC method khác)
  - `aiProvider.rotateKey`: chưa auth (`ctx.userId` rỗng) → `UNAUTHENTICATED`.
  - `aiProvider.rotateKey`: không phải owner → `FORBIDDEN` (qua `assertAccountAccess`, giống `aiProvider.update`).
  - `aiProvider.rotateKey`: input hợp lệ → gọi đúng `service.rotateKey(accountId, {encryptedBlob, iv}, {gracePeriodMs, actorUserId: ctx.userId})`.
  - `aiProvider.update`: patch chứa `status: 'rotating'` → bị zod schema từ chối trước khi tới handler (400/validation error).

**Mục tiêu:** ≥ 20 test mới cho 2 bug này (khớp tinh thần "≥ 40 tests" toàn domain trong TDD-16 §9).

---

## 6. Rủi ro / lưu ý triển khai

1. **Bug độc lập trong `AuditLogger` (mục 2.6) phải được vá song song** — câu SQL `INSERT INTO orca_audit_log (..., ip, user_agent, details_json, ...)` tham chiếu 3 cột không tồn tại trong bảng thật (migration 0005 chỉ có `ip_address`, `detail`, không có `user_agent`). Nếu không vá, mọi lời gọi `auditLogger.log()` (kể cả các call mới thêm ở giải pháp này) sẽ throw SQL error — được `.catch()` nuốt và log ra console (non-fatal, theo đúng thiết kế "audit is best-effort"), nghĩa là **audit log của AI Provider sẽ âm thầm không ghi được gì** cho tới khi bug này được fix. Khuyến nghị: sửa `audit-logger.ts` để map đúng `ip → ip_address`, `details_json → detail`, bỏ `user_agent` (hoặc `ALTER TABLE orca_audit_log ADD COLUMN user_agent TEXT` trong 1 migration riêng) — đây là prerequisite thực tế, không nằm trong 2 bug ticket này nhưng ảnh hưởng trực tiếp tới việc audit log ở mục 3d có hoạt động được không.

2. **`RpcContext` không có `ip`/`userEmail`** — audit entry cho domain AI Provider chỉ có `userId` đáng tin cậy; `ip` bị để rỗng, `userEmail` dùng tạm `userId`. Nếu cần attribution chính xác hơn (địa chỉ IP, email thật), cần mở rộng `RpcContext` ở tầng `runtime/rpc/core.ts` — không thuộc phạm vi 2 bug này, chỉ nêu như một giới hạn đã biết.

3. **Bảo mật credential:** shadow slot `${accountId}::rotating.enc` trên Dev Server **không có cơ chế xoá tự động** sau khi `completeRotation()` xong (relay hiện tại — `desktop/src/relay/ai-provider-handler.ts` — không có handler `ai.provider.deleteCredential`). File cũ bị ghi đè ở lần rotate tiếp theo nên không tích luỹ vô hạn, nhưng vẫn còn 1 bản credential "cũ nhất vừa rotate" nằm trên đĩa Dev Server lâu hơn cần thiết cho tới lần rotate kế tiếp. Khuyến nghị theo dõi riêng (không chặn việc merge 2 fix này) — nếu cần dọn ngay, phải thêm 1 relay method mới `ai.provider.deleteCredential`, nằm ngoài phạm vi các file được liệt kê để sửa.

4. **Không rollback tự động khi commit thất bại:** nếu `completeRotation()` lỗi ở đúng lúc ghi vào `${accountId}.enc` thật (relay call `writeCredential` timeout/disconnect giữa chừng), không thể chắc chắn file thật đã bị ghi 1 phần hay chưa — service chủ động set `'invalid'` thay vì đoán và tự rollback, buộc admin gọi lại `writeCredential`/`rotateKey` thủ công sau khi xác minh trạng thái thật trên Dev Server. Đây là lựa chọn an toàn có chủ đích (fail-closed), cần nêu rõ trong tài liệu vận hành.

5. **Multi-instance Orca Server:** thiết kế hiện tại giả định **1 tiến trình** Orca Server sở hữu `setTimeout` của `rotateKey()`. Nếu tương lai chạy nhiều instance Orca Server (HA) cùng trỏ 1 DB, cần đảm bảo chỉ 1 instance chạy `completeRotation()` cho mỗi account (ví dụ dùng advisory lock qua DB, hoặc chỉ instance "leader" chạy `ProviderHealthChecker`) — nếu không, 2 instance có thể cùng `completeRotation()` song song và gọi trùng `relay.call('ai.provider.writeCredential', ...)` (vô hại vì idempotent — ghi cùng 1 giá trị — nhưng lãng phí và có thể log audit trùng). Hiện tại (single-instance) không phải vấn đề; ghi nhận như rủi ro mở rộng.

6. **`RelayConnectionPool.getOrConnect()` yêu cầu `release()` sau khi dùng** (theo doc-comment `relay-connection-pool.ts:8-13`) nhưng cả `writeCredentialToDevServer`/`testConnection` hiện có **lẫn** `rotateKey`/`completeRotation` mới đều không gọi `this.relayPool.release(devServerId)` — giữ nguyên pattern hiện tại của codebase để nhất quán, nhưng đây là một rò rỉ ref-count tồn tại từ trước (không phải lỗi mới do 2 fix này gây ra), nên được xử lý ở một CR riêng bao trùm toàn bộ `AIProviderService`.

7. **Tương thích ngược:** thêm tham số optional (`auditLogger` ở constructor; `actorUserId` cuối cùng ở `updateAccount`/`deleteAccount`) giữ nguyên mọi lời gọi hiện có (kể cả `ProviderHealthChecker.updateAccount(id, patch)` không truyền actor) — không cần sửa gì khác ngoài `server-bootstrap.ts` (mục 4.6) để bắt đầu ghi audit log thật.
