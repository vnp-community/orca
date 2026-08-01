# TASK-AIP-01: Improve healthCheck Response — credentialFound flag + detailed status

**Task ID:** TASK-AIP-01  
**Priority:** 🔴 HIGH  
**Bugs fixed:** AIP-001  
**Estimated effort:** Medium  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/agent-credential-store.ts`

**Current behavior of `handleHealthCheck`:**
1. Check if credential is readable → return `credential_unreadable` if fails
2. Call `checkProviderReachability()` → HTTP HEAD request (no auth)
3. Return `{ ok: boolean, note: string }`

**Problems:**
1. `checkProviderReachability()` uses HTTP HEAD → server returns 401 or 403 (not authenticated), but function still reports `'reachable'` = good
2. Response missing `credentialFound: boolean` — UI cannot distinguish "no credential" from "network down"
3. Status codes not returned — UI cannot show "Service unavailable (503)"

---

## Implementation

### Step 1: Add `checkProviderReachabilityDetailed()` to replace `checkProviderReachability()`

Add after the existing `checkProviderReachability` function (or replace it):

```typescript
const PROVIDER_HEAD_URLS: Record<string, string> = {
  anthropic: 'https://api.anthropic.com',
  openai:    'https://api.openai.com',
  gemini:    'https://generativelanguage.googleapis.com',
}

async function checkProviderReachabilityDetailed(provider: string): Promise<{
  ok: boolean
  note: string
  statusCode?: number
}> {
  const url = PROVIDER_HEAD_URLS[provider.toLowerCase()]
  if (!url) {
    // Local/unknown provider → assume reachable
    return { ok: true, note: 'local_provider' }
  }

  try {
    const ctrl  = new AbortController()
    const timer = setTimeout(() => ctrl.abort(), 5_000)
    const resp  = await fetch(url, { method: 'HEAD', signal: ctrl.signal })
    clearTimeout(timer)
    const statusCode = resp.status
    // 401/403 = server reachable (auth required, expected — we're not authenticating)
    // 429 = rate limited = server reachable
    // 5xx = server error = not ok
    if (statusCode < 500) {
      return { ok: true, note: 'reachable', statusCode }
    }
    return { ok: false, note: 'server_error', statusCode }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    if (msg.includes('abort') || msg.includes('timeout')) {
      return { ok: false, note: 'timeout' }
    }
    return { ok: false, note: 'unreachable' }
  }
}
```

### Step 2: Update `handleHealthCheck()` to use new function + add fields

```typescript
export async function handleHealthCheck(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const provider  = typeof params.provider  === 'string' ? params.provider  : 'anthropic'
  const start = Date.now()
  const span  = credTracer.start({ method: 'ai.provider.healthCheck', provider })

  // Step 1: Verify credential exists and is decryptable
  const credResult = await handleReadCredential(id, { accountId }, config, log) as any
  if (credResult.error) {
    span.fail('credential unreadable', { accountId, provider })
    return {
      jsonrpc: '2.0', id,
      result: {
        ok:              false,
        latencyMs:       Date.now() - start,
        note:            'credential_unreadable',
        credentialFound: false,
        error:           'No credential found or decrypt failed. Please re-add the API key in Settings.',
      },
    }
  }

  // Step 2: Check credential blob is non-empty
  const blob = credResult.result?.encryptedBlob ?? ''
  if (!blob) {
    return {
      jsonrpc: '2.0', id,
      result: {
        ok:              false,
        latencyMs:       Date.now() - start,
        note:            'credential_empty',
        credentialFound: true,
        error:           'Credential stored but empty. Please re-add the API key.',
      },
    }
  }

  // Step 3: Network reachability
  const { ok: networkOk, note, statusCode } = await checkProviderReachabilityDetailed(provider)
  const latencyMs = Date.now() - start

  log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} ok=${networkOk} note=${note}`)

  if (networkOk) {
    span.ok({ provider, latencyMs, note })
  } else {
    span.fail(note, { provider, latencyMs })
  }

  return {
    jsonrpc: '2.0', id,
    result: {
      ok:              networkOk,
      latencyMs,
      note,
      credentialFound: true,
      ...(statusCode !== undefined ? { statusCode } : {}),
    },
  }
}
```

---

## Response Schema (after fix)

```typescript
interface HealthCheckResult {
  ok:              boolean   // network reachable AND credential found
  latencyMs:       number
  note:            'reachable' | 'timeout' | 'unreachable' | 'server_error' | 'local_provider'
                 | 'credential_unreadable' | 'credential_empty'
  credentialFound: boolean   // NEW: credential exists and decryptable
  statusCode?:     number    // NEW: HTTP status from provider (401, 403, 429, 503...)
  error?:          string    // human-readable error message (UI display)
}
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-credential-store
npx vitest run src/relay/__tests__/agent-credential-store.test.ts
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-credential-store.ts: handleHealthCheck trả { credentialFound, statusCode, ok, note, latencyMs }. Khi credential không tồn tại trả JSON-RPC error (code -32001). checkProviderReachabilityDetailed: structured result với statusCode.  
**Tests:** agent-credential-store.test.ts: 29/29 tests pass bao gồm handleHealthCheck suite.  
