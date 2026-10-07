import { SECRET_ENV_NAME_PATTERN } from './quality-child-env'

export function secretEnvValuesFrom(env: NodeJS.ProcessEnv): string[] {
  const secrets = new Set<string>()
  for (const [key, val] of Object.entries(env)) {
    if (val && val.length >= 8 && SECRET_ENV_NAME_PATTERN.test(key)) {
      secrets.add(val)
    }
  }
  return Array.from(secrets).sort((a, b) => b.length - a.length)
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function normalizePath(p: string): string {
  return p.replace(/\\/g, '/')
}

export function createRedactor(opts: { repoRoot: string; home: string; tmpRoot: string; secretEnvValues: string[] }): (text: string) => string {
  const normRepo = normalizePath(opts.repoRoot)
  const normHome = normalizePath(opts.home)
  const normTmp = normalizePath(opts.tmpRoot)

  // Sort paths by length descending to replace the longest first
  const pathReplacements: Array<{ rx: RegExp; repl: string }> = [
    { p: normRepo, repl: '<repo>' },
    { p: normHome, repl: '~' },
    { p: normTmp, repl: '<tmp>' }
  ]
    .filter(x => x.p)
    .sort((a, b) => b.p.length - a.p.length)
    .map(x => ({ rx: new RegExp(escapeRegex(x.p), 'g'), repl: x.repl }))

  const envSecrets = opts.secretEnvValues
    .filter(x => x.length >= 8)
    .sort((a, b) => b.length - a.length)
    .map(x => new RegExp(escapeRegex(x), 'g'))

  // Token regexes
  const tokenRegexes = [
    /gh[pousr]_[A-Za-z0-9]{20,}/g,
    /github_pat_[A-Za-z0-9_]{20,}/g,
    /AKIA[0-9A-Z]{16}/g,
    /sk-[A-Za-z0-9_-]{20,}/g,
    /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g,
    /-----BEGIN [A-Z ]*PRIVATE KEY-----[^-]+-----END [A-Z ]*PRIVATE KEY-----/g
  ]
  
  const uriRegex = /([a-z]+:\/\/[^:@/]+:)([^:@/]+)(@)/ig

  return function redact(text: string): string {
    if (!text) return text
    let res = normalizePath(text)

    // Redact URIs
    res = res.replace(uriRegex, '$1***$3')

    // Redact explicit patterns
    for (const rx of tokenRegexes) {
      res = res.replace(rx, '***')
    }

    // Redact env secrets
    for (const rx of envSecrets) {
      res = res.replace(rx, '***')
    }

    // Replace paths
    for (const { rx, repl } of pathReplacements) {
      res = res.replace(rx, repl)
    }

    return res
  }
}

export function redactTail(text: string, maxBytes: number, redactor: (text: string) => string): string {
  const buf = Buffer.from(text, 'utf8')
  if (buf.length <= maxBytes) {
    return redactor(text)
  }
  // Cut from the end, ensuring valid UTF-8 boundary
  let start = buf.length - maxBytes
  while (start < buf.length && (buf[start] & 0xC0) === 0x80) {
    start++
  }
  const tail = buf.toString('utf8', start)
  return redactor(tail)
}
