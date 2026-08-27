/**
 * Extra `git.exec` argument validation for Part A (agent-git-handler.ts),
 * layered on top of that file's existing subcommand allowlist + blanket
 * shell-metacharacter check.
 *
 * Why this exists / why it differs from git-exec-validator.ts (Part B,
 * the SSH relay's git.exec): Part B's git.exec is a narrow, mostly-read-only
 * escape hatch — Part B has separate dedicated RPC methods (git.push,
 * git.stage, git.commit, git.fetch, ...) for real mutations, so its
 * validator can restrict subcommands to fixed argument shapes. Part A has
 * no dedicated mutation RPCs — git.exec IS its git API surface — so
 * subcommands like push/pull/fetch/merge/rebase/stash/checkout/add/
 * restore/commit must keep accepting real, variable arguments (branch
 * names, remotes, commit messages, paths). Porting Part B's fixed-shape
 * validators wholesale isn't possible without first porting its ~20
 * dedicated RPC methods — deferred as its own, larger, higher-risk
 * project (see specs/agent/api/gaps-and-findings.md #4).
 *
 * What THIS module closes instead is the class of git-native injection/
 * RCE footguns that don't depend on subcommand shape and have no
 * legitimate use in this codebase:
 *   - `-c key=value` (or any other `-`-prefixed flag) before the
 *     subcommand — lets a caller override arbitrary git config, e.g.
 *     `-c core.sshCommand=...` to run an arbitrary command on the next
 *     network operation, or `--git-dir=`/`--work-tree=`/`--exec-path=`
 *     to escape the intended repository/helper directory entirely.
 *   - `--upload-pack=`/`--receive-pack=`/`--exec=` — git's documented
 *     local-command-execution footgun for fetch/pull/push/archive/clone
 *     (these specify an arbitrary command git runs as the remote-side
 *     helper for local/ext transports).
 *   - `-o`/`--output` — writes command output to an arbitrary
 *     caller-chosen path instead of stdout.
 *   - Unrestricted `git config` writes — path traversal via `--file`
 *     (reads an arbitrary file's contents back as "config"), or planting
 *     a `core.hooksPath`/`core.sshCommand` that runs on the next git
 *     operation.
 *
 * Deliberately NOT blocked: `--git-dir`/`--work-tree`/`--exec-path` when
 * they appear AFTER the subcommand (e.g. `git rev-parse --git-dir`, a
 * real, benign, currently-used call from
 * backend/src/main/providers/dev-server-git-provider.ts — there they're
 * just a query flag `rev-parse` answers, not a redirect). Only their
 * dangerous form — before the subcommand, as a genuine global override —
 * is blocked, by the "no flags before the subcommand" rule above.
 */

// Why: git accepts --flag=value compound syntax (e.g. --upload-pack=evil),
// which bypasses exact-match Set.has() checks. Mirrors git-exec-validator.ts's
// matchesDeniedFlag — duplicated (not imported) since the two validators
// enforce genuinely different policies and this repo's git.exec files are
// each meant to stay self-contained (see agent-git-handler.ts's own header).
function matchesDeniedFlag(arg: string, denySet: Set<string>): boolean {
  if (denySet.has(arg)) {
    return true
  }
  const eqIdx = arg.indexOf('=')
  if (eqIdx > 0) {
    return denySet.has(arg.slice(0, eqIdx))
  }
  return false
}

// Why safe to deny unconditionally (anywhere in args, not just pre-subcommand):
// unlike --git-dir/--work-tree/--exec-path, these have no benign
// after-the-subcommand meaning for any subcommand Part A allows, and no
// caller in this codebase uses them.
const DANGEROUS_ANYWHERE_FLAGS = new Set([
  '--upload-pack',
  '--receive-pack',
  '--exec',
  '-o',
  '--output'
])

const CONFIG_READ_ONLY_FLAGS = new Set(['--get', '--get-all', '--list', '--get-regexp', '-l'])
// Why: checking presence of a read-only flag is insufficient — a request
// could include both --list (passes the check) and --add (performs a
// write). Reject known write operations explicitly.
const CONFIG_WRITE_FLAGS = new Set([
  '--add',
  '--unset',
  '--unset-all',
  '--replace-all',
  '--rename-section',
  '--remove-section',
  '--edit',
  '-e',
  // Why: --file redirects config reads to an arbitrary file, enabling path
  // traversal (e.g. `--file /etc/passwd --list` leaks file contents).
  '--file',
  '-f',
  '--global',
  '--system'
])

export function assertNoGitInjectionFlags(args: string[]): void {
  // Why: git accepts `-c key=value` (and other global options) before the
  // subcommand, which can override config and execute arbitrary commands
  // (e.g. core.sshCommand). Reject any argument before the subcommand that
  // looks like a flag — this is the primary guard; nothing in this codebase
  // legitimately needs a global flag before the subcommand.
  let subcommandIdx = 0
  while (subcommandIdx < args.length && args[subcommandIdx].startsWith('-')) {
    subcommandIdx++
  }
  if (subcommandIdx > 0) {
    throw new Error('Global git flags before the subcommand are not allowed')
  }

  const subcommand = args[0]
  const restArgs = args.slice(1)

  if (restArgs.some((a) => matchesDeniedFlag(a, DANGEROUS_ANYWHERE_FLAGS))) {
    throw new Error('Dangerous git flags are not allowed via git.exec')
  }

  if (subcommand === 'config') {
    if (!restArgs.some((a) => CONFIG_READ_ONLY_FLAGS.has(a))) {
      throw new Error('git config is restricted to read-only operations (--get, --list, etc.) via git.exec')
    }
    if (restArgs.some((a) => matchesDeniedFlag(a, CONFIG_WRITE_FLAGS))) {
      throw new Error('git config write operations are not allowed via git.exec')
    }
  }
}
