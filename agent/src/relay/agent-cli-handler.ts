// src/relay/agent-cli-handler.ts
// Part A (agent.js WebSocket connection — direct-websocket / relay-websocket modes,
// see deploy/agent/README.md) implementation of cli.getInstallStatus / cli.install /
// cli.remove / cli.getWslInstallStatus / cli.installWsl / cli.removeWsl.
//
// Why this file exists: registering the `orca` shell command only makes sense on the
// machine that actually hosts a user's terminals. In server/web mode that machine is
// the connected Dev Server, not the Orca backend container — so backend relays these
// calls here (see backend/src/main/runtime/rpc/methods/cli.ts) instead of running the
// operation itself, mirroring preflight.check's devServerId relay pattern.
//
// Relationship to desktop/src/main/ipc/cli.ts (CliInstaller / WslCliInstaller): those
// classes resolve a *bundled Electron launcher* (packaged resourcesPath, AppImage, a
// generated dev-build launcher) and either symlink or wrap a command to it, including a
// macOS privileged-runner fallback and a Windows-side PowerShell bridge invoked from
// WSL. None of that exists here — the agent process has no Electron packaging, no
// bundled launcher binary, and (for the WSL variants) no separate Windows-host Orca.exe
// to bridge back to. This port therefore manages a single self-contained wrapper script
// directly: the script's own content IS the "launcher" (no separate launcherPath target
// to point at), which is why AgentCliInstallStatus always reports launcherPath: null and
// installMethod: 'wrapper'. Status/conflict/stale detection still follows the same
// "compare on-disk content against expected content" shape desktop's
// inspectWindowsWrapper/inspectAppImageWrapper use.

import { execFile } from 'node:child_process'
import { existsSync } from 'node:fs'
import { lstat, mkdir, readFile, unlink, writeFile } from 'node:fs/promises'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { getDefaultWslDistro, isWslAvailable } from '../main/wsl'
import { createTracer } from '../shared/trace'

const cliTracer = createTracer('agent:cli')

const COMMAND_NAME = 'orca'
const WSL_COMMAND_TIMEOUT_MS = 10_000
const MANAGED_MARKER = 'ORCA_AGENT_CLI_MARKER'

export type AgentCliInstallState =
  | 'installed'
  | 'not_installed'
  | 'stale'
  | 'conflict'
  | 'unsupported'

export type AgentCliInstallUnsupportedReason = 'platform_not_supported'

export type AgentCliInstallStatus = {
  platform: NodeJS.Platform
  commandName: string
  commandPath: string | null
  pathDirectory: string | null
  pathConfigured: boolean
  launcherPath: null
  installMethod: 'wrapper' | null
  supported: boolean
  state: AgentCliInstallState
  currentTarget: null
  unsupportedReason: AgentCliInstallUnsupportedReason | null
  detail: string | null
}

function respond(id: string | number | null, result: unknown): object {
  return { jsonrpc: '2.0', id, result }
}

// ─── Local (non-WSL) command path resolution ────────────────────────────────

function resolveLocalCommandPath(platform: NodeJS.Platform): string | null {
  if (platform === 'darwin' || platform === 'linux') {
    return join(homedir(), '.local', 'bin', COMMAND_NAME)
  }
  if (platform === 'win32') {
    const localAppData = process.env.LOCALAPPDATA ?? join(homedir(), 'AppData', 'Local')
    return join(localAppData, 'Orca', 'bin', `${COMMAND_NAME}.cmd`)
  }
  return null
}

function buildWrapperContent(platform: NodeJS.Platform): string {
  if (platform === 'win32') {
    return [
      '@echo off',
      `rem ${MANAGED_MARKER} — managed by the connected Orca Dev Server Agent.`,
      'echo Orca CLI is registered on this Dev Server via the Orca Dev Server Agent.',
      ''
    ].join('\r\n')
  }
  return [
    '#!/usr/bin/env bash',
    `# ${MANAGED_MARKER} — managed by the connected Orca Dev Server Agent.`,
    'echo "Orca CLI is registered on this Dev Server via the Orca Dev Server Agent."',
    ''
  ].join('\n')
}

function unsupportedStatus(platform: NodeJS.Platform, detail: string): AgentCliInstallStatus {
  return {
    platform,
    commandName: COMMAND_NAME,
    commandPath: null,
    pathDirectory: null,
    pathConfigured: false,
    launcherPath: null,
    installMethod: null,
    supported: false,
    state: 'unsupported',
    currentTarget: null,
    unsupportedReason: 'platform_not_supported',
    detail
  }
}

function isPathDirectoryOnPath(pathDirectory: string, platform: NodeJS.Platform): boolean {
  const pathValue = process.env.PATH ?? process.env.Path ?? ''
  const entries = pathValue.split(platform === 'win32' ? ';' : ':').map((entry) => entry.trim())
  if (platform !== 'win32') {
    return entries.includes(pathDirectory)
  }
  const normalize = (value: string): string =>
    value.replaceAll('/', '\\').replace(/\\+$/, '').toLowerCase()
  return entries.some((entry) => normalize(entry) === normalize(pathDirectory))
}

async function inspectLocalWrapper(
  platform: NodeJS.Platform,
  commandPath: string
): Promise<AgentCliInstallStatus> {
  const pathDirectory = dirname(commandPath)
  const pathConfigured = isPathDirectoryOnPath(pathDirectory, platform)
  const expected = buildWrapperContent(platform)

  let stats
  try {
    stats = await lstat(commandPath)
  } catch {
    return {
      platform,
      commandName: COMMAND_NAME,
      commandPath,
      pathDirectory,
      pathConfigured,
      launcherPath: null,
      installMethod: 'wrapper',
      supported: true,
      state: 'not_installed',
      currentTarget: null,
      unsupportedReason: null,
      detail: `Register ${commandPath} to use Orca from the terminal.`
    }
  }

  if (!stats.isFile()) {
    return {
      platform,
      commandName: COMMAND_NAME,
      commandPath,
      pathDirectory,
      pathConfigured,
      launcherPath: null,
      installMethod: 'wrapper',
      supported: true,
      state: 'conflict',
      currentTarget: null,
      unsupportedReason: null,
      detail: `${commandPath} exists but is not an Orca launcher script.`
    }
  }

  const content = await readFile(commandPath, 'utf8')
  if (content === expected) {
    return {
      platform,
      commandName: COMMAND_NAME,
      commandPath,
      pathDirectory,
      pathConfigured,
      launcherPath: null,
      installMethod: 'wrapper',
      supported: true,
      state: 'installed',
      currentTarget: null,
      unsupportedReason: null,
      detail: `Registered at ${commandPath}.`
    }
  }

  return {
    platform,
    commandName: COMMAND_NAME,
    commandPath,
    pathDirectory,
    pathConfigured,
    launcherPath: null,
    installMethod: 'wrapper',
    supported: true,
    state: content.includes(MANAGED_MARKER) ? 'stale' : 'conflict',
    currentTarget: null,
    unsupportedReason: null,
    detail: content.includes(MANAGED_MARKER)
      ? `${commandPath} was registered by an older Orca Dev Server Agent version.`
      : `${commandPath} exists but is not managed by Orca.`
  }
}

async function getLocalCliInstallStatus(): Promise<AgentCliInstallStatus> {
  const platform = process.platform
  const commandPath = resolveLocalCommandPath(platform)
  if (!commandPath) {
    return unsupportedStatus(platform, 'CLI registration is not implemented on this platform.')
  }
  return inspectLocalWrapper(platform, commandPath)
}

async function installLocalCli(): Promise<AgentCliInstallStatus> {
  const status = await getLocalCliInstallStatus()
  if (!status.supported || !status.commandPath) {
    throw new Error(status.detail ?? 'CLI registration is unavailable on this Dev Server.')
  }
  if (status.state === 'conflict') {
    throw new Error(`Refusing to replace non-Orca command at ${status.commandPath}.`)
  }
  if (status.state === 'installed') {
    return status
  }

  await mkdir(dirname(status.commandPath), { recursive: true })
  // Why: a single-writer settings action does not need tmp-file+rename atomicity —
  // the desktop installer's atomic-replace machinery guards multi-process/SSH races
  // that don't apply to one agent handling one RPC call at a time.
  await writeFile(status.commandPath, buildWrapperContent(status.platform), {
    encoding: 'utf8',
    mode: status.platform === 'win32' ? undefined : 0o755
  })
  return getLocalCliInstallStatus()
}

async function removeLocalCli(): Promise<AgentCliInstallStatus> {
  const status = await getLocalCliInstallStatus()
  if (!status.supported || !status.commandPath) {
    return status
  }
  if (status.state === 'not_installed') {
    return status
  }
  if (status.state === 'conflict') {
    throw new Error(`Refusing to remove non-Orca command at ${status.commandPath}.`)
  }

  await unlink(status.commandPath)
  return getLocalCliInstallStatus()
}

// ─── WSL command path resolution ─────────────────────────────────────────────
// Why: unlike desktop's WslCliInstaller, there is no separate Windows-host
// Orca.exe launcher to bridge WSL back to — this agent process is itself
// whatever OS it was deployed on. So a WSL wrapper installed here is the same
// self-contained script content as the native Windows one, just written
// through `wsl.exe` into the distro's filesystem instead of node:fs.

function quoteShellSingle(value: string): string {
  return `'${value.replaceAll("'", "'\\''")}'`
}

function runWslCommand(distro: string, command: string): Promise<string> {
  return new Promise((resolvePromise, reject) => {
    // Why: raw multiline scripts can be flattened crossing the wsl.exe/Windows
    // command-line boundary — send one shell-safe base64 line and decode inside WSL.
    const encoded = Buffer.from(command, 'utf8').toString('base64')
    const script = `set -o pipefail; printf %s ${quoteShellSingle(encoded)} | base64 -d | bash`

    let settled = false
    const finish = (error: Error | null, stdout = ''): void => {
      if (settled) {return}
      settled = true
      clearTimeout(timeout)
      if (error) {
        reject(error)
        return
      }
      resolvePromise(stdout)
    }

    const timeout = setTimeout(() => {
      child.kill()
      finish(new Error(`WSL command timed out after ${WSL_COMMAND_TIMEOUT_MS}ms.`))
    }, WSL_COMMAND_TIMEOUT_MS)

    const child = execFile(
      'wsl.exe',
      ['-d', distro, '--', 'bash', '-lc', script],
      { encoding: 'utf8', timeout: WSL_COMMAND_TIMEOUT_MS },
      (error, stdout) => finish(error ?? null, stdout)
    )
  })
}

function resolveWslDistro(params: Record<string, unknown>): string | null {
  const requested = typeof params.distro === 'string' ? params.distro.trim() : ''
  return requested || getDefaultWslDistro()
}

async function resolveWslReadyState(
  params: Record<string, unknown>
): Promise<{ status: AgentCliInstallStatus } | { distro: string; commandPath: string }> {
  if (process.platform !== 'win32') {
    return {
      status: unsupportedStatus(
        process.platform,
        'WSL CLI registration is only available when the Dev Server Agent runs on Windows.'
      )
    }
  }
  if (!isWslAvailable()) {
    return {
      status: unsupportedStatus('win32', 'WSL is not available on this Dev Server.')
    }
  }
  const distro = resolveWslDistro(params)
  if (!distro) {
    return {
      status: unsupportedStatus('win32', 'No WSL distribution is available on this Dev Server.')
    }
  }

  const home = (await runWslCommand(distro, 'printf %s "$HOME"')).trim()
  if (!home.startsWith('/')) {
    return {
      status: unsupportedStatus('win32', `Unable to resolve the WSL home directory in ${distro}.`)
    }
  }
  return { distro, commandPath: `${home}/.local/bin/${COMMAND_NAME}` }
}

async function readWslCommandFile(
  distro: string,
  commandPath: string
): Promise<string | 'not_file' | null> {
  const output = await runWslCommand(
    distro,
    [
      `if [ -L ${quoteShellSingle(commandPath)} ]; then`,
      '  printf __ORCA_NOT_FILE__',
      `elif [ ! -e ${quoteShellSingle(commandPath)} ]; then`,
      '  printf __ORCA_MISSING__',
      `elif [ ! -f ${quoteShellSingle(commandPath)} ]; then`,
      '  printf __ORCA_NOT_FILE__',
      'else',
      `  cat ${quoteShellSingle(commandPath)}`,
      'fi'
    ].join('\n')
  )
  if (output === '__ORCA_MISSING__') {return null}
  if (output === '__ORCA_NOT_FILE__') {return 'not_file'}
  return output
}

async function getWslCliInstallStatusFor(
  params: Record<string, unknown>
): Promise<AgentCliInstallStatus> {
  const ready = await resolveWslReadyState(params)
  if ('status' in ready) {return ready.status}

  const pathDirectory = dirname(ready.commandPath)
  const pathConfigured =
    (
      await runWslCommand(
        ready.distro,
        `case ":$PATH:" in *:${quoteShellSingle(pathDirectory)}:*) printf yes ;; *) printf no ;; esac`
      )
    ).trim() === 'yes'

  const content = await readWslCommandFile(ready.distro, ready.commandPath)
  const expected = buildWrapperContent('linux')

  if (content === null) {
    return {
      platform: 'linux',
      commandName: COMMAND_NAME,
      commandPath: ready.commandPath,
      pathDirectory,
      pathConfigured,
      launcherPath: null,
      installMethod: 'wrapper',
      supported: true,
      state: 'not_installed',
      currentTarget: null,
      unsupportedReason: null,
      detail: `Register ${ready.commandPath} to use Orca from WSL.`
    }
  }
  if (content === 'not_file') {
    return {
      platform: 'linux',
      commandName: COMMAND_NAME,
      commandPath: ready.commandPath,
      pathDirectory,
      pathConfigured,
      launcherPath: null,
      installMethod: 'wrapper',
      supported: true,
      state: 'conflict',
      currentTarget: null,
      unsupportedReason: null,
      detail: `${ready.commandPath} exists but is not an Orca launcher script.`
    }
  }
  if (content === expected) {
    return {
      platform: 'linux',
      commandName: COMMAND_NAME,
      commandPath: ready.commandPath,
      pathDirectory,
      pathConfigured,
      launcherPath: null,
      installMethod: 'wrapper',
      supported: true,
      state: 'installed',
      currentTarget: null,
      unsupportedReason: null,
      detail: `Registered in ${ready.distro} at ${ready.commandPath}.`
    }
  }
  return {
    platform: 'linux',
    commandName: COMMAND_NAME,
    commandPath: ready.commandPath,
    pathDirectory,
    pathConfigured,
    launcherPath: null,
    installMethod: 'wrapper',
    supported: true,
    state: content.includes(MANAGED_MARKER) ? 'stale' : 'conflict',
    currentTarget: null,
    unsupportedReason: null,
    detail: content.includes(MANAGED_MARKER)
      ? `${ready.commandPath} was registered by an older Orca Dev Server Agent version.`
      : `${ready.commandPath} exists but is not managed by Orca.`
  }
}

async function installWslCliFor(params: Record<string, unknown>): Promise<AgentCliInstallStatus> {
  const status = await getWslCliInstallStatusFor(params)
  if (!status.supported || !status.commandPath) {
    throw new Error(status.detail ?? 'WSL CLI registration is unavailable on this Dev Server.')
  }
  if (status.state === 'conflict') {
    throw new Error(`Refusing to replace non-Orca command at ${status.commandPath}.`)
  }
  if (status.state === 'installed') {
    return status
  }

  const distro = resolveWslDistro(params)
  if (!distro) {
    throw new Error('No WSL distribution is available on this Dev Server.')
  }
  await runWslCommand(
    distro,
    [
      'set -euo pipefail',
      `mkdir -p ${quoteShellSingle(dirname(status.commandPath))}`,
      `cat > ${quoteShellSingle(status.commandPath)} <<'ORCA_AGENT_CLI'`,
      buildWrapperContent('linux'),
      'ORCA_AGENT_CLI',
      `chmod 755 ${quoteShellSingle(status.commandPath)}`
    ].join('\n')
  )
  return getWslCliInstallStatusFor(params)
}

async function removeWslCliFor(params: Record<string, unknown>): Promise<AgentCliInstallStatus> {
  const status = await getWslCliInstallStatusFor(params)
  if (!status.supported || !status.commandPath) {
    return status
  }
  if (status.state === 'not_installed') {
    return status
  }
  if (status.state === 'conflict') {
    throw new Error(`Refusing to remove non-Orca command at ${status.commandPath}.`)
  }

  const distro = resolveWslDistro(params)
  if (!distro) {
    throw new Error('No WSL distribution is available on this Dev Server.')
  }
  await runWslCommand(distro, `rm -f ${quoteShellSingle(status.commandPath)}`)
  return getWslCliInstallStatusFor(params)
}

// ─── RPC entry points (called from agent-rpc-dispatch.ts's route()) ────────

export async function handleCliGetInstallStatus(id: string | number | null): Promise<object> {
  const span = cliTracer.start({ method: 'cli.getInstallStatus' })
  try {
    const result = await getLocalCliInstallStatus()
    span.ok({ state: result.state })
    return respond(id, result)
  } catch (err) {
    span.fail(err)
    throw err
  }
}

export async function handleCliInstall(id: string | number | null): Promise<object> {
  const span = cliTracer.start({ method: 'cli.install' })
  try {
    const result = await installLocalCli()
    span.ok({ state: result.state })
    return respond(id, result)
  } catch (err) {
    span.fail(err)
    throw err
  }
}

export async function handleCliRemove(id: string | number | null): Promise<object> {
  const span = cliTracer.start({ method: 'cli.remove' })
  try {
    const result = await removeLocalCli()
    span.ok({ state: result.state })
    return respond(id, result)
  } catch (err) {
    span.fail(err)
    throw err
  }
}

export async function handleCliGetWslInstallStatus(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const span = cliTracer.start({ method: 'cli.getWslInstallStatus' })
  try {
    const result = await getWslCliInstallStatusFor(params)
    span.ok({ state: result.state })
    return respond(id, result)
  } catch (err) {
    span.fail(err)
    throw err
  }
}

export async function handleCliInstallWsl(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const span = cliTracer.start({ method: 'cli.installWsl' })
  try {
    const result = await installWslCliFor(params)
    span.ok({ state: result.state })
    return respond(id, result)
  } catch (err) {
    span.fail(err)
    throw err
  }
}

export async function handleCliRemoveWsl(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const span = cliTracer.start({ method: 'cli.removeWsl' })
  try {
    const result = await removeWslCliFor(params)
    span.ok({ state: result.state })
    return respond(id, result)
  } catch (err) {
    span.fail(err)
    throw err
  }
}

// Why: exported for tests only — keeps the file's public RPC surface small
// while letting agent-cli-handler.test.ts assert on wrapper content directly.
export const _internals = { buildWrapperContent, resolveLocalCommandPath, existsSync }
