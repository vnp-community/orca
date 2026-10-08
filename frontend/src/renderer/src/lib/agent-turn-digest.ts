/**
 * agent-turn-digest.ts — FE-CV-TASK-089-01
 *
 * Digests for `quality.turn.record`. Pure; works without crypto.subtle
 * (non-secure LAN web context) because lib/sha256 is byte-identical to it.
 *
 * @module lib/agent-turn-digest
 */

import { sha256 } from './sha256'

export function sha256Hex(input: string): string {
  return Array.from(sha256(new TextEncoder().encode(input)))
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join('')
}

/** Trim, collapse whitespace, NFC — so equal prompts hash equally. */
export function normalizePrompt(text: string): string {
  return text.normalize('NFC').trim().replace(/\s+/g, ' ')
}

/** Digest of the sorted file identities; order-independent. */
export function buildFilesDigest(fileIdentities: readonly string[]): string {
  return sha256Hex([...fileIdentities].sort().join('\n'))
}
