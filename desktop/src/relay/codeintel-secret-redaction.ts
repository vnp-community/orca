export function redactForClient(text: string, opts: { home?: string }): string {
  let redacted = text

  if (opts.home) {
    const homeRegex = new RegExp(opts.home.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g')
    redacted = redacted.replace(homeRegex, '~')
  }

  const rules = [
    /gh[pousr]_[A-Za-z0-9]+/g,
    /github_pat_[A-Za-z0-9_]+/g,
    /AKIA[0-9A-Z]{16}/g,
    /sk-[A-Za-z0-9-]+/g,
    /eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+/g,
    /-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]*?-----END [A-Z ]+ PRIVATE KEY-----/g,
    /(\w+:\/\/)[^:\s]+:[^@\s]+@/g
  ]

  for (let i = 0; i < rules.length; i++) {
    if (i === rules.length - 1) {
      redacted = redacted.replace(rules[i], '$1<REDACTED>@')
    } else {
      redacted = redacted.replace(rules[i], '<REDACTED>')
    }
  }

  return redacted
}

export function tailForStderr(text: string, opts?: { home?: string }): string {
  const noAnsi = text.replace(/\u001b\[[0-9;]*m/g, '')
  let redacted = redactForClient(noAnsi, opts ?? {})
  
  // Redact available repositories list from tool stderr to avoid cross-repo info disclosure
  redacted = redacted.replace(/Available(?:\s+repositories)?:\s*([^\n\r]+)/gi, 'Available: <REDACTED>')

  const maxLength = 2048
  if (redacted.length > maxLength) {
    return '...' + redacted.slice(-maxLength)
  }
  return redacted
}
