# TASK-FE-HLD-013 — Lazy re-encrypt V1/V2 → V3 khi đọc + test cross-user

**Solution:** [SOLUTION-FE-HLD-003](../solutions/SOLUTION-FE-HLD-003-credential-store-kdf.md)
**Bug:** [BUG-FE-HLD-003](../BUG-FE-HLD-003-credential-store-kdf-missing-userid.md)
**File:** `frontend/src/main/credentials/web-credential-store.ts`
**Estimated:** 30 phút
**Status:** ✅ DONE — 2026-08-09
**Phụ thuộc:** TASK-FE-HLD-012

---

## Mục tiêu

Khi đọc thành công 1 credential ở dạng V1/V2, tự động ghi lại theo V3 ngay (self-healing migration) — không cần script batch riêng, không cần downtime.

---

## Context

Đọc trước hàm đọc credential công khai của class (tên thật cần xác nhận qua grep, gọi tạm `getDecryptedCredential` theo solution).

```bash
grep -n "async get\|readCredential\|getDecryptedCredential" frontend/src/main/credentials/web-credential-store.ts
```

---

## Thay đổi cần thực hiện

**File:** `frontend/src/main/credentials/web-credential-store.ts`

Trong hàm đọc credential công khai, thêm bước tự nâng cấp sau khi decrypt thành công:

```typescript
async getDecryptedCredential(service: string): Promise<string | null> {
  const envelope = this.readEnvelopeFromDisk(service)
  if (!envelope) return null

  const plaintext = this.decryptEnvelope(envelope)

  // Why: self-healing lazy migration — mỗi lần credential thực sự được dùng,
  // nâng cấp lên V3 ngay tại chỗ. Không cần batch script, không downtime.
  // Xem SOLUTION-FE-HLD-003 §Bước 3 — lý do chọn lazy thay vì migration 1 lần.
  if (!('version' in envelope) || envelope.version !== 'v3') {
    const upgraded = this.encryptForWrite(plaintext)
    this.writeEnvelopeToDisk(service, upgraded)
  }

  return plaintext
}
```

> [!IMPORTANT]
> Ghi đè file **sau khi** decrypt thành công, không trước — nếu decrypt fail (vd. `serverSecret` sai), không được ghi đè gì, giữ nguyên file gốc để không mất dữ liệu.

---

## ⚠️ Tên hàm thật khác kế hoạch + bổ sung `migrateToV3()`

Hàm đọc credential công khai thật tên là **`getToken(service)`**, không phải `getDecryptedCredential`. Format lưu trữ thật là buffer nhị phân với magic-byte header (`isV3()`/`decryptBlob()`), không phải object `envelope` có field `version` như bản nháp. Ngoài lazy re-encrypt trong `getToken()`, còn phát hiện file đã có sẵn `migrateV1ToV2()` — 1 hàm migration hàng loạt cùng pattern nhưng **không được gọi ở đâu cả** (dead code, đã kiểm bằng grep) — nên bổ sung thêm `migrateToV3()` theo đúng pattern đó cho nhất quán, dù không tự wire vào startup (ngoài phạm vi task này, xem "Kết quả thực thi").

## Verify

```bash
pnpm --filter frontend test -- web-credential-store
```

## Definition of Done

- [x] Đọc thành công 1 credential V1/V2 → file trên đĩa được ghi lại theo V3 ngay lập tức (trong `getToken()`)
- [x] Đọc thất bại (serverSecret sai/file hỏng) → **không** ghi đè gì — `decryptBlob()` throw trước khi chạm tới bước ghi, `getToken()`'s `catch` bắt và trả `null`, file gốc giữ nguyên
- [x] Test round-trip: tạo fixture V2 → đọc → xác nhận magic header trên đĩa đổi `"ORC2"` → `"ORC3"` → đọc lại lần 2 vẫn ra đúng plaintext (test `'lazily re-encrypts a V2 blob to V3 the first time it is read'`)
- [x] Test cross-user (thêm ở TASK-FE-HLD-012) vẫn pass sau khi thêm lazy migration — chạy chung 1 file test, không xung đột
- [~] `pnpm tsc --noEmit` — không chạy được ở mức toàn package (xem `NOTES.md`), không liên quan thay đổi này
- [x] (Bổ sung ngoài kế hoạch) `migrateToV3()` — batch migration mirror `migrateV1ToV2()` có sẵn, test riêng xác nhận migrate đúng số lượng + skip blob đã V3 + idempotent khi chạy lần 2

## Kết quả thực thi

- **File sửa:** `main/credentials/web-credential-store.ts` — logic lazy upgrade nằm trong `getToken()` (viết cùng lúc với TASK-FE-HLD-012, xem file đó cho full diff), thêm `migrateToV3()` cuối class.
- **Không wire `migrateToV3()` vào bất kỳ startup path nào** — ngoài phạm vi bug fix này (bug gốc là thiếu userId trong KDF, không phải thiếu cơ chế batch-migrate); nếu muốn dùng, gọi tương tự cách `migrateV1ToV2()` được thiết kế để gọi (hiện cả 2 đều chưa có call site nào trong `frontend/src`).
