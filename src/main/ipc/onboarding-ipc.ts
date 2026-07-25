// ─── onboarding-ipc.ts ────────────────────────────────────────────────────────
// IPC handlers for the onboarding namespace: agent detection with TTL cache.
// Registered in server-bootstrap.ts after DevServerManager is initialized.

import { ipcMain } from 'electron'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { Store } from '../persistence'
import { buildAgentDetectionCommands } from '../../shared/agent-detection-commands'
import type { RemotePreflightStatus, WindowsTerminalCapabilities } from '../../shared/dev-server-types'
import type { OnboardingChecklistState, PerServerChecklistState } from '../../shared/types'

// ── Cache ─────────────────────────────────────────────────────────────────────

// Per-server cache keyed by devServerId.
// Exported so tests can reset between cases without full module re-require.
export const agentDetectionCache = new Map<
  string,
  {
    result: { agents: string[]; platform: NodeJS.Platform | null }
    cachedAt: number
  }
>()

export const AGENT_DETECTION_CACHE_TTL_MS = 60_000

function getCachedDetection(
  devServerId: string
): { agents: string[]; platform: NodeJS.Platform | null } | null {
  const entry = agentDetectionCache.get(devServerId)
  if (entry && Date.now() - entry.cachedAt < AGENT_DETECTION_CACHE_TTL_MS) {
    return entry.result
  }
  return null
}

// ── IPC channel names ─────────────────────────────────────────────────────────

const ONBOARDING_IPC_CHANNELS = [
  'onboarding.detectAgents',
  'onboarding.detectAgentsAllServers',
  'onboarding.getPreflightStatus',
  'onboarding.setGitIdentity',
  'onboarding.detectGhosttyConfig',
  'onboarding.openGhAuthTerminal',
  'onboarding.detectWindowsCapabilities',
  'onboarding.markChecklistItem'
] as const

// ── Windows capabilities cache (Phase 3) ─────────────────────────────────────

// Per-server cache keyed by devServerId — TTL 60 seconds.
// Exported so tests can reset between cases.
export const windowsCapsCache = new Map<string, { result: WindowsTerminalCapabilities; cachedAt: number }>()
export const WINDOWS_CAPS_CACHE_TTL_MS = 60_000

// ── Preflight cache (Phase 2) ─────────────────────────────────────────────────

// Per-server preflight cache keyed by devServerId — TTL 30 seconds.
// Exported so tests can reset between cases without full module re-require.
export const preflightCache = new Map<string, { result: RemotePreflightStatus; cachedAt: number }>()
export const PREFLIGHT_CACHE_TTL_MS = 30_000

// ── Registration ──────────────────────────────────────────────────────────────

export function registerOnboardingIpcHandlers(
  devServerManager: DevServerManager,
  store?: Store
): void {
  // Idempotent: clear any stale handlers from previous registration.
  for (const channel of ONBOARDING_IPC_CHANNELS) {
    ipcMain.removeHandler(channel)
  }

  // ── onboarding.detectAgents ─────────────────────────────────────────────────

  ipcMain.handle(
    'onboarding.detectAgents',
    async (
      _event,
      params: { devServerId: string | null }
    ): Promise<{
      agents: string[]
      platform: NodeJS.Platform | null
      devServerId: string | null
    }> => {
      const { devServerId } = params

      if (!devServerId) {
        return { agents: [], platform: null, devServerId: null }
      }

      // Check cache first (TTL 60s)
      const cached = getCachedDetection(devServerId)
      if (cached) {
        return { ...cached, devServerId }
      }

      const relay = devServerManager.getRelay(devServerId)
      if (!relay) {
        throw new Error(`Dev server '${devServerId}' not connected`)
      }

      const commands = buildAgentDetectionCommands()
      const result = await relay.detectAgents(commands)

      // Store in cache only on success
      agentDetectionCache.set(devServerId, {
        result: { agents: result.agents, platform: result.platform },
        cachedAt: Date.now()
      })

      return {
        agents: result.agents,
        platform: result.platform,
        devServerId
      }
    }
  )

  // ── onboarding.detectAgentsAllServers ───────────────────────────────────────

  ipcMain.handle(
    'onboarding.detectAgentsAllServers',
    async (): Promise<
      Record<
        string,
        {
          agents: string[]
          platform: NodeJS.Platform | null
          error?: string
        }
      >
    > => {
      // Only query servers that are currently connected
      const servers = devServerManager.list().filter((ds) => ds.status === 'connected')

      const results = await Promise.allSettled(
        servers.map(async (ds) => {
          const relay = devServerManager.getRelay(ds.id)!
          const commands = buildAgentDetectionCommands()
          const result = await relay.detectAgents(commands)
          return { id: ds.id, agents: result.agents, platform: ds.platform }
        })
      )

      const out: Record<
        string,
        { agents: string[]; platform: NodeJS.Platform | null; error?: string }
      > = {}
      results.forEach((r, i) => {
        const serverId = servers[i].id
        if (r.status === 'fulfilled') {
          out[serverId] = { agents: r.value.agents, platform: r.value.platform }
        } else {
          out[serverId] = {
            agents: [],
            platform: servers[i].platform,
            error: (r.reason as Error)?.message ?? 'Unknown error'
          }
        }
      })
      return out
    }
  )

  // ── onboarding.getPreflightStatus (Phase 2) ─────────────────────────────────

  ipcMain.handle(
    'onboarding.getPreflightStatus',
    async (
      _event,
      params: { devServerId: string; force?: boolean }
    ): Promise<RemotePreflightStatus> => {
      const { devServerId, force = false } = params

      // Cache hit: skip relay call unless force=true
      if (!force) {
        const cached = preflightCache.get(devServerId)
        if (cached && Date.now() - cached.cachedAt < PREFLIGHT_CACHE_TTL_MS) {
          return cached.result
        }
      }

      const relay = devServerManager.getRelay(devServerId)
      if (!relay) throw new Error(`Dev server '${devServerId}' not connected`)

      const raw = await relay.call<{
        platform: NodeJS.Platform
        gh: { installed: boolean; authenticated: boolean; version?: string }
        git: { installed: boolean; version?: string; hasUserName: boolean; hasUserEmail: boolean }
      }>('preflight.check', {}, 30_000)

      const result: RemotePreflightStatus = {
        devServerId,
        platform: raw.platform,
        checkedAt: Date.now(),
        gh: raw.gh,
        git: raw.git
      }
      preflightCache.set(devServerId, { result, cachedAt: Date.now() })
      return result
    }
  )

  // ── onboarding.setGitIdentity (Phase 2) ─────────────────────────────────────

  ipcMain.handle(
    'onboarding.setGitIdentity',
    async (
      _event,
      params: { devServerId: string; name: string; email: string }
    ): Promise<void> => {
      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) throw new Error(`Dev server '${params.devServerId}' not connected`)
      await relay.call('preflight.setGitIdentity', {
        name: params.name,
        email: params.email
      })
      // Invalidate preflight cache so next getPreflightStatus is fresh
      preflightCache.delete(params.devServerId)
    }
  )

  // ── onboarding.detectGhosttyConfig (Phase 2) ─────────────────────────────────

  ipcMain.handle(
    'onboarding.detectGhosttyConfig',
    async (
      _event,
      params: { devServerId: string }
    ): Promise<{ configPath: string | null; themeDir: string | null }> => {
      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) throw new Error('Dev server not connected')
      return relay.call<{ configPath: string | null; themeDir: string | null }>(
        'preflight.detectGhosttyConfig',
        {}
      )
    }
  )

  // ── onboarding.openGhAuthTerminal (Phase 2) ──────────────────────────────────
  // Why: gh auth login is interactive — run it in a remote PTY on the dev server
  // and stream the output back to the renderer via the existing PTY bridge.
  // The relay's 'pty.create' method is used (same as terminal PTY sessions).

  ipcMain.handle(
    'onboarding.openGhAuthTerminal',
    async (
      _event,
      params: { devServerId: string }
    ): Promise<{ ptyId: string; devServerId: string }> => {
      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) throw new Error('Dev server not connected')
      // Why: call the relay's pty.spawn to spawn 'gh auth login' in a remote PTY.
      // The ptyId is returned to the renderer to subscribe to pty output events.
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'gh',
        args: ['auth', 'login'],
        env: {},
        cols: 120,
        rows: 30
      })
      return { ptyId, devServerId: params.devServerId }
    }
  )

  // ── onboarding.detectWindowsCapabilities (Phase 3) ───────────────────────────
  // Why: Windows terminal configuration differs per dev server. This handler
  // enforces that only win32 dev servers can be queried, and caches results
  // for 60s to avoid repeated relay calls from the UI polling.

  ipcMain.handle(
    'onboarding.detectWindowsCapabilities',
    async (
      _event,
      params: { devServerId: string }
    ): Promise<WindowsTerminalCapabilities> => {
      const devServer = devServerManager.get(params.devServerId)
      if (!devServer) throw new Error(`Dev server '${params.devServerId}' not found`)
      if (devServer.platform !== 'win32') {
        throw new Error(
          `Dev server '${params.devServerId}' is not Windows (platform: ${devServer.platform ?? 'unknown'})`
        )
      }

      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) throw new Error(`Dev server '${params.devServerId}' not connected`)

      // Cache hit: serve from cache if still fresh
      const cacheKey = params.devServerId
      const cached = windowsCapsCache.get(cacheKey)
      if (cached && Date.now() - cached.cachedAt < WINDOWS_CAPS_CACHE_TTL_MS) {
        return cached.result
      }

      const result = await relay.call<WindowsTerminalCapabilities>(
        'preflight.detectWindowsTerminalCapabilities',
        {}
      )
      windowsCapsCache.set(cacheKey, { result, cachedAt: Date.now() })
      return result
    }
  )

  // ── onboarding.markChecklistItem ────────────────────────────────────────────
  // TASK-039: mark a global or per-server checklist item.
  // devServerId absent → global item; present → per-server item.

  ipcMain.handle(
    'onboarding.markChecklistItem',
    async (
      _event,
      params: {
        item: keyof OnboardingChecklistState | keyof PerServerChecklistState
        devServerId?: string
        value?: boolean
      }
    ): Promise<void> => {
      if (!store) {
        // store is optional for backward compat — skip silently in headless tests
        return
      }
      const { item, devServerId, value = true } = params
      store.mutate((state) => {
        const cl = state.onboarding?.checklist ?? {}
        if (devServerId) {
          // Per-server item
          cl.perServer = cl.perServer ?? {}
          cl.perServer[devServerId] = cl.perServer[devServerId] ?? {}
          ;(cl.perServer[devServerId] as Record<string, unknown>)[item] = value
        } else {
          // Global item
          ;(cl as Record<string, unknown>)[item] = value
        }
        if (!state.onboarding) {
          state.onboarding = {
            flowVersion: 1,
            closedAt: null,
            outcome: null,
            lastCompletedStep: -1,
            checklist: cl as OnboardingChecklistState
          }
        }
        state.onboarding.checklist = cl as OnboardingChecklistState
      })
    }
  )
}
