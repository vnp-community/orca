# SOLUTION: Remote Integration Domain — Fix tất cả Bugs

**Domain:** remote-integration  
**TDD Reference:** TDD-05 (SSH Relay §Security), TDD-20 (Remote Git UI)  
**Files cần thay đổi:** `src/main/auth/web-credential-store.ts`, `src/main/project/CredentialService.ts`  
**Tổng số bugs:** 1 (RI-001)

---

## BUG-RI-001 — Fix credential store fixed salt (PBKDF security flaw)

**Mức độ:** 🔴 CRITICAL (Security)  
**Root cause:** AES encryption key được derive bằng PBKDF2 với fixed salt (constant) → dictionary attacks feasible.

### Phân tích vấn đề

```typescript
// Code sai hiện tại (ví dụ):
const FIXED_SALT = 'orca-credential-salt'  // BUG: constant salt
const key = pbkdf2Sync(masterKey, FIXED_SALT, 100_000, 32, 'sha256')

// Vấn đề:
// 1. Tất cả credentials dùng cùng salt → nếu salt bị lộ, attacker precompute keys
// 2. Rainbow table attacks feasible với fixed salt
// 3. Nếu 2 credentials share cùng key (derived từ same salt+masterKey) → ciphertext analysis
```

### Fix — Per-credential random salt

```typescript
// src/main/auth/web-credential-store.ts

import {
  randomBytes,
  pbkdf2Sync,
  createCipheriv,
  createDecipheriv,
  timingSafeEqual,
} from 'node:crypto'

export interface EncryptedCredential {
  version:    number          // schema version (for future migrations)
  salt:       string          // base64 — unique per credential
  iv:         string          // base64 — unique per encryption
  authTag:    string          // base64 — GCM auth tag
  ciphertext: string          // base64 — encrypted payload
}

export class WebCredentialStore {
  private readonly PBKDF2_ITERATIONS = 200_000  // OWASP recommended 2024
  private readonly KEY_LENGTH        = 32        // 256-bit

  constructor(
    private readonly masterKey: string,  // ORCA_CREDENTIAL_KEY env var (hex-encoded 32 bytes)
  ) {
    if (!masterKey || masterKey.length < 64) {
      throw new Error('ORCA_CREDENTIAL_KEY must be at least 32 bytes (64 hex chars)')
    }
  }

  /**
   * Encrypt credential payload với per-credential random salt + random IV.
   * FIX RI-001: Random salt thay vì fixed salt.
   */
  encrypt(plaintext: string): EncryptedCredential {
    // FIX: Random salt per credential (không phải constant)
    const salt = randomBytes(32)  // 256-bit random salt
    const iv   = randomBytes(12)  // 96-bit random IV for GCM

    // Derive key từ masterKey + per-credential salt
    const key = pbkdf2Sync(
      Buffer.from(this.masterKey, 'hex'),
      salt,
      this.PBKDF2_ITERATIONS,
      this.KEY_LENGTH,
      'sha256',
    )

    const cipher = createCipheriv('aes-256-gcm', key, iv)
    const ciphertext = Buffer.concat([
      cipher.update(plaintext, 'utf-8'),
      cipher.final(),
    ])
    const authTag = cipher.getAuthTag()

    return {
      version:    2,
      salt:       salt.toString('base64'),
      iv:         iv.toString('base64'),
      authTag:    authTag.toString('base64'),
      ciphertext: ciphertext.toString('base64'),
    }
  }

  /**
   * Decrypt credential.
   * Hỗ trợ cả version 1 (old fixed salt) và version 2 (random salt).
   */
  decrypt(encrypted: EncryptedCredential): string {
    if (encrypted.version === 1) {
      // Legacy decryption with fixed salt — for migration only
      return this.decryptLegacy(encrypted)
    }

    const salt      = Buffer.from(encrypted.salt, 'base64')
    const iv        = Buffer.from(encrypted.iv, 'base64')
    const authTag   = Buffer.from(encrypted.authTag, 'base64')
    const ciphertext = Buffer.from(encrypted.ciphertext, 'base64')

    // Re-derive key với salt từ credential (random per credential)
    const key = pbkdf2Sync(
      Buffer.from(this.masterKey, 'hex'),
      salt,
      this.PBKDF2_ITERATIONS,
      this.KEY_LENGTH,
      'sha256',
    )

    const decipher = createDecipheriv('aes-256-gcm', key, iv)
    decipher.setAuthTag(authTag)

    return Buffer.concat([
      decipher.update(ciphertext),
      decipher.final(),
    ]).toString('utf-8')
  }

  /**
   * Migration utility: re-encrypt all v1 (fixed salt) credentials.
   */
  async migrateCredentials(repository: IWebCredentialRepository): Promise<number> {
    const all = await repository.listAllEncrypted()
    let migrated = 0

    for (const record of all) {
      if (record.encryptedData.version === 1) {
        const plaintext = this.decryptLegacy(record.encryptedData)
        const newEncrypted = this.encrypt(plaintext)
        await repository.updateEncrypted(record.id, newEncrypted)
        migrated++
      }
    }

    return migrated
  }

  private decryptLegacy(encrypted: EncryptedCredential): string {
    // Fixed salt used in old version (for migration only)
    const LEGACY_SALT = Buffer.from('orca-credential-salt-v1', 'utf-8')
    const iv          = Buffer.from(encrypted.iv, 'base64')
    const authTag     = Buffer.from(encrypted.authTag, 'base64')
    const ciphertext  = Buffer.from(encrypted.ciphertext, 'base64')

    const key = pbkdf2Sync(
      Buffer.from(this.masterKey, 'hex'),
      LEGACY_SALT,
      100_000,  // Old iteration count
      this.KEY_LENGTH,
      'sha256',
    )

    const decipher = createDecipheriv('aes-256-gcm', key, iv)
    decipher.setAuthTag(authTag)
    return Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString('utf-8')
  }
}
```

### Migration Plan

```typescript
// src/main/db/migrations/0015_migrate_credential_salts.ts

export async function up(db: Connection): Promise<void> {
  // Thêm version column nếu chưa có
  await db.query(`ALTER TABLE orca_web_credentials ADD COLUMN IF NOT EXISTS salt_version INTEGER DEFAULT 1`)
  
  // Mark existing records as v1 (fixed salt)
  await db.query(`UPDATE orca_web_credentials SET salt_version = 1 WHERE salt_version IS NULL`)
}

// server-bootstrap.ts — migration tự động:
// const migrated = await webCredentialStore.migrateCredentials(repository)
// if (migrated > 0) log.info(`[CredentialStore] Migrated ${migrated} credentials to v2 (random salt)`)
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/auth/web-credential-store.ts` | Fix random salt per credential | RI-001 |
| `src/main/db/migrations/0015_migrate_credential_salts.ts` | NEW migration | RI-001 |
| `src/main/server-bootstrap.ts` | Run credential migration on startup | RI-001 |

---

## Verification Plan

```bash
# Security test:
# 1. Encrypt same plaintext twice → verify different ciphertext (random IV + salt)
# 2. Decrypt v2 credential → verify correct plaintext
# 3. Decrypt v1 credential (legacy) → verify migration path works
# 4. Tamper authTag → verify decryption fails (GCM auth)

# Migration test:
# 1. Create v1 credential → run migration → verify re-encrypted with v2 schema

pnpm vitest run src/main/auth/__tests__/web-credential-store.test.ts
```
