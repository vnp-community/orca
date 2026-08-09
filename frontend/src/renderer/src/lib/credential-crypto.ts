/**
 * credential-crypto.ts
 *
 * SECURITY:
 * - Plaintext API keys NEVER stored in state after encryption
 * - Session-derived AES-GCM key (PBKDF2, 100k iterations)
 * - Caller must clear rawValue after calling encryptCredential()
 */

const PBKDF2_ITERATIONS = 100_000
const SALT = new TextEncoder().encode('orca-cred-v1')

async function deriveKey(sessionToken: string): Promise<CryptoKey> {
  const keyMaterial = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(sessionToken),
    { name: 'PBKDF2' },
    false,
    ['deriveKey']
  )
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt: SALT, iterations: PBKDF2_ITERATIONS, hash: 'SHA-256' },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt']
  )
}

export type EncryptedCredential = {
  encryptedBlob: string   // base64
  iv:            string   // base64
}

/**
 * Encrypt plaintext credential using session-derived key.
 * Returns base64-encoded ciphertext + IV.
 * Caller MUST clear the plaintext string from state after calling this.
 */
export async function encryptCredential(
  plaintext: string,
  sessionToken: string
): Promise<EncryptedCredential> {
  const key = await deriveKey(sessionToken)
  const iv  = crypto.getRandomValues(new Uint8Array(16))

  const encrypted = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    new TextEncoder().encode(plaintext)
  )

  return {
    encryptedBlob: btoa(String.fromCharCode(...new Uint8Array(encrypted))),
    iv:            btoa(String.fromCharCode(...iv)),
  }
}
