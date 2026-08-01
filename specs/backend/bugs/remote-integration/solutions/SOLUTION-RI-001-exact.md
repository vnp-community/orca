# SOLUTION: remote-integration — Code-Level Exact Fix

**Source-verified:** Dựa trên bug file chi tiết  
**Bug:** `RI-001` — Credential store dùng fixed PBKDF2 salt

---

## BUG-RI-001: Fixed salt trong PBKDF2 — toàn bộ credential store

**File cần tìm:** 
```bash
grep -rn "pbkdf2\|PBKDF\|fixed.*salt\|credential.*salt" src/main/ | head -20
grep -rn "WebCredentialStore\|credential-store" src/main/ | head -10
```

Tìm file chứa credential encryption:
```bash
ls src/main/credentials/ 2>/dev/null || ls src/main/auth/ | grep -i cred
```

### Fix — Per-credential random salt (256-bit)

**File:** `src/main/credentials/web-credential-store.ts` (hoặc path tương đương)

```typescript
import { randomBytes, pbkdf2Sync, createCipheriv, createDecipheriv } from 'node:crypto'

const PBKDF2_ITERATIONS = 200_000    // OWASP 2024 recommendation
const KEY_LENGTH        = 32         // 256-bit AES key
const SCHEMA_VERSION    = 2          // v1 = fixed salt, v2 = random salt

export interface EncryptedBlob {
  v:    number    // schema version
  s:    string    // base64: random salt (32 bytes) — ONLY in v2
  iv:   string    // base64: random IV (12 bytes)
  tag:  string    // base64: GCM auth tag (16 bytes)
  ct:   string    // base64: ciphertext
}

export class WebCredentialStore {
  constructor(
    // Master key: hex-encoded 32 bytes from ORCA_CREDENTIAL_KEY env
    private readonly masterKey: Buffer
  ) {
    if (masterKey.length < 32) throw new Error('Credential key must be at least 32 bytes')
  }

  encrypt(plaintext: string): EncryptedBlob {
    // FIX RI-001: Per-credential random salt (NOT a fixed constant)
    const salt = randomBytes(32)    // 256-bit random salt
    const iv   = randomBytes(12)    // 96-bit random IV for GCM

    // Derive key: masterKey + per-credential random salt
    const key = pbkdf2Sync(this.masterKey, salt, PBKDF2_ITERATIONS, KEY_LENGTH, 'sha256')

    const cipher = createCipheriv('aes-256-gcm', key, iv)
    const ct = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])

    return {
      v:   SCHEMA_VERSION,
      s:   salt.toString('base64'),
      iv:  iv.toString('base64'),
      tag: cipher.getAuthTag().toString('base64'),
      ct:  ct.toString('base64'),
    }
  }

  decrypt(blob: EncryptedBlob): string {
    if (blob.v === 1) return this.decryptLegacy(blob)

    const salt = Buffer.from(blob.s, 'base64')
    const iv   = Buffer.from(blob.iv, 'base64')
    const tag  = Buffer.from(blob.tag, 'base64')
    const ct   = Buffer.from(blob.ct, 'base64')

    // Derive key with STORED salt (per-credential)
    const key = pbkdf2Sync(this.masterKey, salt, PBKDF2_ITERATIONS, KEY_LENGTH, 'sha256')

    const decipher = createDecipheriv('aes-256-gcm', key, iv)
    decipher.setAuthTag(tag)
    return Buffer.concat([decipher.update(ct), decipher.final()]).toString('utf8')
  }

  // V1 legacy: decrypt with the old fixed salt so we can migrate
  private decryptLegacy(blob: EncryptedBlob): string {
    // Replace 'orca-credential-salt-v1' with whatever the actual fixed salt was
    const LEGACY_SALT = Buffer.from('orca-credential-salt-v1', 'utf8')
    const iv  = Buffer.from(blob.iv, 'base64')
    const tag = Buffer.from(blob.tag, 'base64')
    const ct  = Buffer.from(blob.ct, 'base64')

    const key = pbkdf2Sync(this.masterKey, LEGACY_SALT, 100_000, KEY_LENGTH, 'sha256')
    const decipher = createDecipheriv('aes-256-gcm', key, iv)
    decipher.setAuthTag(tag)
    return Buffer.concat([decipher.update(ct), decipher.final()]).toString('utf8')
  }

  /**
   * Re-encrypt all v1 credentials with v2 (random salt).
   * Call on server startup.
   */
  async migrateToV2(repository: { listAll(): Promise<{ id: string; blob: EncryptedBlob }[]>; update(id: string, blob: EncryptedBlob): Promise<void> }): Promise<number> {
    const all = await repository.listAll()
    let count = 0

    for (const record of all) {
      if (record.blob.v === 1) {
        const plaintext = this.decryptLegacy(record.blob)
        const newBlob   = this.encrypt(plaintext)
        await repository.update(record.id, newBlob)
        count++
      }
    }

    return count
  }
}
```

---

## Verification

```bash
# Test random salt:
# Encrypt same string twice → different ciphertext (different salt + IV)
# node -e "
#   const { WebCredentialStore } = require('./dist/main/credentials/web-credential-store')
#   const store = new WebCredentialStore(Buffer.alloc(32, 0xaa))
#   const b1 = store.encrypt('test')
#   const b2 = store.encrypt('test')
#   console.assert(b1.s !== b2.s, 'Salt must differ')
#   console.assert(b1.ct !== b2.ct, 'Ciphertext must differ')
#   console.assert(store.decrypt(b1) === 'test', 'Decrypt b1 must work')
#   console.assert(store.decrypt(b2) === 'test', 'Decrypt b2 must work')
#   console.log('All assertions passed')
# "

pnpm vitest run src/main/credentials/__tests__/web-credential-store.test.ts
```
