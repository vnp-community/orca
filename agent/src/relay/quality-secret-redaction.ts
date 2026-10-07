import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'

const FORBIDDEN_SECRET_KEYS = /^(secret|match|rawsecret|raw_secret|fingerprint)$/i

/**
 * Asserts that a record from an external security tool does not contain
 * unredacted raw secret values.
 * Throws an error if an unredacted secret is found.
 */
export function assertNoSecretFields(record: Record<string, any>): void {
  if (!record || typeof record !== 'object') return

  for (const [key, value] of Object.entries(record)) {
    if (FORBIDDEN_SECRET_KEYS.test(key)) {
      if (typeof value === 'string' && value.length > 0) {
        const trimmed = value.trim()
        if (trimmed !== 'REDACTED' && trimmed !== '***' && trimmed !== '') {
          throw new Error(`Unredacted secret field detected: ${key}`)
        }
      }
    }
    // Check nested objects if applicable
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      assertNoSecretFields(value)
    }
  }
}

/**
 * Test helper: dynamically constructs a unique canary secret at runtime.
 * Never stores or hardcodes secret patterns in code.
 */
export function makeCanary(): string {
  // Construct AWS-like or token-like canary dynamically
  const prefix = ['A', 'K', 'I', 'A'].join('')
  const randomSuffix = crypto.randomBytes(8).toString('hex').toUpperCase()
  return prefix + randomSuffix
}

/**
 * Recursively scans directories to detect if a canary string leaked into any file.
 * Returns array of file paths where canary was detected.
 */
export function scanTreeForCanary(dirs: readonly string[], canary: string): string[] {
  const leakedFiles: string[] = []
  if (!canary) return leakedFiles

  function walk(currentPath: string) {
    if (!fs.existsSync(currentPath)) return
    const stat = fs.statSync(currentPath)
    if (stat.isDirectory()) {
      const entries = fs.readdirSync(currentPath)
      for (const entry of entries) {
        walk(path.join(currentPath, entry))
      }
    } else if (stat.isFile()) {
      try {
        const content = fs.readFileSync(currentPath, 'utf8')
        if (content.includes(canary)) {
          leakedFiles.push(currentPath)
        }
      } catch {
        // Skip unreadable files
      }
    }
  }

  for (const dir of dirs) {
    walk(dir)
  }

  return leakedFiles
}
