// ─── Onboarding RPC Methods ───────────────────────────────────────────────────
// Ports desktop/src/main/runtime/rpc/methods/onboarding.ts to backend/server
// mode. Handlers reuse the exact same extracted functions
// backend/src/main/ipc/onboarding-ipc.ts already exposes (agent detection,
// remote preflight/git-identity/gh-auth/Windows-capability probes via
// DevServerManager) plus the local onboarding checklist Store methods —
// no detection/persistence logic is duplicated here.
//
// Every dev-server-backed method below delegates through ctx.devServerManager,
// which — unlike Electron/local mode, where it's undefined — is always
// injected by RpcDispatcher in backend/server mode (see RpcContext in
// ../core.ts). That's why onboarding.detectGhosttyConfig and
// onboarding.openGhAuthTerminal are real, working implementations here too:
// they proxy to the connected Dev Server's relay/PTY, not to the Orca Server
// host itself, so there's no "desktop-only" local-machine assumption baked
// into them.
import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import {
  detectAgentsAllDevServers,
  detectAgentsForDevServer,
  detectGhosttyConfigForDevServer,
  detectWindowsCapabilitiesForDevServer,
  getActiveOnboardingStore,
  getPreflightStatusForDevServer,
  markOnboardingChecklistItem,
  openGhAuthTerminalForDevServer,
  setGitIdentityForDevServer
} from '../../../ipc/onboarding-ipc'
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
      return openGhAuthTerminalForDevServer(params)
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
  // registerOnboardingIpcHandlers at real bootstrap time in
  // server-bootstrap.ts) instead of closing over an eagerly-passed instance —
  // this file is a static array evaluated at module load, before the store
  // exists. Throws a clear error if called before bootstrap finishes, same
  // convention as the dev-server-backed methods above.
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
