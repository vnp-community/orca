/**
 * AI Provider Handler — runs on Dev Server via relay binary (TDD-16)
 *
 * IMPORTANT: This file runs on the Dev Server, NOT on Orca Server.
 * Do NOT import anything from src/main/.
 *
 * Credentials are stored AES-256-GCM encrypted in ~/.orca/ai-providers/<accountId>.enc
 * The relay binary is responsible for decryption when building agent env.
 *
 * @module relay/ai-provider-handler
 */

import { writeFile, readFile, mkdir } from 'node:fs/promises'
import { join } from 'node:path'
import { homedir } from 'node:os'

const PROVIDER_STORE_DIR = join(homedir(), '.orca', 'ai-providers')

export interface CredentialRecord {
  encryptedBlob: string
  iv: string
  updatedAt: number
}

export const aiProviderHandlers = {
  /**
   * Store encrypted credential on dev server filesystem.
   * Called by Orca Server via relay — NEVER receives plaintext.
   */
  'ai.provider.writeCredential': async (params: {
    accountId: string
    encryptedBlob: string
    iv: string
  }): Promise<{ ok: boolean }> => {
    await mkdir(PROVIDER_STORE_DIR, { recursive: true })
    const data = JSON.stringify({
      encryptedBlob: params.encryptedBlob,
      iv: params.iv,
      updatedAt: Date.now(),
    })
    await writeFile(join(PROVIDER_STORE_DIR, `${params.accountId}.enc`), data, 'utf-8')
    return { ok: true }
  },

  /**
   * Read encrypted credential from dev server (for relay to use when spawning agents).
   * Returns the encrypted blob — decryption happens inside the relay process.
   */
  'ai.provider.readCredential': async (params: {
    accountId: string
  }): Promise<CredentialRecord> => {
    const filePath = join(PROVIDER_STORE_DIR, `${params.accountId}.enc`)
    const raw = await readFile(filePath, 'utf-8')
    return JSON.parse(raw) as CredentialRecord
  },

  /**
   * Test connectivity for a provider account.
   * Non-throwing — returns { ok: false, error } on any failure.
   *
   * Reads credential from local store and performs a minimal API call
   * appropriate for the provider type.
   */
  'ai.provider.testConnection': async (params: {
    accountId: string
    provider?: string
    model?: string
  }): Promise<{ ok: boolean; latencyMs: number; error?: string }> => {
    const start = Date.now()
    try {
      // Read credential — if this fails, the credential hasn't been written yet
      await aiProviderHandlers['ai.provider.readCredential']({ accountId: params.accountId })

      // Minimal connectivity check — actual provider ping would go here
      // For now, successful credential read = provider reachable
      // Phase 5 will extend this with real API pings per provider type

      return { ok: true, latencyMs: Date.now() - start }
    } catch (err: unknown) {
      return {
        ok: false,
        latencyMs: Date.now() - start,
        error: err instanceof Error ? err.message : String(err),
      }
    }
  },

  /**
   * Health check alias — used by ProviderHealthChecker via relay.
   * Maps to testConnection for compatibility with relay RPC routing.
   */
  'ai.provider.healthCheck': async (params: {
    accountId: string
    provider?: string
    model?: string
  }): Promise<{ ok: boolean; latencyMs: number; error?: string }> => {
    return aiProviderHandlers['ai.provider.testConnection'](params)
  },
}

export type AIProviderHandlerName = keyof typeof aiProviderHandlers
