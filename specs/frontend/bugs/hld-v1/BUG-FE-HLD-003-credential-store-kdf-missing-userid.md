# BUG-FE-HLD-003 — `WebCredentialStore` key derivation không đưa `userId` vào, trái thiết kế "per-user key"

**Mức độ:** 🟠 High
**Status:** 🔴 Open
**Module:** `frontend/src/main/credentials/web-credential-store.ts`
**Phát hiện:** 2026-08-08 (audit `frontend/` code vs thiết kế — `audit/frontend/01-security-conformance.md` §3)

---

## Mô tả

`docs/hld/v1/security.md` §11 (WebCredentialStore) tuyên bố: *"Encryption: AES-256-GCM … Per-user key từ userId + server secret"*.

Constructor của `WebCredentialStore` nhận `userId` (`web-credential-store.ts:54-61`) nhưng **chỉ dùng để build đường dẫn thư mục** (`userCredDir`). Khoá mã hoá thật sự lại không đưa `userId` vào:

```
scryptSync(this.serverSecret, salt, 32)                              // V2, dòng 77
scryptSync(serverSecret, 'orca-web-credential-store-v1', 32)         // V1 legacy, dòng 59
```

`userId` không xuất hiện trong bất kỳ input nào của `scryptSync`.

## Hậu quả

Cách ly giữa các user hiện chỉ dựa vào quyền thư mục filesystem (`mode: 0o700`/`0o600`), **không phải mật mã học** như doc mô tả. Salt được lưu plaintext ngay trong chính credential blob (`web-credential-store.ts:86-91`). Nếu `serverSecret` (biến môi trường `ORCA_SERVER_SECRET`/`ORCA_CREDENTIAL_KEY`) và **bất kỳ 1** credential blob nào bị lộ (vd. backup không đúng quyền, lỗi filesystem permission, log leak), kẻ tấn công giải mã được credential của **mọi user khác** trên cùng server, không chỉ user sở hữu blob bị lộ — vì tất cả user dùng chung 1 khoá dẫn xuất từ `serverSecret`.

Đây là lỗ hổng nghiêm trọng hơn model đã công bố, vì nó phá vỡ đúng thuộc tính "per-user isolation" mà thiết kế multi-user (F23/F24) dựa vào.

## Bằng chứng

```
web-credential-store.ts:54-61  → constructor nhận userId, chỉ dùng cho path
web-credential-store.ts:59     → scryptSync(serverSecret, 'orca-web-credential-store-v1', 32) — V1, không có userId
web-credential-store.ts:77     → scryptSync(this.serverSecret, salt, 32) — V2, vẫn không có userId
web-credential-store.ts:86-91  → salt lưu plaintext trong blob
```

## Đề xuất fix

1. Đưa `userId` (hoặc `sha256(userId)`) vào input của `scryptSync`, ví dụ: `scryptSync(`${serverSecret}:${userId}`, salt, 32)`.
2. **Cần kế hoạch migration**: đổi KDF làm mọi credential blob cũ không giải mã được nữa — cần script re-encrypt (giải mã bằng KDF cũ, mã hoá lại bằng KDF mới) chạy 1 lần khi deploy, hoặc versioning KDF (thêm V3) với fallback đọc V2 rồi ghi lại theo V3 lúc truy cập tiếp theo.
3. Cập nhật `security.md` §11 khớp với IV 16 byte thực tế nếu quyết định giữ nguyên (khác 12 byte doc ghi), hoặc đổi code về đúng 12 byte nếu không có lý do đặc biệt.

## Tham khảo

- Audit: `audit/frontend/01-security-conformance.md` §3
- Doc gốc: `docs/hld/v1/security.md` §11
