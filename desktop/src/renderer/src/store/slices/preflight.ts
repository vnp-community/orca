import type { StateCreator } from 'zustand'
import type { PreflightRuntimeContext, PreflightStatus } from '../../../../preload/api-types'
import type { AppState } from '../types'
import type { RemotePreflightStatus } from '../../../../shared/dev-server-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import {
  getLocalPreflightContext,
  localPreflightContextKey,
  type LocalPreflightContext
} from '@/lib/local-preflight-context'
import { Tracers } from '../../../../shared/trace/tracers'

export type PreflightSlice = {
  preflightStatus: PreflightStatus | null
  preflightStatusChecked: boolean
  preflightStatusContextKey: string | null
  preflightStatusLoading: boolean
  preflightStatusError: string | null
  // ── Remote preflight (CR-OB-005) ──────────────────────────────────────────
  remotePreflightByServer: Record<string, RemotePreflightStatus>
  activeRemotePreflightStatus: RemotePreflightStatus | null

  refreshPreflightStatus: (options?: { force?: boolean }) => Promise<void>
  setRemotePreflightStatus: (devServerId: string, status: RemotePreflightStatus) => void
  clearRemotePreflightStatus: (devServerId: string) => void
}

let nonForcedPreflightRequest: { key: string; promise: Promise<void> } | null = null
let forcedPreflightRequest: { key: string; promise: Promise<void> } | null = null
let latestPreflightRequestId = 0

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Failed to check integrations.'
}

function buildPreflightArgs(
  force: boolean,
  context: LocalPreflightContext
): (PreflightRuntimeContext & { force?: boolean }) | undefined {
  const wslDistro = context?.wslDistro
  const wslDefault = context?.wslDefault === true
  const projectRuntime = context?.projectRuntime
  if (!force && !wslDistro && !wslDefault && !projectRuntime) {
    return undefined
  }
  return {
    ...(force ? { force: true } : {}),
    ...(projectRuntime ? { projectRuntime } : {}),
    ...(wslDistro ? { wslDistro } : {}),
    ...(wslDefault ? { wslDefault: true } : {})
  }
}

export const createPreflightSlice: StateCreator<AppState, [], [], PreflightSlice> = (set, get) => ({
  preflightStatus: null,
  preflightStatusChecked: false,
  preflightStatusContextKey: null,
  preflightStatusLoading: false,
  preflightStatusError: null,
  remotePreflightByServer: {},
  activeRemotePreflightStatus: null,

  setRemotePreflightStatus: (devServerId, status) =>
    set((state) => {
      const updated = { ...state.remotePreflightByServer, [devServerId]: status }
      return {
        remotePreflightByServer: updated,
        activeRemotePreflightStatus:
          devServerId === state.activeDevServerId ? status : state.activeRemotePreflightStatus,
      }
    }),

  clearRemotePreflightStatus: (devServerId) =>
    set((state) => {
      const { [devServerId]: _, ...rest } = state.remotePreflightByServer
      return { remotePreflightByServer: rest }
    }),

  refreshPreflightStatus: async (options) => {
    const force = options?.force === true
    const context = getLocalPreflightContext(get())
    const contextKey = localPreflightContextKey(context)
    if (!force && forcedPreflightRequest?.key === contextKey) {
      return forcedPreflightRequest.promise
    }
    if (!force && nonForcedPreflightRequest?.key === contextKey) {
      return nonForcedPreflightRequest.promise
    }
    if (force && forcedPreflightRequest?.key === contextKey) {
      return forcedPreflightRequest.promise
    }

    const requestId = ++latestPreflightRequestId
    const contextChanged = get().preflightStatusContextKey !== contextKey
    const runtimeTarget = getActiveRuntimeTarget(get().settings)
    const preflightArgs = buildPreflightArgs(force, context)
    set({
      preflightStatus: contextChanged ? null : get().preflightStatus,
      preflightStatusChecked: contextChanged ? false : get().preflightStatusChecked,
      preflightStatusLoading: true,
      preflightStatusError: null
    })

    // Why: span bọc toàn bộ request kể cả khi bị coalesce bởi 3 guard phía trên —
    // mỗi request THẬT (không return sớm) là 1 user-perceived "check" action.
    const span = Tracers.uiRemoteIntegrationPreflightFlow.start({ force, mode: runtimeTarget.kind })

    const request = (
      runtimeTarget.kind === 'environment'
        ? (() => {
            // Why: in web/session-auth mode, preflight.check must route to the
            // active Dev Server's relay (where gh/git/glab are installed), not
            // run locally on the Orca Server container (no tools there).
            const activeDevServerId = get().activeDevServerId
            const params: Record<string, unknown> = force ? { force } : {}
            if (activeDevServerId) params.devServerId = activeDevServerId
            // Why: traceId forwarded only over WS RPC so the Dev Server relay's
            // remoteIntegration:preflight span can resume this same trace id —
            // Electron IPC below is same-machine and has no downstream hop to resume.
            params.traceId = span.id
            span.step('relayDelegate', { devServerId: activeDevServerId ?? '' })
            return callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', params)
          })()
        : window.api.preflight.check(preflightArgs)
    )
      .then((status) => {
        if (requestId !== latestPreflightRequestId) {
          span.ok({ stale: true })
          return
        }
        set({
          preflightStatus: status,
          preflightStatusChecked: true,
          preflightStatusContextKey: contextKey,
          preflightStatusLoading: false,
          preflightStatusError: null
        })
        // Why: field names match the real `PreflightStatus` shape (`gh`/`glab`,
        // not `ghStatus`/`glabStatus` as the original spec sample assumed —
        // see api-types.ts:592-598).
        span.ok({
          ghAuthenticated: Boolean(status?.gh?.authenticated),
          glabAuthenticated: Boolean(status?.glab?.authenticated)
        })
      })
      .catch((error) => {
        if (requestId !== latestPreflightRequestId) {
          span.fail(error, { stale: true })
          return
        }
        set({
          preflightStatusChecked: true,
          preflightStatusContextKey: contextKey,
          preflightStatusLoading: false,
          preflightStatusError: getErrorMessage(error)
        })
        span.fail(error, { force, mode: runtimeTarget.kind })
      })
      .finally(() => {
        if (!force && nonForcedPreflightRequest?.promise === request) {
          nonForcedPreflightRequest = null
        }
        if (force && forcedPreflightRequest?.promise === request) {
          forcedPreflightRequest = null
        }
      })

    if (!force) {
      nonForcedPreflightRequest = { key: contextKey, promise: request }
    } else {
      forcedPreflightRequest = { key: contextKey, promise: request }
    }

    return request
  }
})
