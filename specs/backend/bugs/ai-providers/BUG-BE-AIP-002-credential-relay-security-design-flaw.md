# BUG-BE-AIP-002: `ProviderCredentialWriter` phải relay plaintext apiKey qua WS — security risk nếu implement theo HLD

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-BE-AIP-002  
**Note:** Web credential store V2 + agent-credential-store double encryption architecture  

## Mức độ: 🟡 MEDIUM (Security Design Issue)

## Tóm tắt

HLD (BL-AIP-01) mô tả security flow:
```
Browser: SubtleCrypto.encrypt(apiKey) → encryptedBlob + iv → POST (blob only)
Orca Server: AES-GCM-decrypt(sessionKey, blob) → plaintext apiKey (in-memory)
           → JSON-RPC: ai.credential.write { credentials: { apiKey: plaintext } }
Dev Server: AES-256-GCM encrypt → ghi .enc file
```

Orca Server decrypt browser layer để lấy **plaintext apiKey**, sau đó gửi qua WS đến Dev Server. Điều này có nghĩa:

1. Orca Server phải biết **session-derived key** của user để decrypt browser layer.
2. Plaintext apiKey đi qua **WebSocket** (dù encrypted bởi TLS).
3. Nếu Orca Server bị compromise → plaintext apiKey có thể bị intercept.

## Vấn đề

Theo comment trong HLD: `"Plaintext apiKey: tồn tại in-memory tại Orca Server trong thời gian transit"` — đây là acknowledged risk.

Nhưng bước **Orca Server decrypt browser layer** yêu cầu:
- Session key `deriveServerKey(sessionToken, userId)` — server phải có khả năng derive key từ session token.
- Điều này đòi hỏi session token phải chứa **entropy đủ để làm encryption key** — nhưng `auth-session-store.ts` dùng `crypto.randomBytes(32).toString('hex')` (random opaque token, không phải keying material).

## Ảnh hưởng

1. Không có cách nào để server derive encryption key từ session token (theo implementation hiện tại).
2. Browser layer AES-GCM encryption vô nghĩa nếu server không thể decrypt.
3. Alternative: Browser gửi plaintext apiKey và server tự encrypt → mất đi zero-knowledge guarantee.

## Cách fix đề xuất

Cần thiết kế lại credential flow:
- Option A: Dev Server trực tiếp nhận `encryptedBlob` và `iv` từ Browser (không qua Orca Server decrypt) — cần end-to-end path.
- Option B: Browser encrypt với Dev Server public key (không phải session key) — Dev Server decrypt bằng private key.

## Liên quan đến luồng

- **BL-AIP-01**: STEP 1-3 — credential write flow security model.
