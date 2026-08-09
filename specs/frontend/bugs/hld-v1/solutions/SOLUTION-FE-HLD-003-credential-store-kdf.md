# SOLUTION: BUG-FE-HLD-003 — `WebCredentialStore` key derivation không đưa `userId` vào

**Source-verified:** ✅ Dựa trên source code thực tế
**TDD tham chiếu:** [tdd/v5/13-ai-provider-ui.md:19](../../../tdd/v5/13-ai-provider-ui.md#L19) — *"credential NEVER logged or sent in plaintext"*, dòng 122 — *"relay write (server never sees plaintext)"*. TDD không đặc tả chi tiết KDF phía `main/credentials/`, nhưng nguyên tắc per-user isolation là bất biến xuyên suốt toàn bộ thiết kế credential (áp dụng tương tự cho AI Provider credential relay).

---

## Root cause

```ts
// web-credential-store.ts:77 (V2)
scryptSync(this.serverSecret, salt, 32)
// web-credential-store.ts:59 (V1 legacy)
scryptSync(serverSecret, 'orca-web-credential-store-v1', 32)
```

`userId` chỉ dùng cho path thư mục (`userCredDir`), không vào KDF — mọi user share chung 1 khoá dẫn xuất từ `serverSecret`.

## Fix — thêm `userId` vào KDF, có đường di chuyển dữ liệu cũ (V3)

### Bước 1: KDF mới (V3) — đưa `userId` vào info/context của derivation

```ts
// web-credential-store.ts
const V3_IV_BYTES = 12 // Why: đúng chuẩn AES-GCM 96-bit đã ghi trong security.md §11
                         // (V2's 16-byte IV vẫn hỗ trợ đọc cũ, không tạo mới nữa)

function deriveKeyV3(serverSecret: string, userId: string, salt: Buffer): Buffer {
  // Why: userId đi vào password input của scrypt (không phải chỉ salt) để
  // 1 secret leak (serverSecret) + 1 blob leak không đủ giải mã blob của
  // user khác — phải leak đúng userId đó nữa (đã biết công khai qua path,
  // nhưng scrypt cost làm brute-force userId list không khả thi ở quy mô lớn
  // hơn vài chục nghìn user, và mục tiêu chính là chặn "1 serverSecret + 1
  // blob ngẫu nhiên nào đó" thay vì cần đúng cặp secret+blob của target).
  return scryptSync(`${serverSecret}:${userId}`, salt, 32)
}
```

### Bước 2: Versioned envelope — đọc cũ được, ghi mới theo V3

```ts
type CredentialEnvelope = { version: 'v1' | 'v2' | 'v3'; salt: string; iv: string; ciphertext: string }

function decrypt(envelope: CredentialEnvelope, serverSecret: string, userId: string): string {
  const salt = Buffer.from(envelope.salt, 'base64')
  const key =
    envelope.version === 'v3'
      ? deriveKeyV3(serverSecret, userId, salt)
      : envelope.version === 'v2'
        ? scryptSync(serverSecret, salt, 32)               // legacy, không có userId
        : scryptSync(serverSecret, 'orca-web-credential-store-v1', 32) // v1 legacy
  // ... decipher với IV tương ứng (16 byte cho v1/v2, 12 byte cho v3)
}

function encrypt(plaintext: string, serverSecret: string, userId: string): CredentialEnvelope {
  const salt = randomBytes(16)
  const key = deriveKeyV3(serverSecret, userId, salt)
  const iv = randomBytes(V3_IV_BYTES)
  // ... luôn ghi mới theo v3
}
```

### Bước 3: Re-encrypt lười (lazy migration) — không cần script chạy 1 lần riêng

```ts
// getDecryptedCredential(userId, service):
//   1. Đọc envelope hiện có (có thể v1/v2/v3)
//   2. decrypt() theo đúng version cũ
//   3. Nếu version !== 'v3' → encrypt() lại theo v3, ghi đè file ngay
//      (self-healing: mỗi lần user thực sự dùng credential, nó tự nâng cấp)
//   4. Trả plaintext cho caller
```

**Lý do chọn lazy re-encrypt thay vì batch migration script:** không cần downtime, không cần biết trước toàn bộ user/credential nào đang tồn tại, tự động hoàn tất trong vòng đời sử dụng bình thường của app. Rủi ro duy nhất: credential nào **không bao giờ được đọc lại** sau khi deploy fix vẫn ở dạng V2 mãi mãi — chấp nhận được vì mức độ rủi ro giảm dần theo thời gian, và có thể bổ sung 1 cron nhỏ quét + re-encrypt toàn bộ nếu muốn dứt điểm ngay.

## Test cần thêm

- `web-credential-store.test.ts`: v3 roundtrip đúng; đọc v1/v2 cũ vẫn giải mã được; sau khi đọc v1/v2, file trên đĩa được ghi lại theo v3 (kiểm bằng đọc lại envelope version).
- Test riêng: 2 user khác nhau, cùng `serverSecret`, cùng shared 1 bản sao ciphertext (giả lập leak) — xác nhận decrypt bằng `userId` sai **throw**, không trả plaintext.

## Tóm tắt thay đổi

| File | Thay đổi |
|---|---|
| `main/credentials/web-credential-store.ts` | Thêm `deriveKeyV3()` (đưa `userId` vào KDF, IV về 12 byte); versioned envelope đọc v1/v2/v3; lazy re-encrypt lên v3 khi đọc |
| `docs/hld/v1/security.md` §11 | Không cần sửa — code giờ khớp đúng "Per-user key từ userId + server secret" và "IV 12 byte" đã ghi sẵn |
