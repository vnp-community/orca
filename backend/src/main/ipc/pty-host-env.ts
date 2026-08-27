/* eslint-disable max-lines -- Why (TASK-BIGFILE-251, BUG-FE-BIGFILE-011):
verbatim extraction of buildPtyHostEnv + 19 co-located host-env helper
symbols out of ipc/pty.ts, per the task doc's designed scope (409 counted
lines vs the 300 budget) — further splitting was out of this Move task's
scope; see the task doc for the full symbol list. */
// Host-local PTY env assembly, extracted from ipc/pty.ts (TASK-BIGFILE-251).
// Why: both the LocalPtyProvider.buildSpawnEnv closure and the daemon-active
// fallback in pty:spawn need the same set of host-local env injections
// (OpenCode plugin dir, agent-hook server coordinates, Pi/OMP managed
// extensions, Codex account home, dev-mode CLI overrides, GitHub attribution
// shims). They used to be implemented twice, which silently drifted —
// daemon-backed PTYs never got the OpenCode plugin, Pi integration, Codex
// home, or dev CLI PATH prepend, so status dots, Pi state, Codex switching, and CLI→dev
// routing were all broken for daemon users (the common case).
//
// Centralizing the injections here makes future additions fail-safe: a new
// variable added to this function lands in BOTH spawn paths or NEITHER.
import { join, delimiter } from 'node:path'
import { isWslShellName } from '../../shared/local-windows-terminal-runtime'
import { openCodeHookService } from '../opencode/hook-service'
import { mimoCodeHookService } from '../mimo/hook-service'
import {
  getCommandTokenPathBasename,
  getFirstCommandToken
} from '../../shared/command-token-scanner'
import { agentHookServer } from '../agent-hooks/server'
import { wslHookRelayManager } from '../agent-hooks/wsl-hook-relay-manager'
import { piTitlebarExtensionService } from '../pi/titlebar-extension-service'
import { detectPiAgentKindFromCommand, type PiAgentKind } from '../../shared/pi-agent-kind'
import type { ClaudeRuntimeAuthPreparation } from '../claude-accounts/runtime-auth-service'
import type { ClaudeAccountSelectionTarget } from '../claude-accounts/runtime-selection'
import {
  applyTerminalAttributionEnv,
  resolveAttributionShellFamily
} from '../attribution/terminal-attribution'
import { ensureLinuxTerminalOrcaCliShimDir } from '../cli/linux-terminal-orca-cli-shim'
import { readShellStartupEnvVar } from '../pty/shell-startup-env'
import { parseWslPath } from '../wsl'
import { mergePersistedWindowsPath } from '../pty/windows-environment-path'
import type { CodexAccountSelectionTarget } from '../codex-accounts/runtime-selection'
import { isHostCodexHomeForWsl, isWslCodexHomeForHost } from '../pty/codex-home-wsl-env'
import { buildConfiguredProxyEnv, type NetworkProxySettings } from '../../shared/network-proxy'
import { resolveSetupAgentSequenceLaunchCommand } from '../../shared/setup-agent-sequencing'

export type BuildPtyHostEnvOptions = {
  isPackaged: boolean
  userDataPath: string
  selectedCodexHomePath: string | null
  skipCodexHomeEnv?: boolean
  githubAttributionEnabled: boolean
  /** The launch command the renderer chose for this PTY (e.g. 'pi', 'omp',
   *  'claude'). Used to resolve the per-agent managed extension target for
   *  Pi / OMP - both consume `PI_CODING_AGENT_DIR` but default to different
   *  `~/.<kind>/agent` paths. Undefined for bare-shell spawns; defaults
   *  resolve to Pi for back-compat. NEVER infer from disk presence; that's
   *  the bug this option fixes (cross-agent shadowing when both dirs exist). */
  launchCommand?: string
  shellPath?: string
  isWsl?: boolean
  /** Distro for WSL spawns (null = Windows default distro). Drives the WSL
   *  hook relay ensure + guest endpoint repoint; only read when isWsl. */
  wslDistro?: string | null
  agentStatusHooksEnabled: boolean
  networkProxySettings?: NetworkProxySettings
}

const AGENT_HOOK_RUNTIME_ENV_KEYS = [
  'ORCA_AGENT_HOOK_PORT',
  'ORCA_AGENT_HOOK_TOKEN',
  'ORCA_AGENT_HOOK_ENV',
  'ORCA_AGENT_HOOK_VERSION',
  'ORCA_AGENT_HOOK_ENDPOINT',
  // Why: PR 2778 briefly exported this scoped Claude settings path. Keep
  // deleting stale inherited values so older PTYs cannot leak the reverted path.
  'ORCA_CLAUDE_AGENT_STATUS_SETTINGS'
] as const

function readInheritedPath(baseEnv: Record<string, string>): string {
  return baseEnv.PATH ?? baseEnv.Path ?? process.env.PATH ?? process.env.Path ?? ''
}

function firstPathEntry(pathValue: string | undefined): string | null {
  const first = pathValue?.split(delimiter).find((entry) => entry.trim().length > 0)
  return first ?? null
}

function promoteAgentTeamsShimPath(
  env: Record<string, string> | undefined,
  requestedPath: string | undefined
): void {
  if (!env?.ORCA_AGENT_TEAMS_TEAM_ID) {
    return
  }
  const shimPath = firstPathEntry(requestedPath)
  if (!shimPath) {
    return
  }
  const currentPathKey = env.PATH !== undefined || env.Path === undefined ? 'PATH' : 'Path'
  const currentPath = env[currentPathKey] ?? ''
  const remaining = currentPath
    .split(delimiter)
    .filter((entry) => entry.length > 0 && entry !== shimPath)
  // Why: host env injection can prepend Orca's attribution/dev shims. Claude
  // Agent Teams must still resolve our fake tmux before any real tmux.
  env[currentPathKey] = [shimPath, ...remaining].join(delimiter)
}

function deleteRequestedEnvKeys(
  env: Record<string, string> | undefined,
  keys: string[] | undefined
): void {
  if (!env || !keys) {
    return
  }
  for (const key of keys) {
    delete env[key]
  }
}

function shouldSkipCodexHomeEnvForWindowsShell(
  shellPath: string | undefined,
  cwd: string | undefined
): boolean {
  return isWslShellName(shellPath) || (typeof cwd === 'string' && parseWslPath(cwd) !== null)
}

const CODEX_HOME_ENV_KEYS = ['CODEX_HOME', 'ORCA_CODEX_HOME'] as const
type GetSelectedCodexHomePath = (target?: CodexAccountSelectionTarget) => string | null
type PrepareClaudeAuth = (
  target?: ClaudeAccountSelectionTarget
) => Promise<ClaudeRuntimeAuthPreparation>

function getCodexSelectionTargetForPty(
  shellPath: string | undefined,
  cwd: string | undefined,
  wslDistro?: string | null
): CodexAccountSelectionTarget {
  const wslPath = typeof cwd === 'string' ? parseWslPath(cwd) : null
  if (isWslShellName(shellPath) || wslPath) {
    return { runtime: 'wsl', wslDistro: wslPath?.distro ?? wslDistro ?? null }
  }
  return { runtime: 'host' }
}

function getCompatibleSelectedCodexHomePath(
  target: CodexAccountSelectionTarget,
  selectedCodexHomePath: string | null
): string | null {
  if (!selectedCodexHomePath) {
    return null
  }
  const wslInfo = parseWslPath(selectedCodexHomePath)
  if (target.runtime === 'wsl') {
    return wslInfo || !isHostCodexHomeForWsl(selectedCodexHomePath) ? selectedCodexHomePath : null
  }
  return wslInfo || (process.platform === 'win32' && isWslCodexHomeForHost(selectedCodexHomePath))
    ? null
    : selectedCodexHomePath
}

function readEnvWithProcessFallback(
  baseEnv: Record<string, string>,
  key: string
): string | undefined {
  return baseEnv[key] ?? process.env[key]
}

function resolvePiAgentSourceDir(
  baseEnv: Record<string, string>,
  kind: PiAgentKind
): string | undefined {
  const sourceKey = kind === 'omp' ? 'ORCA_OMP_SOURCE_AGENT_DIR' : 'ORCA_PI_SOURCE_AGENT_DIR'
  const overlayKey = kind === 'omp' ? 'ORCA_OMP_CODING_AGENT_DIR' : 'ORCA_PI_CODING_AGENT_DIR'
  const otherOverlayKey = kind === 'omp' ? 'ORCA_PI_CODING_AGENT_DIR' : 'ORCA_OMP_CODING_AGENT_DIR'

  const sourceDir = readEnvWithProcessFallback(baseEnv, sourceKey)
  if (sourceDir) {
    return sourceDir
  }

  const publicDir = readEnvWithProcessFallback(baseEnv, 'PI_CODING_AGENT_DIR')
  const ownOverlayDir = readEnvWithProcessFallback(baseEnv, overlayKey)
  const otherOverlayDir = readEnvWithProcessFallback(baseEnv, otherOverlayKey)
  // Why: if PI_CODING_AGENT_DIR is just a restored Orca overlay from either
  // kind and the matching source shadow is absent, remirroring it would leak
  // another agent's overlay tree into this launch. Fall through to defaults.
  if (publicDir && publicDir !== ownOverlayDir && publicDir !== otherOverlayDir) {
    return publicDir
  }

  return readShellStartupEnvVar(
    'PI_CODING_AGENT_DIR',
    baseEnv.HOME ?? process.env.HOME,
    baseEnv.SHELL ?? process.env.SHELL
  )
}

function resolveScopedPiAgentSourceDir(
  baseEnv: Record<string, string>,
  kind: PiAgentKind
): string | undefined {
  const sourceKey = kind === 'omp' ? 'ORCA_OMP_SOURCE_AGENT_DIR' : 'ORCA_PI_SOURCE_AGENT_DIR'
  return readEnvWithProcessFallback(baseEnv, sourceKey)
}

function clearPiAgentShadowEnv(baseEnv: Record<string, string>, kind: PiAgentKind): void {
  if (kind === 'omp') {
    delete baseEnv.ORCA_OMP_CODING_AGENT_DIR
    delete baseEnv.ORCA_OMP_SOURCE_AGENT_DIR
    delete baseEnv.ORCA_OMP_STATUS_EXTENSION
    return
  }
  delete baseEnv.ORCA_PI_CODING_AGENT_DIR
  delete baseEnv.ORCA_PI_SOURCE_AGENT_DIR
}

function exposePiManagedExtensionEnv(
  baseEnv: Record<string, string>,
  kind: PiAgentKind,
  managedEnv: Record<string, string>
): void {
  if (kind === 'omp') {
    delete baseEnv.ORCA_OMP_CODING_AGENT_DIR
    if (managedEnv.ORCA_OMP_SOURCE_AGENT_DIR) {
      baseEnv.ORCA_OMP_SOURCE_AGENT_DIR = managedEnv.ORCA_OMP_SOURCE_AGENT_DIR
    } else {
      delete baseEnv.ORCA_OMP_SOURCE_AGENT_DIR
    }
    if (managedEnv.ORCA_OMP_STATUS_EXTENSION) {
      baseEnv.ORCA_OMP_STATUS_EXTENSION = managedEnv.ORCA_OMP_STATUS_EXTENSION
    } else {
      delete baseEnv.ORCA_OMP_STATUS_EXTENSION
    }
    return
  }
  delete baseEnv.ORCA_PI_CODING_AGENT_DIR
  if (managedEnv.ORCA_PI_SOURCE_AGENT_DIR) {
    baseEnv.ORCA_PI_SOURCE_AGENT_DIR = managedEnv.ORCA_PI_SOURCE_AGENT_DIR
  } else {
    delete baseEnv.ORCA_PI_SOURCE_AGENT_DIR
  }
}

function mergePtyEnvDeletions(
  existingKeys: string[] | undefined,
  additionalKeys: readonly string[]
): string[] | undefined {
  if (!existingKeys && additionalKeys.length === 0) {
    return undefined
  }
  return Array.from(new Set([...(existingKeys ?? []), ...additionalKeys]))
}

function getInheritedAgentHookEnvKeysToDelete(
  spawnEnv: Record<string, string> | undefined
): string[] {
  const env = spawnEnv ?? {}
  // Why: daemon/local providers merge process.env after main-process cleanup.
  // Delete reverted or unavailable hook env keys there without dropping fresh
  // receiver coordinates that buildPtyHostEnv intentionally set.
  return AGENT_HOOK_RUNTIME_ENV_KEYS.filter((key) => env[key] === undefined)
}

// Why: when agent status is disabled, a nested Orca terminal can still pass
// through prior OpenCode or legacy Pi/OMP overlay env. Restore the user's
// original source dir when Orca recorded one, otherwise strip only values
// known to be ours.
function restoreOrStripOverlayEnv(
  baseEnv: Record<string, string>,
  keys: {
    primary: string
    overlay: string
    source: string
  }
): void {
  const sourceValue = baseEnv[keys.source] ?? process.env[keys.source]
  const overlayValue = baseEnv[keys.overlay] ?? process.env[keys.overlay]
  if (sourceValue) {
    baseEnv[keys.primary] = sourceValue
  } else if (overlayValue && baseEnv[keys.primary] === overlayValue) {
    delete baseEnv[keys.primary]
  }
  delete baseEnv[keys.overlay]
  delete baseEnv[keys.source]
}

function isMimoLaunchCommand(launchCommand: string | undefined): boolean {
  const binary = getCommandTokenPathBasename(getFirstCommandToken(launchCommand ?? ''))
    .toLowerCase()
    .replace(/\.(?:cmd|exe|sh)$/, '')
  return binary === 'mimo'
}

function resolveMimocodeSourceHome(baseEnv: Record<string, string>): string | undefined {
  const sourceHome = baseEnv.ORCA_MIMOCODE_SOURCE_HOME ?? process.env.ORCA_MIMOCODE_SOURCE_HOME
  if (sourceHome) {
    return sourceHome
  }
  const configHome = baseEnv.MIMOCODE_HOME ?? process.env.MIMOCODE_HOME
  const orcaHome = baseEnv.ORCA_MIMOCODE_HOME ?? process.env.ORCA_MIMOCODE_HOME
  if (configHome && orcaHome && configHome === orcaHome) {
    return undefined
  }
  return configHome
}

function resolveOpenCodeSourceConfigDir(baseEnv: Record<string, string>): string | undefined {
  const sourceDir =
    baseEnv.ORCA_OPENCODE_SOURCE_CONFIG_DIR ?? process.env.ORCA_OPENCODE_SOURCE_CONFIG_DIR
  if (sourceDir) {
    return sourceDir
  }

  const configDir = baseEnv.OPENCODE_CONFIG_DIR ?? process.env.OPENCODE_CONFIG_DIR
  const orcaConfigDir = baseEnv.ORCA_OPENCODE_CONFIG_DIR ?? process.env.ORCA_OPENCODE_CONFIG_DIR
  // Why: nested Orca terminals inherit OPENCODE_CONFIG_DIR from the parent
  // PTY. If there is no recorded source dir, that value is Orca-owned, not a
  // user config. Treating it as user config makes child Orcas mirror Orca's
  // hook dir and can create large OpenCode runtime trees per terminal.
  if (configDir && orcaConfigDir && configDir === orcaConfigDir) {
    return undefined
  }

  return (
    configDir ??
    readShellStartupEnvVar(
      'OPENCODE_CONFIG_DIR',
      baseEnv.HOME ?? process.env.HOME,
      baseEnv.SHELL ?? process.env.SHELL
    )
  )
}

/**
 * Mutates `baseEnv` in place with all host-local PTY env vars and returns it.
 *
 * This is the single source of truth for the env shape an Orca PTY needs
 * BEFORE the provider-specific wrapper (LocalPtyProvider's TERM/LANG defaults,
 * DaemonPtyAdapter's subprocess env). Callers are responsible for the SSH
 * guard — if `args.connectionId` is set, do NOT call this function, because
 * every injection here is either host-loopback (hook server, attribution
 * shims) or references paths on the local filesystem that would be meaningless
 * to a remote shell.
 */
export function buildPtyHostEnv(
  id: string,
  baseEnv: Record<string, string>,
  opts: BuildPtyHostEnvOptions
): Record<string, string> {
  mergePersistedWindowsPath(baseEnv)
  Object.assign(baseEnv, buildConfiguredProxyEnv(opts.networkProxySettings))

  // Why: the Local path passes a baseEnv that already includes process.env
  // (LocalPtyProvider.spawn merges it before calling buildSpawnEnv). The
  // daemon path passes only args.env since process.env propagates to the
  // daemon subprocess via fork inheritance, not the IPC wire. Checking both
  // sources when reading a potentially-user-provided value keeps the guards
  // in lock-step across spawn paths without pushing process.env onto the
  // IPC wire unnecessarily.
  const preexistingOpenCodeConfigDir = resolveOpenCodeSourceConfigDir(baseEnv)
  const launchCommandHint = resolveSetupAgentSequenceLaunchCommand(baseEnv, opts.launchCommand)
  const piAgentKind = detectPiAgentKindFromCommand(launchCommandHint)
  const hasLaunchCommand =
    typeof launchCommandHint === 'string' && launchCommandHint.trim().length > 0
  const shouldPrepareOmpShadow = piAgentKind === 'omp' || !hasLaunchCommand
  // Why: source shadows are agent-scoped. Trusting the other kind's source
  // would reintroduce the exact Pi/OMP extension-state shadowing this PR fixes.
  const preexistingPiAgentDir = resolvePiAgentSourceDir(baseEnv, 'pi')
  const preexistingOmpAgentDir =
    piAgentKind === 'omp'
      ? resolvePiAgentSourceDir(baseEnv, 'omp')
      : resolveScopedPiAgentSourceDir(baseEnv, 'omp')

  if (opts.agentStatusHooksEnabled) {
    // Why: OPENCODE_CONFIG_DIR is a singular path, not a colon-list, so a user
    // value cannot coexist with an Orca-only injection. Hand the user's value
    // (when present) to the hook service and let it materialize a source-scoped
    // mirror overlay that lets the user's plugins and Orca's status plugin
    // load together. See docs/opencode-config-dir-collision.md.
    Object.assign(baseEnv, openCodeHookService.buildPtyEnv(id, preexistingOpenCodeConfigDir))
    if (baseEnv.OPENCODE_CONFIG_DIR) {
      // Why: ~/.zshrc can re-export the user's default after spawn; shell-ready
      // wrappers restore this PTY-scoped value after user startup files run.
      baseEnv.ORCA_OPENCODE_CONFIG_DIR = baseEnv.OPENCODE_CONFIG_DIR
      if (preexistingOpenCodeConfigDir) {
        // Why: terminals launched from another Orca terminal inherit the overlay
        // as OPENCODE_CONFIG_DIR; keep the original source so overlays do not
        // mirror overlays and drop the user's real config.
        baseEnv.ORCA_OPENCODE_SOURCE_CONFIG_DIR = preexistingOpenCodeConfigDir
      } else {
        delete baseEnv.ORCA_OPENCODE_SOURCE_CONFIG_DIR
      }
    }
    if (isMimoLaunchCommand(launchCommandHint)) {
      const preexistingMimocodeHome = resolveMimocodeSourceHome(baseEnv)
      Object.assign(baseEnv, mimoCodeHookService.buildPtyEnv(id, preexistingMimocodeHome))
      if (baseEnv.MIMOCODE_HOME) {
        baseEnv.ORCA_MIMOCODE_HOME = baseEnv.MIMOCODE_HOME
        if (preexistingMimocodeHome) {
          baseEnv.ORCA_MIMOCODE_SOURCE_HOME = preexistingMimocodeHome
        } else {
          delete baseEnv.ORCA_MIMOCODE_SOURCE_HOME
        }
      }
    }
  } else {
    restoreOrStripOverlayEnv(baseEnv, {
      primary: 'OPENCODE_CONFIG_DIR',
      overlay: 'ORCA_OPENCODE_CONFIG_DIR',
      source: 'ORCA_OPENCODE_SOURCE_CONFIG_DIR'
    })
    restoreOrStripOverlayEnv(baseEnv, {
      primary: 'MIMOCODE_HOME',
      overlay: 'ORCA_MIMOCODE_HOME',
      source: 'ORCA_MIMOCODE_SOURCE_HOME'
    })
  }

  // Why: Claude/Codex native hooks run inside the shell process, so Orca
  // must inject the loopback receiver coordinates before the agent starts.
  // Without these env vars the global hook config cannot map callbacks back
  // to the correct Orca pane.
  // Why: nested Orca terminals can inherit another process's hook endpoint or
  // token. Strip all hook runtime coordinates before injecting this PTY's fresh
  // server values so callbacks route to the owning app/runtime.
  for (const key of AGENT_HOOK_RUNTIME_ENV_KEYS) {
    delete baseEnv[key]
  }
  if (opts.agentStatusHooksEnabled) {
    Object.assign(baseEnv, agentHookServer.buildPtyEnv())
    if (opts.isWsl === true) {
      // Why: hook POSTs to 127.0.0.1 die inside WSL's NAT namespace. Ensure
      // the guest-resident relay for this distro (covers fresh spawns and
      // post-restart reattach re-spawns), and once the relay has reported the
      // guest home, point restart re-coordination at the relay-written
      // guest-side endpoint file instead of the /p-translated Windows one.
      const distro = opts.wslDistro ?? null
      wslHookRelayManager.ensureForDistro(distro)
      const guestEndpoint = wslHookRelayManager.getGuestEndpointFilePath(distro)
      if (guestEndpoint) {
        baseEnv.ORCA_AGENT_HOOK_ENDPOINT = guestEndpoint
      }
    }
  }

  // Why: PI_CODING_AGENT_DIR owns Pi's / OMP's full config/session root. Keep
  // that home as the user's normal source of truth and install only Orca-owned,
  // env-guarded extension files into the selected agent's extension dir.
  if (opts.agentStatusHooksEnabled) {
    clearPiAgentShadowEnv(baseEnv, 'pi')
    clearPiAgentShadowEnv(baseEnv, 'omp')
    if (piAgentKind === 'pi') {
      const piEnv = piTitlebarExtensionService.buildPtyEnv(id, preexistingPiAgentDir, 'pi')
      Object.assign(baseEnv, piEnv)
      exposePiManagedExtensionEnv(baseEnv, 'pi', piEnv)
    }

    if (shouldPrepareOmpShadow) {
      const ompEnv = piTitlebarExtensionService.buildPtyEnv(id, preexistingOmpAgentDir, 'omp')
      Object.assign(baseEnv, ompEnv)
      exposePiManagedExtensionEnv(baseEnv, 'omp', ompEnv)
    }
  } else {
    // Why: when agent status is disabled we must strip BOTH kinds' shadow vars
    // so a nested PTY does not inherit a stale overlay from either agent.
    restoreOrStripOverlayEnv(baseEnv, {
      primary: 'PI_CODING_AGENT_DIR',
      overlay: 'ORCA_PI_CODING_AGENT_DIR',
      source: 'ORCA_PI_SOURCE_AGENT_DIR'
    })
    restoreOrStripOverlayEnv(baseEnv, {
      primary: 'PI_CODING_AGENT_DIR',
      overlay: 'ORCA_OMP_CODING_AGENT_DIR',
      source: 'ORCA_OMP_SOURCE_AGENT_DIR'
    })
    delete baseEnv.ORCA_OMP_STATUS_EXTENSION
  }

  // Why: Codex account switching now materializes auth into an Orca-scoped
  // runtime home, and Codex launched inside Orca terminals must use that same
  // prepared home as quota fetches and other entry points. Keep the override
  // PTY-scoped so dev/prod Orcas do not share hooks through ~/.codex.
  if (opts.skipCodexHomeEnv) {
    delete baseEnv.CODEX_HOME
    delete baseEnv.ORCA_CODEX_HOME
  } else if (opts.selectedCodexHomePath) {
    baseEnv.CODEX_HOME = opts.selectedCodexHomePath
    // Why: user startup files may re-export CODEX_HOME; shell-ready wrappers
    // restore this runtime home before Codex can be launched from the prompt.
    baseEnv.ORCA_CODEX_HOME = opts.selectedCodexHomePath
  }

  // Why: WSL shells need the managed userData root for shell-ready wrappers; dev-mode terminals need the same export so `orca` targets the live dev instance.
  if (opts.isWsl) {
    baseEnv.ORCA_USER_DATA_PATH = opts.userDataPath
  } else if (!opts.isPackaged) {
    baseEnv.ORCA_USER_DATA_PATH ??= opts.userDataPath
  }
  // Why: dev mode needs the launcher PATH override so `orca` resolves to the dev build instead of the production binary at /usr/local/bin/orca.
  if (!opts.isPackaged) {
    const devCliBin = join(opts.userDataPath, 'cli', 'bin')
    const inheritedPath = readInheritedPath(baseEnv)
    // Why: avoid a trailing delimiter when PATH is empty — some shells
    // treat an empty segment as `.`, which would let commands resolve from
    // the current working directory (a foot-gun we don't want to create
    // for dev terminals).
    baseEnv.PATH = inheritedPath ? `${devCliBin}${delimiter}${inheritedPath}` : devCliBin
  } else if (process.platform === 'linux') {
    // Why: the Linux CLI installs as `orca-ide` (never shadowing GNOME's
    // /usr/bin/orca screen reader), but agent-facing guidance invokes bare
    // `orca`. Scope a bare-`orca` shim to Orca-managed PTYs so agents reach
    // the Orca CLI instead of the screen reader (stablyai/orca#7904).
    const shimDir = ensureLinuxTerminalOrcaCliShimDir({ userDataPath: opts.userDataPath })
    if (shimDir) {
      const inheritedEntries = readInheritedPath(baseEnv)
        .split(delimiter)
        .filter((entry) => entry.length > 0 && entry !== shimDir)
      baseEnv.PATH = [shimDir, ...inheritedEntries].join(delimiter)
    }
  }

  // Why: GitHub attribution should only affect commands launched from
  // Orca's own PTYs. Injecting lightweight PATH shims at spawn-time keeps
  // the behavior local to Orca instead of rewriting user git config or
  // touching external shells.
  if (!opts.githubAttributionEnabled) {
    delete baseEnv.ORCA_ENABLE_GIT_ATTRIBUTION
    delete baseEnv.ORCA_GIT_COMMIT_TRAILER
    delete baseEnv.ORCA_GH_PR_FOOTER
    delete baseEnv.ORCA_GH_ISSUE_FOOTER
    delete baseEnv.ORCA_ATTRIBUTION_SHIM_DIR
  }
  applyTerminalAttributionEnv(baseEnv, {
    enabled: opts.githubAttributionEnabled,
    userDataPath: opts.userDataPath,
    shellFamily: resolveAttributionShellFamily({
      shellPath: opts.shellPath,
      isWsl: opts.isWsl
    })
  })

  return baseEnv
}

export {
  CODEX_HOME_ENV_KEYS,
  getCodexSelectionTargetForPty,
  getCompatibleSelectedCodexHomePath,
  promoteAgentTeamsShimPath,
  deleteRequestedEnvKeys,
  mergePtyEnvDeletions,
  getInheritedAgentHookEnvKeysToDelete,
  shouldSkipCodexHomeEnvForWindowsShell,
  type GetSelectedCodexHomePath,
  type PrepareClaudeAuth
}
