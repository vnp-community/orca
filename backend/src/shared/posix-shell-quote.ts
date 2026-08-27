// Quotes a single value for safe inclusion in a POSIX shell command line.
//
// Why this exists: gh/glab auth-login PTYs deliver their command by typing
// it into a freshly-spawned shell as literal keystrokes (see
// dev-server-pty-provider.ts / ssh-pty-provider.ts's commandDelivery:
// 'provider', and the agent-side pty.create/pty.spawn handlers that submit
// it) — not via execFile/spawn's argv array. A caller-influenced value
// (e.g. a GitLab self-hosted `--hostname`) that ends up in that command line
// unescaped is a real shell-injection risk, not just a correctness bug. See
// specs/agent/api/gaps-and-findings.md #5.
export function posixShellQuote(value: string): string {
  return `'${value.replace(/'/g, "'\\''")}'`
}

/** Joins already-trusted-literal tokens (e.g. 'gh', 'auth', 'login') with
 *  caller-influenced ones, quoting only what needs it. Pass every token
 *  through here rather than hand-building a template string — quoting a
 *  literal like 'auth' is harmless and keeps this call site the single
 *  place that decides what's safe to leave bare. */
export function buildPosixShellCommand(tokens: string[]): string {
  return tokens.map((t) => posixShellQuote(t)).join(' ')
}
