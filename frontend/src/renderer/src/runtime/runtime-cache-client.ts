import type { PersistedState } from '../../../shared/types'
import { callRuntimeRpc } from './runtime-rpc-client'

// Why: `cache:getGitHub` / `cache:setGitHub` are native/local-only Electron
// IPC channels backing the renderer's GitHub PR/issue cache, with zero prior
// RPC coverage — routed through window.api.runtime.call for the same uniform
// calling convention as every other runtime-*-client. Gated on
// `window.api.agentTrust` (desktop-only) rather than `window.api.runtime` —
// see runtime-computer-use-permissions-client.ts's Why; web keeps using its
// existing `window.api.cache` localStorage-backed stub untouched.
function isDesktopElectronBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.api?.agentTrust)
}

export async function getRuntimeGitHubCache(): Promise<PersistedState['githubCache'] | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<PersistedState['githubCache']>(
    { kind: 'local' },
    'cache.getGitHub',
    undefined,
    { timeoutMs: 15_000 }
  )
}

export async function setRuntimeGitHubCache(
  cache: PersistedState['githubCache']
): Promise<void> {
  if (!isDesktopElectronBridge()) {
    return
  }
  await callRuntimeRpc<void>(
    { kind: 'local' },
    'cache.setGitHub',
    { cache },
    { timeoutMs: 15_000 }
  )
}
