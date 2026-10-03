// Wire format for AI-provider API keys sent to `aiProvider.writeCredential`.
//
// Why no client-side crypto: the previous browser AES-GCM envelope used the constant
// 'fallback-dev-token' as its key (the auth slice has no session token), and nothing
// downstream can decrypt it — credential-broker treats the envelope as opaque bytes and
// the dev-server agent has no decrypt path — so it gave a false sense of security and
// stored credentials nobody could use. The key now travels once over the authenticated
// TLS channel; credential-broker encrypts it at rest with Vault Transit.
//
// The field names stay `encryptedBlob`/`iv` because the backend contract is unchanged
// (it forwards `iv || blob` as opaque bytes); `iv` is intentionally empty.
export type CredentialWirePayload = { encryptedBlob: string; iv: string }

export function toCredentialWirePayload(secret: string): CredentialWirePayload {
  const bytes = new TextEncoder().encode(secret)
  let binary = ''
  for (const b of bytes) {
    binary += String.fromCharCode(b)
  }
  return { encryptedBlob: btoa(binary), iv: '' }
}

// Rounded so span fields never reveal the exact key length.
export function credentialLengthBucket(payload: CredentialWirePayload): number {
  return Math.ceil(payload.encryptedBlob.length / 64) * 64
}
