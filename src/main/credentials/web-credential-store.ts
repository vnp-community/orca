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

/**
 * WebCredentialStore — AES-256-GCM encrypted per-user token storage.
 *
 * Used in Web Server mode (ORCA_MULTI_USER=1) as a drop-in replacement for
 * Electron's safeStorage API, which is not available in headless Node.js.
 *
 * Each user gets an isolated credential directory scoped to their userId:
 *   <userDataPath>/users/<userId>/credentials/<service>.enc
 *
 * Encryption: AES-256-GCM with a key derived from the server's ORCA_SERVER_SECRET
 * using scrypt. Each token gets a fresh random IV, ensuring that repeated writes
 * of the same token produce different ciphertext.
 *
 * Why: In multi-user server mode, user processes run as separate forks and cannot
 * share credential state. This store gives each process its own encrypted storage
 * that is safe to read from the file system without cross-user leakage.
 */
export class WebCredentialStore {
  private readonly userCredDir: string
  private readonly encryptionKey: Buffer

  constructor(userDataPath: string, userId: string, serverSecret: string) {
    // Per-user directory with restrictive permissions
    this.userCredDir = join(userDataPath, 'users', userId, 'credentials')
    // Derive a stable 32-byte AES key from the server secret.
    // The salt is fixed so the key is deterministic across restarts.
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

  // ── Token storage ──────────────────────────────────────────────────────────

  async setToken(
    service: CredentialService,
    token: string,
    config?: CredentialConfig
  ): Promise<void> {
    const iv = randomBytes(16)
    const cipher = createCipheriv('aes-256-gcm', this.encryptionKey, iv)
    const encrypted = Buffer.concat([cipher.update(token, 'utf8'), cipher.final()])
    const authTag = cipher.getAuthTag()
    // Wire format: iv(16 bytes) + authTag(16 bytes) + ciphertext
    const stored = Buffer.concat([iv, authTag, encrypted])
    writeFileSync(this.tokenPath(service), stored, { mode: 0o600 })

    if (config) {
      writeFileSync(this.configPath(service), JSON.stringify(config), { mode: 0o600 })
    }
  }

  async getToken(service: CredentialService): Promise<string | null> {
    try {
      const raw = readFileSync(this.tokenPath(service))
      if (raw.length < 33) return null // must have at least iv + authTag + 1 byte
      const iv = raw.subarray(0, 16)
      const authTag = raw.subarray(16, 32)
      const encrypted = raw.subarray(32)
      const decipher = createDecipheriv('aes-256-gcm', this.encryptionKey, iv)
      decipher.setAuthTag(authTag)
      return decipher.update(encrypted).toString('utf8') + decipher.final('utf8')
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
}
