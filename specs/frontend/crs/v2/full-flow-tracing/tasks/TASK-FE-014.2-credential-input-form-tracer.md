# TASK-FE-014.2: Instrument `CredentialInputForm.tsx` — code thật, chưa mount, bảo mật cao

**Phase:** 2
**SOL Ref:** [SOL-FE-TRACE-014 §1.2, §2.3](../solutions/SOL-FE-TRACE-014-remote-integration.md)
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-014.1 (tracer `remoteIntegrationCredentialStoreFlow` đã khai báo)
**Status:** ✅ Done (2026-08-04) — implemented; uses `Tracers.uiRemoteIntegrationCredentialStoreFlow` (see TASK-FE-014.1 drift note for the key-name collision reason)

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "CredentialInputForm"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "CredentialInputForm", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

**Lưu ý orphan component:** `CredentialInputForm.tsx` hiện KHÔNG được mount/render ở đâu trong app (xem mục Mô tả) — `gitnexus_impact` trên symbol này nhiều khả năng trả về risk LOW hoặc không có caller thực nào. Đây là kết quả ĐÚNG NHƯ MONG ĐỢI, không phải dấu hiệu sai sót — không cần điều tra thêm vì lý do này. Riêng nội dung bảo mật (không log token/config) vẫn phải tuân thủ nghiêm ngặt bất kể mount status.

## Mô tả

`src/renderer/src/components/settings/CredentialInputForm.tsx` — form thật gọi `window.api.credentials.set(service, token, config)`/`window.api.credentials.revoke(service)` (BL-INT-02). Đã grep xác nhận **0 kết quả** mount component này ở đâu trong app. `CredentialService` type (`'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'`) không có `'github'`/`'gitlab'` trực tiếp — GitHub/GitLab PAT "mượn" slot `bitbucket`/`gitea`.

**Ràng buộc bảo mật bắt buộc (kế thừa từ CR-TRACE-014 §4):** `token`/`config` (raw PAT/API key) **không bao giờ** được đưa vào `TraceFields` — chỉ `service`/`op`. `serializeFields()` không có redaction tự động, và browser console log (`ORCA_TRACE=1`) hiển thị field y nguyên cho bất kỳ ai mở DevTools.

## File: `src/renderer/src/components/settings/CredentialInputForm.tsx` [MODIFY]

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

const handleSave = async () => {
  const missing = fields.filter(f => f.required && !values[f.key]?.trim())
  if (missing.length > 0) {
    setError(`Required: ${missing.map(f => f.label).join(', ')}`)
    return
  }

  setSaving(true)
  setError(null)
  // Why: span bọc trước validate token/config để cover cả case validate fail —
  // nhưng KHÔNG đưa token/config vào TraceFields (bảo mật).
  const span = Tracers.remoteIntegrationCredentialStoreFlow.start({ service, op: 'set' })
  try {
    const tokenKey = fields.find(f => f.type === 'password')?.key ?? 'token'
    const token = values[tokenKey] ?? ''

    const config: Record<string, string> = {}
    for (const field of fields) {
      if (field.key !== tokenKey && values[field.key]?.trim()) {
        config[field.key] = values[field.key].trim()
      }
    }

    await window.api.credentials.set(service, token, Object.keys(config).length ? config : undefined)

    setValues({})
    setSaved(true)
    setTimeout(() => setSaved(false), 3000)
    onSaved()
    span.ok({ service })
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to save credentials')
    span.fail(err, { service })
  } finally {
    setSaving(false)
  }
}

const handleRevoke = async () => {
  if (!confirm(`Remove ${service} credentials? This cannot be undone.`)) return
  setRevoking(true)
  const span = Tracers.remoteIntegrationCredentialStoreFlow.start({ service, op: 'revoke' })
  try {
    await window.api.credentials.revoke(service)
    onRevoked()
    span.ok({ service })
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to revoke credentials')
    span.fail(err, { service })
  } finally {
    setRevoking(false)
  }
}
```

> `window.api.credentials.set/revoke` là Electron `contextBridge` IPC — không có `traceId` để forward theo quy ước WS RPC; span chỉ đo latency phía renderer.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/settings/__tests__/CredentialInputForm.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.remoteIntegrationCredentialStoreFlow` không BAO GIỜ chứa `token`/`config`/giá trị credential trong bất kỳ field nào của `start()`/`ok()`/`fail()` — chỉ `service`/`op`
- [ ] `CredentialInputForm.tsx` được instrument đầy đủ dù hiện KHÔNG mount ở đâu trong app — Acceptance Criteria không yêu cầu span này thực sự emit cho tới khi có companion CR mount component
- [ ] `handleSave()` với validate fail (missing required field) → KHÔNG tạo span (return sớm trước `Tracers.start()`)
- [ ] `handleSave()` thành công → `ok({ service })`; reject → `fail(err, { service })` — KHÔNG chứa `token`/`config` trong fields
- [ ] `handleRevoke()` thành công → span với `op: 'revoke'` → `ok()`
- [ ] Test suite bắt buộc có ≥ 1 test bảo mật: assert trực tiếp `expect(Object.keys(fields)).not.toContain('encryptedBlob')`-style trên object truyền vào `start()`/`fail()`, không chỉ kiểm tra UI không hiển thị token
- [ ] Test suite đạt ≥ 4 test case mới: `handleSave` thành công → `start({ service, op: 'set' })` rồi `ok()`; `handleSave` reject → `fail()` không leak; `handleRevoke` thành công → `op: 'revoke'` → `ok()`; validate fail → không tạo span
