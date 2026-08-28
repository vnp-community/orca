# TASK-AIP-SHARED-01: Implement `ai.testProviderConnection` JSON-RPC handler in the Dev Server Agent

**From Solution:** SOL-AIP-01, SOL-AIP-03
**Priority:** P0
**Service:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-rpc-dispatch.ts`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Both `SOL-AIP-01`'s test-before-save gate (`verify_connection.go`, see
`TASK-AIP-01-05`) and `SOL-AIP-03`'s 15-minute health-check job (see
`TASK-AIP-03-04`) call `InfraFleetClient.Relay(ctx, devServerID,
"ai.testProviderConnection", {"credentialRef": ..., "providerType": ...})`
and parse the reply as `{success: bool, message: string}`
(`backend-go/services/ai-provider-service/internal/usecase/test_connection.go:40-65`).
This method **does not exist on the agent side today** —
`agent-rpc-dispatch.ts` only has `ai.provider.testConnection` (note the
different segment order), which delegates to `handleHealthCheck` and takes
`{accountId, provider}` params, returning a `{ok, latencyMs, note,
credentialFound}` shape — a different method name AND a different
params/result contract, not just a naming variant. Do this task **once**;
both backend-go solutions depend on it rather than each building their own
handler.

This handler cannot do a full authenticated provider call yet:
`credentialRef` is a credential-broker-service pointer, and this repo's
ciphertext-push path (`PushCiphertext`, README "Known gaps") isn't
implemented, so there's no plaintext key available locally to authenticate
with. Implement the honest subset today — validate params, look up
whatever local credential material exists under `credentialRef` (reusing
the existing accountId-keyed local credential store as a best-effort
lookup, since `credentialRef` is the closest stand-in until
`PushCiphertext` lands), and fall back to the existing reachability check
(`checkProviderReachabilityDetailed`) keyed by `providerType`. Flag the
credential-lookup gap in a code comment rather than silently faking a
"key is valid" result.

## Changes to make

In `agent/src/relay/agent-credential-store.ts`, add a new handler next to
`handleTestConnection`/`handleHealthCheck`:

```ts
// ── ai.testProviderConnection ──────────────────────────────────────────
// Called by ai-provider-service's verifyConnection (backend-go) — see
// specs/backend-go/bugs/logic-v1/tasks/TASK-AIP-SHARED-01-add-agent-test-connection-rpc.md.
// Distinct from ai.provider.testConnection (accountId-keyed, delegates to
// handleHealthCheck): this method is keyed by credentialRef + providerType
// and returns {success, message} to match backend-go's ConnectionTestResult
// exactly, not the {ok, latencyMs, note} shape the older method uses.
export async function handleTestProviderConnection(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const credentialRef = typeof params.credentialRef === 'string' ? params.credentialRef : ''
  const providerType   = typeof params.providerType  === 'string' ? params.providerType  : ''

  if (!credentialRef || !providerType) {
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required params: credentialRef, providerType' },
    }
  }

  const span = credTracer.start({ method: 'ai.testProviderConnection', providerType })

  // credentialRef is a credential-broker-service pointer; this agent's local
  // credential store is still keyed by accountId (ai.provider.writeCredential),
  // and the ciphertext-push path (PushCiphertext) that would make credentialRef
  // resolvable locally isn't implemented yet. Treating credentialRef as the
  // local store key is a best-effort stand-in — if nothing is found, that's
  // not necessarily "no credential", so it does NOT fail the check outright,
  // it just skips straight to the reachability probe below.
  const credResult = await handleReadCredential(id, { accountId: credentialRef }, config, log) as { error?: unknown }
  const credentialFound = !credResult.error

  const reachability = await checkProviderReachabilityDetailed(providerType)
  log.info(`ai.testProviderConnection: providerType=${providerType} credentialFound=${credentialFound} ok=${reachability.ok} note=${reachability.note}`)

  if (reachability.ok) {
    span.ok({ providerType, note: reachability.note })
  } else {
    span.fail(reachability.note, { providerType })
  }

  return {
    jsonrpc: '2.0', id,
    result: {
      success: reachability.ok,
      message: credentialFound ? reachability.note : `${reachability.note} (credential not found locally for this ref — ciphertext-push path not yet implemented)`,
    },
  }
}
```

In `agent/src/relay/agent-rpc-dispatch.ts`, add the routing case in the
`ai.*` group (right after the existing `ai.provider.testConnection` case,
~line 705):

```ts
    // ── ai.testProviderConnection ────────────────────────────────────────────
    // Distinct from ai.provider.testConnection above — see
    // specs/backend-go/bugs/logic-v1/tasks/TASK-AIP-SHARED-01-add-agent-test-connection-rpc.md.
    // Called by ai-provider-service (backend-go) via infra-fleet-service's Relay.
    case 'ai.testProviderConnection': {
      try {
        const { handleTestProviderConnection } = await import('./agent-credential-store')
        return (await handleTestProviderConnection(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.testProviderConnection unavailable: ${msg}`)
      }
    }
```

## Verify

```bash
cd /opt/repos/orca/agent
pnpm exec tsc --noEmit -p tsconfig.json
pnpm exec vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
```

Add a table-driven case to `agent/src/relay/__tests__/agent-rpc-dispatch.test.ts`
(or a new `agent-credential-store.test.ts` case) asserting:
- missing `credentialRef` or `providerType` → JSON-RPC error `InvalidParams`.
- valid params, `checkProviderReachabilityDetailed` mocked to return
  `{ok: true, note: 'reachable'}` → `result.success === true`.
- valid params, mocked to return `{ok: false, note: 'unreachable'}` →
  `result.success === false`, `result.message` includes `'unreachable'`.
