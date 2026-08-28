// github.auth.status / gitlab.auth.status registration for Part B
// (RelayDispatcher, relay-ssh mode) — closes the Part A/Part B divergence
// infra-fleet-service.md §10 flags by name (TASK-INT-01-02/SOL-INT-01).
// Reuses external-api-connector.ts's handleGitHubAuthStatus/
// handleGitLabAuthStatus verbatim — the same transport-agnostic handlers
// Part A's agent-rpc-dispatch.ts already imports — rather than duplicating
// the gh/glab exec logic here. Extracted into its own module (out of
// relay.ts's already-long main()) so it is unit-testable against a real
// RelayDispatcher instance.
import type { RelayDispatcher } from './dispatcher'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

/** The subset of a JsonRpcResponse handleGitHubAuthStatus/
 * handleGitLabAuthStatus actually return — result XOR error, id omitted
 * since RelayDispatcher assigns the wire id itself. */
type ExternalApiRpcResult = {
  result?: unknown
  error?: { code: number; message: string }
}

function unwrapExternalApiResponse(response: ExternalApiRpcResult): unknown {
  if (response.error) {
    // Why: RelayDispatcher.handleRequest reads a thrown error's `.code`
    // (falling back to -32000) to build the JSON-RPC error frame — see
    // dispatcher.ts's handleRequest catch block.
    throw Object.assign(new Error(response.error.message), { code: response.error.code })
  }
  return response.result
}

/**
 * Registers 'github.auth.status'/'gitlab.auth.status' on dispatcher.
 * config/log are resolved once by the caller (relay.ts has no long-lived
 * AgentConfig of its own, unlike the WS-mode entry point — see relay.ts's
 * call site for why loadAgentConfig()/a relayLogLine-backed logger are
 * passed in rather than constructed here).
 */
export function registerAuthStatusHandlers(
  dispatcher: RelayDispatcher,
  config: AgentConfig,
  log: AgentLogger
): void {
  dispatcher.onRequest('github.auth.status', async (params) => {
    const { handleGitHubAuthStatus } = await import('./external-api-connector')
    const response = await handleGitHubAuthStatus(null, params, config, log)
    return unwrapExternalApiResponse(response as ExternalApiRpcResult)
  })
  dispatcher.onRequest('gitlab.auth.status', async (params) => {
    const { handleGitLabAuthStatus } = await import('./external-api-connector')
    const response = await handleGitLabAuthStatus(null, params, config, log)
    return unwrapExternalApiResponse(response as ExternalApiRpcResult)
  })
}
