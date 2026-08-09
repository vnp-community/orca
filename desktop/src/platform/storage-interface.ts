/**
 * ISecureStorage — abstraction over Electron's safeStorage.
 *
 * NodeAdapter uses AES-256-GCM with a file-based key.
 * ElectronAdapter delegates to Electron's OS keychain integration.
 */
export type ISecureStorage = {
  /** True if encryption is available (false = will store as plain bytes) */
  isEncryptionAvailable(): boolean

  /** Encrypt a plaintext string → Buffer */
  encryptString(plaintext: string): Buffer

  /** Decrypt a Buffer → plaintext string */
  decryptString(encrypted: Buffer): string
}
