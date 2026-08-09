/**
 * SSH User Resolver — Map userId+email to per-user Linux username
 *
 * Converts an Orca user identity → stable, safe Linux username for SSH.
 * Format: orca-{sanitized_local_part}[-{4char_hash_suffix}]
 *
 * Examples:
 *   alice@company.com           → orca-alice
 *   alice.smith@a.com           → orca-alice-smith
 *   alice.smith@a.com (uid-1)   → orca-alice-sm-a1b2  (with suffix)
 *
 * @module main/ssh/ssh-user-resolver
 */

import { createHash } from 'node:crypto'
import type { SshTarget } from '../../shared/ssh-types'

const USERNAME_PREFIX  = 'orca-'
const MAX_LOCAL_LENGTH = 20    // orca-(5) + 20 = 25 chars (well within Linux 32 char limit)
const SUFFIX_LENGTH    = 4     // 4-char hex suffix for collision disambiguation
const VALID_LINUX_USER = /^[a-z][a-z0-9-]{0,31}$/

/**
 * Convert email + optional userId → stable, safe Linux username.
 *
 * Without userId:  simple sanitization (for display/lookup)
 * With userId:     appends 4-char hash suffix to disambiguate similar emails
 */
export function toLinuxUsername(email: string, userId?: string): string {
  const localPart = email.split('@')[0]!
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '-')   // non-alphanumeric → hyphen
    .replace(/-+/g, '-')           // collapse consecutive hyphens
    .replace(/^-|-$/g, '')         // trim leading/trailing hyphens

  if (!userId) {
    return `${USERNAME_PREFIX}${localPart.slice(0, MAX_LOCAL_LENGTH)}`
  }

  // Deterministic 4-char hex suffix from sha256(email+userId)
  const suffix    = createHash('sha256').update(email + userId).digest('hex').slice(0, SUFFIX_LENGTH)
  const truncated = localPart.slice(0, MAX_LOCAL_LENGTH - SUFFIX_LENGTH - 1)  // -1 for the hyphen
  return `${USERNAME_PREFIX}${truncated}-${suffix}`
}

/**
 * Validate a Linux username.
 * Rules: starts with lowercase letter, only [a-z0-9-], max 32 chars.
 */
export function isValidLinuxUsername(username: string): boolean {
  return VALID_LINUX_USER.test(username) && username.length <= 32
}

/**
 * Create a per-user SshTarget by overriding the username.
 * Instead of a shared 'ubuntu' account, uses 'orca-alice' (per-user).
 * Does NOT mutate the original target.
 */
export function resolveUserSshTarget(
  baseTarget: SshTarget,
  userId:     string,
  userEmail:  string
): SshTarget {
  return {
    ...baseTarget,
    username: toLinuxUsername(userEmail, userId)
  }
}
