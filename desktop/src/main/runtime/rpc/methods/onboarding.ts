// ─── Onboarding RPC Methods ───────────────────────────────────────────────────
// Wraps the onboarding.* ipcMain handlers (desktop/src/main/ipc/onboarding-ipc.ts,
// desktop/src/main/ipc/onboarding.ts) so the same logic — dev-server agent
// detection, remote preflight/git-identity/gh-auth/Windows-capability probes,
// and the local onboarding checklist state — is reachable over the runtime RPC
// channel, not only through Electron's window.api.onboarding bridge. Handlers
// call the exact same extracted functions the ipcMain handlers call; no logic
// is duplicated.
import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import {
  detectAgentsAllDevServers,
  detectAgentsForDevServer,
  detectGhosttyConfigForDevServer,
  detectWindowsCapabilitiesForDevServer,
  getPreflightStatusForDevServer,
  markOnboardingChecklistItem,
  openGhAuthTerminalForDevServer,
  setGitIdentityForDevServer
} from '../../../ipc/onboarding-ipc'
import { getActiveOnboardingStore } from '../../../ipc/onboarding'
import { sanitizeOnboardingUpdate } from '../../../persistence'
import type {
  OnboardingChecklistState,
  PerServerChecklistState
} from '../../../../shared/types'

// ── Schemas ──────────────────────────────────────────────────────────────────

const OnboardingDetectAgents = z.object({
  devServerId: z.string().nullable()
})

const OnboardingDevServerIdOnly = z.object({
  devServerId: z.string().min(1)
})

const OnboardingGetPreflightStatus = z.object({
  devServerId: z.string().min(1),
  force: z.boolean().optional()
})

const OnboardingSetGitIdentity = z.object({
  devServerId: z.string().min(1),
  name: z.string(),
  email: z.string()
})

const OnboardingMarkChecklistItem = z.object({
  item: z.string().min(1),
  devServerId: z.string().optional(),
  value: z.boolean().optional()
})

// ── Dev-server-backed methods ────────────────────────────────────────────────
// Why: ctx.devServerManager is injected by RpcDispatcher from
// ServerBootstrapResult.devServerManager — undefined only in the Electron
// local-IPC dispatch path (which the frontend wrapper never routes through
// window.api.runtime.call for onboarding; it calls window.api.onboarding.*
// directly there instead). Guard matches the sibling dev-server.ts pattern.

export const ONBOARDING_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'onboarding.detectAgents',
    params: OnboardingDetectAgents,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      return detectAgentsForDevServer(ctx.devServerManager, params)
    }
  }),
  defineMethod({
    name: 'onboarding.detectAgentsAllServers',
    params: null,
    handler: async (_params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      return detectAgentsAllDevServers(ctx.devServerManager)
    }
  }),
  defineMethod({
    name: 'onboarding.getPreflightStatus',
    params: OnboardingGetPreflightStatus,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      return getPreflightStatusForDevServer(ctx.devServerManager, params)
    }
  }),
  defineMethod({
    name: 'onboarding.setGitIdentity',
    params: OnboardingSetGitIdentity,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      return setGitIdentityForDevServer(ctx.devServerManager, params)
    }
  }),
  defineMethod({
    name: 'onboarding.detectGhosttyConfig',
    params: OnboardingDevServerIdOnly,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      return detectGhosttyConfigForDevServer(ctx.devServerManager, params)
    }
  }),
  defineMethod({
    name: 'onboarding.openGhAuthTerminal',
    params: OnboardingDevServerIdOnly,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      return openGhAuthTerminalForDevServer(ctx.devServerManager, params)
    }
  }),
  defineMethod({
    name: 'onboarding.detectWindowsCapabilities',
    params: OnboardingDevServerIdOnly,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {throw new Error('DevServerManager unavailable')}
      return detectWindowsCapabilitiesForDevServer(ctx.devServerManager, params)
    }
  }),
  // ── Store-backed methods ───────────────────────────────────────────────────
  // Why: reads the store lazily via getActiveOnboardingStore() (set by
  // registerOnboardingHandlers at real bootstrap time in
  // ipc/register-core-handlers.ts) instead of closing over an eagerly-passed
  // instance — this file is a static array evaluated at module load, before
  // the store exists, so it can't take `store` as a constructor argument the
  // way a factory would. Throws a clear error if called before bootstrap
  // finishes, same convention as the dev-server-backed methods above.
  defineMethod({
    name: 'onboarding.get',
    params: null,
    handler: () => {
      const store = getActiveOnboardingStore()
      if (!store) {throw new Error('onboarding_store_unavailable')}
      return store.getOnboarding()
    }
  }),
  defineMethod({
    name: 'onboarding.update',
    params: z.unknown(),
    handler: (params) => {
      const store = getActiveOnboardingStore()
      if (!store) {throw new Error('onboarding_store_unavailable')}
      return store.updateOnboarding(sanitizeOnboardingUpdate(params))
    }
  }),
  defineMethod({
    name: 'onboarding.markChecklistItem',
    params: OnboardingMarkChecklistItem,
    handler: (params) => {
      const store = getActiveOnboardingStore()
      if (!store) {throw new Error('onboarding_store_unavailable')}
      markOnboardingChecklistItem(store, {
        item: params.item as keyof OnboardingChecklistState | keyof PerServerChecklistState,
        devServerId: params.devServerId,
        value: params.value
      })
      return { marked: true }
    }
  })
]
