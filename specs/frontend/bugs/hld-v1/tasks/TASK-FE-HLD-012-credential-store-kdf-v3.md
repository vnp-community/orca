# TASK-FE-HLD-012 — Thêm KDF V3 (userId + IV 12 byte) + versioned envelope

**Solution:** [SOLUTION-FE-HLD-003](../solutions/SOLUTION-FE-HLD-003-credential-store-kdf.md)
**Bug:** [BUG-FE-HLD-003](../BUG-FE-HLD-003-credential-store-kdf-missing-userid.md)
**File:** `frontend/src/main/credentials/web-credential-store.ts`
**Estimated:** 45 phút
**Status:** ✅ DONE — 2026-08-09

---

## Mục tiêu

Thêm biến thể KDF V3 đưa `userId` vào input của `scryptSync`, IV về đúng 12 byte, và envelope có version để đọc được cả blob V1/V2 cũ — **chưa** làm migration dữ liệu cũ ở task này (xem TASK-FE-HLD-013).

---

## Context

```bash
grep -n "scryptSync\|V2_IV_BYTES\|userCredDir\|class WebCredentialStore" frontend/src/main/credentials/web-credential-store.ts
```

Đọc trước toàn bộ `web-credential-store.ts` — đặc biệt dòng 28 (IV bytes), 54-61 (constructor), 59/77 (2 chỗ scryptSync hiện tại), 86-91 (nơi lưu salt).

---

## Thay đổi cần thực hiện

**File:** `frontend/src/main/credentials/web-credential-store.ts`

Thêm hằng số và hàm KDF mới, đặt cạnh `V2_IV_BYTES` hiện có:

```typescript
const V3_IV_BYTES = 12 // Why: đúng chuẩn AES-GCM 96-bit đã ghi trong security.md §11
                         // (V2's 16-byte IV vẫn hỗ trợ đọc cũ, không tạo mới nữa)

function deriveKeyV3(serverSecret: string, userId: string, salt: Buffer): Buffer {
  // Why: userId đi vào password input của scrypt (không phải chỉ path) để
  // 1 serverSecret leak + 1 blob leak không đủ giải mã blob của user khác —
  // xem BUG-FE-HLD-003.
  return scryptSync(`${serverSecret}:${userId}`, salt, 32)
}
```

Đổi cấu trúc envelope lưu trên đĩa từ dạng cũ (không version) sang có `version` field:

```typescript
type CredentialEnvelopeV1V2 = { /* cấu trúc cũ hiện tại, giữ nguyên để đọc */ }
type CredentialEnvelopeV3 = {
  version: 'v3'
  salt: string    // base64
  iv: string      // base64, 12 byte
  ciphertext: string // base64
}
```

Sửa hàm ghi (encrypt path) — MỌI lần ghi mới đều dùng V3:
```typescript
private encryptForWrite(plaintext: string): CredentialEnvelopeV3 {
  const salt = randomBytes(16)
  const key = deriveKeyV3(this.serverSecret, this.userId, salt)
  const iv = randomBytes(V3_IV_BYTES)
  const cipher = createCipheriv('aes-256-gcm', key, iv)
  const ciphertext = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final(), cipher.getAuthTag()])
  return {
    version: 'v3',
    salt: salt.toString('base64'),
    iv: iv.toString('base64'),
    ciphertext: ciphertext.toString('base64')
  }
}
```

Sửa hàm đọc (decrypt path) — nhánh theo `version`, giữ nguyên logic V1/V2 hiện có, thêm nhánh V3:
```typescript
private decryptEnvelope(envelope: CredentialEnvelopeV1V2 | CredentialEnvelopeV3): string {
  if ('version' in envelope && envelope.version === 'v3') {
    const salt = Buffer.from(envelope.salt, 'base64')
    const key = deriveKeyV3(this.serverSecret, this.userId, salt)
    const iv = Buffer.from(envelope.iv, 'base64')
    // ... decipher AES-256-GCM với key/iv này, ciphertext + auth tag tách đúng offset
  }
  // ... giữ nguyên toàn bộ logic V1/V2 hiện có bên dưới, không sửa
}
```

> [!IMPORTANT]
> KHÔNG sửa/xoá logic decrypt V1/V2 hiện có — chỉ thêm nhánh V3 phía trên. Test hiện có cho V1/V2 phải tiếp tục pass nguyên vẹn sau task này.

---

## ⚠️ Chi tiết code thật khác kế hoạch (tên hàm/type) — xem "Kết quả thực thi"

Đoạn code mẫu phía trên dùng tên giả định (`CredentialEnvelopeV3`, `encryptForWrite`, `decryptEnvelope`) vì lúc viết task chưa đọc file thật. Code thật dùng đúng convention đã có sẵn trong file (`encryptV2`/`decryptBlob`/magic-byte header `Buffer`, không phải JSON envelope) — xem "Kết quả thực thi" bên dưới cho tên hàm/hằng số chính xác đã áp dụng.

## Verify

```bash
pnpm --filter frontend test -- web-credential-store
```

## Definition of Done

- [x] `deriveKeyV3()` đưa `userId` vào input `scryptSync` (`` `${serverSecret}:${userId}` ``), không chỉ dùng cho path — `this.userId` giờ được lưu lại trong constructor (trước chỉ dùng xây `userCredDir`)
- [x] Ghi mới (`setToken` → `encryptV3`) luôn dùng V3, IV 12 byte (hằng số mới `V3_IV_BYTES=12`, giữ nguyên `V2_IV_BYTES=16` cho code đọc blob cũ)
- [x] Đọc (`decryptBlob`) xử lý đúng cả V1/V2 (giữ nguyên logic cũ, không sửa 1 dòng nào) và V3 (nhánh mới, check trước tiên bằng magic `"ORC3"`)
- [x] Module này **chưa từng có test trước đây** (xác nhận qua `find` trước khi bắt đầu) — không có "test cũ" để giữ nguyên; viết mới hoàn toàn `web-credential-store.test.ts` với test riêng cho V1 và V2 legacy decrypt (dựng blob bằng đúng format cũ) để đảm bảo không phá backward-compat
- [x] Test cross-user: encrypt bằng `userId='user-a'`, ghi đè blob đó vào thư mục của `userId='user-b'` (mô phỏng đúng kịch bản BUG-FE-HLD-003 — blob leak + serverSecret chung) → `getToken()` trả **`null`** (không throw ra ngoài class — `decryptBlob` throw nội bộ, `getToken()` catch và trả `null`, đúng hành vi hiện có của class cho mọi lỗi giải mã, không phải ngoại lệ riêng cho case này)

## Kết quả thực thi

- **File sửa:** `main/credentials/web-credential-store.ts` — thêm `V3_MAGIC`/`deriveKeyV3()`/`encryptV3()`/`isV3()`, nhánh V3 trong `decryptBlob()`, `setToken()` chuyển sang `encryptV3()`, `getToken()` thêm lazy re-encrypt (chi tiết ở TASK-FE-HLD-013), thêm `migrateToV3()`.
- **File test mới:** `web-credential-store.test.ts` — 7 test, **7/7 pass**: round-trip, magic header đúng, cross-user isolation, V1 legacy decrypt, V2 legacy decrypt, lazy upgrade, batch migration.
