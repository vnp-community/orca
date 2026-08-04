# SOL-BE-TRACE-016: AI Provider Management — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**TDD Ref:** TDD-16 (AI Provider Account Management — [16-ai-provider-management.md](../../../../tdd/v5/16-ai-provider-management.md), §3 `AIProviderService`, §4 `ProviderResolver`, §5 `ProviderHealthChecker`)
**Date:** 2026-08-02
**Status:** Proposed
**Strategy:** Additive-only — chỉ thêm tracer calls, **KHÔNG bao giờ** đưa credential value (plaintext/encrypted) vào `TraceFields`

---

## 1. Phân tích phạm vi (Backend-side only)

Đã Read trực tiếp cả 4 file backend liên quan — code thực tế hiện tại **khác đôi chỗ so với code minh hoạ trong CR-TRACE-016** (CR viết dựa trên snapshot cũ hơn); solution này bám theo implementation thật, không theo 1:1 pseudo-code của CR.

| File | Hàm cần patch | Vị trí thực tế đã verify | Gap / khác biệt so với CR |
|------|---------------|---------------------------|----------------------------|
| `src/shared/trace/tracers.ts` | thêm 3 tracer | — | ❌ Thiếu `aiProviderWriteCredFlow`/`aiProviderResolveFlow`/`aiProviderHealthFlow` |
| `src/main/ai-providers/AIProviderService.ts` | `writeCredentialToDevServer()` (227-245) | ✅ khớp CR (dòng 227) | Code thật KHÔNG có bước lookup tách rời "account + dev server" ở 2 dòng riêng như pseudo-code CR — `getAccount()` (232-233) rồi `devServerManager.get()` (235-236) là 2 câu lệnh liên tiếp, gộp thành 1 step |
| `src/main/ai-providers/ProviderResolver.ts` | `resolve()` (39-89) | ✅ khớp CR (dòng 39) | Thuật toán thật là **2-pass** (`modelHint` trước, rồi không `modelHint`) qua `scopePriority` array, không phải luồng "quota-filter → scope-match" tuyến tính đơn giản như CR minh hoạ — cần điều chỉnh field `reason`/`matchedScope` cho khớp 2-pass thật |
| `src/main/ai-providers/ProviderHealthChecker.ts` | `runCheck()` (private, 76-119) | class tại dòng 36 (khớp CR), nhưng **method thật tên `runCheck()`, không phải cycle handler inline trong `setInterval` như CR mô tả** | Code thật đã có sẵn cơ chế `onStatusChanged` callback (BUG-AIP-003) và phân loại 3 trạng thái `active`/`quota_exceeded`/`invalid` (không phải `healthy`/`degraded` nhị phân như CR) — solution bám theo 3 trạng thái thật |
| `src/main/ai-providers/ai-provider-rpc-handler.ts` | `aiProvider.writeCredential` (140-149) | ✅ khớp CR gần như tuyệt đối (dòng 141) | Method RPC tên thật là `aiProvider.writeCredential` (namespace `aiProvider.*`, không phải `ai.provider.*` như đôi chỗ trong text CR) |

**Ngoài phạm vi (agent-side):** relay handler `ai.provider.writeCredential`/`ai.provider.readCredential`/`ai.provider.testConnection` chạy **trên Dev Server** (`src/relay/ai-provider-handler.ts` theo TDD-16 §6) — đây là nơi thực sự AES-256-GCM encrypt/decrypt credential. Backend server **không bao giờ** thấy plaintext hay decrypt bất cứ gì — solution này chỉ trace tới điểm `relay.call('ai.provider.writeCredential', ...)` rời khỏi `AIProviderService`.

**Ràng buộc bảo mật (nhắc lại từ CR, bắt buộc tuân thủ tuyệt đối):** không field nào trong bất kỳ `span.start()/step()/ok()/fail()` nào của 3 tracer trong solution này được chứa `encryptedBlob`, `iv`, hoặc bất kỳ giá trị credential nào — kể cả dưới dạng debug tạm thời. Chỉ trace `accountId`, `provider`, `devServerId`, `scope`, `blobLength` (số, không phải nội dung), `status`, boolean `ok`, và latency.

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts` — thêm 3 tracer

```typescript
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (bao gồm 4 tracer profile:* từ SOL-BE-TRACE-015)...

  // ── AI Provider Management (CR-TRACE-016) ───────────────────────────────────
  /** BL-AIP-01: write encrypted credential to dev server via relay */
  aiProviderWriteCredFlow: createTracer('aiProvider:writeCredential'),
  /** BL-AIP-02: priority + quota resolution cho agent/workflow spawn */
  aiProviderResolveFlow:   createTracer('aiProvider:resolve'),
  /** BL-AIP-03: background health check cron (15 phút/lần) */
  aiProviderHealthFlow:    createTracer('aiProvider:healthCheck'),
} as const
```

### 2.2 `src/main/ai-providers/AIProviderService.ts` — BL-AIP-01

```typescript
import { Tracers } from '../../shared/trace/tracers'

/**
 * Write an encrypted credential to the dev server via relay.
 * ORCA SERVER NEVER SEES PLAINTEXT CREDENTIAL.
 */
async writeCredentialToDevServer(
  accountId: string,
  encryptedBlob: string,
  iv: string
): Promise<void> {
  // SECURITY: chỉ trace blobLength (số byte), KHÔNG BAO GIỜ trace encryptedBlob/iv.
  const span = Tracers.aiProviderWriteCredFlow.start({ accountId, blobLength: encryptedBlob.length })

  try {
    const account = await this.getAccount(accountId)
    if (!account) {
      span.fail('ACCOUNT_NOT_FOUND', { accountId })
      throw new Error(`ACCOUNT_NOT_FOUND: ${accountId}`)
    }

    const server = this.devServerManager.get(account.devServerId)
    if (!server) {
      span.fail('DEV_SERVER_NOT_FOUND', { accountId, devServerId: account.devServerId })
      throw new Error(`DEV_SERVER_NOT_FOUND: ${account.devServerId}`)
    }
    span.step('lookup-account', { accountId, devServerId: account.devServerId })

    const relay = await this.relayPool.getOrConnect(account.devServerId, server)
    span.step('relay-connect', { devServerId: account.devServerId })

    // NOTE bảo mật: params đi kèm encryptedBlob/iv thật, nhưng KHÔNG đưa chúng vào trace fields.
    span.step('agent-call', { method: 'ai.provider.writeCredential', accountId })
    await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })

    // FIX TASK-AIP-001 (đã có trong code hiện tại) — status pending → active
    await this.updateAccount(accountId, { status: 'active' })
    span.ok({ accountId, status: 'active' })
  } catch (err) {
    span.fail(err, { accountId })
    throw err
  }
}
```

### 2.3 `src/main/ai-providers/ProviderResolver.ts` — BL-AIP-02

Code thật dùng thuật toán 2-pass (`modelHint` trước, sau đó bỏ `modelHint`) qua `scopePriority = [user, project, server]`. Instrumentation bám theo đúng nhánh rẽ thật, không theo pseudo-code "quota-filter → scope-match" đơn giản của CR.

```typescript
import { Tracers } from '../../shared/trace/tracers'

async resolve(options: ResolveOptions): Promise<AIProviderAccount> {
  const { devServerId, projectId, userId, modelHint } = options
  const span = Tracers.aiProviderResolveFlow.start({ devServerId, projectId, userId, modelHint })

  try {
    const all = await this.service.listAccounts(devServerId)
    const active = all.filter(a => a.status === 'active')

    // Quota filter — điểm rẽ nhánh quan trọng (CR-TRACE-000 §5 rule 3), có network hop
    // ẩn bên trong getUsageToday() (SQLite, nhưng chạy song song N lần — đáng đo)
    const quotaCheckPromises = active
      .filter(a => a.quotaLimitDay > 0)
      .map(async (a) => {
        const usage = await this.service.getUsageToday(a.id)
        return { id: a.id, withinQuota: usage.tokens < a.quotaLimitDay }
      })
    const quotaResults = await Promise.all(quotaCheckPromises)
    const overQuotaIds = new Set(quotaResults.filter(r => !r.withinQuota).map(r => r.id))
    const available = active.filter(a => a.quotaLimitDay === 0 || !overQuotaIds.has(a.id))

    span.step('quota-filter', { totalAccounts: all.length, activeCount: active.length, overQuotaCount: overQuotaIds.size })

    if (available.length === 0) {
      span.fail('NO_PROVIDER_AVAILABLE', { reason: 'quota-or-inactive' })
      throw new Error('NO_PROVIDER_AVAILABLE: no active AI provider accounts within quota')
    }

    const scopePriority: Array<{ scope: AIProviderScope; scopeRefId?: string }> = [
      { scope: 'user', scopeRefId: userId },
      { scope: 'project', scopeRefId: projectId },
      { scope: 'server' },
    ]

    // Pass 1: với modelHint
    if (modelHint) {
      for (const { scope, scopeRefId } of scopePriority) {
        const match = this.findInScope(available, scope, scopeRefId, modelHint)
        if (match) {
          span.step('scope-match', { matchedScope: scope, usedModelHint: true })
          span.ok({ accountId: match.id, scope })
          return match
        }
      }
    }

    // Pass 2: không modelHint (any model)
    for (const { scope, scopeRefId } of scopePriority) {
      const match = this.findInScope(available, scope, scopeRefId, undefined)
      if (match) {
        span.step('scope-match', { matchedScope: scope, usedModelHint: false })
        span.ok({ accountId: match.id, scope })
        return match
      }
    }

    span.fail('NO_PROVIDER_AVAILABLE', { reason: 'no-scope-match' })
    throw new Error('NO_PROVIDER_AVAILABLE: no matching AI provider account found')
  } catch (err) {
    // fail() đã được gọi ở 2 nhánh throw phía trên; nhánh này bắt lỗi hạ tầng khác
    // (vd. listAccounts() ném exception) — tránh double span.fail() nếu err đã trace.
    if (!(err instanceof Error) || !err.message.startsWith('NO_PROVIDER_AVAILABLE')) {
      span.fail(err)
    }
    throw err
  }
}
```

> **Lưu ý:** field `reason` phân biệt rõ 2 nguyên nhân fail đúng theo acceptance criteria CR — `'quota-or-inactive'` (không còn account nào active/trong quota) vs `'no-scope-match'` (còn account nhưng không khớp scope nào ở cả 2 pass).

### 2.4 `src/main/ai-providers/ProviderHealthChecker.ts` — BL-AIP-03

Code thật là `private async runCheck(service: AIProviderService)`, không có tham số `relayPool` (đã bị remove theo BUG-AIP-004) và không throw ở mức account — mỗi account có try/catch riêng, cập nhật 1-trong-3 status (`active`/`quota_exceeded`/`invalid`) hoặc giữ nguyên khi bản thân `testConnection()`/`updateAccount()` ném lỗi.

```typescript
import { Tracers } from '../../shared/trace/tracers'

private async runCheck(service: AIProviderService): Promise<void> {
  let accounts
  try {
    accounts = await service.getAllAccounts()
  } catch (err) {
    console.warn('[ProviderHealthChecker] Failed to fetch accounts:', err)
    return // không tạo span nếu không load được account list — không có gì để đo
  }

  const span = Tracers.aiProviderHealthFlow.start({ accountCount: accounts.length })
  let activeCount = 0, quotaExceededCount = 0, invalidCount = 0, errorCount = 0

  for (const account of accounts) {
    try {
      const oldStatus = account.status
      span.step('ping-account', { accountId: account.id, provider: account.provider })

      const result = await service.testConnection(account.id)
      let newStatus: 'active' | 'quota_exceeded' | 'invalid'
      if (result.ok) newStatus = 'active'
      else if (result.error?.toLowerCase().includes('quota')) newStatus = 'quota_exceeded'
      else newStatus = 'invalid'

      span.step('ping-result', { accountId: account.id, ok: result.ok, latencyMs: result.latencyMs, newStatus })

      const checkedAt = new Date()
      await service.updateAccount(account.id, { status: newStatus, lastHealthCheck: checkedAt })

      if (newStatus === 'active') activeCount++
      else if (newStatus === 'quota_exceeded') quotaExceededCount++
      else invalidCount++

      if (oldStatus !== newStatus && this.onStatusChanged) {
        this.onStatusChanged({ accountId: account.id, oldStatus, newStatus, checkedAt })
      }
    } catch (err) {
      // Per-account failure — KHÔNG làm fail toàn bộ cycle (đúng hành vi hiện tại,
      // xem CR-TRACE-016 §4 "Lưu ý"). Đếm riêng để phân biệt với invalid do provider trả về.
      errorCount++
      console.warn(`[ProviderHealthChecker] Failed to check account ${account.id}:`, err)
    }
  }

  span.ok({ activeCount, quotaExceededCount, invalidCount, errorCount })
}
```

### 2.5 `src/main/ai-providers/ai-provider-rpc-handler.ts` — forward `traceId`

```typescript
const WriteCredentialParam = z.object({
  accountId: z.string().min(1),
  encryptedBlob: z.string().min(1),
  iv: z.string().min(1),
  traceId: z.string().optional(),  // [NEW] CR-TRACE-000 §3.3 — WS RPC row
})
```

```typescript
// ── aiProvider.writeCredential (dòng 140-149 hiện tại) ────────────────────────
defineMethod({
  name: 'aiProvider.writeCredential',
  params: WriteCredentialParam,
  handler: async (params, ctx) => {
    if (!ctx.userId) throw new Error('UNAUTHENTICATED')
    await assertAccountAccess(service, params.accountId, ctx.userId)
    // traceId resume xảy ra BÊN TRONG writeCredentialToDevServer() — nhưng hàm đó hiện
    // không nhận resume param. Patch tối thiểu: AIProviderService.writeCredentialToDevServer()
    // cần overload nhận traceId optional để resume đúng span thay vì luôn tạo span mới.
    await service.writeCredentialToDevServer(params.accountId, params.encryptedBlob, params.iv, params.traceId)
    return { success: true }
  }
}),
```

```typescript
// AIProviderService.ts — chữ ký mở rộng để nhận traceId từ RPC layer
async writeCredentialToDevServer(
  accountId: string,
  encryptedBlob: string,
  iv: string,
  traceId?: string   // [NEW] — optional, forward từ ai-provider-rpc-handler.ts khi FE gửi kèm
): Promise<void> {
  const span = Tracers.aiProviderWriteCredFlow.start(
    { accountId, blobLength: encryptedBlob.length },
    traceId ? { id: traceId } : undefined
  )
  // ...phần còn lại giữ nguyên như §2.2...
}
```

---

## 3. Test Plan (Vitest)

| Test file | Test case | Verify |
|-----------|-----------|--------|
| `src/main/ai-providers/__tests__/AIProviderService.test.ts` | `writeCredentialToDevServer() thành công → span.ok({ accountId, status: 'active' })` | mock trace sink |
| | `writeCredentialToDevServer() KHÔNG BAO GIỜ emit field encryptedBlob/iv trong bất kỳ event nào` | **security test bắt buộc** — grep toàn bộ `TraceEvent.fields` trong test, assert không có key `encryptedBlob`/`iv`/`apiKey` |
| | `writeCredentialToDevServer() với accountId không tồn tại → span.fail('ACCOUNT_NOT_FOUND')` | |
| | `writeCredentialToDevServer(traceId) → span.id === traceId` (resume) | |
| `src/main/ai-providers/__tests__/ProviderResolver.test.ts` | `resolve() hết quota → span.fail('NO_PROVIDER_AVAILABLE', { reason: 'quota-or-inactive' })` | |
| | `resolve() còn account nhưng không khớp scope → span.fail(..., { reason: 'no-scope-match' })` | |
| | `resolve() match ở pass 1 (modelHint) → step('scope-match', { usedModelHint: true })` | |
| | `resolve() match ở pass 2 (fallback, không modelHint) → step('scope-match', { usedModelHint: false })` | |
| `src/main/ai-providers/__tests__/ProviderHealthChecker.test.ts` | `runCheck() với N account → span.ok({ activeCount, quotaExceededCount, invalidCount, errorCount })` cộng đúng tổng N | |
| | `runCheck() 1 account throw exception → errorCount tăng, span vẫn ok() (không fail toàn cycle)` | |
| `src/main/ai-providers/__tests__/ai-provider-rpc.test.ts` | `aiProvider.writeCredential với params.traceId → forward đúng vào writeCredentialToDevServer()` | mock service, assert 4th arg |

**Test Targets:**

| Module | Target tests |
|--------|--------------|
| AIProviderService tracing (bao gồm security test) | ≥ 5 |
| ProviderResolver tracing | ≥ 5 |
| ProviderHealthChecker tracing | ≥ 4 |
| ai-provider-rpc-handler tracing | ≥ 2 |
| **Total** | **≥ 16** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.aiProviderWriteCredFlow` bao phủ toàn bộ `writeCredentialToDevServer()`: `lookup-account` → `relay-connect` → `agent-call` → `ok`/`fail`
- [ ] **Security-critical:** Không có bất kỳ trace event nào (start/step/ok/fail) trong toàn bộ solution chứa field `apiKey`, `encryptedBlob`, hoặc `iv` — verify bằng test grep tự động, không chỉ code review thủ công
- [ ] `Tracers.aiProviderResolveFlow` phân biệt đúng 2 lý do fail: `quota-or-inactive` vs `no-scope-match`, và phân biệt `usedModelHint: true/false` khi match thành công
- [ ] `Tracers.aiProviderHealthFlow` báo cáo đủ 4 counter (`activeCount`/`quotaExceededCount`/`invalidCount`/`errorCount`) khớp với 3 trạng thái thật của `ProviderHealthChecker` (không dùng nhị phân `healthy`/`degraded` như bản nháp CR)
- [ ] `traceId` từ RPC request `aiProvider.writeCredential` (nếu FE gửi) resume đúng vào span thay vì tạo id mới — yêu cầu mở rộng chữ ký `writeCredentialToDevServer(accountId, encryptedBlob, iv, traceId?)`
- [ ] `traceId: span.id` được forward vào `relay.call('ai.provider.writeCredential', ...)` — lưu ý: đính vào **object riêng đưa cho `relay.call()`**, không lẫn vào object credential (`{ accountId, encryptedBlob, iv }`) để tránh nhầm lẫn field nào là an toàn để log
- [ ] Review code không thêm `span.step()` cho các SQLite SELECT/UPDATE đơn (`listAccounts`, `updateAccount`, `getAccount`) theo nguyên tắc CR-TRACE-000 §5
- [ ] `gitnexus_detect_changes()` xác nhận thay đổi chỉ giới hạn trong 4 file backend + `tracers.ts`, không lan sang `src/relay/ai-provider-handler.ts` (agent-side, ngoài phạm vi)
