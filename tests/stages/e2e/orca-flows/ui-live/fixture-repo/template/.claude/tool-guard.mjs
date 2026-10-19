// PreToolUse hook of the e2e fixture repo. The agent runs `claude --dangerously-skip-permissions` on the dev machine,
// so this is the only brake: it refuses file access outside the fixture's own directory and obvious escapes in Bash.
// Best effort, not a sandbox; the spec also compares the real repos before and after the run.
import { appendFileSync, existsSync, readFileSync, realpathSync } from 'node:fs'
import { dirname, isAbsolute, join, resolve, sep } from 'node:path'

const here = import.meta.dirname
const config = JSON.parse(readFileSync(join(here, 'tool-guard.config.json'), 'utf8'))
const root = realpathSync(config.allowedRoot)

function realOrParent(p) {
  let cur = resolve(p)
  const tail = []
  while (!existsSync(cur) && dirname(cur) !== cur) {
    tail.unshift(cur.slice(dirname(cur).length + 1))
    cur = dirname(cur)
  }
  return join(realpathSync(cur), ...tail)
}

function inside(p) {
  const real = realOrParent(isAbsolute(p) ? p : resolve(process.cwd(), p))
  return real === root || real.startsWith(root + sep)
}

function record(file, line) {
  try {
    appendFileSync(join(root, file), `${new Date().toISOString()} ${line}\n`)
  } catch {
    // the verdict below does not depend on the log
  }
}

function deny(reason, input) {
  record('tool-guard-denials.log', `${input.tool_name} ${reason}`)
  process.stderr.write(`tool-guard: ${reason}\n`)
  process.exit(2)
}

// Absolute tokens that may appear in a command: the fixture itself and system binaries/devices.
const SYSTEM_PREFIXES = [
  '/usr/bin',
  '/bin',
  '/usr/local/bin',
  '/opt/homebrew',
  '/dev/null',
  '/dev/stdout',
  '/dev/stderr'
]
const ABS_ROOTS =
  /^\/(Users|home|tmp|var|private|etc|opt|usr|root|Library|System|Volumes|mnt|proc|dev|bin|sbin|srv|boot|snap|Applications)(\/|$)/
const FORBIDDEN_WORDS =
  /\b(sudo|curl|wget|ssh|scp|rsync|ncat|docker|kubectl|chmod|chown|kill|pkill|killall|launchctl|osascript)\b/
const FORBIDDEN_GIT = /\bgit\s+(-C\s+\S+\s+)?(push|remote|config|clone|fetch|pull|credential)\b/

function checkBash(command, input) {
  // Heredoc bodies are file content (JS with '../src/x.mjs' imports, comments, regexes), not commands: check the rest.
  const code = command.replace(/<<-?\s*(['"]?)(\w+)\1[^\n]*\n[\s\S]*?\n\s*\2(?=\s|$)/g, '')
  if (FORBIDDEN_WORDS.test(code)) {
    deny(`command uses a forbidden program: ${command.slice(0, 120)}`, input)
  }
  if (FORBIDDEN_GIT.test(code)) {
    deny(`command uses a forbidden git subcommand: ${command.slice(0, 120)}`, input)
  }
  if (/(^|[\s'"=:])(~|\$HOME|\$\{HOME\})/.test(code)) {
    deny(`command reaches for the home directory: ${command.slice(0, 120)}`, input)
  }
  // A path with ".." is fine while it stays inside the fixture (a worktree's ../repo, a test's ../src), so resolve it first.
  const cwd = input.cwd || process.cwd()
  for (const m of code.matchAll(/[^\s'"`;|&<>()=]*(?:^|\/)\.\.(?:\/[^\s'"`;|&<>()]*)?/g)) {
    if (!inside(resolve(cwd, m[0]))) {
      deny(`command climbs out of the fixture with "..": ${m[0]}`, input)
    }
  }
  // A path token starts at a boundary and at a real filesystem root, so "src/calc.mjs", a "/** doc */" comment or a JS regex
  // literal inside the command is not read as an absolute path.
  for (const m of code.matchAll(/(?:^|[\s'"=(:])(\/[^\s'"`;|&<>()]+)/g)) {
    const token = m[1]
    if (
      !ABS_ROOTS.test(token) ||
      SYSTEM_PREFIXES.some((p) => token === p || token.startsWith(`${p}/`))
    ) {
      continue
    }
    if (!inside(token)) {
      deny(`command names a path outside the fixture: ${token}`, input)
    }
  }
}

const input = JSON.parse(readFileSync(0, 'utf8'))
const t = input.tool_input ?? {}
// Every call is logged so the spec can prove the hook was live when the agent ran.
record('tool-guard-calls.log', input.tool_name)
switch (input.tool_name) {
  case 'Write':
  case 'Edit':
  case 'MultiEdit':
  case 'NotebookEdit':
  case 'Read': {
    const target = t.file_path ?? t.notebook_path
    if (target && !inside(target)) {
      deny(`${input.tool_name} outside the fixture: ${target}`, input)
    }
    break
  }
  case 'Glob':
  case 'Grep': {
    if (t.path && !inside(t.path)) {
      deny(`${input.tool_name} outside the fixture: ${t.path}`, input)
    }
    break
  }
  case 'Bash':
    checkBash(String(t.command ?? ''), input)
    break
  case 'WebFetch':
  case 'WebSearch':
    deny('network tools are not allowed in the e2e fixture', input)
    break
  default:
    // TodoWrite, Task and the like touch no files; anything unknown that names a path is checked above only for known tools.
    if (typeof t.file_path === 'string' || typeof t.path === 'string') {
      const p = t.file_path ?? t.path
      if (!inside(p)) {
        deny(`${input.tool_name} outside the fixture: ${p}`, input)
      }
    }
}
