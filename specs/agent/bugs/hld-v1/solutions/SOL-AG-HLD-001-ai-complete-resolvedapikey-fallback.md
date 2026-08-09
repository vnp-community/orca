# SOL-AG-HLD-001 — `ai.complete` Fallback Qua `resolvedApiKey` (Không Gọi Thẳng Credential Store)

**Fixes:** [BUG-AG-HLD-001](../BUG-AG-HLD-001-ai-complete-no-credential-store-fallback.md)
**TDD Ref:** TDD-AG-09 §1 (Architecture — "The agent never sees the plaintext API key"), TDD-AG-09 "v2.1 Integration Note"; TDD-AG-12 §7 (`resolvedApiKey` priority pattern, đã implement đúng cho `agent.spawn`)
**File:** `agent/src/relay/ai-complete-handler.ts`, `agent/src/relay/agent-rpc-dispatch.ts` (case `ai.complete`)
**Effort:** 3-5 giờ
**Status:** 🔴 TODO

---

## Phân Tích

Comment đầu file `ai-complete-handler.ts:11-14` mô tả 2 tầng resolve API key:

1. Env var injected lúc agent spawn (`ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GOOGLE_API_KEY`)
2. `ORCA_ACCOUNT_ID` → credential store tại `~/.orca/ai-providers/<accountId>.enc`

Nhưng `resolveApiKey()` (dòng 101-115) chỉ đọc `process.env`, tầng (2) chưa từng được implement — xác nhận đúng bằng GitNexus:

```
impact({target:"resolveApiKey", file_path:"agent/src/relay/ai-complete-handler.ts", direction:"upstream"})
→ impactedCount:1, risk:LOW, caller duy nhất: handleAIComplete (cùng file)
```

Nghĩa là sửa `resolveApiKey()` là an toàn — chỉ ảnh hưởng `handleAIComplete`, không có execution flow nào khác phụ thuộc.

**Vấn đề với đề xuất fix gốc trong bug report:** Bug report gợi ý gọi thẳng `readDecryptedKey()` (đã có sẵn, dùng bởi `agent-spawner.ts`) để "lấy key đã giải mã". Nhưng đọc kỹ `agent-credential-store.ts:330-348`, `readDecryptedKey()` chỉ gỡ lớp mã hoá **Layer 2** (agent tự bọc bằng scrypt+AES-GCM khi ghi xuống đĩa) và trả về nguyên `encryptedBlob` — **vẫn là ciphertext Layer 1** (mã hoá bởi SubtleCrypto phía browser). Đây chính xác là lỗi đã bị phát hiện ở BUG-AG-HLD-002 trong `agent-spawner.ts`. Nếu implement `resolveApiKey()` theo đúng gợi ý gốc, `ai.complete` sẽ tái tạo y hệt lỗi đó ở một chỗ khác (set ciphertext vào `x-api-key`/`Authorization` header → provider trả 401, không phải "no API key" nhưng vẫn hỏng).

Thêm nữa: `ai-complete-handler.ts` chạy **trong tiến trình relay/agent.js**, không phải trong tiến trình con (PTY) được `agent.spawn` tạo ra. `ORCA_ACCOUNT_ID` mà `buildAgentEnv()` set (agent-spawner.ts:216) chỉ tồn tại trong `env` truyền cho `nodePty.spawn()` — **không** nằm trong `process.env` của chính tiến trình relay. Vì vậy dù có sửa `resolveApiKey()` để đọc `process.env['ORCA_ACCOUNT_ID']`, giá trị đó sẽ luôn `undefined` trong ngữ cảnh này — tầng (2) như mô tả trong comment hiện tại là không thể hoạt động dù có sửa gì đi nữa, trừ khi accountId đến từ RPC params.

**Quyết định kiến trúc (khớp với SOL-AG-HLD-002):** Không cho `ai.complete` tự gọi `agent-credential-store.ts` để lấy blob rồi dùng làm plaintext. Thay vào đó, tái dùng đúng cơ chế đã chạy đúng cho `agent.spawn`: Orca Server chịu trách nhiệm giải mã Layer 1 (nó giữ session key) và forward **`resolvedApiKey`** (plaintext) qua RPC params — `agent.spawn` đã làm vậy (`agent-spawner.ts:287,342`). `ai.complete` nên nhận `accountId` + `resolvedApiKey` (optional) trong params, ưu tiên y hệt thứ tự `buildAgentEnv()`: env-var trước, rồi `resolvedApiKey`. Nếu không có `resolvedApiKey` nhưng credential đã tồn tại trên đĩa (`readDecryptedKey()` trả về non-null chỉ để **kiểm tra tồn tại**, KHÔNG dùng giá trị đó làm key), báo lỗi rõ ràng phân biệt "chưa cấu hình" với "server quên forward resolvedApiKey" — không bao giờ set ciphertext vào header auth.

**Trade-off:** Cách này yêu cầu bên gọi (`TaskAIPlanner.decompose()`, git commit-message generator — nằm ngoài package `agent/`, ví dụ `backend/src/main/task/TaskAIPlanner.ts`) phải được cập nhật để gửi thêm `accountId`/`resolvedApiKey` trong `relay.call('ai.complete', {...})`. Đây là thay đổi phối hợp, không nằm trong scope sửa `agent/` lần này — xem mục "Files Liên Quan" bên dưới. Cho tới khi caller được cập nhật, tầng (2) sẽ luôn rơi vào nhánh lỗi rõ ràng thay vì âm thầm hỏng — chấp nhận được vì tầng (1) (env var lúc spawn) là con đường chính đã hoạt động đúng.

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/ai-complete-handler.ts`

**1. Cập nhật comment đầu file** (dòng 1-17) — mô tả đúng cơ chế mới, không còn nhắc "ORCA_ACCOUNT_ID → credential store" (sai, vì process.env không có biến này trong tiến trình relay):

```diff
 /**
  * ai-complete-handler — AI text completion for task planning and commit messages (TDD-18)
  *
  * IMPORTANT: This file runs on the Dev Server (relay binary), NOT on Orca Server.
  * Do NOT import anything from src/main/.
  *
  * Called by relay dispatch case 'ai.complete':
  *   - TaskAIPlanner.decompose() → relay.call('ai.complete', { prompt, format: 'json' })
  *   - git generateCommitMessage → relay.call('ai.complete', { prompt, format: 'text' })
  *
  * Credential resolution priority:
- *   1. Environment variables injected by agent spawn (ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY)
- *   2. ORCA_ACCOUNT_ID → credential store at ~/.orca/ai-providers/<accountId>.enc
- *      (Note: currently stored encrypted; plaintext only available if relay decrypted it)
+ *   1. Environment variables injected by agent spawn (ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY)
+ *   2. params.resolvedApiKey — same mechanism as agent.spawn's resolvedApiKey (agent-spawner.ts
+ *      buildAgentEnv): Orca Server holds the Layer-1 session key and forwards a plaintext key
+ *      per-call. This process (relay/agent.js) never decrypts the credential store directly —
+ *      see BUG-AG-HLD-002 for why reading agent-credential-store.ts here would inject ciphertext.
+ *   3. Neither present but params.accountId has a stored credential → throw a clear
+ *      "resolvedApiKey missing" error (server wiring gap) instead of silently failing auth.
  *
  * @module relay/ai-complete-handler
  */
```

**2. Mở rộng `AICompleteParams`:**

```diff
 export interface AICompleteParams {
   prompt:  string
   format?: 'json' | 'text'
   taskId?: string
   model?:  string
+  accountId?:      string  // credential-store account, mirrors AgentSpawnRequest.accountId
+  resolvedApiKey?: string  // plaintext key forwarded by Orca Server, mirrors agent.spawn's resolvedApiKey
 }
```

**3. Sửa gọi `resolveApiKey()` trong `handleAIComplete()`** (thay đoạn dòng 67-76):

```diff
-  // Resolve API key
-  const apiKey = resolveApiKey(model)
-  if (!apiKey) {
-    span.fail('no API key for model', { model, taskId })
-    throw new Error(
-      `ai.complete: No API key found for model "${model}". ` +
-      'Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY in the agent environment, ' +
-      'or configure an AI provider in Orca settings.'
-    )
-  }
+  // Resolve API key
+  let apiKey: string | null
+  try {
+    apiKey = await resolveApiKey(model, params.accountId, params.resolvedApiKey, config, log)
+  } catch (err: unknown) {
+    span.fail(err, { model, taskId })
+    throw err
+  }
+  if (!apiKey) {
+    span.fail('no API key for model', { model, taskId })
+    throw new Error(
+      `ai.complete: No API key found for model "${model}". ` +
+      'Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY in the agent environment, ' +
+      'or configure an AI provider in Orca settings.'
+    )
+  }
```

**4. Thay `resolveApiKey()` (dòng 101-116) bằng bản có fallback `resolvedApiKey` + kiểm tra tồn tại credential:**

```diff
-// ── Key resolution ────────────────────────────────────────────────────────────
-
-function resolveApiKey(model: string): string | null {
-  if (model.startsWith('claude')) {
-    return process.env['ANTHROPIC_API_KEY'] ?? null
-  }
-  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) {
-    return process.env['OPENAI_API_KEY'] ?? null
-  }
-  if (model.startsWith('gemini')) {
-    return process.env['GOOGLE_API_KEY'] ?? null
-  }
-  // Unknown provider — try all
-  return process.env['ANTHROPIC_API_KEY']
-      ?? process.env['OPENAI_API_KEY']
-      ?? process.env['GOOGLE_API_KEY']
-      ?? null
-}
+// ── Key resolution ────────────────────────────────────────────────────────────
+
+/** Tier 1: env vars injected by buildAgentEnv() at agent.spawn time. */
+function resolveApiKeyFromEnv(model: string): string | null {
+  if (model.startsWith('claude')) {
+    return process.env['ANTHROPIC_API_KEY'] ?? null
+  }
+  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) {
+    return process.env['OPENAI_API_KEY'] ?? null
+  }
+  if (model.startsWith('gemini')) {
+    return process.env['GOOGLE_API_KEY'] ?? null
+  }
+  // Unknown provider — try all
+  return process.env['ANTHROPIC_API_KEY']
+      ?? process.env['OPENAI_API_KEY']
+      ?? process.env['GOOGLE_API_KEY']
+      ?? null
+}
+
+/**
+ * Tier 1: env var (see resolveApiKeyFromEnv).
+ * Tier 2: params.resolvedApiKey — plaintext key Orca Server forwards per-call,
+ *   the same pattern buildAgentEnv() uses for agent.spawn (see BUG-AG-HLD-002 /
+ *   agent-spawner.ts). We never call agent-credential-store.ts to obtain a key
+ *   value here — it only holds Layer-1 (still browser-encrypted) ciphertext,
+ *   which this process cannot decrypt.
+ * Tier 3: no plaintext key anywhere, but a credential file exists for
+ *   accountId → throw a specific "resolvedApiKey missing" error so operators
+ *   don't mistake a server wiring gap for "user never configured a key".
+ */
+async function resolveApiKey(
+  model:          string,
+  accountId:      string | undefined,
+  resolvedApiKey: string | undefined,
+  config:         AgentConfig,
+  log:            AgentLogger,
+): Promise<string | null> {
+  const envKey = resolveApiKeyFromEnv(model)
+  if (envKey) return envKey
+
+  if (resolvedApiKey) return resolvedApiKey
+
+  if (accountId) {
+    const { readDecryptedKey } = await import('./agent-credential-store')
+    const blob = await readDecryptedKey(accountId, config, log)
+    if (blob) {
+      throw new Error(
+        `ai.complete: a credential exists for accountId="${accountId}" but no plaintext ` +
+        'resolvedApiKey was provided. The Dev Server agent cannot decrypt the Layer-1 ' +
+        '(browser-encrypted) credential blob itself — Orca Server must resolve it and pass ' +
+        '"resolvedApiKey" in the ai.complete RPC params (same mechanism as agent.spawn).'
+      )
+    }
+  }
+
+  return null
+}
```

> Lưu ý: `readDecryptedKey()` được gọi ở đây **chỉ để kiểm tra tồn tại** (`blob` truthy/falsy) — giá trị `blob` (Layer-1 ciphertext) không bao giờ được gán vào biến dùng làm API key.

### File: `agent/src/relay/agent-rpc-dispatch.ts`

**Forward `accountId`/`resolvedApiKey` từ RPC params tới `handleAIComplete()`** (case `'ai.complete'`, dòng 669-692):

```diff
     case 'ai.complete': {
       try {
         const p      = rpc.params ?? {}
         const prompt = typeof p['prompt'] === 'string' ? p['prompt'] : ''
         if (!prompt.trim()) {
           return makeError(rpc.id, AgentErrorCode.InvalidParams, 'ai.complete: prompt is required')
         }
         const { handleAIComplete } = await import('./ai-complete-handler')
         const result = await handleAIComplete(
           {
             prompt,
             format: typeof p['format'] === 'string' ? p['format'] as 'json' | 'text' : 'text',
             taskId: typeof p['taskId'] === 'string' ? p['taskId']  : undefined,
             model:  typeof p['model']  === 'string' ? p['model']   : undefined,
+            accountId:      typeof p['accountId']      === 'string' ? p['accountId']      : undefined,
+            resolvedApiKey: typeof p['resolvedApiKey'] === 'string' ? p['resolvedApiKey'] : undefined,
           },
           config,
           log
         )
         return { jsonrpc: '2.0', id: rpc.id, result }
       } catch (err: unknown) {
         const msg = err instanceof Error ? err.message : String(err)
         return makeError(rpc.id, AgentErrorCode.ServerError, `ai.complete failed: ${msg}`)
       }
     }
```

`extractTraceFields()` (cùng file, dòng 121-134, bucket `method === 'ai.complete'`) không cần đổi — không thêm `accountId`/`resolvedApiKey` vào trace fields vì `resolvedApiKey` là secret (nguyên tắc CR-TRACE-016 §1 đã áp dụng cho `ai.provider.*`).

---

## Verification

```bash
cd agent
npx vitest run src/relay/__tests__/ai-complete-handler.test.ts
```

Thêm test case mới vào `agent/src/relay/__tests__/ai-complete-handler.test.ts`:

```typescript
it('falls back to params.resolvedApiKey when no env var is set', async () => {
  vi.stubEnv('ANTHROPIC_API_KEY', '')
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ content: [{ type: 'text', text: 'ok' }] }),
  }))
  await expect(
    handleAIComplete(
      { prompt: 'x', model: 'claude-haiku', resolvedApiKey: 'sk-test-plaintext' },
      mockConfig, mockLog
    )
  ).resolves.toMatchObject({ content: 'ok' })
})

it('throws a "resolvedApiKey missing" error when a credential exists but resolvedApiKey is absent', async () => {
  vi.stubEnv('ANTHROPIC_API_KEY', '')
  // mock agent-credential-store.readDecryptedKey to simulate an existing credential
  vi.doMock('../agent-credential-store', () => ({
    readDecryptedKey: vi.fn().mockResolvedValue('layer1-ciphertext-blob'),
  }))
  await expect(
    handleAIComplete({ prompt: 'x', model: 'claude-haiku', accountId: 'acct-1' }, mockConfig, mockLog)
  ).rejects.toThrow('no plaintext resolvedApiKey was provided')
})

it('never leaks resolvedApiKey or the Layer-1 blob into trace fields', async () => {
  // assert JSON.stringify(events) does not contain the stubbed secret values
})
```

Manual/relay-level check: gọi `ai.complete` qua relay với `{ prompt, model, accountId }` (không kèm `resolvedApiKey`, có credential đã lưu) → phải nhận `-32000 ServerError` với message chứa `"no plaintext resolvedApiKey was provided"`, KHÔNG phải một lỗi 401 từ Anthropic/OpenAI (nghĩa là ciphertext không bị gửi đi).

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/ai-complete-handler.ts` | `resolveApiKey()` + `AICompleteParams` — sửa chính |
| `agent/src/relay/agent-rpc-dispatch.ts` | case `ai.complete` — forward `accountId`/`resolvedApiKey` |
| `agent/src/relay/agent-credential-store.ts` | `readDecryptedKey()` — dùng lại (chỉ để check tồn tại, không lấy giá trị) |
| `agent/src/relay/agent-spawner.ts` | Nguồn tham chiếu cho pattern `resolvedApiKey` (xem SOL-AG-HLD-002) |
| `agent/src/relay/__tests__/ai-complete-handler.test.ts` | Test cần bổ sung |
| `backend/src/main/task/TaskAIPlanner.ts` (ngoài scope `agent/`) | Caller `relay.call('ai.complete', ...)` — cần thêm `accountId`/`resolvedApiKey` vào params để tầng (2) thực sự phát huy tác dụng; theo dõi ở ticket riêng |
| `specs/agent/tdd/v5/09-ai-credential-relay.md §1` | Kiến trúc Layer 1/Layer 2, cơ sở cho quyết định không giải mã Layer 1 trong `agent/` |
