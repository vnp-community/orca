# TASK-009: Tạo `ssh-remote-commands.ts`

**Source:** SOL-004  
**Phase:** 1 | **Effort:** M (1.5–3 giờ)  
**Depends on:** —

---

## Objective

Tạo module mới `src/main/ssh/ssh-remote-commands.ts` — cung cấp các utilities để thực thi commands trên remote server qua SSH connection:
- Execute arbitrary shell commands
- Detect remote platform (Ubuntu/Debian/CentOS/Alpine)
- Install Node.js 22
- Ensure Git is installed
- Clone or update git repos
- Install OS packages
- Run arbitrary shell scripts

---

## File to create

**`src/main/ssh/ssh-remote-commands.ts`** (NEW)

---

## Step 1: Understand existing SSH connection API

Trước khi implement, đọc `src/main/ssh/ssh-connection.ts` để hiểu:
- Interface/class `SshConnection`
- Method signature để execute remote command (likely `exec()` or `execCommand()`)
- How stdout/stderr/exitCode are returned

---

## Implementation

```typescript
// src/main/ssh/ssh-remote-commands.ts
import type { SshConnection } from './ssh-connection'

// ── Types ─────────────────────────────────────────────────────

export type RemoteCommandResult = {
  stdout: string
  stderr: string
  exitCode: number
}

export type RemotePlatform = {
  distro: 'ubuntu' | 'debian' | 'centos' | 'rhel' | 'fedora' | 'alpine' | 'unknown'
  arch: 'x64' | 'arm64' | 'unknown'
}

export type RepoCloneAction = 'cloned' | 'updated'

// ── Core exec ────────────────────────────────────────────────

const DEFAULT_TIMEOUT_MS = 30_000

/**
 * Execute a shell command on the remote server.
 * Throws if exitCode !== 0 and throwOnError = true (default).
 */
export async function execRemoteCommand(
  connection: SshConnection,
  command: string,
  options?: {
    timeout?: number
    throwOnError?: boolean
    sudo?: boolean
  }
): Promise<RemoteCommandResult> {
  const cmd = options?.sudo ? `sudo sh -c '${command.replace(/'/g, "'\\''")}'` : command
  const timeout = options?.timeout ?? DEFAULT_TIMEOUT_MS
  const throwOnError = options?.throwOnError ?? true

  // NOTE: Adapt this to the actual SshConnection API.
  // Common patterns:
  //   connection.exec(cmd, { timeout })
  //   connection.execCommand(cmd)
  //   connection.run(cmd)
  const result = await connection.exec(cmd, { timeout })

  if (throwOnError && result.exitCode !== 0) {
    throw new Error(
      `Remote command failed (exit ${result.exitCode}):\n` +
      `  cmd: ${cmd}\n` +
      `  stderr: ${result.stderr}`
    )
  }

  return result
}

// ── Platform detection ────────────────────────────────────────

let platformCache: WeakMap<SshConnection, RemotePlatform> = new WeakMap()

export async function detectRemotePlatform(connection: SshConnection): Promise<RemotePlatform> {
  if (platformCache.has(connection)) {
    return platformCache.get(connection)!
  }

  const distroResult = await execRemoteCommand(
    connection,
    `cat /etc/os-release 2>/dev/null | grep ^ID= | cut -d= -f2 | tr -d '"' | tr '[:upper:]' '[:lower:]'`,
    { throwOnError: false }
  )
  const archResult = await execRemoteCommand(
    connection,
    `uname -m`,
    { throwOnError: false }
  )

  const distroRaw = distroResult.stdout.trim()
  const archRaw = archResult.stdout.trim()

  const distroMap: Record<string, RemotePlatform['distro']> = {
    ubuntu: 'ubuntu', debian: 'debian', centos: 'centos',
    rhel: 'rhel', fedora: 'fedora', alpine: 'alpine',
  }

  const archMap: Record<string, RemotePlatform['arch']> = {
    'x86_64': 'x64', 'amd64': 'x64',
    'aarch64': 'arm64', 'arm64': 'arm64',
  }

  const platform: RemotePlatform = {
    distro: distroMap[distroRaw] ?? 'unknown',
    arch: archMap[archRaw] ?? 'unknown',
  }

  platformCache.set(connection, platform)
  return platform
}

// ── Node.js ───────────────────────────────────────────────────

/**
 * Install Node.js on the remote server.
 * Uses the appropriate method for the detected distro.
 */
export async function installNodeJs(
  connection: SshConnection,
  version: string = '22'
): Promise<void> {
  const platform = await detectRemotePlatform(connection)

  let installCmd: string
  switch (platform.distro) {
    case 'ubuntu':
    case 'debian':
      installCmd = [
        `curl -fsSL https://deb.nodesource.com/setup_${version}.x | sudo -E bash -`,
        `sudo apt-get install -y nodejs`,
      ].join(' && ')
      break
    case 'centos':
    case 'rhel':
      installCmd = [
        `curl -fsSL https://rpm.nodesource.com/setup_${version}.x | sudo bash -`,
        `sudo yum install -y nodejs`,
      ].join(' && ')
      break
    case 'fedora':
      installCmd = `sudo dnf install -y nodejs`
      break
    case 'alpine':
      installCmd = `sudo apk add --no-cache nodejs npm`
      break
    default:
      // Fallback: nvm
      installCmd = [
        `curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash`,
        `export NVM_DIR="$HOME/.nvm"`,
        `[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"`,
        `nvm install ${version} && nvm alias default ${version}`,
      ].join(' && ')
  }

  await execRemoteCommand(connection, installCmd, { timeout: 120_000 })
}

// ── Git ───────────────────────────────────────────────────────

/**
 * Ensure git is installed. Installs if missing.
 */
export async function ensureGitInstalled(connection: SshConnection): Promise<void> {
  const check = await execRemoteCommand(connection, 'git --version', { throwOnError: false })
  if (check.exitCode === 0) return  // Already installed

  const platform = await detectRemotePlatform(connection)
  let installCmd: string
  switch (platform.distro) {
    case 'ubuntu': case 'debian':
      installCmd = 'sudo apt-get update -qq && sudo apt-get install -y git'
      break
    case 'centos': case 'rhel':
      installCmd = 'sudo yum install -y git'
      break
    case 'fedora':
      installCmd = 'sudo dnf install -y git'
      break
    case 'alpine':
      installCmd = 'sudo apk add --no-cache git'
      break
    default:
      throw new Error(`Cannot auto-install Git on distro: ${platform.distro}. Please install manually.`)
  }
  await execRemoteCommand(connection, installCmd, { timeout: 60_000 })
}

// ── Repo management ────────────────────────────────────────────

/**
 * Clone a repo if not exists, or fetch/update if already cloned.
 * Idempotent: safe to run multiple times.
 */
export async function cloneOrUpdateRepo(
  connection: SshConnection,
  args: { url: string; path: string; branch?: string }
): Promise<RepoCloneAction> {
  // Check if path is already a git repo
  const checkResult = await execRemoteCommand(
    connection,
    `test -d "${args.path}/.git" && echo exists || echo missing`,
    { throwOnError: false }
  )

  if (checkResult.stdout.trim() === 'exists') {
    // Update: fetch all remotes
    await execRemoteCommand(
      connection,
      `git -C "${args.path}" fetch --all --prune` +
      (args.branch ? ` && git -C "${args.path}" checkout ${args.branch} && git -C "${args.path}" pull` : ''),
      { timeout: 120_000 }
    )
    return 'updated'
  } else {
    // Clone fresh
    const branchFlag = args.branch ? `--branch ${args.branch} ` : ''
    await execRemoteCommand(
      connection,
      `mkdir -p "${args.path}" && git clone ${branchFlag}"${args.url}" "${args.path}"`,
      { timeout: 300_000 }
    )
    return 'cloned'
  }
}

// ── Package installation ──────────────────────────────────────

/**
 * Install OS packages. Detects package manager automatically.
 */
export async function installPackages(
  connection: SshConnection,
  packages: string[]
): Promise<void> {
  if (!packages.length) return
  const platform = await detectRemotePlatform(connection)

  let cmd: string
  switch (platform.distro) {
    case 'ubuntu': case 'debian':
      cmd = `sudo apt-get update -qq && sudo apt-get install -y ${packages.join(' ')}`
      break
    case 'centos': case 'rhel':
      cmd = `sudo yum install -y ${packages.join(' ')}`
      break
    case 'fedora':
      cmd = `sudo dnf install -y ${packages.join(' ')}`
      break
    case 'alpine':
      cmd = `sudo apk add --no-cache ${packages.join(' ')}`
      break
    default:
      throw new Error(`Package manager unknown for distro: ${platform.distro}`)
  }

  await execRemoteCommand(connection, cmd, { timeout: 120_000 })
}

// ── Remote script ─────────────────────────────────────────────

/**
 * Run a multi-line shell script on the remote server.
 * Script is uploaded via heredoc and executed.
 */
export async function runRemoteScript(
  connection: SshConnection,
  script: string,
  cwd?: string
): Promise<RemoteCommandResult> {
  // Escape script for heredoc safety
  const escaped = script.replace(/\\/g, '\\\\')
  const cdPrefix = cwd ? `cd "${cwd}" && ` : ''
  const cmd = `${cdPrefix}bash -s <<'__ORCA_EOF__'\n${escaped}\n__ORCA_EOF__`
  return execRemoteCommand(connection, cmd, { timeout: 300_000 })
}
```

---

## Notes for AI

1. The `connection.exec()` call at line ~40 **must be adapted** to match the actual `SshConnection` API. Read `src/main/ssh/ssh-connection.ts` first.
2. If `SshConnection` uses `ssh2` library under the hood, the exec API may be: `connection.client.exec(cmd, callback)` — wrap in Promise.
3. `platformCache` uses `WeakMap` so entries are GC'd when connection closes.

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep ssh-remote-commands | head -20
```

---

## Done criteria

- [x] `detectRemotePlatform()` function exported
- [x] `installNodeJs()` function exported
- [x] `ensureGitInstalled()` function exported
- [x] `cloneOrUpdateRepo()` function exported
- [x] `installPackages()` function exported
- [x] `runRemoteScript()` function exported
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Created `src/main/ssh/fleet-remote-commands.ts` (separate from existing `ssh-remote-commands.ts`). Implements platform detection + caching (WeakMap), distro-aware install, idempotent clone/update. Builds on `execCommand()` from `ssh-relay-deploy-helpers.ts`.
