# ADR-006 — WebCredentialStore — AES-256-GCM Per-User Credential Isolation

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-006 |
| **Trạng thái** | ✅ Accepted |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C3.9 |
| **Code Ref** | `src/main/credentials/web-credential-store.ts`, `src/main/credentials/index.ts` |

---

## Bối cảnh

Electron Desktop dùng `electron.safeStorage` (OS keychain) để lưu credentials (GitHub token, Bitbucket password...). Trong Web Server mode (Node.js headless), không có `electron.safeStorage`.

**Requirements:**
- Mỗi user có credentials riêng (không thể cross-user access)
- Credentials phải encrypt at rest (file system có thể bị đọc bởi root)
- Phải hoạt động không cần Electron
- Phải hoạt động trong per-user child processes

---

## Quyết định

### WebCredentialStore — AES-256-GCM + scrypt

```typescript
// src/main/credentials/web-credential-store.ts
class WebCredentialStore {
  // Storage: <userDataPath>/users/<userId>/credentials/<service>.enc
  // Encryption: AES-256-GCM
  // Key derivation: scrypt(ORCA_SERVER_SECRET + ':' + userId, salt)
  // Each write: fresh random 16-byte IV → different ciphertext each time

  async set(service: string, config: CredentialConfig): Promise<void>
  async get(service: string): Promise<CredentialConfig | null>
  async delete(service: string): Promise<void>
  async list(): Promise<string[]>  // service names only, no values
}
```

### Encrypted file format

```
[IV: 16 bytes][AuthTag: 16 bytes][Ciphertext: variable]
```

File path: `<userDataPath>/users/<userId>/credentials/<service>.enc`

### Key derivation

```typescript
const key = scryptSync(
  `${ORCA_SERVER_SECRET}:${userId}`,
  userId,                          // salt = userId
  32                               // 256-bit key
)
const cipher = createCipheriv('aes-256-gcm', key, randomIV)
```

**Tại sao scrypt?** — memory-hard, chống brute-force, OWASP recommended

### Services hiện tại hỗ trợ

```typescript
type CredentialService = 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'
```

**v5.0 bổ sung:** AI Provider credential metadata (nhưng key material chỉ trên Dev Server — xem ADR-008)

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **AES-256-GCM + scrypt + per-user file** ✅ | No external deps; GCM provides authenticity; scrypt is memory-hard |
| electron.safeStorage | Electron-only, không chạy headless |
| Node.js `keytar` (OS keychain) | Cần native addon, không hoạt động trong container |
| HashiCorp Vault | External dependency, quá heavy cho embedded use |
| Env variables | Không isolate per-user, không persist |
| AES-256-CBC | Không có authenticity check (GCM tốt hơn) |

---

## Hậu quả

**Tích cực:**
- Zero external dependencies (dùng Node.js `node:crypto`)
- GCM mode: nếu file bị tamper → `decipher.final()` throw (authenticity)
- Fresh random IV mỗi write → same token → khác ciphertext
- File isolation per userId

**Tiêu cực:**
- `ORCA_SERVER_SECRET` phải được backup; nếu mất → credentials không decrypt được
- scrypt blocking call → nên cache key trong memory (hiện không cache)
- File system permissions cần đúng (user process chạy với đúng uid)

---

## Security Notes

- `ORCA_SERVER_SECRET` = environment variable, không commit vào git
- `list()` chỉ trả service names, không bao giờ return raw credentials
- Admin API không có endpoint để read credentials của user khác
- Credential files không sync qua git hay backup tự động

---

## Trạng thái Implementation

✅ WebCredentialStore class  
✅ AES-256-GCM encryption/decryption  
✅ scrypt key derivation  
✅ Per-user directory isolation  
✅ initWebCredentialStore() trong server-bootstrap  
✅ RPC methods: credentials.set/get/delete/list  
🚧 In-memory key cache (optimization)  
🚧 AI Provider credential scope (ADR-008)
