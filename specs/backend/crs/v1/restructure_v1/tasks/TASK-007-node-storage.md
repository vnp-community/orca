# TASK-007: Tạo `NodeSecureStorage` + `NodeSystemInfo`

**Source:** SOL-BE-002  
**Phase:** 1 | **Effort:** S (45–60 min)  
**Depends on:** TASK-002, TASK-004

---

## Objective

Tạo hai files còn lại trong NodeAdapter:
1. `src/platform/adapters/node/storage.ts` — AES-256-GCM file-based encryption
2. `src/platform/adapters/node/system.ts` — OS system info queries

---

## Files to create

### 1. `src/platform/adapters/node/storage.ts`

```typescript
import {
  randomBytes,
  createCipheriv,
  createDecipheriv,
  pbkdf2Sync
} from 'node:crypto'
import {
  existsSync,
  readFileSync,
  writeFileSync,
  mkdirSync
} from 'node:fs'
import { join } from 'node:path'
import type { IApp } from '../../app-interface'
import type { ISecureStorage } from '../../storage-interface'

const ALGORITHM = 'aes-256-gcm'
const KEY_LENGTH = 32    // 256 bits
const IV_LENGTH = 12     // GCM standard
const TAG_LENGTH = 16    // GCM auth tag
const KEY_DIR = '.crypto'
const KEY_FILE = 'storage.key'

/**
 * NodeSecureStorage — ISecureStorage using AES-256-GCM.
 *
 * Key is stored as a hex file in userData/.crypto/storage.key.
 * Each encryption uses a fresh random IV → same plaintext produces different ciphertext.
 * Wire format: [iv(12)] + [tag(16)] + [ciphertext]
 */
export class NodeSecureStorage implements ISecureStorage {
  private readonly _key: Buffer
  private readonly _available: boolean

  constructor(app: IApp) {
    try {
      this._key = this._loadOrCreateKey(app.getPath('userData'))
      this._available = true
    } catch (err) {
      console.warn('[NodeSecureStorage] Failed to init encryption key:', err)
      this._key = Buffer.alloc(KEY_LENGTH)
      this._available = false
    }
  }

  isEncryptionAvailable(): boolean {
    return this._available
  }

  encryptString(plaintext: string): Buffer {
    const iv = randomBytes(IV_LENGTH)
    const cipher = createCipheriv(ALGORITHM, this._key, iv)
    const encrypted = Buffer.concat([
      cipher.update(Buffer.from(plaintext, 'utf-8')),
      cipher.final()
    ])
    const tag = cipher.getAuthTag()
    return Buffer.concat([iv, tag, encrypted])
  }

  decryptString(encrypted: Buffer): string {
    if (encrypted.length < IV_LENGTH + TAG_LENGTH) {
      throw new Error('[NodeSecureStorage] Invalid ciphertext: too short')
    }
    const iv = encrypted.subarray(0, IV_LENGTH)
    const tag = encrypted.subarray(IV_LENGTH, IV_LENGTH + TAG_LENGTH)
    const ciphertext = encrypted.subarray(IV_LENGTH + TAG_LENGTH)

    const decipher = createDecipheriv(ALGORITHM, this._key, iv)
    decipher.setAuthTag(tag)

    return Buffer.concat([
      decipher.update(ciphertext),
      decipher.final()
    ]).toString('utf-8')
  }

  private _loadOrCreateKey(userDataPath: string): Buffer {
    const keyDir = join(userDataPath, KEY_DIR)
    const keyPath = join(keyDir, KEY_FILE)

    if (existsSync(keyPath)) {
      const hex = readFileSync(keyPath, 'utf-8').trim()
      return Buffer.from(hex, 'hex')
    }

    // Generate new key
    mkdirSync(keyDir, { recursive: true })
    const key = randomBytes(KEY_LENGTH)
    // Write with restrictive permissions
    writeFileSync(keyPath, key.toString('hex'), { mode: 0o600 })
    return key
  }
}
```

### 2. `src/platform/adapters/node/system.ts`

```typescript
import { cpus, totalmem, freemem, hostname } from 'node:os'
import type { ISystemInfo } from '../../system-interface'

/**
 * NodeSystemInfo — ISystemInfo using Node.js os module.
 */
export class NodeSystemInfo implements ISystemInfo {
  getPlatform(): NodeJS.Platform {
    return process.platform
  }

  getTotalMemory(): number {
    return totalmem()
  }

  getFreeMemory(): number {
    return freemem()
  }

  getCpuCount(): number {
    return cpus().length
  }

  getHostname(): string {
    return hostname()
  }
}
```

### 3. `src/platform/adapters/node/__tests__/storage.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { NodeApp } from '../app'
import { NodeSecureStorage } from '../storage'

const testDataPath = join(tmpdir(), `orca-storage-test-${Date.now()}`)

afterEach(() => {
  if (existsSync(testDataPath)) rmSync(testDataPath, { recursive: true })
})

describe('NodeSecureStorage', () => {
  let storage: NodeSecureStorage

  beforeEach(() => {
    const app = new NodeApp({ userDataPath: testDataPath })
    storage = new NodeSecureStorage(app)
  })

  describe('isEncryptionAvailable()', () => {
    it('returns true when key loads successfully', () => {
      expect(storage.isEncryptionAvailable()).toBe(true)
    })
  })

  describe('encrypt/decrypt roundtrip', () => {
    it('ASCII string roundtrip', () => {
      const pt = 'hello-secret-value'
      expect(storage.decryptString(storage.encryptString(pt))).toBe(pt)
    })

    it('Unicode string roundtrip', () => {
      const pt = '秘密データ 🔐 мой пароль'
      expect(storage.decryptString(storage.encryptString(pt))).toBe(pt)
    })

    it('empty string roundtrip', () => {
      expect(storage.decryptString(storage.encryptString(''))).toBe('')
    })

    it('long string (1KB) roundtrip', () => {
      const pt = 'x'.repeat(1024)
      expect(storage.decryptString(storage.encryptString(pt))).toBe(pt)
    })

    it('encrypted value is Buffer', () => {
      expect(storage.encryptString('test')).toBeInstanceOf(Buffer)
    })

    it('encrypted differs from plaintext', () => {
      const pt = 'test-value'
      const enc = storage.encryptString(pt)
      expect(enc.toString('utf-8')).not.toBe(pt)
    })

    it('two encryptions of same plaintext differ (random IV)', () => {
      const pt = 'same-value'
      const enc1 = storage.encryptString(pt)
      const enc2 = storage.encryptString(pt)
      expect(enc1).not.toEqual(enc2)
    })
  })

  describe('key persistence', () => {
    it('decrypts successfully with new instance (same key file)', () => {
      const app = new NodeApp({ userDataPath: testDataPath })
      const storage2 = new NodeSecureStorage(app)
      const enc = storage.encryptString('persistent-test')
      expect(storage2.decryptString(enc)).toBe('persistent-test')
    })
  })

  describe('error handling', () => {
    it('throws on corrupted ciphertext (too short)', () => {
      expect(() => storage.decryptString(Buffer.from('short'))).toThrow('too short')
    })
  })
})
```

### 4. `src/platform/adapters/node/__tests__/system.test.ts`

```typescript
import { describe, it, expect } from 'vitest'
import { NodeSystemInfo } from '../system'

describe('NodeSystemInfo', () => {
  const system = new NodeSystemInfo()

  it('getPlatform() returns known platform', () => {
    const known = ['linux', 'darwin', 'win32', 'freebsd', 'openbsd']
    expect(known).toContain(system.getPlatform())
  })

  it('getTotalMemory() > 0', () => {
    expect(system.getTotalMemory()).toBeGreaterThan(0)
  })

  it('getFreeMemory() > 0 and <= total', () => {
    expect(system.getFreeMemory()).toBeGreaterThan(0)
    expect(system.getFreeMemory()).toBeLessThanOrEqual(system.getTotalMemory())
  })

  it('getCpuCount() >= 1', () => {
    expect(system.getCpuCount()).toBeGreaterThanOrEqual(1)
  })

  it('getHostname() is non-empty string', () => {
    expect(typeof system.getHostname()).toBe('string')
    expect(system.getHostname().length).toBeGreaterThan(0)
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "storage|system" | grep "node/" | head -10
npx vitest run src/platform/adapters/node/__tests__/storage.test.ts
npx vitest run src/platform/adapters/node/__tests__/system.test.ts
```

Expected: **15+ tests pass**, 0 errors.

---

## Done criteria

- [x] `src/platform/adapters/node/storage.ts` tạo thành công
- [x] `src/platform/adapters/node/system.ts` tạo thành công
- [x] Key được lưu tại `userData/.crypto/storage.key`
- [x] Random IV: hai lần mã hoá cùng plaintext cho kết quả khác nhau
- [x] Key persistence: decrypt với instance mới vẫn thành công
- [x] 15+ tests pass
