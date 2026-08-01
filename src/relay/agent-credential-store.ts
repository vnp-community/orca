// src/relay/agent-credential-store.ts
// AI API credential store for Orca Dev Agent v5.0.
//
// Security layering:
//   Layer 1: Browser encrypts API key with SubtleCrypto (AES-GCM) → encryptedBlob + iv
//   Layer 2: Agent double-encrypts the blob using scrypt + AES-256-GCM before writing to disk
//   Storage: ~/.orca/credentials/<accountId>.enc (mode 0600, dir mode 0700)
//
// Master key: ORCA_AI_CREDENTIAL_KEY env var (set by admin on dev server)
// The agent never sees the plaintext API key — only the browser-encrypted blob.

import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { mkdirSync, writeFileSync, readFileSync, existsSync, unlinkSync } from 'node:fs'
import { join } from 'node:path'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const credTracer = createTracer('agent:credential')

// ─── Constants ────────────────────────────────────────────────────────────────

const ALGORITHM    = 'aes-256-gcm'
const KEY_LEN      = 32   // 256-bit AES key
const SALT_BYTES   = 16
const IV_BYTES     = 12   // GCM recommended IV size
const FILE_VERSION = 1

// ─── Types ────────────────────────────────────────────────────────────────────

interface StoredCredential {
  version: number
  salt:    string  // base64 16 bytes (scrypt salt)
  iv2:     string  // base64 12 bytes (AES-GCM IV for server-side encryption)
  authTag: string  // base64 16 bytes (GCM authentication tag)
  data:    string  // base64 ciphertext
  // Plaintext inside data (before outer encryption):
  //   JSON: { encryptedBlob: string, iv: string, algorithm: string }
}

// ─── Private helpers ─────────────────────────────────────────────────────────

function getCredentialKey(): string {
  const key = process.env.ORCA_AI_CREDENTIAL_KEY?.trim()
  if (!key) {
    const err = new Error('ORCA_AI_CREDENTIAL_KEY environment variable is not set or empty')
    Object.assign(err, { agentErrorCode: AgentErrorCode.PermissionDenied })
    throw err
  }
  return key
}

function credentialFilePath(credentialDir: string, accountId: string): string {
  // Allow only alphanumeric, dash, underscore — prevents path traversal
  if (!/^[\w-]+$/.test(accountId)) {
    const err = new Error(`Invalid accountId: "${accountId}". Only alphanumeric, dash, underscore allowed.`)
    Object.assign(err, { agentErrorCode: AgentErrorCode.InvalidParams })
    throw err
  }
  return join(credentialDir, `${accountId}.enc`)
}

function encryptPayload(masterKey: string, plaintext: string): Omit<StoredCredential, 'version'> {
  const salt    = randomBytes(SALT_BYTES)
  const iv2     = randomBytes(IV_BYTES)
  const key     = scryptSync(masterKey, salt, KEY_LEN)
  const cipher  = createCipheriv(ALGORITHM, key, iv2)
  const encrypted = Buffer.concat([
    cipher.update(Buffer.from(plaintext, 'utf8')),
    cipher.final(),
  ])
  const authTag = cipher.getAuthTag()
  return {
    salt:    salt.toString('base64'),
    iv2:     iv2.toString('base64'),
    authTag: authTag.toString('base64'),
    data:    encrypted.toString('base64'),
  }
}

function decryptPayload(masterKey: string, stored: StoredCredential): string {
  const salt    = Buffer.from(stored.salt, 'base64')
  const iv2     = Buffer.from(stored.iv2, 'base64')
  const authTag = Buffer.from(stored.authTag, 'base64')
  const data    = Buffer.from(stored.data, 'base64')

  const key      = scryptSync(masterKey, salt, KEY_LEN)
  const decipher = createDecipheriv(ALGORITHM, key, iv2)
  decipher.setAuthTag(authTag)

  return Buffer.concat([decipher.update(data), decipher.final()]).toString('utf8')
}

function errorResponse(id: string | number | null, err: unknown): object {
  const msg  = err instanceof Error ? err.message : String(err)
  const code = (err as { agentErrorCode?: number }).agentErrorCode ?? AgentErrorCode.ServerError
  return { jsonrpc: '2.0', id, error: { code, message: msg } }
}

// ─── RPC handlers ─────────────────────────────────────────────────────────────

export async function handleWriteCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId     = typeof params.accountId     === 'string' ? params.accountId     : ''
  const encryptedBlob = typeof params.encryptedBlob === 'string' ? params.encryptedBlob : ''
  const iv            = typeof params.iv            === 'string' ? params.iv            : ''
  const algorithm     = typeof params.algorithm     === 'string' ? params.algorithm     : 'AES-GCM'
  const span          = credTracer.start({ method: 'ai.provider.writeCredential', accountId })

  if (!accountId || !encryptedBlob || !iv) {
    span.fail('missing required params', { accountId })
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required params: accountId, encryptedBlob, iv' },
    }
  }

  try {
    const masterKey = getCredentialKey()
    const plaintext = JSON.stringify({ encryptedBlob, iv, algorithm })
    const encrypted = encryptPayload(masterKey, plaintext)
    const stored: StoredCredential = { version: FILE_VERSION, ...encrypted }

    // Ensure directory exists with restrictive permissions
    mkdirSync(config.credentialDir, { recursive: true, mode: 0o700 })

    const filePath = credentialFilePath(config.credentialDir, accountId)
    writeFileSync(filePath, JSON.stringify(stored), { mode: 0o600 })

    log.info(`ai.provider.writeCredential: stored accountId=${accountId}`)
    span.ok({ accountId })
    return { jsonrpc: '2.0', id, result: { ok: true } }

  } catch (err: unknown) {
    log.error(`ai.provider.writeCredential failed: ${err instanceof Error ? err.message : String(err)}`)
    span.fail(err, { accountId })
    return errorResponse(id, err)
  }
}

export async function handleReadCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const span      = credTracer.start({ method: 'ai.provider.readCredential', accountId })

  if (!accountId) {
    span.fail('missing accountId', { method: 'ai.provider.readCredential' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: accountId' } }
  }

  try {
    const masterKey = getCredentialKey()
    const filePath  = credentialFilePath(config.credentialDir, accountId)

    if (!existsSync(filePath)) {
      span.fail('credential not found', { accountId })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `Credential not found: ${accountId}` } }
    }

    const stored: StoredCredential = JSON.parse(readFileSync(filePath, 'utf8'))
    if (stored.version !== FILE_VERSION) {
      span.fail('unknown version', { accountId, version: stored.version })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `Unknown credential version: ${stored.version}` } }
    }

    const plaintext = decryptPayload(masterKey, stored)
    const payload   = JSON.parse(plaintext) as { encryptedBlob: string; iv: string; algorithm: string }
    span.ok({ accountId })
    return {
      jsonrpc: '2.0', id,
      result: {
        accountId,
        encryptedBlob: payload.encryptedBlob,
        iv:            payload.iv,
        algorithm:     payload.algorithm,
      },
    }

  } catch (err: unknown) {
    log.error(`ai.provider.readCredential failed: ${err instanceof Error ? err.message : String(err)}`)
    span.fail(err, { accountId })
    return errorResponse(id, err)
  }
}

export async function handleHealthCheck(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const provider  = typeof params.provider  === 'string' ? params.provider  : 'anthropic'
  const start = Date.now()
  const span  = credTracer.start({ method: 'ai.provider.healthCheck', provider })

  // AIP-001 Step 1: Verify credential exists and is readable
  const credResult = await handleReadCredential(id, { accountId }, config, log) as { error?: unknown; result?: unknown }
  if (credResult.error) {
    span.fail('credential unreadable', { accountId, provider })
    return {
      jsonrpc: '2.0', id,
      error: {
        code:    -32001,
        message: 'No credential found or decrypt failed. Please re-add the API key in Settings.',
        data:    { credentialFound: false, provider },
      },
    }
  }

  // AIP-001 Step 2: Network reachability with structured response
  const reachability = await checkProviderReachabilityDetailed(provider)
  const latencyMs = Date.now() - start
  log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} ok=${reachability.ok} note=${reachability.note}`)

  if (reachability.ok) {
    span.ok({ provider, latencyMs, note: reachability.note })
  } else {
    span.fail(reachability.note, { provider, latencyMs })
  }

  return {
    jsonrpc: '2.0', id,
    result: {
      ok:              reachability.ok,
      latencyMs,
      note:            reachability.note,
      credentialFound: true,  // AIP-001: credential exists and decryptable
      ...(reachability.statusCode !== undefined ? { statusCode: reachability.statusCode } : {}),
    },
  }
}

// AIP-001: Structured reachability check — returns ok, note, and optional HTTP statusCode
const PROVIDER_HEALTH_URLS: Record<string, string> = {
  anthropic: 'https://api.anthropic.com',
  openai:    'https://api.openai.com',
  gemini:    'https://generativelanguage.googleapis.com',
}

async function checkProviderReachabilityDetailed(provider: string): Promise<{
  ok: boolean; note: string; statusCode?: number
}> {
  const url = PROVIDER_HEALTH_URLS[provider]

  // Local providers (Ollama, vLLM, LM Studio) — no external check needed
  if (!url) return { ok: true, note: 'local_provider' }

  try {
    const ctrl  = new AbortController()
    const timer = setTimeout(() => ctrl.abort(), 5_000)
    const resp  = await fetch(url, { method: 'HEAD', signal: ctrl.signal })
    clearTimeout(timer)
    const statusCode = resp.status
    // 401/403/429 = server reachable (auth required / rate-limited — expected without auth)
    // 5xx = server error = not ok
    if (statusCode < 500) return { ok: true, note: 'reachable', statusCode }
    return { ok: false, note: 'server_error', statusCode }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    if (msg.includes('abort') || msg.includes('timeout')) {
      return { ok: false, note: 'timeout' }
    }
    return { ok: false, note: 'unreachable' }
  }
}


// ─── ai.provider.deleteCredential ────────────────────────────────────────────


export async function handleDeleteCredential(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const span      = credTracer.start({ method: 'ai.provider.deleteCredential', accountId })

  if (!accountId) {
    span.fail('missing accountId', { method: 'ai.provider.deleteCredential' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: accountId' } }
  }

  try {
    const filePath = credentialFilePath(config.credentialDir, accountId)

    if (!existsSync(filePath)) {
      // Idempotent — if not found, still return ok
      log.info(`ai.provider.deleteCredential: not found (idempotent ok) accountId=${accountId}`)
      span.ok({ accountId, deleted: false })
      return { jsonrpc: '2.0', id, result: { ok: true, deleted: false } }
    }

    unlinkSync(filePath)
    log.info(`ai.provider.deleteCredential: deleted accountId=${accountId}`)
    span.ok({ accountId, deleted: true })
    return { jsonrpc: '2.0', id, result: { ok: true, deleted: true } }

  } catch (err: unknown) {
    log.error(`ai.provider.deleteCredential failed: ${err instanceof Error ? err.message : String(err)}`)
    span.fail(err, { accountId })
    return errorResponse(id, err)
  }
}

// ─── readDecryptedKey (used by agent-spawner.ts) ─────────────────────────────

/**
 * Read and decrypt the stored credential, returning the encryptedBlob
 * (which in v5.0 is the outer-encrypted plaintext API key).
 * Returns null if credential not found or decryption fails.
 */
export async function readDecryptedKey(
  accountId: string,
  config:    AgentConfig,
  log:       AgentLogger
): Promise<string | null> {
  const result = await handleReadCredential(null, { accountId }, config, log) as {
    result?: { encryptedBlob: string }; error?: unknown
  }
  if (result.error || !result.result) return null
  return result.result.encryptedBlob
}
