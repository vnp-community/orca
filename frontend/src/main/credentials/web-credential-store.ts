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

// All integration service names that can store credentials
export type CredentialService =
  | 'bitbucket'
  | 'azure-devops'
  | 'gitea'
  | 'linear'
  | 'jira'

export type CredentialConfig = Record<string, string>

// ── Wire format versions ───────────────────────────────────────────────────────
// V1 (legacy): iv(16) + authTag(16) + ciphertext — fixed derived key, no per-blob salt
// V2 (legacy): magic(4) + salt(32) + iv(16) + authTag(16) + ciphertext — per-blob random salt
// V3 (current): magic(4) + salt(32) + iv(12) + authTag(16) + ciphertext — salt AND userId
//   both feed the KDF, and IV is the AES-GCM-standard 96 bits (12 bytes).
//
// FIX BUG-FE-HLD-003: V2's key (scryptSync(serverSecret, salt, 32)) does not
// depend on userId at all — every user's credentials decrypt under the same
// key given the same salt, so cross-user isolation relied only on filesystem
// permissions (0o700/0o600), not cryptography, contradicting the documented
// "Per-user key từ userId + server secret" design (docs/hld/v1/security.md §11).
// V3 fixes this by mixing userId into the scrypt password input.

const V2_MAGIC = Buffer.from([0x4f, 0x52, 0x43, 0x32])  // "ORC2"
const V2_SALT_BYTES  = 32
const V2_IV_BYTES    = 16
const V2_AUTH_TAG_BYTES = 16
const V2_HEADER_SIZE = V2_MAGIC.length + V2_SALT_BYTES + V2_IV_BYTES + V2_AUTH_TAG_BYTES  // 68 bytes

const V3_MAGIC = Buffer.from([0x4f, 0x52, 0x43, 0x33])  // "ORC3"
const V3_SALT_BYTES  = 32
const V3_IV_BYTES    = 12  // AES-GCM standard 96-bit IV (security.md §11), vs V2's 16
const V3_AUTH_TAG_BYTES = 16
const V3_HEADER_SIZE = V3_MAGIC.length + V3_SALT_BYTES + V3_IV_BYTES + V3_AUTH_TAG_BYTES  // 64 bytes

/**
 * WebCredentialStore — AES-256-GCM encrypted per-user token storage.
 *
 * Used in Web Server mode (ORCA_MULTI_USER=1) as a drop-in replacement for
 * Electron's safeStorage API, which is not available in headless Node.js.
 *
 * Each user gets an isolated credential directory scoped to their userId:
 *   <userDataPath>/users/<userId>/credentials/<service>.enc
 *
 * FIX TASK-RI-001: V2 format adds per-blob 32-byte random salt for key derivation.
 * Previously (V1), the key was derived from fixed salt 'orca-web-credential-store-v1',
 * making all credentials vulnerable to dictionary attacks with the same derived key.
 *
 * V2 wire format: magic(4) + salt(32) + iv(16) + authTag(16) + ciphertext
 * V1 backward compat: blobs without "ORC2" magic are decrypted using the legacy fixed key.
 */
export class WebCredentialStore {
  private readonly userCredDir: string
  private readonly serverSecret: string
  // FIX BUG-FE-HLD-003: kept (not just used for the directory path) so V3
  // encryption/decryption can mix it into the KDF — see module header.
  private readonly userId: string
  // V1 legacy fixed key — only used for decrypting old V1 blobs
  private readonly legacyKey: Buffer

  constructor(userDataPath: string, userId: string, serverSecret: string) {
    // Per-user directory with restrictive permissions
    this.userCredDir  = join(userDataPath, 'users', userId, 'credentials')
    this.userId        = userId
    this.serverSecret = serverSecret
    // FIX TASK-RI-001: Keep legacy key for V1 backward compat only
    this.legacyKey = scryptSync(serverSecret, 'orca-web-credential-store-v1', 32) as Buffer
    mkdirSync(this.userCredDir, { recursive: true, mode: 0o700 })
  }

  private tokenPath(service: CredentialService): string {
    return join(this.userCredDir, `${service}.enc`)
  }

  private configPath(service: CredentialService): string {
    return join(this.userCredDir, `${service}.config.json`)
  }

  // ── V3 encryption helpers ──────────────────────────────────────────────────

  private deriveKeyV3(salt: Buffer): Buffer {
    // FIX BUG-FE-HLD-003: userId feeds the KDF password (not just the salt),
    // so decrypting a blob written for one user with another user's userId
    // fails even with the same serverSecret + salt.
    return scryptSync(`${this.serverSecret}:${this.userId}`, salt, 32) as Buffer
  }

  private encryptV3(plaintext: string): Buffer {
    const salt    = randomBytes(V3_SALT_BYTES)
    const iv      = randomBytes(V3_IV_BYTES)
    const key     = this.deriveKeyV3(salt)
    const cipher  = createCipheriv('aes-256-gcm', key, iv)
    const ct      = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])
    const authTag = cipher.getAuthTag()
    return Buffer.concat([V3_MAGIC, salt, iv, authTag, ct])
  }

  // ── V2 encryption helpers (legacy — decrypt-only, kept for migration) ──────

  private encryptV2(plaintext: string): Buffer {
    // FIX TASK-RI-001: Per-blob random 32-byte salt → unique key per credential write
    const salt    = randomBytes(V2_SALT_BYTES)
    const iv      = randomBytes(V2_IV_BYTES)
    const key     = scryptSync(this.serverSecret, salt, 32) as Buffer
    const cipher  = createCipheriv('aes-256-gcm', key, iv)
    const ct      = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])
    const authTag = cipher.getAuthTag()
    return Buffer.concat([V2_MAGIC, salt, iv, authTag, ct])
  }

  private isV3(raw: Buffer): boolean {
    return raw.length >= V3_HEADER_SIZE && raw.subarray(0, 4).equals(V3_MAGIC)
  }

  private decryptBlob(raw: Buffer): string {
    // Check for V3 magic header first (current format)
    if (this.isV3(raw)) {
      // V3: magic(4) + salt(32) + iv(12) + authTag(16) + ct
      const salt    = raw.subarray(4, 4 + V3_SALT_BYTES)
      const iv      = raw.subarray(4 + V3_SALT_BYTES, 4 + V3_SALT_BYTES + V3_IV_BYTES)
      const authTag = raw.subarray(4 + V3_SALT_BYTES + V3_IV_BYTES, V3_HEADER_SIZE)
      const ct      = raw.subarray(V3_HEADER_SIZE)
      const key     = this.deriveKeyV3(salt)
      const d       = createDecipheriv('aes-256-gcm', key, iv)
      d.setAuthTag(authTag)
      return d.update(ct).toString('utf8') + d.final('utf8')
    }

    // Check for V2 magic header
    if (raw.length >= V2_HEADER_SIZE && raw.subarray(0, 4).equals(V2_MAGIC)) {
      // V2: magic(4) + salt(32) + iv(16) + authTag(16) + ct
      const salt    = raw.subarray(4, 4 + V2_SALT_BYTES)
      const iv      = raw.subarray(4 + V2_SALT_BYTES, 4 + V2_SALT_BYTES + V2_IV_BYTES)
      const authTag = raw.subarray(4 + V2_SALT_BYTES + V2_IV_BYTES, V2_HEADER_SIZE)
      const ct      = raw.subarray(V2_HEADER_SIZE)
      const key     = scryptSync(this.serverSecret, salt, 32) as Buffer
      const d       = createDecipheriv('aes-256-gcm', key, iv)
      d.setAuthTag(authTag)
      return d.update(ct).toString('utf8') + d.final('utf8')
    }

    // V1 legacy: iv(16) + authTag(16) + ciphertext — uses fixed derived key
    if (raw.length >= 33) {
      const iv      = raw.subarray(0, 16)
      const authTag = raw.subarray(16, 32)
      const ct      = raw.subarray(32)
      const d       = createDecipheriv('aes-256-gcm', this.legacyKey, iv)
      d.setAuthTag(authTag)
      return d.update(ct).toString('utf8') + d.final('utf8')
    }

    throw new Error('Invalid credential blob: too short')
  }

  // ── Token storage ──────────────────────────────────────────────────────────

  async setToken(
    service: CredentialService,
    token: string,
    config?: CredentialConfig
  ): Promise<void> {
    // FIX BUG-FE-HLD-003: Always write V3 format (userId-bound key, 12-byte IV)
    const stored = this.encryptV3(token)
    writeFileSync(this.tokenPath(service), stored, { mode: 0o600 })

    if (config) {
      writeFileSync(this.configPath(service), JSON.stringify(config), { mode: 0o600 })
    }
  }

  async getToken(service: CredentialService): Promise<string | null> {
    try {
      const raw = readFileSync(this.tokenPath(service))
      const plaintext = this.decryptBlob(raw)
      // FIX BUG-FE-HLD-003: lazy self-heal — the first time a pre-V3 credential
      // is actually read, re-encrypt it under V3 immediately. No batch/startup
      // migration step required (see migrateToV3() below for that path too;
      // this lazy path guarantees upgrade even if nothing ever calls it).
      // Write only after a SUCCESSFUL decrypt, so a bad serverSecret/corrupt
      // blob never overwrites the original file.
      if (!this.isV3(raw)) {
        try {
          writeFileSync(this.tokenPath(service), this.encryptV3(plaintext), { mode: 0o600 })
        } catch (err) {
          // Why: failing to persist the upgrade must not fail the read — the
          // caller still gets a valid token; we just retry the upgrade next read.
          console.error(`[WebCredentialStore] V3 lazy re-encrypt failed for service=${service}:`, err)
        }
      }
      return plaintext
    } catch {
      // Token missing, corrupt, or decryption failed (wrong key)
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
    try {
      unlinkSync(this.tokenPath(service))
    } catch {
      // Ignore — token may not exist
    }
    try {
      unlinkSync(this.configPath(service))
    } catch {
      // Ignore — config may not exist
    }
  }

  async listServices(): Promise<CredentialService[]> {
    try {
      return readdirSync(this.userCredDir)
        .filter((f) => f.endsWith('.enc'))
        .map((f) => f.replace('.enc', '') as CredentialService)
    } catch {
      return []
    }
  }

  /**
   * FIX TASK-RI-001: Re-encrypt all V1 credentials to V2 format.
   * Safe to call at startup — skips V2 blobs, re-encrypts V1 blobs only.
   * Returns { migrated, failed } count.
   */
  async migrateV1ToV2(): Promise<{ migrated: number; failed: number }> {
    let migrated = 0, failed = 0
    const services = await this.listServices()
    for (const service of services) {
      try {
        const raw = readFileSync(this.tokenPath(service))
        // Skip blobs already in V2 format
        if (raw.length >= 4 && raw.subarray(0, 4).equals(V2_MAGIC)) {continue}
        // Decrypt V1, re-encrypt as V2
        const plaintext = this.decryptBlob(raw)
        const newBlob   = this.encryptV2(plaintext)
        writeFileSync(this.tokenPath(service), newBlob, { mode: 0o600 })
        migrated++
      } catch (err) {
        console.error(`[WebCredentialStore] V2 migration failed for service=${service}:`, err)
        failed++
      }
    }
    if (migrated > 0 || failed > 0) {
      console.log(`[WebCredentialStore] V2 migration: ${migrated} migrated, ${failed} failed`)
    }
    return { migrated, failed }
  }

  /**
   * FIX BUG-FE-HLD-003: Re-encrypt all V1/V2 credentials to V3 format (userId-bound
   * key, 12-byte IV). Mirrors migrateV1ToV2() above — safe to call at startup,
   * skips blobs already at V3. getToken() also lazily upgrades on read, so this
   * is a belt-and-suspenders batch pass, not the only path to V3.
   * Returns { migrated, failed } count.
   */
  async migrateToV3(): Promise<{ migrated: number; failed: number }> {
    let migrated = 0, failed = 0
    const services = await this.listServices()
    for (const service of services) {
      try {
        const raw = readFileSync(this.tokenPath(service))
        if (this.isV3(raw)) {continue}
        const plaintext = this.decryptBlob(raw)
        writeFileSync(this.tokenPath(service), this.encryptV3(plaintext), { mode: 0o600 })
        migrated++
      } catch (err) {
        console.error(`[WebCredentialStore] V3 migration failed for service=${service}:`, err)
        failed++
      }
    }
    if (migrated > 0 || failed > 0) {
      console.log(`[WebCredentialStore] V3 migration: ${migrated} migrated, ${failed} failed`)
    }
    return { migrated, failed }
  }
}
