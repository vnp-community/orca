import { describe, expect, it } from 'vitest'
import { credentialLengthBucket, toCredentialWirePayload } from './credential-wire'

describe('toCredentialWirePayload', () => {
  it('round-trips the secret through base64 and leaves iv empty', () => {
    const secret = 'sk-ant-api03-abc_DEF-123'
    const wire = toCredentialWirePayload(secret)
    expect(wire.iv).toBe('')
    expect(atob(wire.encryptedBlob)).toBe(secret)
  })

  it('encodes non-ASCII as UTF-8 bytes', () => {
    const wire = toCredentialWirePayload('khoá-đặc-biệt')
    const bytes = Uint8Array.from(atob(wire.encryptedBlob), (c) => c.charCodeAt(0))
    expect(new TextDecoder().decode(bytes)).toBe('khoá-đặc-biệt')
  })
})

describe('credentialLengthBucket', () => {
  it('rounds up so two different key lengths in a bucket look identical', () => {
    const a = credentialLengthBucket(toCredentialWirePayload('k'.repeat(40)))
    const b = credentialLengthBucket(toCredentialWirePayload('k'.repeat(44)))
    expect(a).toBe(b)
    expect(a % 64).toBe(0)
  })
})
