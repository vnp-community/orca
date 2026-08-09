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
// V2 (current): magic(4) + salt(32) + iv(16) + authTag(16) + ciphertext — per-blob random salt

const V2_MAGIC = Buffer.from([0x4f, 0x52, 0x43, 0x32])  // "ORC2"
const V2_SALT_BYTES  = 32
const V2_IV_BYTES    = 16
const V2_AUTH_TAG_BYTES = 16
const V2_HEADER_SIZE = V2_MAGIC.length + V2_SALT_BYTES + V2_IV_BYTES + V2_AUTH_TAG_BYTES  // 68 bytes

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
  // V1 legacy fixed key — only used for decrypting old V1 blobs
  private readonly legacyKey: Buffer

  constructor(userDataPath: string, userId: string, serverSecret: string) {
    // Per-user directory with restrictive permissions
    this.userCredDir  = join(userDataPath, 'users', userId, 'credentials')
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

  // ── V2 encryption helpers ──────────────────────────────────────────────────

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

  private decryptBlob(raw: Buffer): string {
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
    // FIX TASK-RI-001: Always write V2 format (random salt per token)
    const stored = this.encryptV2(token)
    writeFileSync(this.tokenPath(service), stored, { mode: 0o600 })

    if (config) {
      writeFileSync(this.configPath(service), JSON.stringify(config), { mode: 0o600 })
    }
  }

  async getToken(service: CredentialService): Promise<string | null> {
    try {
      const raw = readFileSync(this.tokenPath(service))
      return this.decryptBlob(raw)
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
}
