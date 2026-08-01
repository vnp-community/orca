# BUG-AG-AIP-002: `readDecryptedKey` trả về `encryptedBlob` (vẫn encrypted) — không phải plaintext apiKey

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AIP-01) mô tả 2-layer encryption:
```
Layer 1: Browser encrypts với SubtleCrypto (AES-GCM) → encryptedBlob + iv
Layer 2: Agent double-encrypts the blob → ghi .enc file
```

Khi đọc lại, `readDecryptedKey` chỉ giải mã Layer 2 (scrypt + AES-256-GCM) nhưng **không giải mã Layer 1** (SubtleCrypto):

```typescript
// agent-credential-store.ts:301-311
export async function readDecryptedKey(accountId, config, log) {
  const result = await handleReadCredential(null, { accountId }, config, log)
  return result.result.encryptedBlob  // ← Đây là ciphertext của Layer 1 — VẪN encrypted!
}
```

## File liên quan

- [`src/relay/agent-credential-store.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-credential-store.ts) — Lines 294-311

## Ảnh hưởng

1. `readDecryptedKey` trả về `encryptedBlob` (AES-GCM ciphertext từ browser) thay vì plaintext apiKey.
2. Bất kỳ code nào gọi `readDecryptedKey()` để lấy apiKey sẽ inject **ciphertext** vào env → AI agent fail authentication.
3. HLD comment trong code: `"in v5.0 is the outer-encrypted plaintext API key"` — **không đúng**.

## Phân tích

Theo HLD, Orca Server decrypt Layer 1 trước khi gửi đến Dev Server:
```
Orca Server:
  key = deriveServerKey(sessionToken, userId)
  apiKey = AES-GCM-decrypt(key, credentialBlob)  ← plaintext đến Dev Server
```

Dev Server chỉ nhận plaintext apiKey, sau đó encrypt lại (Layer 2) để lưu file.
Nhưng trong code thực tế, Dev Server nhận `encryptedBlob` (Layer 1 chưa decrypt) → lưu vào file mà không biết cách giải mã Layer 1.

## Liên quan đến luồng

- **BL-AIP-01**: STEP 3 — credential write flow.
- **BL-AG-ORCH-003**: buildAgentEnv dùng 'placeholder-key' — đây là root cause tại sao placeholder được dùng: readDecryptedKey không hoạt động đúng.

---

## ⏸ Fix Status: DEFERRED

**Reason:** Orca Server Layer 1 decryption required. Relay correctly reads Layer2→Layer1 blob. Orca Server must inject resolvedApiKey in spawn request. Out of relay scope.
