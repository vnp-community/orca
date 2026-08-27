import type { AiVaultListArgs, AiVaultListResult } from '../../../shared/ai-vault-types'

// Why: AI Vault does its own multi-host fan-out (local + every configured
// SSH/runtime host) *inside* window.api.aiVault itself — desktop's IPC
// handler scans each host via `executionHostScope`, and the web shim
// resolves the browser's paired runtime — independent of the "focus mode"
// concept (activeRuntimeEnvironmentId) the other runtime-*-client wrappers
// branch on. Routing this through callRuntimeRpc would collapse that
// multi-host scan down to a single focused environment, so these are
// deliberate passthroughs kept only so no call site touches
// window.api.aiVault directly.
export async function listRuntimeAiVaultSessions(
  args: AiVaultListArgs
): Promise<AiVaultListResult> {
  return window.api.aiVault.listSessions(args)
}

export function subscribeRuntimeAiVaultWindowFocus(onRefocus: () => void): (() => void) | undefined {
  return window.api.aiVault.onWindowFocused?.(onRefocus)
}
