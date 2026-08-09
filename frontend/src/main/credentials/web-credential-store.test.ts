import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createCipheriv, randomBytes, scryptSync } from 'node:crypto'
import { WebCredentialStore } from './web-credential-store'

const SERVER_SECRET = 'test-server-secret'

// Mirrors the store's private V1 format (iv(16) + authTag(16) + ciphertext,
// fixed legacy key) so we can plant a "real" V1 blob on disk for migration tests.
function buildV1Blob(plaintext: string, serverSecret: string): Buffer {
  const key = scryptSync(serverSecret, 'orca-web-credential-store-v1', 32) as Buffer
  const iv = randomBytes(16)
  const cipher = createCipheriv('aes-256-gcm', key, iv)
  const ct = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])
  return Buffer.concat([iv, cipher.getAuthTag(), ct])
}

// Mirrors the store's private (pre-fix) V2 format — no userId in the KDF.
function buildV2Blob(plaintext: string, serverSecret: string): Buffer {
  const magic = Buffer.from([0x4f, 0x52, 0x43, 0x32])
  const salt = randomBytes(32)
  const iv = randomBytes(16)
  const key = scryptSync(serverSecret, salt, 32) as Buffer
  const cipher = createCipheriv('aes-256-gcm', key, iv)
  const ct = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])
  return Buffer.concat([magic, salt, iv, cipher.getAuthTag(), ct])
}

describe('WebCredentialStore — V3 KDF (BUG-FE-HLD-003)', () => {
  let userDataPath: string

  beforeEach(() => {
    userDataPath = mkdtempSync(join(tmpdir(), 'orca-cred-store-test-'))
  })

  afterEach(() => {
    rmSync(userDataPath, { recursive: true, force: true })
  })

  it('round-trips a token written and read by the same user', async () => {
    const store = new WebCredentialStore(userDataPath, 'user-a', SERVER_SECRET)
    await store.setToken('linear', 'secret-token-value')
    expect(await store.getToken('linear')).toBe('secret-token-value')
  })

  it('writes tokens in V3 format (ORC3 magic)', async () => {
    const store = new WebCredentialStore(userDataPath, 'user-a', SERVER_SECRET)
    await store.setToken('linear', 'secret-token-value')
    const raw = readFileSync(join(userDataPath, 'users', 'user-a', 'credentials', 'linear.enc'))
    expect(raw.subarray(0, 4).toString('latin1')).toBe('ORC3')
  })

  it('does NOT decrypt a token written by a different userId (cross-user isolation)', async () => {
    const storeA = new WebCredentialStore(userDataPath, 'user-a', SERVER_SECRET)
    await storeA.setToken('linear', 'user-a-secret')

    const storeB = new WebCredentialStore(userDataPath, 'user-b', SERVER_SECRET)
    // Why: simulate a leaked blob landing in user B's directory with the same
    // serverSecret — this is exactly the attack BUG-FE-HLD-003 was about.
    writeFileSync(
      join(userDataPath, 'users', 'user-b', 'credentials', 'linear.enc'),
      readFileSync(join(userDataPath, 'users', 'user-a', 'credentials', 'linear.enc'))
    )
    expect(await storeB.getToken('linear')).toBeNull()
  })

  it('still decrypts a legacy V1 blob', async () => {
    const store = new WebCredentialStore(userDataPath, 'user-a', SERVER_SECRET)
    writeFileSync(
      join(userDataPath, 'users', 'user-a', 'credentials', 'jira.enc'),
      buildV1Blob('legacy-v1-secret', SERVER_SECRET)
    )
    expect(await store.getToken('jira')).toBe('legacy-v1-secret')
  })

  it('still decrypts a legacy V2 blob', async () => {
    const store = new WebCredentialStore(userDataPath, 'user-a', SERVER_SECRET)
    writeFileSync(
      join(userDataPath, 'users', 'user-a', 'credentials', 'gitea.enc'),
      buildV2Blob('legacy-v2-secret', SERVER_SECRET)
    )
    expect(await store.getToken('gitea')).toBe('legacy-v2-secret')
  })

  it('lazily re-encrypts a V2 blob to V3 the first time it is read', async () => {
    const store = new WebCredentialStore(userDataPath, 'user-a', SERVER_SECRET)
    const path = join(userDataPath, 'users', 'user-a', 'credentials', 'gitea.enc')
    writeFileSync(path, buildV2Blob('legacy-v2-secret', SERVER_SECRET))

    expect(readFileSync(path).subarray(0, 4).toString('latin1')).toBe('ORC2')
    const value = await store.getToken('gitea')
    expect(value).toBe('legacy-v2-secret')
    expect(readFileSync(path).subarray(0, 4).toString('latin1')).toBe('ORC3')

    // Reading again still works from the now-V3 blob.
    expect(await store.getToken('gitea')).toBe('legacy-v2-secret')
  })

  it('migrateToV3() upgrades V1 and V2 blobs and skips ones already at V3', async () => {
    const store = new WebCredentialStore(userDataPath, 'user-a', SERVER_SECRET)
    const dir = join(userDataPath, 'users', 'user-a', 'credentials')
    writeFileSync(join(dir, 'jira.enc'), buildV1Blob('v1-secret', SERVER_SECRET))
    writeFileSync(join(dir, 'gitea.enc'), buildV2Blob('v2-secret', SERVER_SECRET))
    await store.setToken('linear', 'already-v3-secret') // written as V3 directly

    const result = await store.migrateToV3()
    expect(result).toEqual({ migrated: 2, failed: 0 })

    expect(readFileSync(join(dir, 'jira.enc')).subarray(0, 4).toString('latin1')).toBe('ORC3')
    expect(readFileSync(join(dir, 'gitea.enc')).subarray(0, 4).toString('latin1')).toBe('ORC3')
    expect(await store.getToken('jira')).toBe('v1-secret')
    expect(await store.getToken('gitea')).toBe('v2-secret')
    expect(await store.getToken('linear')).toBe('already-v3-secret')

    // Running again is a no-op — nothing left to migrate.
    expect(await store.migrateToV3()).toEqual({ migrated: 0, failed: 0 })
  })
})
