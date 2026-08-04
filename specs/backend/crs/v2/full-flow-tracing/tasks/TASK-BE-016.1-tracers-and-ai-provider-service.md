# TASK-BE-016.1: Đăng ký 3 tracer `aiProvider:*` và instrument `AIProviderService.writeCredentialToDevServer()` (BL-AIP-01)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-016](../solutions/SOL-BE-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + none (task đầu tiên của CR-TRACE-016)
**Status:** ✅ Done (2026-08-04) — Implemented as specified: 3 tracers added to `tracers.ts` (additive), `writeCredentialToDevServer()` instrumented with `lookup-account`/`relay-connect`/`agent-call` steps, `traceId?` param forwarded from `ai-provider-rpc-handler.ts`'s `WriteCredentialParam`. DRIFT (transient, now resolved): mid-task the shared working directory was hard-reset to HEAD multiple times by an external process (`git reflog` showed repeated `reset: moving to HEAD`), wiping this task's first pass and, separately, `src/shared/trace/index.ts`'s `Tracer.start(fields, resume)` API (TASK-BE-000). Re-applied this task's 5 files (`tracers.ts`, `AIProviderService.ts`, `ai-provider-rpc-handler.ts`, plus 016.2/016.3 files) after the reset; `index.ts` was outside this task's file scope and was independently restored by its owning task before this task closed. Final `pnpm run typecheck:node` and test run are clean for all files this task touches.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "AIProviderService.writeCredentialToDevServer"
```

Symbol đã tồn tại (MODIFY case), xử lý credential nhạy cảm (encrypted blob). Chạy:

```
gitnexus_impact({ target: "AIProviderService.writeCredentialToDevServer", direction: "upstream" })
```

(Phần đăng ký 3 tracer mới vào `Tracers` là thay đổi additive-only — chỉ cần `codegraph explore "Tracers"` để tránh trùng tên.) Báo cáo blast radius trước khi sửa — tuyệt đối không log `encryptedBlob`/`iv`/`apiKey` vào bất kỳ trace field nào. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 3 tracer mới (`aiProviderWriteCredFlow`/`aiProviderResolveFlow`/`aiProviderHealthFlow`) trong `tracers.ts`, rồi bọc `AIProviderService.writeCredentialToDevServer()` bằng span `aiProvider:writeCredential`, và forward `traceId` từ `ai-provider-rpc-handler.ts`. **Ràng buộc bảo mật tuyệt đối:** không field nào trong bất kỳ `span.start()/step()/ok()/fail()` được chứa `encryptedBlob`, `iv`, hoặc bất kỳ giá trị credential nào — kể cả debug tạm thời. Chỉ trace `accountId`, `provider`, `devServerId`, `blobLength` (số byte, không phải nội dung), `status`.

## File: `src/shared/trace/tracers.ts` [MODIFY]

```typescript
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (bao gồm 4 tracer profile:* từ TASK-BE-015.1)...

  // ── AI Provider Management (CR-TRACE-016) ───────────────────────────────────
  /** BL-AIP-01: write encrypted credential to dev server via relay */
  aiProviderWriteCredFlow: createTracer('aiProvider:writeCredential'),
  /** BL-AIP-02: priority + quota resolution cho agent/workflow spawn */
  aiProviderResolveFlow:   createTracer('aiProvider:resolve'),
  /** BL-AIP-03: background health check cron (15 phút/lần) */
  aiProviderHealthFlow:    createTracer('aiProvider:healthCheck'),
} as const
```

## File: `src/main/ai-providers/AIProviderService.ts` [MODIFY]

Chữ ký `writeCredentialToDevServer()` mở rộng ngay từ đầu để nhận `traceId` optional từ RPC layer (forward tại `ai-provider-rpc-handler.ts`, xem bên dưới):

```typescript
import { Tracers } from '../../shared/trace/tracers'

/**
 * Write an encrypted credential to the dev server via relay.
 * ORCA SERVER NEVER SEES PLAINTEXT CREDENTIAL.
 */
async writeCredentialToDevServer(
  accountId: string,
  encryptedBlob: string,
  iv: string,
  traceId?: string   // [NEW] — optional, forward từ ai-provider-rpc-handler.ts khi FE gửi kèm
): Promise<void> {
  // SECURITY: chỉ trace blobLength (số byte), KHÔNG BAO GIỜ trace encryptedBlob/iv.
  const span = Tracers.aiProviderWriteCredFlow.start(
    { accountId, blobLength: encryptedBlob.length },
    traceId ? { id: traceId } : undefined
  )

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

## File: `src/main/ai-providers/ai-provider-rpc-handler.ts` [MODIFY]

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
    // traceId resume xảy ra BÊN TRONG writeCredentialToDevServer() qua overload traceId? ở trên.
    await service.writeCredentialToDevServer(params.accountId, params.encryptedBlob, params.iv, params.traceId)
    return { success: true }
  }
}),
```

## Verification

```bash
pnpm tsc --noEmit
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] 3 tracer `aiProviderWriteCredFlow`/`aiProviderResolveFlow`/`aiProviderHealthFlow` tồn tại trong `tracers.ts` với đúng flow name `aiProvider:writeCredential`/`aiProvider:resolve`/`aiProvider:healthCheck`
- [ ] `Tracers.aiProviderWriteCredFlow` bao phủ toàn bộ `writeCredentialToDevServer()`: `lookup-account` → `relay-connect` → `agent-call` → `ok`/`fail`
- [ ] **Security-critical:** Không có bất kỳ trace event nào (start/step/ok/fail) chứa field `apiKey`, `encryptedBlob`, hoặc `iv`
- [ ] `writeCredentialToDevServer(accountId, encryptedBlob, iv, traceId?)` — chữ ký mới nhận `traceId` optional, resume đúng span khi có, tạo span mới khi không có
- [ ] `traceId: span.id` KHÔNG được đính vào cùng object `{ accountId, encryptedBlob, iv }` gửi cho `relay.call()` (theo code thật, `relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })` không mang traceId — object credential giữ nguyên tách biệt khỏi trace fields)
- [ ] `WriteCredentialParam` (`ai-provider-rpc-handler.ts`) có field `traceId?: string` optional, forward đúng vào `writeCredentialToDevServer()`
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
