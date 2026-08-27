import type { E2EConfig } from '../../../shared/e2e-config'
import { createE2EConfig } from '../../../shared/e2e-config'
import { callRuntimeRpc } from './runtime-rpc-client'

// Why: `e2e.getConfig` is a synchronous preload-time read on desktop
// (`window.api.e2e.getConfig()`), so this RPC wrapper is not on the startup
// critical path — it exists for the uniform RPC calling convention (e.g. an
// external RPC caller that never loads the preload bridge). Renderer startup
// code should keep using `window.api.e2e.getConfig()` directly.
export async function getRuntimeE2EConfig(): Promise<E2EConfig> {
  if (typeof window === 'undefined' || !window.api?.agentTrust) {
    return createE2EConfig({})
  }
  return callRuntimeRpc<E2EConfig>({ kind: 'local' }, 'e2e.getConfig', undefined, {
    timeoutMs: 15_000
  })
}
