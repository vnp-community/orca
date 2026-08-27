import { callRuntimeRpc } from './runtime-rpc-client'

export type RuntimeAgentTrustPreset = 'cursor' | 'copilot' | 'codex'

// Why: agentTrust is a native/local-only Electron feature (writes trust
// artifacts to disk for cursor-agent/Copilot CLI/Codex) with zero prior RPC
// coverage — this wrapper routes it through the same window.api.runtime.call
// convention as every other runtime-*-client so callers have one calling
// shape regardless of whether the underlying feature happens to be local.
// `window.api.agentTrust` only exists on the real Electron preload bridge
// (never on web-preload-api.ts), so its presence is also this module's
// "are we actually in desktop Electron" gate — routing an RPC call at
// `window.api.runtime.call` on web would hit the backend registry, which
// intentionally has no `agentTrust.*` methods (there is no web equivalent of
// "trust this local folder for an agent CLI").
export async function markRuntimeAgentTrusted(args: {
  preset: RuntimeAgentTrustPreset
  workspacePath: string
  connectionId?: string
}): Promise<void> {
  if (typeof window === 'undefined' || !window.api?.agentTrust) {
    return
  }
  await callRuntimeRpc<void>(
    { kind: 'local' },
    'agentTrust.markTrusted',
    args,
    { timeoutMs: 15_000 }
  )
}
