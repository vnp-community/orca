import {
  randomBytes,
  createCipheriv,
  createDecipheriv
} from 'node:crypto'
import { existsSync, readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'
import type { IApp } from '../../app-interface'
import type { ISecureStorage } from '../../storage-interface'

const ALGORITHM = 'aes-256-gcm'
const KEY_LENGTH = 32 // 256 bits
const IV_LENGTH = 12 // GCM standard
const TAG_LENGTH = 16 // GCM auth tag
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

    return Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString('utf-8')
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
