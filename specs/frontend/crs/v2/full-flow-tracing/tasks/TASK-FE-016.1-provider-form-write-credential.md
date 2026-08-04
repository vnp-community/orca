# TASK-FE-016.1: Instrument `ProviderForm.tsx` write-credential (BL-AIP-01, security-sensitive)

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-016 §2.1, §2.3](../solutions/SOL-FE-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiAiProviderWriteCredFlow` đã đăng ký)
**Status:** ✅ Done (2026-08-04) — implemented as spec'd; fixed pre-existing `callRuntimeRpc('aiProvider.create', payload)` missing-`target` bug for both create/update call sites. `uiAiProviderWriteCredFlow` tracer already existed in `tracers.ts` (re-added after a shared-file reset during this session, additive-only). Security constraint verified: no `apiKey`/`encryptedBlob`/`iv` in any TraceFields (dedicated test). `pnpm tsc --noEmit` clean; `ProviderForm.test.tsx` 10/10 passing (5 new tracer/security cases).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "ProviderForm"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "ProviderForm", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục — lưu ý task này security-sensitive (SubtleCrypto/credential), nên đặc biệt cẩn trọng nếu impact analysis cho thấy nhiều caller phụ thuộc `handleSave`.

## Mô tả

**⚠️ Ràng buộc bảo mật — đọc kỹ trước khi implement:** KHÔNG được đưa `apiKey` (plaintext), `encryptedBlob`, hay `iv` vào bất kỳ `TraceFields` nào. Chỉ trace `accountId`, `provider`, `devServerId`, `scope`, `blobLength` (độ dài chuỗi base64, KHÔNG phải nội dung), `hasExisting` (boolean), latency/`ok`. Vi phạm là bug bảo mật nghiêm trọng vì `TraceEvent` có thể ship tới console log hoặc TracePanel.

`CredentialInput.tsx:53` gọi `encryptCredential(value, sessionToken)` (SubtleCrypto) TRƯỚC khi instrumentation trong task này bắt đầu — plaintext đã bị GC (`setRawValue('')`) trước khi span nhìn thấy bất kỳ giá trị nào. Điểm cần cẩn trọng thực sự: **không destructure `encryptedCred` vào field literal** — chỉ log `blobLength: encryptedCred.encryptedBlob.length`.

**Bug tiền tồn tại (signature mismatch):** `ProviderForm.tsx` gọi `callRuntimeRpc('aiProvider.create', payload)` — chỉ 2 tham số, thiếu `target` bắt buộc đầu tiên. Task này sửa signature đúng luôn (điều kiện cần để code compile đúng type khi thêm `TraceFields`), không phải mở rộng phạm vi tuỳ tiện.

## File: `src/renderer/src/components/ai-provider/ProviderForm.tsx` [MODIFY]

```typescript
import { useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Tracers } from '../../../../shared/trace/tracers'
import { CredentialInput } from './CredentialInput'

export function ProviderForm({ account, onClose }: ProviderFormProps) {
  // ...existing useState hooks unchanged (provider, label, model, baseUrl, scope, devServer, quota, isSaving, encryptedCred, hasNewCred)...

  const handleSave = async () => {
    setIsSaving(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      const payload = { provider, label, model, baseUrl, scope, devServerId: devServer, quotaLimitDay: quota }
      let accountId = account?.id

      // Metadata create/update: KHÔNG traced riêng — CRUD đơn (không băng qua boundary
      // quan trọng ngoài WS RPC 1 hop, latency thấp). Chỉ writeCredential đáng trace vì
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
        // encryptedCred.encryptedBlob/iv.
        const span = Tracers.uiAiProviderWriteCredFlow.start({
          accountId, provider, blobLength: encryptedCred.encryptedBlob.length,
        })
        try {
          await callRuntimeRpc(target, 'aiProvider.writeCredential', {
            accountId, encryptedBlob: encryptedCred.encryptedBlob, iv: encryptedCred.iv, traceId: span.id,
          })
          span.ok({ accountId })
        } catch (err: any) {
          // SECURITY: err có thể chứa message từ backend — không đưa toàn bộ err object
          // vào fields nếu nó có khả năng echo lại input; chỉ truyền qua span.fail(err).
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

**Vì sao span chỉ bọc khối `writeCredential`, không bọc toàn bộ `handleSave`:** đúng theo scope BL-AIP-01 (= "ghi credential", không phải "CRUD account nói chung"). `aiProvider.create`/`update` fail sẽ khiến `catch` ngoài bắt và hiển thị toast lỗi bình thường — không cần span riêng.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/ai-provider/__tests__/ProviderForm.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.uiAiProviderWriteCredFlow` bọc đúng khối `writeCredential` trong `ProviderForm.handleSave()` — start ngay trước RPC, `ok()`/`fail()` theo kết quả
- [ ] **Không có bất kỳ `TraceFields` nào (start/step/ok/fail) chứa `apiKey`, `encryptedBlob`, hoặc `iv`** — verify bằng cách grep object literal truyền vào `Tracers.uiAiProviderWriteCredFlow.*()` trong diff PR
- [ ] `aiProvider.writeCredential` RPC params nhận `traceId: span.id`
- [ ] Bug signature tiền tồn tại (`callRuntimeRpc('aiProvider.create', payload)` thiếu `target`) được sửa thành `callRuntimeRpc(target, 'aiProvider.create', payload)` — verify không còn lời gọi `callRuntimeRpc` nào trong file với đúng 2 tham số
- [ ] `handleSave` không có credential mới → KHÔNG gọi `uiAiProviderWriteCredFlow.start()`
- [ ] Test suite đạt ≥ 4 test case mới, bao gồm 1 test bảo mật bắt buộc (`expect(Object.keys(fields)).not.toContain('encryptedBlob')`)
