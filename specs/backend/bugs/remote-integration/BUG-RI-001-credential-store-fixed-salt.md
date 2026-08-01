# BUG-RI-001 [BACKEND]: `WebCredentialStore` salt cố định `'orca-web-credential-store-v1'` — toàn bộ users share cùng encryption salt

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-RI-001  
**Note:** web-credential-store.ts: V2 format with random 32-byte salt per credential  

## Mức độ: 🟡 MEDIUM (Security)

## Tóm tắt

`src/main/credentials/web-credential-store.ts:48-52`:
```typescript
this.encryptionKey = scryptSync(
  serverSecret,
  'orca-web-credential-store-v1',  ← FIXED SALT ⚠️
  32
) as Buffer
```

Salt là cố định cho mọi user. Nếu `ORCA_SERVER_SECRET` bị lộ, tất cả credentials của mọi user đều có thể decrypt bằng cùng key.

**HLD BL-INT-02 yêu cầu**: `key = scrypt(ORCA_CREDENTIAL_KEY, userId + service)` — salt phải include `userId` + `service`.

Thực tế code: salt = fixed string `'orca-web-credential-store-v1'`.

## So sánh

| HLD BL-INT-02 | Thực tế code |
|---------------|-------------|
| `key = scrypt(ORCA_CREDENTIAL_KEY, userId + service)` | `key = scrypt(serverSecret, 'orca-web-credential-store-v1')` |
| Per-user + per-service key | Same key cho mọi user và service |

## Ảnh hưởng

1. Tất cả users dùng cùng encryption key → nếu attacker có file `.enc` của user khác + server secret, có thể decrypt
2. Salt không include userId → không có "at rest" per-user isolation
3. Salt không include service → GitHub token và Linear token dùng cùng key (chỉ khác ở filename)

## Fix đề xuất

```typescript
constructor(userDataPath: string, userId: string, serverSecret: string) {
  this.userCredDir = join(userDataPath, 'users', userId, 'credentials')
  // Per-user salt: includes userId to provide user-level key isolation
  const perUserSalt = `orca-credential-store-v1-user-${userId}`
  this.encryptionKey = scryptSync(serverSecret, perUserSalt, 32) as Buffer
}
```

Và trong `setToken()`, include service in the IV derivation:
```typescript
// Per-service additional context
const serviceBytes = Buffer.from(service, 'utf8')
const iv = randomBytes(16)  // Already random per write — OK
```

## Files liên quan

- `src/main/credentials/web-credential-store.ts:48-52`: fixed salt
