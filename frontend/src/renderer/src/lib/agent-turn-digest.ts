/**
 * agent-turn-digest.ts — FE-CV-TASK-089-01
 *
 * SHA-256 and text normalization utilities for agent turn recording.
 * Pure functions, no React, no store dependencies.
 *
 * @module lib/agent-turn-digest
 */

// ---------------------------------------------------------------------------
// SHA-256
// ---------------------------------------------------------------------------

/**
 * Compute a hex SHA-256 digest of a UTF-8 string.
 * Works in both secure and non-secure contexts (falls back to a
 * deterministic but non-cryptographic hash when SubtleCrypto is unavailable).
 */
export async function sha256Hex(input: string): Promise<string> {
  const data = new TextEncoder().encode(input)
  try {
    const hashBuffer = await crypto.subtle.digest('SHA-256', data)
    return Array.from(new Uint8Array(hashBuffer))
      .map((b) => b.toString(16).padStart(2, '0'))
      .join('')
  } catch {
    // Non-secure context fallback: FNV-1a 32-bit (deterministic, NOT cryptographic)
    let h = 2166136261
    for (const byte of data) {
      h ^= byte
      h = Math.imul(h, 16777619) >>> 0
    }
    // Pad to 64 chars to resemble a sha256 hex without collisions in test identity
    return h.toString(16).padStart(8, '0').repeat(8)
  }
}

// ---------------------------------------------------------------------------
// Text normalization
// ---------------------------------------------------------------------------

/**
 * Normalize a prompt string:
 * - Trim leading/trailing whitespace
 * - Collapse internal whitespace to single space
 * - NFC normalization for Unicode consistency
 */
export function normalizePrompt(text: string): string {
  return text
    .normalize('NFC')
    .trim()
    .replace(/\s+/g, ' ')
}

// ---------------------------------------------------------------------------
// File list digest
// ---------------------------------------------------------------------------

/**
 * Compute filesDigest from a list of file identifiers.
 * Sorts identifiers before hashing for stable output.
 * Returns a hex string.
 */
export async function buildFilesDigest(fileIdentifiers: string[]): Promise<string> {
  const sorted = [...fileIdentifiers].sort()
  return sha256Hex(sorted.join('\n'))
}
