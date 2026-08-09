/**
 * Compute predicted linux username from Orca user email.
 * Mirrors backend toLinuxUsername() in src/main/ssh/ssh-user-resolver.ts
 *
 * Examples:
 *   "alice@company.com"      → "orca-alice"
 *   "alice.smith@co.com"     → "orca-alice-smith"
 *   "alice+filter@co.com"    → "orca-alice-filter"
 *   "verylongemailname@x.co" → "orca-verylongemailnam" (truncated at 20)
 */
export function toLinuxUsername(email: string): string {
  const local = email.split('@')[0]
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '-')
    .slice(0, 20)
  const sanitized = local.replace(/^-+|-+$/g, '') || 'user'
  return `orca-${sanitized}`
}
