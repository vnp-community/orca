import type { GlobalSettings } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

// Why: credential management (Web Server mode / ORCA_MULTI_USER=1) is backed
// by the same `credentials.*` RPC methods on every host — Electron mode has
// no separate native handler, it just answers with mode:'electron' stubs
// (see backend/src/main/runtime/rpc/methods/credentials.ts). Route through the
// active runtime target the same way as the other runtime-*-client wrappers so
// a paired web client calls the RPC directly instead of round-tripping
// through window.api.credentials.

export type RuntimeCredentialService = 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'

export type RuntimeCredentialStatus = {
  configured: boolean
  mode: string
  config?: Record<string, string>
}

export async function setRuntimeCredential(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  service: RuntimeCredentialService,
  token: string,
  config?: Record<string, string>
): Promise<{ success: boolean }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.credentials.set(service, token, config)
  }
  return callRuntimeRpc<{ success: boolean }>(
    target,
    'credentials.set',
    { service, token, config },
    { timeoutMs: 15_000 }
  )
}

export async function revokeRuntimeCredential(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  service: RuntimeCredentialService
): Promise<{ success: boolean }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.credentials.revoke(service)
  }
  return callRuntimeRpc<{ success: boolean }>(
    target,
    'credentials.revoke',
    { service },
    { timeoutMs: 15_000 }
  )
}

export async function getRuntimeCredentialStatus(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  service: RuntimeCredentialService
): Promise<RuntimeCredentialStatus> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.credentials.status(service)
  }
  return callRuntimeRpc<RuntimeCredentialStatus>(
    target,
    'credentials.status',
    { service },
    { timeoutMs: 15_000 }
  )
}

export async function listRuntimeCredentials(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined
): Promise<{ services: string[]; mode: string }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.credentials.list()
  }
  return callRuntimeRpc<{ services: string[]; mode: string }>(
    target,
    'credentials.list',
    undefined,
    { timeoutMs: 15_000 }
  )
}
