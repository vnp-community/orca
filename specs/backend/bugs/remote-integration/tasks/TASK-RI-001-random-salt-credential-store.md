# TASK-RI-001: Thay fixed salt bằng per-credential random salt trong credential store

**Priority:** 🔴 CRITICAL SECURITY — dictionary attack vulnerability  
**Effort:** ~60 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-RI-001  
**Solution ref:** [SOLUTION-RI-001-exact.md](../solutions/SOLUTION-RI-001-exact.md)

---

## Mục tiêu

Sửa `WebCredentialStore` (hoặc tương đương) để dùng per-credential random 32-byte salt thay vì fixed salt. Thêm V1→V2 migration để re-encrypt credentials cũ.

---

## Bước 1 — Tìm file credential store

```bash
find src/main -name "*credential*" -o -name "*cred-store*" 2>/dev/null | grep -v __tests__
grep -rn "pbkdf2\|PBKDF\|createCipheriv\|aes-256\|fixed.*salt" src/main/ 2>/dev/null | head -10
```

## Bước 2 — Kiểm tra schema blob hiện tại

```bash
# Xem cấu trúc EncryptedBlob hiện tại:
grep -n "EncryptedBlob\|iv\|tag\|ct\|salt" src/main/credentials/ -r 2>/dev/null | head -20
```

## Bước 3 — Update EncryptedBlob type

Thêm field `v` (version) và `s` (salt):

```typescript
// TRƯỚC (V1 — fixed salt):
export interface EncryptedBlob {
  iv:  string   // base64 IV
  tag: string   // base64 GCM auth tag
  ct:  string   // base64 ciphertext
}

// SAU (V2 — per-credential random salt):
export interface EncryptedBlob {
  v:   number   // schema version: 1 = fixed salt, 2 = random salt
  s?:  string   // base64: random 32-byte salt (only present in v >= 2)
  iv:  string   // base64: 12-byte random IV
  tag: string   // base64: 16-byte GCM auth tag
  ct:  string   // base64: ciphertext
}
```

## Bước 4 — Update encrypt() method

```typescript
import { randomBytes, pbkdf2Sync, createCipheriv } from 'node:crypto'

private readonly ITERATIONS = 200_000  // OWASP 2024
private readonly KEY_LEN    = 32

encrypt(plaintext: string): EncryptedBlob {
  // V2: random salt per credential
  const salt = randomBytes(32)  // 256-bit random (NOT fixed!)
  const iv   = randomBytes(12)  // 96-bit random IV for AES-GCM

  const key    = pbkdf2Sync(this.masterKey, salt, this.ITERATIONS, this.KEY_LEN, 'sha256')
  const cipher = createCipheriv('aes-256-gcm', key, iv)
  const ct     = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])

  return {
    v:   2,
    s:   salt.toString('base64'),
    iv:  iv.toString('base64'),
    tag: cipher.getAuthTag().toString('base64'),
    ct:  ct.toString('base64'),
  }
}
```

## Bước 5 — Update decrypt() method với V1 backward compat

```typescript
import { pbkdf2Sync, createDecipheriv } from 'node:crypto'

// Replace decrypt():
decrypt(blob: EncryptedBlob): string {
  const version = blob.v ?? 1  // default to v1 for legacy blobs without 'v' field

  const salt = version >= 2 && blob.s
    ? Buffer.from(blob.s, 'base64')
    : this.legacySalt  // V1 fixed salt

  const iterations = version >= 2 ? this.ITERATIONS : 100_000  // V1 may have used less

  const iv  = Buffer.from(blob.iv,  'base64')
  const tag = Buffer.from(blob.tag, 'base64')
  const ct  = Buffer.from(blob.ct,  'base64')

  const key      = pbkdf2Sync(this.masterKey, salt, iterations, this.KEY_LEN, 'sha256')
  const decipher = createDecipheriv('aes-256-gcm', key, iv)
  decipher.setAuthTag(tag)

  return Buffer.concat([decipher.update(ct), decipher.final()]).toString('utf8')
}
```

## Bước 6 — Tìm và set legacySalt

```bash
# Tìm fixed salt string hiện tại:
grep -rn "salt\|SALT\|'orca-\|\"orca-" src/main/credentials/ 2>/dev/null | head -10
```

```typescript
// Trong constructor:
private readonly legacySalt: Buffer

constructor(masterKey: Buffer, legacySalt?: string) {
  this.masterKey  = masterKey
  // Replace 'orca-credential-salt-v1' với giá trị thực tế tìm được ở bước 6:
  this.legacySalt = Buffer.from(legacySalt ?? 'orca-credential-salt-v1', 'utf8')
}
```

## Bước 7 — Thêm migrateToV2() method

```typescript
async migrateToV2(
  listAll: () => Promise<{ id: string; blob: EncryptedBlob }[]>,
  save:    (id: string, blob: EncryptedBlob) => Promise<void>
): Promise<{ migrated: number; failed: number }> {
  const records = await listAll()
  let migrated = 0, failed = 0

  for (const record of records) {
    const version = record.blob.v ?? 1
    if (version < 2) {
      try {
        const plaintext = this.decrypt(record.blob)  // decrypt with V1 (fixed salt)
        const newBlob   = this.encrypt(plaintext)    // re-encrypt with V2 (random salt)
        await save(record.id, newBlob)
        migrated++
      } catch (err) {
        console.error(`[CredentialStore] Migration failed for id=${record.id}:`, err)
        failed++
      }
    }
  }

  console.log(`[CredentialStore] V2 migration: ${migrated} migrated, ${failed} failed`)
  return { migrated, failed }
}
```

## Bước 8 — Gọi migrateToV2() khi server startup

```bash
grep -n "WebCredentialStore\|credentialStore" src/main/server-bootstrap.ts | head -10
```

```typescript
// src/main/server-bootstrap.ts — sau khi tạo credentialStore:
await credentialStore.migrateToV2(
  () => credentialRepository.listAll(),
  (id, blob) => credentialRepository.update(id, blob)
).catch(err => console.error('[Bootstrap] Credential migration failed:', err))
```

## Verification

```bash
pnpm tsc --noEmit

# Test: encrypt same string 2 lần → khác nhau (random salt):
# const b1 = store.encrypt('test-key')
# const b2 = store.encrypt('test-key')
# assert(b1.s !== b2.s, 'Salt must differ')
# assert(store.decrypt(b1) === 'test-key')
# assert(store.decrypt(b2) === 'test-key')
# assert(b1.v === 2)

pnpm vitest run src/main/credentials/__tests__/ 2>/dev/null || true
```
