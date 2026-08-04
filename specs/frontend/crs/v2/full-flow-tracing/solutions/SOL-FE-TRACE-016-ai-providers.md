# SOL-FE-TRACE-016: AI Provider Management — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**TDD Ref:** [TDD-FE-13](../../../../tdd/v5/13-ai-provider-ui.md) (AI Provider Admin UI, F35, ADR-008)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement); core API `Tracer.start(fields?, resume?)` từ CR-TRACE-000 §3.1 (**chưa ship** — xem SOL-FE-TRACE-015 §2.0, áp dụng tương tự ở đây)

---

## 1. Điểm khởi tạo trace trong Renderer

RPC của luồng AI Provider UI nằm ở 2 nơi khác nhau — **không phải 1 hook duy nhất** như Profile UI:

| BL | Hành động user | Component/Hook kích hoạt | RPC method | File:line hiện tại |
|----|-----------------|----------------------------|------------|----------------------|
| BL-AIP-01 (ghi credential) | Click "Save" trong `ProviderForm` sau khi đã nhập + encrypt API key | `ProviderForm.tsx:31` (`handleSave`), gọi trực tiếp `callRuntimeRpc` — **không qua hook `useAIProviders`** | `aiProvider.create`/`aiProvider.update` (metadata) rồi `aiProvider.writeCredential` (nếu `hasNewCred`) | `src/renderer/src/components/ai-provider/ProviderForm.tsx:38,41,46-50` |
| BL-AIP-03 (test connection, manual trigger) | Click nút 🧪 (`data-testid="test-btn-{id}"`) trên 1 dòng trong `ProviderList` | `ProviderList.tsx:107` (`onClick={() => testConnection(account.id)}`) → `useAIProviders().testConnection` | `aiProvider.testConnection` | `src/renderer/src/hooks/useAIProviders.ts:59-75` |
| BL-AIP-02 (resolution cascade) | — không có UI trigger trực tiếp | `ProviderResolver.resolve()` chạy phía backend, được gọi ngầm khi agent/workflow spawn — **không phải hành động click nào trong AI Provider Admin UI** | — | Ngoài phạm vi FE — xem §5 |

**Phát hiện quan trọng — bug tiền tồn tại (signature mismatch):** `ProviderForm.tsx` gọi `callRuntimeRpc('aiProvider.create', payload)` — **chỉ 2 tham số**, thiếu `target` bắt buộc đầu tiên theo signature thực tế của `callRuntimeRpc` (`src/renderer/src/runtime/runtime-rpc-client.ts:68`: `callRuntimeRpc<TResult>(target, method, params?, options?)`). Đối chiếu `useAIProviders.ts` (dùng đúng `callRuntimeRpc(target, 'aiProvider.list', ...)` nhờ `getActiveRuntimeTarget()`), `ProviderForm.tsx` là call site duy nhất trong domain AI Provider bị thiếu `target`. Vì CR này bắt buộc phải sửa đúng 3 lời gọi này để thêm `traceId` vào params, solution tiện thể sửa luôn signature cho đúng — không phải phạm vi mở rộng ngoài CR, mà là điều kiện cần để đoạn code compile đúng type khi thêm `TraceFields`.

## 2. Full Implementation

### 2.1. Ràng buộc bảo mật — nhắc lại theo CR-TRACE-016 §1

**KHÔNG được đưa `apiKey` (plaintext), `encryptedBlob`, hay `iv` vào bất kỳ `TraceFields` nào.** Chỉ trace `accountId`, `provider`, `devServerId`, `scope`, `blobLength` (độ dài chuỗi base64, không phải nội dung), `hasExisting` (boolean), latency/`ok`. Vi phạm là bug bảo mật nghiêm trọng vì `TraceEvent` có thể bị ship tới console log hoặc hiển thị trong TracePanel — bảng field bên dưới liệt kê chính xác field nào được phép trên từng span.

Điểm rủi ro cụ thể trong code hiện tại: `CredentialInput.tsx:53` gọi `encryptCredential(value, sessionToken)` (SubtleCrypto, `src/renderer/src/lib/credential-crypto.ts:40-57`) — `value` là plaintext, không bao giờ đi qua `Tracers.*`. Instrumentation trong solution này bắt đầu **SAU** khi `encryptedCred` đã có (nghĩa là plaintext đã bị GC — `CredentialInput.tsx:62` `setRawValue('')`), nên span không bao giờ nhìn thấy plaintext. Điều cần cẩn trọng thực sự là **không destructure `encryptedCred` vào field literal** khi gọi `span.start()`/`step()`/`ok()` — chỉ log `blobLength: encryptedCred.encryptedBlob.length`.

### 2.2. Thêm tracer phía browser vào `src/shared/trace/tracers.ts`

```typescript
// src/shared/trace/tracers.ts
export const Tracers = {
  // ...existing entries (bao gồm uiProfileUpdateFlow, uiProfileResolveFlow từ SOL-FE-TRACE-015)...
  // ...existing backend entries từ CR-TRACE-016 (aiProviderWriteCredFlow, aiProviderResolveFlow, aiProviderHealthFlow)...

  /** Browser-initiated: click "Save" trong ProviderForm khi có credential mới (BL-AIP-01) */
  uiAiProviderWriteCredFlow: createTracer('ui:aiProvider.writeCredential'),
  /** Browser-initiated: click "Test" trên 1 provider account (BL-AIP-03, manual trigger) */
  uiAiProviderTestConnFlow:  createTracer('ui:aiProvider.testConnection'),
} as const
```

Cùng lý do prefix `ui:` như SOL-FE-TRACE-015 §1 (tránh trùng badge với `aiProvider:writeCredential`/`aiProvider:testConnection`... của backend trong TracePanel `isBackend` heuristic).

### 2.3. `ProviderForm.tsx` — instrument + sửa signature bug (BL-AIP-01)

```typescript
// src/renderer/src/components/ai-provider/ProviderForm.tsx
import { useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Tracers } from '../../../../shared/trace/tracers'
import { CredentialInput } from './CredentialInput'
// ...existing imports unchanged...

export function ProviderForm({ account, onClose }: ProviderFormProps) {
  // ...existing useState hooks unchanged (provider, label, model, baseUrl, scope, devServer, quota, isSaving, encryptedCred, hasNewCred)...

  const handleSave = async () => {
    setIsSaving(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      const payload = { provider, label, model, baseUrl, scope, devServerId: devServer, quotaLimitDay: quota }
      let accountId = account?.id

      // Metadata create/update: KHÔNG traced riêng — theo CR-TRACE-000 §5, đây là 1 lệnh
      // CRUD đơn (không băng qua boundary quan trọng nào ngoài WS RPC 1 hop, latency thấp,
      // không phải nơi user thường gặp sự cố). Chỉ writeCredential (bước sau) đáng trace vì
      // nó băng qua relay tới Dev Server và có thể timeout/fail độc lập.
      if (!accountId) {
        const created = await callRuntimeRpc<AIProviderAccount>(target, 'aiProvider.create', payload)
        accountId = created.id
      } else {
        await callRuntimeRpc(target, 'aiProvider.update', { accountId, ...payload })
      }

      // Write credential if new one provided — BL-AIP-01, băng qua relay tới Dev Server.
      if (hasNewCred && encryptedCred) {
        // SECURITY: span fields chỉ chứa accountId/provider/blobLength — KHÔNG bao giờ
        // encryptedCred.encryptedBlob/iv. Xem §2.1.
        const span = Tracers.uiAiProviderWriteCredFlow.start({
          accountId,
          provider,
          blobLength: encryptedCred.encryptedBlob.length,
        })
        try {
          await callRuntimeRpc(target, 'aiProvider.writeCredential', {
            accountId,
            encryptedBlob: encryptedCred.encryptedBlob,
            iv:            encryptedCred.iv,
            traceId:       span.id,
          })
          span.ok({ accountId })
        } catch (err: any) {
          // SECURITY: err có thể chứa message từ backend — không đưa toàn bộ err object
          // vào fields nếu nó có khả năng echo lại input; chỉ truyền qua span.fail(err)
          // (được xử lý bởi Tracer nội bộ như message string, không phải field tuỳ ý).
          span.fail(err, { accountId })
          throw err
        }
      }

      toast.success(account ? 'Account updated' : 'Account created')
      onClose()
    } catch (err: any) {
      toast.error(err?.message ?? 'Save failed')
    } finally {
      setIsSaving(false)
    }
  }

  // ...existing JSX unchanged...
}
```

**Vì sao span chỉ bọc khối `writeCredential`, không bọc toàn bộ `handleSave`:** đúng theo scope của CR-TRACE-016 (BL-AIP-01 = "ghi credential", không phải "CRUD account nói chung"). Việc `aiProvider.create`/`update` fail sẽ khiến `catch` ngoài bắt và hiển thị toast lỗi bình thường — không cần span riêng theo nguyên tắc chống over-instrumentation (CR-TRACE-000 §5).

### 2.4. `useAIProviders.ts` — instrument `testConnection()` (BL-AIP-03, manual trigger)

```typescript
// src/renderer/src/hooks/useAIProviders.ts
import { Tracers } from '../../../shared/trace/tracers'
// ...existing imports unchanged...

export function useAIProviders(devServerIdOrFilter?: string | AIProvidersFilter) {
  // ...existing filter/accounts/refresh logic unchanged...

  const testConnection = useCallback(async (accountId: string) => {
    const target       = getActiveRuntimeTarget(useAppStore.getState().settings)
    const store        = useAppStore.getState() as any
    const updateStatus = store.updateAIAccountStatus ?? store.updateAccountStatus
    // BL-AIP-03: field `trigger: 'manual'` phân biệt với cron backend (aiProvider:healthCheck
    // không có field này vì nó luôn là background — xem CR-TRACE-016 §4 BL-AIP-03).
    const span = Tracers.uiAiProviderTestConnFlow.start({ accountId, trigger: 'manual' })
    try {
      const result = await callRuntimeRpc<{
        ok: boolean; latencyMs: number; error?: string
      }>(target, 'aiProvider.testConnection', { accountId, traceId: span.id })

      if (updateStatus) updateStatus(accountId, result.ok ? 'healthy' : 'invalid')
      span.ok({ accountId, ok: result.ok, latencyMs: result.latencyMs })
      return result
    } catch (err) {
      if (updateStatus) updateStatus(accountId, 'invalid')
      span.fail(err, { accountId })
      throw new Error('Connection test failed')
    }
  }, [])

  // ...existing deleteAccount/createAccount/updateAccount unchanged (không traced — CRUD đơn,
  // theo nguyên tắc §5 CR-TRACE-000, không phải BL-AIP-01/02/03)...

  return { accounts, isLoading: isLoadingAccounts, refresh, testConnection, deleteAccount, createAccount, updateAccount }
}
```

## 3. Test Plan (Vitest)

| File | Test case mới |
|------|----------------|
| `src/renderer/src/components/ai-provider/__tests__/ProviderForm.test.tsx` | `handleSave` với credential mới → `Tracers.uiAiProviderWriteCredFlow.start()` được gọi với field `{ accountId, provider, blobLength }` |
| | **Assert bảo mật:** field object truyền vào `start()`/`ok()`/`fail()` KHÔNG chứa key `encryptedBlob`, `iv`, hay `apiKey` (`expect(Object.keys(fields)).not.toContain('encryptedBlob')`) |
| | `aiProvider.writeCredential` RPC call nhận `traceId: span.id` trong params |
| | `handleSave` không có credential mới → KHÔNG gọi `uiAiProviderWriteCredFlow.start()` |
| | RPC `writeCredential` reject → `span.fail(err, { accountId })` được gọi trước khi lỗi propagate lên toast |
| `src/renderer/src/hooks/__tests__/useAIProviders.test.ts` | `testConnection()` gọi `Tracers.uiAiProviderTestConnFlow.start({ accountId, trigger: 'manual' })` |
| | `testConnection()` ok → `span.ok({ accountId, ok: true, latencyMs })` |
| | `testConnection()` reject → `span.fail(err, { accountId })`, status account set `'invalid'` |
| | `aiProvider.testConnection` RPC nhận `traceId: span.id` |

**Mục tiêu:** +4 test trong `ProviderForm.test.tsx` (bao gồm 1 test bảo mật bắt buộc), +3 test trong `useAIProviders.test.ts`.

## 4. Acceptance Criteria

- [ ] `Tracers.uiAiProviderWriteCredFlow` bọc đúng khối `writeCredential` trong `ProviderForm.handleSave()` — start ngay trước RPC, `ok()`/`fail()` theo kết quả
- [ ] **Không có bất kỳ `TraceFields` nào (start/step/ok/fail) trong toàn bộ CR này chứa `apiKey`, `encryptedBlob`, hoặc `iv`** — verify bằng cách grep object literal truyền vào `Tracers.uiAiProviderWriteCredFlow.*()` trong diff PR (đối chiếu CR-TRACE-016 AC gốc)
- [ ] `aiProvider.writeCredential` RPC params nhận `traceId: span.id`, cho phép backend `resume` vào cùng span (khi CR-TRACE-000 core API ship)
- [ ] Bug signature tiền tồn tại (`callRuntimeRpc('aiProvider.create', payload)` thiếu `target`) được sửa thành `callRuntimeRpc(target, 'aiProvider.create', payload)` trong `ProviderForm.tsx` — verify bằng cách kiểm tra không còn lời gọi `callRuntimeRpc` nào trong file với đúng 2 tham số
- [ ] `Tracers.uiAiProviderTestConnFlow` bọc `testConnection()` trong `useAIProviders.ts`, field `trigger: 'manual'` phân biệt với cron backend
- [ ] Tracer flow name dùng prefix `ui:`, không trùng `aiProvider:writeCredential`/`aiProvider:testConnection` phía backend (CR-TRACE-016 §3)
- [ ] BL-AIP-02 (resolution cascade) không có tracer FE nào — xác nhận không có UI trigger trực tiếp trong AI Provider Admin UI
- [ ] Test suite đạt tối thiểu 7 test case mới, bao gồm ít nhất 1 test assert KHÔNG rò rỉ credential vào trace fields
