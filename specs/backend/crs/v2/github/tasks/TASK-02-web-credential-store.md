# TASK-02: Tạo `WebCredentialStore` — Encrypted Per-User Credential Storage

**Status:** ✅ DONE — 2026-07-25  
**Phase:** 1 — Foundation  
**Priority:** 🔴 Critical  
**Depends on:** Không có  
**Solution:** SOL-04-Credential-Store.md  
**CRs:** CR-INT-002, CR-INT-003, CR-INT-004  
**Estimated effort:** ~60 phút

---

## Mục tiêu

Tạo module `src/main/credentials/web-credential-store.ts` mới — lưu trữ API tokens cho các integration (Bitbucket, AzDO, Gitea, Linear, Jira) với mã hóa AES-256-GCM, per-user isolation, không phụ thuộc Electron `safeStorage`.

---

## Hiện trạng

Hiện tại các integration lưu credentials theo 2 cách không an toàn trong Web mode:
- **Category B** (Bitbucket/AzDO/Gitea): `process.env.ORCA_*_TOKEN` — global, shared giữa users
- **Category C** (Linear/Jira): file `~/.orca/linear-token.enc` — dùng `safeStorage` (Electron-only)

---

## File cần tạo: `src/main/credentials/web-credential-store.ts` [NEW]

```typescript
import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  unlinkSync,
  readdirSync,
  existsSync
} from 'node:fs'
import { join } from 'node:path'

// Tất cả service names cần lưu credentials
export type CredentialService =
  | 'bitbucket'
  | 'azure-devops'
  | 'gitea'
  | 'linear'
  | 'jira'

export type CredentialConfig = Record<string, string>

/**
 * WebCredentialStore — AES-256-GCM encrypted per-user token storage.
 * Dùng trong Web Server mode (ORCA_MULTI_USER=1) thay cho Electron safeStorage.
 * Mỗi user process có 1 instance gắn với userCredDir của user đó.
 */
export class WebCredentialStore {
  private readonly userCredDir: string
  private readonly encryptionKey: Buffer

  constructor(userDataPath: string, userId: string, serverSecret: string) {
    // Per-user directory
    this.userCredDir = join(userDataPath, 'users', userId, 'credentials')
    // Derive 32-byte AES key từ server secret (stable, deterministic)
    this.encryptionKey = scryptSync(
      serverSecret,
      'orca-web-credential-store-v1',
      32
    ) as Buffer
    mkdirSync(this.userCredDir, { recursive: true, mode: 0o700 })
  }

  private tokenPath(service: CredentialService): string {
    return join(this.userCredDir, `${service}.enc`)
  }

  private configPath(service: CredentialService): string {
    return join(this.userCredDir, `${service}.config.json`)
  }

  // ── Token storage ─────────────────────────────────────────────

  async setToken(
    service: CredentialService,
    token: string,
    config?: CredentialConfig
  ): Promise<void> {
    const iv = randomBytes(16)
    const cipher = createCipheriv('aes-256-gcm', this.encryptionKey, iv)
    const encrypted = Buffer.concat([cipher.update(token, 'utf8'), cipher.final()])
    const authTag = cipher.getAuthTag()
    // Format: iv(16) + authTag(16) + ciphertext
    const stored = Buffer.concat([iv, authTag, encrypted])
    writeFileSync(this.tokenPath(service), stored, { mode: 0o600 })

    if (config) {
      writeFileSync(
        this.configPath(service),
        JSON.stringify(config),
        { mode: 0o600 }
      )
    }
  }

  async getToken(service: CredentialService): Promise<string | null> {
    try {
      const raw = readFileSync(this.tokenPath(service))
      if (raw.length < 33) return null
      const iv = raw.subarray(0, 16)
      const authTag = raw.subarray(16, 32)
      const encrypted = raw.subarray(32)
      const decipher = createDecipheriv('aes-256-gcm', this.encryptionKey, iv)
      decipher.setAuthTag(authTag)
      return decipher.update(encrypted) + decipher.final('utf8')
    } catch {
      return null
    }
  }

  async getConfig(service: CredentialService): Promise<CredentialConfig | null> {
    try {
      const raw = readFileSync(this.configPath(service), 'utf8')
      return JSON.parse(raw) as CredentialConfig
    } catch {
      return null
    }
  }

  async hasToken(service: CredentialService): Promise<boolean> {
    return existsSync(this.tokenPath(service))
  }

  async deleteToken(service: CredentialService): Promise<void> {
    try { unlinkSync(this.tokenPath(service)) } catch { /* ignore */ }
    try { unlinkSync(this.configPath(service)) } catch { /* ignore */ }
  }

  async listServices(): Promise<CredentialService[]> {
    try {
      return readdirSync(this.userCredDir)
        .filter(f => f.endsWith('.enc'))
        .map(f => f.replace('.enc', '') as CredentialService)
    } catch {
      return []
    }
  }
}
```

---

## File cần tạo: `src/main/credentials/index.ts` [NEW]

```typescript
import { WebCredentialStore } from './web-credential-store'

let _store: WebCredentialStore | null = null

/**
 * Khởi tạo singleton WebCredentialStore cho web mode.
 * Gọi 1 lần trong server-bootstrap khi ORCA_MULTI_USER=1.
 */
export function initWebCredentialStore(
  userDataPath: string,
  userId: string,
  serverSecret: string
): void {
  _store = new WebCredentialStore(userDataPath, userId, serverSecret)
}

/**
 * Trả về singleton đã được khởi tạo.
 * Throws nếu chưa init — cần gọi initWebCredentialStore() trước.
 */
export function getWebCredentialStore(): WebCredentialStore {
  if (!_store) {
    throw new Error('[WebCredentialStore] Not initialized. Call initWebCredentialStore() first.')
  }
  return _store
}

/**
 * Kiểm tra Web mode có đang dùng WebCredentialStore không.
 * Dựa vào ORCA_MULTI_USER env var.
 */
export function isWebCredentialMode(): boolean {
  return process.env['ORCA_MULTI_USER'] === '1'
}

export { WebCredentialStore } from './web-credential-store'
export type { CredentialService, CredentialConfig } from './web-credential-store'
```

---

## Tests cần viết: `src/main/credentials/__tests__/web-credential-store.test.ts` [NEW]

```typescript
describe('WebCredentialStore', () => {
  let store: WebCredentialStore
  let tempDir: string

  beforeEach(() => {
    tempDir = mkdtempSync(join(tmpdir(), 'orca-cred-test-'))
    store = new WebCredentialStore(tempDir, 'test-user-123', 'test-secret')
  })

  afterEach(() => rmSync(tempDir, { recursive: true }))

  describe('setToken / getToken', () => {
    it('stores and retrieves a token')
    it('returns null when no token stored')
    it('encrypts token on disk (raw file != plaintext token)')
    it('each call to setToken overwrites the previous one')
  })

  describe('hasToken', () => {
    it('returns false before setToken')
    it('returns true after setToken')
    it('returns false after deleteToken')
  })

  describe('deleteToken', () => {
    it('removes token file')
    it('removes config file')
    it('does not throw if already deleted')
  })

  describe('config storage', () => {
    it('stores and retrieves config alongside token')
    it('returns null config when not set')
  })

  describe('user isolation', () => {
    it('two stores for different users have separate token files')
    it('user A token cannot be decrypted by user B key')
  })

  describe('listServices', () => {
    it('returns empty array when no tokens')
    it('returns list of service names with tokens')
  })
})
```

---

## Acceptance Criteria

1. `WebCredentialStore` tạo file tại `userDataPath/users/{userId}/credentials/{service}.enc`
2. File mã hóa không chứa plaintext token (kiểm tra bằng `cat file | strings`)
3. `getToken()` sau `setToken()` trả về đúng token gốc
4. 2 users khác nhau → 2 thư mục riêng biệt
5. Tests pass: `pnpm test -- credentials`

---

## Files cần tạo

- `src/main/credentials/web-credential-store.ts` [NEW]
- `src/main/credentials/index.ts` [NEW]  
- `src/main/credentials/__tests__/web-credential-store.test.ts` [NEW]
