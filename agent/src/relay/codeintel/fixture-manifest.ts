import { z } from 'zod'
import fs from 'fs'
import path from 'path'

export const FIXTURE_BUDGET = {
  fileMaxBytes: 20 * 1024,
  dirMaxBytes: 300 * 1024
}

export const FixtureManifestSchema = z.object({
  version: z.string(),
  tool: z.enum(['gitnexus', 'codegraph']),
  createdAt: z.string(),
  indexedCommit: z.string(),
  files: z.record(z.object({
    sha256: z.string(),
    bytes: z.number()
  }))
})

export type FixtureManifest = z.infer<typeof FixtureManifestSchema>

export interface LeakFinding {
  rule: string
  line: number
  match: string
}

export function scanTextForLeaks(text: string): LeakFinding[] {
  const lines = text.split('\n')
  const findings: LeakFinding[] = []
  
  const rules = [
    { name: 'absolute_home', regex: /\/(home|Users)\/[A-Za-z0-9_-]+/ },
    { name: 'absolute_opt', regex: /\/opt\/[A-Za-z0-9_-]+/ },
    { name: 'absolute_windows', regex: /[C-Z]:\\[A-Za-z0-9_-]+/i },
    { name: 'forbidden_keys', regex: /"?(fileHashes|cacheKeys)"?\s*:/ },
    { name: 'github_token', regex: /(gh[pousr]_[A-Za-z0-9_]{36}|github_pat_[A-Za-z0-9_]{82})/ },
    { name: 'aws_key', regex: /AKIA[0-9A-Z]{16}/ },
    { name: 'secret_key', regex: /sk-[a-zA-Z0-9]{20,}/ },
    { name: 'basic_auth', regex: /[a-zA-Z][a-zA-Z0-9+.-]*:\/\/[^\s:@]+:[^\s:@]+@[^\s\/]+/ },
    { name: 'pem_key', regex: /-----BEGIN (RSA )?PRIVATE KEY-----/ }
  ]

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    for (const rule of rules) {
      const match = line.match(rule.regex)
      if (match) {
        findings.push({ rule: rule.name, line: i + 1, match: match[0] })
      }
    }
  }

  return findings
}

export function loadFixtureManifest(manifestPath: string): FixtureManifest {
  let content: string
  try {
    content = fs.readFileSync(manifestPath, 'utf8')
  } catch (e: any) {
    throw new Error(`Cannot read manifest at ${manifestPath}: ${e.message}`)
  }
  
  let json: any
  try {
    json = JSON.parse(content)
  } catch (e: any) {
    throw new Error(`Invalid JSON in manifest ${manifestPath}: ${e.message}`)
  }
  
  const result = FixtureManifestSchema.safeParse(json)
  if (!result.success) {
    throw new Error(`Invalid schema in manifest ${manifestPath}: ${result.error.message}`)
  }
  
  return result.data
}

export function listFixtureVersionDirs(fixturesRoot: string): string[] {
  if (!fs.existsSync(fixturesRoot)) return []
  return fs.readdirSync(fixturesRoot, { withFileTypes: true })
    .filter(dirent => dirent.isDirectory() && dirent.name !== 'mini-repo')
    .map(dirent => dirent.name)
}
