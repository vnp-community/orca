import type { MemorySnapshot } from '../../../shared/types'
import { callRuntimeRpc } from './runtime-rpc-client'

// Why: `memory:getSnapshot` is a native/local-only Electron IPC channel with
// zero prior RPC coverage — routed through window.api.runtime.call for the
// same uniform calling convention as every other runtime-*-client. Not to be
// confused with the unrelated `diagnostics.memory` RPC method. Gated on
// `window.api.agentTrust` (desktop-only) rather than `window.api.runtime` —
// see runtime-computer-use-permissions-client.ts's Why; web keeps using its
// existing `window.api.memory.getSnapshot()` empty-snapshot stub untouched.
export async function getRuntimeMemorySnapshot(): Promise<MemorySnapshot | null> {
  if (typeof window === 'undefined' || !window.api?.agentTrust) {
    return null
  }
  return callRuntimeRpc<MemorySnapshot>(
    { kind: 'local' },
    'memory.getSnapshot',
    undefined,
    { timeoutMs: 15_000 }
  )
}
