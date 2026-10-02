import type { McpRisk } from '../../../../shared/mcp-types'

// Lock before Approve becomes clickable; blocks reflex Enter/Space while typing elsewhere.
export const APPROVE_LOCK_MS: Record<McpRisk, number> = {
  read: 0,
  write_reversible: 600,
  exec: 1500,
  destructive: 2000,
  admin: 2000
}

// C0 controls except \n \t, DEL, zero-width, bidi overrides/isolates, word-joiners, BOM.
const INVISIBLE =
  // eslint-disable-next-line no-control-regex
  /[\u0000-\u0008\u000B-\u001F\u007F​-‏‪-‮⁠-⁤⁦-⁩﻿]/g

/** Display-only: makes invisible/reordering characters visible. Never alters what is sent. */
export function visualizeControlChars(text: string): string {
  return text.replace(INVISIBLE, (ch) => {
    const hex = (ch.codePointAt(0) ?? 0).toString(16).toUpperCase().padStart(4, '0')
    return `\\u{${hex}}`
  })
}

export function formatCountdown(remainingMs: number): string {
  const total = Math.max(0, Math.ceil(remainingMs / 1000))
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${m}:${String(s).padStart(2, '0')}`
}
