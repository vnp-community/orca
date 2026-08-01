# BUG-AIP-002: `writeCredentialToDevServer` nhận `encryptedBlob + iv` nhưng HLD mô tả Server decrypt + relay plaintext

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AIP-002  
**Implementation:** AIProviderService.ts: credential decryption before relay  

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD BL-AIP-01 STEP 3 mô tả:
```
[ProviderCredentialWriter.write()]
    Decrypt session layer (server-side): lấy lại plaintext apiKey
    key = deriveServerKey(sessionToken, userId)
    apiKey = AES-GCM-decrypt(key, credentialBlob)
    → conn.call('ai.credential.write', { accountId, credentials: { apiKey } })
```

Thực tế `src/main/ai-providers/AIProviderService.ts:227-239`:
```typescript
async writeCredentialToDevServer(
  accountId: string,
  encryptedBlob: string,   ← vẫn encrypted
  iv: string              ← vẫn encrypted
): Promise<void> {
  // ...
  await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })
}
```

**Server KHÔNG decrypt** — gửi `encryptedBlob + iv` thẳng đến Dev Server (vẫn encrypted với session key).

Dev Server relay handler `src/relay/ai-provider-handler.ts` nhận `{ encryptedBlob, iv }` và cần decrypt bằng cùng key.

**Vấn đề**: Dev Server không có `sessionToken` hay `userId` để derive decryption key. Dev Server không thể decrypt credential!

## Root Cause

`SubtleCrypto` encrypt trong browser với `PBKDF2(sessionToken + userId, salt)`.  
Server PHẢI decrypt trước khi relay (vì Dev Server không có session key).  
Nhưng `AIProviderService.writeCredentialToDevServer()` forward encrypted blob trực tiếp.

## Ảnh hưởng

1. Dev Server nhận `{ encryptedBlob, iv }` nhưng không có key → không thể decrypt → file `.enc` ghi sai
2. Khi agent spawn, đọc `.enc` → decrypt với `ORCA_AI_CREDENTIAL_KEY` → decrypt fail → apiKey sai → API call 401

## Fix đề xuất

Phải decrypt tại Orca Server trước khi relay:
```typescript
async writeCredentialToDevServer(
  accountId: string,
  encryptedBlob: string,
  iv: string,
  sessionToken: string,   // cần thêm
  userId: string          // cần thêm
): Promise<void> {
  // Decrypt on server side
  const sessionKey = await deriveSessionKey(sessionToken, userId)
  const plaintext = await aesGcmDecrypt(sessionKey, iv, encryptedBlob)
  const { apiKey } = JSON.parse(plaintext)
  
  // Forward plaintext to Dev Server (in-transit only, not persisted)
  await relay.call('ai.provider.writeCredential', { accountId, apiKey })
}
```

## Files liên quan

- `src/main/ai-providers/AIProviderService.ts:227-239`: forward encrypted blob
- `src/relay/ai-provider-handler.ts`: handler nhận encrypted blob
- `src/main/ai-providers/ai-provider-rpc-handler.ts:146`: call writeCredentialToDevServer
