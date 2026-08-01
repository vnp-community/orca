# SUPPLEMENT: AI Providers & Agent-WS — Source-Aligned Implementation Details

**Mục đích:** Bổ sung cho SOLUTION-ai-providers.md và SOLUTION-agent-ws.md  
**Căn cứ:** `agent-credential-store.ts` (L1-312 đã đọc), `agent-session.ts` (L1-189 đã đọc)

---

## AIP-002 — Clarification về Design Intent vs Bug

### Design Intent (từ source code comments L5-10):
```typescript
// Security layering:
//   Layer 1: Browser encrypts API key with SubtleCrypto (AES-GCM) → encryptedBlob + iv
//   Layer 2: Agent double-encrypts the blob using scrypt + AES-256-GCM before writing to disk
//   Storage: ~/.orca/credentials/<accountId>.enc (mode 0600, dir mode 0700)
//
// Master key: ORCA_AI_CREDENTIAL_KEY env var (set by admin on dev server)
// The agent never sees the plaintext API key — only the browser-encrypted blob.
```

### Vấn đề thực sự

`handleWriteCredential` (L103-144) nhận `{ encryptedBlob, iv, algorithm }` từ browser và lưu:
```typescript
const plaintext = JSON.stringify({ encryptedBlob, iv, algorithm })
const encrypted = encryptPayload(masterKey, plaintext)
```

`readDecryptedKey` (L301-311) trả về `encryptedBlob` — đây là **Layer 1 ciphertext**.

`buildAgentEnv` trong `agent-spawner.ts` L147 dùng `'placeholder-key'` thay vì gọi `readDecryptedKey`.

**Nếu gọi `readDecryptedKey` và inject vào `ANTHROPIC_API_KEY` → AI CLI nhận Layer 1 ciphertext thay vì plaintext key → auth fail.**

### Two Options:

**Option A (Simple — giữ security model):**  
Orca Server là người duy nhất có thể decrypt Layer 1 (giữ SubtleCrypto session key phía browser/Orca Server).  
Dev Server agent relay nhận plaintext key từ Orca Server via spawn request.  
→ Thêm `resolvedApiKey` field vào `agent.spawn` params, `buildAgentEnv` sử dụng field này.

**Option B (Full relay — thay đổi security model):**  
Dev Server nhận thêm Layer 1 decrypt key từ Orca Server.  
Agent decrypt Layer 1 khi cần spawn.  
→ Phức tạp hơn, cần thay đổi protocol.

**Khuyến nghị: Option A** (ít thay đổi nhất, giữ security model):

```typescript
// Orca Server (src/main/project/ProfileAwareAgentSpawner.ts):
// Trước khi gọi relay.call('agent.spawn', params):
// 1. Decrypt Layer 1: SubtleCrypto.decrypt(sessionKey, encryptedBlob) → plaintext apiKey
// 2. Thêm resolvedApiKey vào params (sẽ được encrypted in-transit qua WS TLS)

// Dev Server (agent-spawner.ts buildAgentEnv):
// Nhận resolvedApiKey từ spawn params → inject vào env var đúng
```

---

## AIP-001 — Fix Authenticated Health Check

### Code thực tế (L195-228):

```typescript
export async function handleHealthCheck(...) {
  // Step 1: Verify credential is readable
  const credResult = await handleReadCredential(id, { accountId }, config, log)
  if (credResult.error) { ... return credResult }

  // Step 2: Chỉ check HTTP HEAD reachability (KHÔNG authenticated)
  const note = await checkProviderReachability(provider)
}
```

**`checkProviderReachability` (L239-253):** dùng `method: 'HEAD'` — server trả về bất kỳ response nào (kể cả 401) được tính là 'reachable'.

### Fix AIP-001 đúng

Giải pháp: Thêm authenticated call **nếu encryptedBlob có thể được dùng làm key** (trong simplified flow).

Trong practice, vì Dev Server không có Layer 1 key, best we can do là:
1. Verify credential exists và readable ✅ (đã có)
2. Check network reachability ✅ (đã có, cần improve)
3. Phân biệt `credential_unreadable` vs `network_unreachable` vs `auth_failed`

```diff
// agent-credential-store.ts

 export async function handleHealthCheck(
   id: string | number | null,
   params: Record<string, unknown>,
   config: AgentConfig,
   log: AgentLogger
 ): Promise<object> {
   const accountId = typeof params.accountId === 'string' ? params.accountId : ''
   const provider  = typeof params.provider  === 'string' ? params.provider  : 'anthropic'
   const start = Date.now()
   const span  = credTracer.start({ method: 'ai.provider.healthCheck', provider })

   // Step 1: Verify credential exists and decryptable
   const credResult = await handleReadCredential(id, { accountId }, config, log) as { error?: unknown; result?: { encryptedBlob: string; iv: string } }
   if (credResult.error) {
     span.fail('credential unreadable', { accountId, provider })
     return {
       jsonrpc: '2.0', id,
       result: {
         ok: false,
         latencyMs: Date.now() - start,
-        note: 'credential_unreadable',
+        note: 'credential_unreadable',
+        error: 'Credential not found or decrypt failed. Please re-add the API key in Settings.',
       },
     }
   }

+  // Step 2: Parse credential to validate format (not to authenticate)
+  const blob = credResult.result?.encryptedBlob ?? ''
+  if (!blob) {
+    return {
+      jsonrpc: '2.0', id,
+      result: {
+        ok: false,
+        latencyMs: Date.now() - start,
+        note: 'credential_empty',
+        error: 'Credential stored but empty. Please re-add the API key.',
+      },
+    }
+  }

-  const note = await checkProviderReachability(provider)
+  // Step 3: Network reachability check (HTTP level — provider connectivity)
+  const { ok: networkOk, note, statusCode } = await checkProviderReachabilityDetailed(provider)
   const latencyMs = Date.now() - start
   log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} → ${note}`)
-  if (note === 'reachable' || note === 'local_provider') {
-    span.ok({ provider, latencyMs, note })
-  } else {
-    span.fail(note, { provider, latencyMs })
-  }
+  if (networkOk) {
+    span.ok({ provider, latencyMs, note })
+  } else {
+    span.fail(note, { provider, latencyMs })
+  }
   return {
     jsonrpc: '2.0', id,
     result: {
-      ok:        note === 'reachable' || note === 'local_provider',
+      ok:        networkOk,
       latencyMs,
       note,
+      ...(statusCode !== undefined ? { statusCode } : {}),
+      credentialFound: true,  // credential exists và decryptable
     },
   }
 }

+// Thay thế checkProviderReachability với version chi tiết hơn:
+const PROVIDER_HEALTH_URLS: Record<string, string> = {
+  anthropic: 'https://api.anthropic.com',
+  openai:    'https://api.openai.com',
+  gemini:    'https://generativelanguage.googleapis.com',
+}
+
+async function checkProviderReachabilityDetailed(provider: string): Promise<{
+  ok: boolean; note: string; statusCode?: number
+}> {
+  const url = PROVIDER_HEALTH_URLS[provider.toLowerCase()]
+  if (!url) return { ok: true, note: 'local_provider' }
+
+  try {
+    const ctrl  = new AbortController()
+    const timer = setTimeout(() => ctrl.abort(), 5_000)
+    const resp  = await fetch(url, { method: 'HEAD', signal: ctrl.signal })
+    clearTimeout(timer)
+    const statusCode = resp.status
+    // 401/403 = server reachable but auth needed (expected — we're not authenticating here)
+    // 429 = rate limited = server reachable
+    // 5xx = server error
+    if (statusCode < 500) {
+      return { ok: true, note: 'reachable', statusCode }
+    }
+    return { ok: false, note: 'server_error', statusCode }
+  } catch (err: unknown) {
+    const msg = err instanceof Error ? err.message : String(err)
+    if (msg.includes('aborted')) return { ok: false, note: 'timeout' }
+    return { ok: false, note: 'unreachable' }
+  }
+}
```

---

## AWS-001 — Handshake Method Name

### Source code thực tế:

```typescript
// agent-session.ts L61:
method: AGENT_HANDSHAKE_METHOD,
```

```typescript
// shared/agent-wire-protocol.ts (cần đọc để confirm):
export const AGENT_HANDSHAKE_METHOD = ???
```

Cần kiểm tra giá trị thực của `AGENT_HANDSHAKE_METHOD`:

```bash
grep -n "AGENT_HANDSHAKE_METHOD" \
  src/shared/agent-wire-protocol.ts \
  src/main/dev-server/
```

### Capabilities list — fix thực sự

Từ `agent-session.ts` L67:
```typescript
capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees'] as const,
```

Thiếu `'pty'` capability. Fix đã được bao gồm trong SUPPLEMENT-source-aligned.md.

### Handshake fix chỉ cần nếu server và agent dùng khác tên

Check bằng lệnh:
```bash
grep -rn "agent.handshake\|agent.hello\|AGENT_HANDSHAKE" \
  src/main/dev-server/ \
  src/shared/
```

Nếu kết quả cho thấy mismatch → sửa `AGENT_HANDSHAKE_METHOD` constant value.

---

## Tóm tắt bổ sung

### Files được bổ sung giải pháp:

| Domain | File supplement | Thay đổi chính |
|--------|----------------|----------------|
| ai-providers | SUPPLEMENT-source-aligned.md (new) | Clarify AIP-002 design intent, fix AIP-001 health check detail |
| agent-ws | Cùng file | Verify AWS-001 bằng grep trước khi fix |
| agent-orchestration | SUPPLEMENT-source-aligned.md | Diff chính xác dựa trên actual source code |
| terminal-management. | SUPPLEMENT-source-aligned.md | Phân tích PtyHandler vs agent-rpc-dispatch, pty-agent-bridge.ts mới |

### Lệnh kiểm tra trước khi implement:

```bash
# 1. Verify AGENT_HANDSHAKE_METHOD value:
grep -n "AGENT_HANDSHAKE_METHOD\s*=" src/shared/agent-wire-protocol.ts

# 2. Verify server expects which handshake method:
grep -rn "agent.handshake\|agent.hello\|AGENT_HANDSHAKE" src/main/dev-server/

# 3. Check pty-handler.ts spawn() return type expectation from caller:
grep -rn "pty.spawn\|ptyId\|pty_id" src/main/dev-server/ | grep -v ".test."

# 4. Check how Orca Server passes API key to agent.spawn:
grep -rn "resolvedApiKey\|apiKey\|ANTHROPIC_API_KEY" src/main/project/

# 5. Verify relay bridge callWithTimeout locations:
grep -n "callWithTimeout\|Not connected\|agentToken" src/main/dev-server/dev-server-relay-bridge.ts
```
