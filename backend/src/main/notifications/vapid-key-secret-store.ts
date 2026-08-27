/**
 * VapidKeySecretStore — AES-256-GCM encrypted file store for the Web Push
 * VAPID *private* key (ADR-021 §4)
 *
 * Why a new, separate store instead of reusing `WebCredentialStore`
 * (credentials/web-credential-store.ts): that store is per-user
 * (`<userDataPath>/users/<userId>/credentials/<service>.enc`) and only
 * initialized when `ORCA_MULTI_USER=1` (`isWebCredentialMode()`) — VAPID keys
 * are server-instance-wide (one keypair per Orca server, not per user) and
 * used by `WebPushManager` regardless of multi-user mode, so depending on
 * `WebCredentialStore` would make Web Push break in single-user server mode.
 * Same AES-256-GCM/scrypt scheme as `WebCredentialStore`'s V2 format (for
 * crypto-review consistency across the codebase — see that file's
 * `encryptV2`/`decryptBlob`), simplified to one secret / one file (no
 * per-service dimension, no V1-legacy-format fallback to carry).
 *
 * @module main/notifications/vapid-key-secret-store
 */

import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

const MAGIC = Buffer.from('ORVK') // "ORca Vapid Key"
const SALT_BYTES = 32
const IV_BYTES = 16
const HEADER_SIZE = MAGIC.length + SALT_BYTES + IV_BYTES + 16 // + authTag

export class VapidKeySecretStore {
  private readonly filePath: string

  constructor(userDataPath: string, private readonly serverSecret: string) {
    this.filePath = join(userDataPath, 'system-secrets', 'vapid-private-key.enc')
  }

  async getPrivateKey(): Promise<string | null> {
    try {
      const raw = readFileSync(this.filePath)
      const salt = raw.subarray(MAGIC.length, MAGIC.length + SALT_BYTES)
      const iv = raw.subarray(MAGIC.length + SALT_BYTES, MAGIC.length + SALT_BYTES + IV_BYTES)
      const authTag = raw.subarray(MAGIC.length + SALT_BYTES + IV_BYTES, HEADER_SIZE)
      const ciphertext = raw.subarray(HEADER_SIZE)
      const key = scryptSync(this.serverSecret, salt, 32) as Buffer
      const decipher = createDecipheriv('aes-256-gcm', key, iv)
      decipher.setAuthTag(authTag)
      return decipher.update(ciphertext).toString('utf8') + decipher.final('utf8')
    } catch {
      // Missing, corrupt, or wrong serverSecret — treat as "no key yet",
      // same non-fatal posture as WebCredentialStore.getToken().
      return null
    }
  }

  async setPrivateKey(privateKey: string): Promise<void> {
    const dir = dirname(this.filePath)
    if (!existsSync(dir)) {mkdirSync(dir, { recursive: true, mode: 0o700 })}
    const salt = randomBytes(SALT_BYTES)
    const iv = randomBytes(IV_BYTES)
    const key = scryptSync(this.serverSecret, salt, 32) as Buffer
    const cipher = createCipheriv('aes-256-gcm', key, iv)
    const ciphertext = Buffer.concat([cipher.update(privateKey, 'utf8'), cipher.final()])
    const authTag = cipher.getAuthTag()
    writeFileSync(this.filePath, Buffer.concat([MAGIC, salt, iv, authTag, ciphertext]), { mode: 0o600 })
  }
}
