// src/main/ssh/fleet-remote-commands.ts
// Fleet-specific remote command execution: platform detection, Node.js/Git install,
// repo clone/update, package install, remote script execution.
// Builds on execCommand() from ssh-relay-deploy-helpers — same pattern as relay deploy.
import type { SshConnection } from './ssh-connection'
import { execCommand } from './ssh-relay-deploy-helpers'

// ── Types ──────────────────────────────────────────────────────

export type FleetRemotePlatform = {
  distro: 'ubuntu' | 'debian' | 'centos' | 'rhel' | 'fedora' | 'alpine' | 'unknown'
  arch: 'x64' | 'arm64' | 'unknown'
}

export type RepoCloneAction = 'cloned' | 'updated'

// ── Platform detection ─────────────────────────────────────────

// Cache per-connection to avoid repeated remote calls
const platformCache = new WeakMap<SshConnection, FleetRemotePlatform>()

/**
 * Detect the remote host's OS distro and CPU architecture.
 * Results are cached for the lifetime of the connection object.
 */
export async function detectRemotePlatform(
  conn: SshConnection
): Promise<FleetRemotePlatform> {
  const cached = platformCache.get(conn)
  if (cached) {return cached}

  let distroRaw = 'unknown'
  let archRaw = 'unknown'

  try {
    distroRaw = (
      await execCommand(
        conn,
        `cat /etc/os-release 2>/dev/null | grep ^ID= | cut -d= -f2 | tr -d '"' | tr '[:upper:]' '[:lower:]'`
      )
    ).trim()
  } catch {
    // Non-Linux host or no /etc/os-release — keep 'unknown'
  }

  try {
    archRaw = (await execCommand(conn, `uname -m`)).trim()
  } catch {
    // Keep 'unknown'
  }

  const distroMap: Record<string, FleetRemotePlatform['distro']> = {
    ubuntu: 'ubuntu',
    debian: 'debian',
    centos: 'centos',
    rhel: 'rhel',
    fedora: 'fedora',
    alpine: 'alpine',
  }

  const archMap: Record<string, FleetRemotePlatform['arch']> = {
    x86_64: 'x64',
    amd64: 'x64',
    aarch64: 'arm64',
    arm64: 'arm64',
  }

  const platform: FleetRemotePlatform = {
    distro: distroMap[distroRaw] ?? 'unknown',
    arch: archMap[archRaw] ?? 'unknown',
  }

  platformCache.set(conn, platform)
  return platform
}

// ── Node.js installation ───────────────────────────────────────

/**
 * Install Node.js on the remote server using the appropriate package manager.
 * Falls back to nvm for unknown distros.
 */
export async function installNodeJs(conn: SshConnection, version = '22'): Promise<void> {
  const platform = await detectRemotePlatform(conn)

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
      // Fallback: nvm — works without root on any distro
      installCmd = [
        `curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash`,
        `export NVM_DIR="$HOME/.nvm"`,
        `[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"`,
        `nvm install ${version} && nvm alias default ${version}`,
      ].join(' && ')
  }

  await execCommand(conn, installCmd, { timeoutMs: 120_000 })
}

// ── Git installation ───────────────────────────────────────────

/**
 * Ensure git is installed on the remote server.
 * Installs via OS package manager if the `git` binary is not found.
 */
export async function ensureGitInstalled(conn: SshConnection): Promise<void> {
  try {
    await execCommand(conn, 'git --version')
    return // Already installed
  } catch {
    // Not installed — proceed
  }

  const platform = await detectRemotePlatform(conn)
  let cmd: string
  switch (platform.distro) {
    case 'ubuntu':
    case 'debian':
      cmd = 'sudo apt-get update -qq && sudo apt-get install -y git'
      break
    case 'centos':
    case 'rhel':
      cmd = 'sudo yum install -y git'
      break
    case 'fedora':
      cmd = 'sudo dnf install -y git'
      break
    case 'alpine':
      cmd = 'sudo apk add --no-cache git'
      break
    default:
      throw new Error(
        `Cannot auto-install git on distro: ${platform.distro}. Please install manually.`
      )
  }

  await execCommand(conn, cmd, { timeoutMs: 60_000 })
}

// ── Repository management ──────────────────────────────────────

/**
 * Clone a git repo to a remote path, or fetch + checkout if already cloned.
 * Idempotent: safe to call multiple times.
 */
export async function cloneOrUpdateRepo(
  conn: SshConnection,
  args: { url: string; path: string; branch?: string }
): Promise<RepoCloneAction> {
  // Check if path is already a git repo
  let isExisting = false
  try {
    await execCommand(conn, `test -d "${args.path}/.git"`)
    isExisting = true
  } catch {
    isExisting = false
  }

  if (isExisting) {
    // Update: fetch all + optionally checkout branch
    const fetchCmd = `git -C "${args.path}" fetch --all --prune`
    const checkoutCmd = args.branch
      ? ` && git -C "${args.path}" checkout "${args.branch}" && git -C "${args.path}" pull --ff-only`
      : ''
    await execCommand(conn, fetchCmd + checkoutCmd, { timeoutMs: 120_000 })
    return 'updated'
  } else {
    // Clone fresh
    const branchFlag = args.branch ? `--branch "${args.branch}" ` : ''
    await execCommand(
      conn,
      `mkdir -p "${args.path}" && git clone ${branchFlag}"${args.url}" "${args.path}"`,
      { timeoutMs: 300_000 }
    )
    return 'cloned'
  }
}

// ── OS package installation ────────────────────────────────────

/**
 * Install a list of OS packages using the detected package manager.
 * No-op if the packages list is empty.
 */
export async function installPackages(
  conn: SshConnection,
  packages: string[]
): Promise<void> {
  if (!packages.length) {return}
  const platform = await detectRemotePlatform(conn)

  let cmd: string
  switch (platform.distro) {
    case 'ubuntu':
    case 'debian':
      cmd = `sudo apt-get update -qq && sudo apt-get install -y ${packages.join(' ')}`
      break
    case 'centos':
    case 'rhel':
      cmd = `sudo yum install -y ${packages.join(' ')}`
      break
    case 'fedora':
      cmd = `sudo dnf install -y ${packages.join(' ')}`
      break
    case 'alpine':
      cmd = `sudo apk add --no-cache ${packages.join(' ')}`
      break
    default:
      throw new Error(
        `Package install not supported on distro: ${platform.distro}. Install manually: ${packages.join(', ')}`
      )
  }

  await execCommand(conn, cmd, { timeoutMs: 120_000 })
}

// ── Remote shell script execution ──────────────────────────────

/**
 * Run a multi-line shell script on the remote host via bash.
 * The script content is transmitted via stdin, no temp file is written.
 * @param cwd - Optional working directory to cd into before running.
 */
export async function runRemoteScript(
  conn: SshConnection,
  script: string,
  cwd?: string
): Promise<string> {
  const cdPrefix = cwd ? `cd "${cwd}" && ` : ''
  // Escape single quotes in the script for inclusion in a shell single-quoted string
  const escapedScript = script.replace(/'/g, "'\\''")
  const cmd = `${cdPrefix}printf '%s' '${escapedScript}' | bash -s`
  return execCommand(conn, cmd, { timeoutMs: 300_000 })
}

// ── Resource metrics (health monitoring) ───────────────────────

export type RemoteResourceMetrics = {
  cpuPercent: number | null
  ramPercent: number | null
  diskPercent: number | null
}

// Why: one exec round trip covers CPU + RAM + disk instead of three —
// health checks run every 30s per server, minimizing SSH channel churn
// matters at fleet scale. The 1s `sleep` is CPU-sampling delta time
// (see readCpuPercentAndRam below), not per-metric overhead.
const RESOURCE_METRICS_SCRIPT = [
  `free -m | awk 'NR==2{printf "RAM=%.1f\\n", ($3/$2)*100}'`,
  `df -h . | awk 'NR==2{gsub("%","",$5); printf "DISK=%s\\n", $5}'`,
  'read cpu a b c d e f g h < /proc/stat',
  't1=$((a+b+c+d+e+f+g+h))',
  'i1=$d',
  'sleep 1',
  'read cpu a b c d e f g h < /proc/stat',
  't2=$((a+b+c+d+e+f+g+h))',
  'i2=$d',
  `awk -v t1="$t1" -v t2="$t2" -v i1="$i1" -v i2="$i2" 'BEGIN{dt=t2-t1; di=i2-i1; if (dt>0) printf "CPU=%.1f\\n", (1-di/dt)*100; else print "CPU=0"}'`
].join('; ')

/**
 * Best-effort CPU/RAM/disk usage snapshot from the remote host.
 * Never throws — any field that fails to parse comes back as `null` so a
 * flaky metrics probe can never break the health-check loop.
 */
export async function collectRemoteResourceMetrics(
  conn: SshConnection
): Promise<RemoteResourceMetrics> {
  try {
    const output = await execCommand(conn, RESOURCE_METRICS_SCRIPT, { timeoutMs: 5_000 })
    return parseResourceMetricsOutput(output)
  } catch {
    return { cpuPercent: null, ramPercent: null, diskPercent: null }
  }
}

export function parseResourceMetricsOutput(output: string): RemoteResourceMetrics {
  const ram = /RAM=([\d.]+)/.exec(output)
  const disk = /DISK=([\d.]+)/.exec(output)
  const cpu = /CPU=([\d.]+)/.exec(output)
  return {
    ramPercent: ram ? Number.parseFloat(ram[1]) : null,
    diskPercent: disk ? Number.parseFloat(disk[1]) : null,
    cpuPercent: cpu ? Number.parseFloat(cpu[1]) : null
  }
}

// ── Disk space check ────────────────────────────────────────────

export type DiskSpaceCheck = {
  availableGb: number
  ok: boolean
}

export const MIN_BOOTSTRAP_DISK_SPACE_GB = 5

/**
 * Check free disk space in the current remote working directory (typically
 * $HOME for a fresh SSH session) via `df -h .`, per CR-004's disk-check step.
 */
export async function checkRemoteDiskSpace(
  conn: SshConnection,
  minGb: number = MIN_BOOTSTRAP_DISK_SPACE_GB
): Promise<DiskSpaceCheck> {
  const output = await execCommand(conn, 'df -h .')
  const dataLine = output.trim().split('\n')[1] ?? ''
  const columns = dataLine.trim().split(/\s+/)
  // df -h columns: Filesystem Size Used Avail Use% Mounted-on
  const availableRaw = columns[3] ?? '0'
  const availableGb = parseDfSizeToGb(availableRaw)
  return { availableGb, ok: availableGb >= minGb }
}

/** Parses a `df -h`-style size string ("47G", "512M", "1.2T", "900K") into GB. */
export function parseDfSizeToGb(raw: string): number {
  const match = /^([\d.]+)\s*([KMGT]?)/i.exec(raw.trim())
  if (!match) {return 0}
  const value = Number.parseFloat(match[1])
  if (Number.isNaN(value)) {return 0}
  switch (match[2].toUpperCase()) {
    case 'T':
      return value * 1024
    case 'G':
      return value
    case 'M':
      return value / 1024
    case 'K':
      return value / (1024 * 1024)
    default:
      // No suffix — df reported raw bytes (rare with -h, but be safe).
      return value / (1024 * 1024 * 1024)
  }
}
