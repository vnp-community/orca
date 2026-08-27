// src/relay/hosted-cli-api-allowlist.ts
// Validation for the generic `github.exec`/`gitlab.exec` RPCs — the ADR-018
// migration that moves gh/glab execution out of backend/ (which must never
// execute dev-server work) into agent/. See specs/agent/api/gaps-and-findings.md.
//
// `git.exec`'s subcommand whitelist (agent-git-exec-validator.ts) doesn't
// translate directly here: `gh api`/`glab api` is a raw REST/GraphQL
// passthrough that can reach ANY endpoint on behalf of the authenticated
// user — org-admin operations, other users' data, arbitrary hosts — not
// just the target repo. A subcommand-level whitelist alone would let a
// compromised caller reach far more than the ~132 real backend call sites
// ever use. So `api`/`graphql` gets an extra layer: the endpoint path must
// match one of a small set of PATH CLASSES (repo-scoped, the one known
// user-starred toggle, or a short literal list) rather than being
// enumerated call-by-call — enumerating every exact endpoint string by hand
// would be both huge and brittle (one missed variant breaks a real
// caller). This is deliberately a class-level allowlist, not a full
// per-endpoint one — see isAllowedGhApiPath/isAllowedGlabApiPath.

// No '&' here (unlike git-exec-validator.ts's set) — it's a legitimate,
// necessary character in `gh api`/`glab api` query strings
// (repos/.../pulls?head=main&state=all&per_page=1, a real caller in
// client.ts). Safe to exclude: these argv elements reach execFile/spawn in
// array mode (shell: false), so no shell ever interprets them — this
// check is defense-in-depth against a future refactor, not the actual
// injection boundary.
const SHELL_METACHARACTERS = /[|;$`<>\\!]/
const NUL_BYTE = /\0/

function assertNoInjectionChars(args: string[], cli: 'gh' | 'glab'): void {
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg) || NUL_BYTE.test(arg)) {
      throw new Error(`${cli}.exec: argument contains a disallowed character: ${JSON.stringify(arg)}`)
    }
  }
}

const ALLOWED_HTTP_METHODS = new Set(['GET', 'POST', 'PATCH', 'PUT', 'DELETE'])

// ── gh ───────────────────────────────────────────────────────────────────

const ALLOWED_GH_SUBCOMMANDS = new Set(['pr', 'issue', 'repo', 'user', 'auth', 'api'])

// Flags that consume the following argv element as their value (must be
// skipped as a pair when scanning for the endpoint path/method).
const GH_API_VALUE_FLAGS = new Set(['-X', '--method', '--cache', '-f', '-F', '--jq', '--template'])
const GH_ALLOWED_LITERAL_PATHS = new Set(['rate_limit', 'graphql', 'user'])
// repos/{owner}/{repo}[/...] — the dominant real shape (issues, pulls,
// check-runs, actions/jobs/.../logs, labels, comments, contents, etc.)
const GH_REPO_SCOPED_PATH = /^repos\/[^/?]+\/[^/?]+(\/.*)?$/
// user/starred/{owner}/{repo} — the one non-repo-prefixed real caller
// (client.ts's "star the orca repo" toggle).
const GH_USER_STARRED_PATH = /^user\/starred\/[^/?]+(\/[^/?]+)?$/

function isAllowedGhApiPath(path: string): boolean {
  const withoutQuery = path.split('?')[0] ?? path
  return (
    GH_ALLOWED_LITERAL_PATHS.has(withoutQuery) ||
    GH_REPO_SCOPED_PATH.test(withoutQuery) ||
    GH_USER_STARRED_PATH.test(withoutQuery)
  )
}

function extractApiPathAndMethod(
  args: string[]
): { path: string | null; method: string } {
  let method = 'GET'
  let path: string | null = null
  for (let i = 0; i < args.length; i++) {
    const a = args[i]!
    if (a === '-X' || a === '--method') {
      method = (args[i + 1] ?? 'GET').toUpperCase()
      i++
      continue
    }
    if (GH_API_VALUE_FLAGS.has(a)) {
      i++
      continue
    }
    if (a.startsWith('-')) {
      continue
    }
    if (path === null) {
      path = a
    }
  }
  return { path, method }
}

/**
 * Validates a full `gh <args>` argv (args[0] is the subcommand) before it
 * reaches execGhCaptured(). Throws on anything not in the allowlist.
 */
export function assertAllowedGhArgs(args: string[]): void {
  assertNoInjectionChars(args, 'gh')
  const subcommand = args[0]
  if (!subcommand || !ALLOWED_GH_SUBCOMMANDS.has(subcommand)) {
    throw new Error(`github.exec: subcommand "${subcommand ?? ''}" is not allowed`)
  }
  if (subcommand !== 'api') {
    return
  }
  const rest = args.slice(1)
  const { path, method } = extractApiPathAndMethod(rest)
  if (!path) {
    throw new Error('github.exec: "gh api" call has no endpoint path')
  }
  if (!ALLOWED_HTTP_METHODS.has(method)) {
    throw new Error(`github.exec: HTTP method "${method}" is not allowed`)
  }
  if (!isAllowedGhApiPath(path)) {
    throw new Error(`github.exec: endpoint path "${path}" is not in the allowlist`)
  }
}

// ── glab ─────────────────────────────────────────────────────────────────

const ALLOWED_GLAB_SUBCOMMANDS = new Set(['mr', 'issue', 'user', 'auth', 'api'])

const GLAB_API_VALUE_FLAGS = new Set(['-X', '--method', '--hostname'])
const GLAB_ALLOWED_LITERAL_PATHS = new Set(['user'])
// projects/{id-or-url-encoded-path}[/...] — the dominant real shape
// (merge_requests, issues, labels, etc.).
const GLAB_PROJECT_SCOPED_PATH = /^projects\/[^/?]+(\/.*)?$/

function isAllowedGlabApiPath(path: string): boolean {
  const withoutQuery = path.split('?')[0] ?? path
  return GLAB_ALLOWED_LITERAL_PATHS.has(withoutQuery) || GLAB_PROJECT_SCOPED_PATH.test(withoutQuery)
}

function extractGlabApiPathAndMethod(args: string[]): { path: string | null; method: string } {
  let method = 'GET'
  let path: string | null = null
  for (let i = 0; i < args.length; i++) {
    const a = args[i]!
    if (a === '-X' || a === '--method') {
      method = (args[i + 1] ?? 'GET').toUpperCase()
      i++
      continue
    }
    if (GLAB_API_VALUE_FLAGS.has(a)) {
      i++
      continue
    }
    if (a.startsWith('-')) {
      continue
    }
    if (path === null) {
      path = a
    }
  }
  return { path, method }
}

/**
 * Validates a full `glab <args>` argv before it reaches execGlabCaptured().
 * Mirrors assertAllowedGhArgs — see its comments for the path-class rationale.
 */
export function assertAllowedGlabArgs(args: string[]): void {
  assertNoInjectionChars(args, 'glab')
  const subcommand = args[0]
  if (!subcommand || !ALLOWED_GLAB_SUBCOMMANDS.has(subcommand)) {
    throw new Error(`gitlab.exec: subcommand "${subcommand ?? ''}" is not allowed`)
  }
  if (subcommand !== 'api') {
    return
  }
  const rest = args.slice(1)
  const { path, method } = extractGlabApiPathAndMethod(rest)
  if (!path) {
    throw new Error('gitlab.exec: "glab api" call has no endpoint path')
  }
  if (!ALLOWED_HTTP_METHODS.has(method)) {
    throw new Error(`gitlab.exec: HTTP method "${method}" is not allowed`)
  }
  if (!isAllowedGlabApiPath(path)) {
    throw new Error(`gitlab.exec: endpoint path "${path}" is not in the allowlist`)
  }
}
