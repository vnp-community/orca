# SOL-AG-HLD-002 — `buildAgentEnv` Fail-Fast Thay Vì Inject Ciphertext Layer-1

**Fixes:** [BUG-AG-HLD-002](../BUG-AG-HLD-002-credential-fallback-injects-ciphertext.md)
**TDD Ref:** TDD-AG-09 §1 (Architecture) + header ("CRITICAL CONSTRAINT" — Dev Server lưu credential, Orca Server chỉ lưu metadata); TDD-AG-12 §7 (`resolvedApiKey` priority — pattern đã đúng, chỉ nhánh fallback sai)
**File:** `agent/src/relay/agent-spawner.ts` (`buildAgentEnv`)
**Effort:** 2-3 giờ
**Status:** 🔴 TODO

---

## Phân Tích

`buildAgentEnv()` (agent-spawner.ts:194-257) có 2 nhánh set API key:

- Có `resolvedApiKey` (Orca Server đã inject plaintext) → set thẳng, đúng.
- Không có → gọi `readDecryptedKey()`, hàm này (agent-credential-store.ts:337-348) chỉ gỡ **Layer 2** (double-encryption do chính agent áp dụng khi ghi `.enc` xuống đĩa — xem header comment `agent-credential-store.ts:4-10`) và trả về nguyên `encryptedBlob` — vẫn là **Layer 1 ciphertext**, mã hoá bởi SubtleCrypto phía browser lúc user nhập API key. Chuỗi ciphertext này bị gán thẳng vào biến env (`ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GEMINI_API_KEY`), khiến AI CLI con nhận một "API key" chắc chắn sai.

GitNexus xác nhận blast radius trước khi sửa:

```
impact({target:"buildAgentEnv", file_path:"agent/src/relay/agent-spawner.ts", direction:"upstream"})
→ impactedCount:1, risk:LOW, caller duy nhất: handleAgentSpawn (cùng file)

impact({target:"readDecryptedKey", file_path:"agent/src/relay/agent-credential-store.ts", direction:"upstream"})
→ impactedCount:2, risk:LOW — depth1: buildAgentEnv, depth2: handleAgentSpawn (transitive)
```

Risk LOW, phạm vi ảnh hưởng đúng như bug report mô tả — chỉ `agent-spawner.ts`, không có execution flow khác phụ thuộc `readDecryptedKey()` ngoài đường này. An toàn để sửa.

### Lựa chọn hướng fix

Bug report đưa 2 hướng:

- **(a)** Giải mã Layer 1 ngay trong agent.
- **(b)** Xoá nhánh fallback, trả lỗi rõ ràng.

**Chọn hướng (b).** Căn cứ:

1. **Header comment của chính `agent-credential-store.ts` (dòng 9-10)** — nguồn tài liệu gần code nhất, không mơ hồ: *"Master key: ORCA_AI_CREDENTIAL_KEY env var (set by admin on dev server) — The agent never sees the plaintext API key — only the browser-encrypted blob."* Đây là constraint bảo mật rõ ràng, không phải chỉ là mô tả hành vi hiện tại.
2. **TDD-AG-09 §1 (Architecture)** mô tả đúng luồng 2 lớp: browser mã hoá Layer 1 bằng SubtleCrypto, Dev Server chỉ double-encrypt/lưu/giải mã Layer 2. Không có bước "Dev Server giải mã Layer 1" nào được mô tả trong kiến trúc chính (§1-§3). Việc "Đề xuất fix (a)" đòi hỏi trao đổi với đội bảo mật trước — đúng như bug report tự lưu ý — vì nó phá vỡ mô hình "Orca Server (và cả Dev Server) không thấy plaintext" nếu implement sai cách (ví dụ cần thêm một kênh trao đổi session key riêng, ngoài phạm vi `agent-credential-store.ts` hiện có).
3. **Chính comment trong code hiện tại (agent-spawner.ts:242)** đã tự thừa nhận nhánh này sai (`"agent may fail auth if key not plaintext"`) — nghĩa là đội phát triển trước đó cũng không coi đây là giải pháp đã hoàn thiện, chỉ là placeholder.
4. Mục §4 của TDD-AG-09 (`resolveApiKey(accountId)` trả trực tiếp `apiKey` dùng làm `ANTHROPIC_API_KEY`) là đoạn code mẫu **thuộc kiến trúc v1.0 cũ** (trước khi double-encryption Layer 1/Layer 2 được thêm ở "v2.1 Integration Note" cuối tài liệu) — không phản ánh kiến trúc hiện hành, không dùng làm căn cứ cho `agent/src/relay/agent-spawner.ts` (module v5.0).

**Trade-off của hướng (b):** Bất kỳ agent nào được spawn mà Orca Server **không** cung cấp `resolvedApiKey` (kể cả khi credential đã lưu hợp lệ trong `agent-credential-store`) sẽ spawn thất bại ngay lập tức thay vì "chạy nhưng auth lỗi". Đây là hành vi **mong muốn** — fail nhanh, thông báo rõ nguyên nhân, thay vì để AI CLI con khởi động rồi báo lỗi auth khó truy nguồn. Cái giá phải trả: nếu có luồng hợp lệ nào đó hiện đang dựa vào nhánh fallback này để hoạt động "tình cờ đúng" (không có bằng chứng trong code/test hiện tại), luồng đó sẽ hỏng rõ ràng ngay — cần Orca Server đảm bảo luôn gửi `resolvedApiKey` khi gọi `agent.spawn` cho model cần API key.

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/agent-spawner.ts`

**1. Cập nhật comment kiến trúc** (dòng 169-181) — bỏ phần ngụ ý fallback đọc Layer-1 là hành vi hợp lệ:

```diff
 // ── buildAgentEnv (testable with mock credStore) ──────────────────────────────
 //
 // ORCH-003: Removed 'placeholder-key'. Now reads the real encryptedBlob from
 // the credential store (Layer 1 ciphertext from browser). Injects only the
 // env var that corresponds to the specific model's provider.
 //
 // NOTE on credential architecture (TDD-AG-09):
 //   Layer 1: Browser encrypts apiKey with SubtleCrypto → encryptedBlob
 //   Layer 2: Dev Server double-encrypts encryptedBlob → .enc file
 //   readDecryptedKey() decrypts Layer 2 → returns encryptedBlob (Layer 1)
 //   The Orca Server is responsible for injecting resolvedApiKey (plaintext)
 //   via the spawn request params when it has the Layer 1 session key.
 //   If resolvedApiKey is provided, it takes priority over credStore lookup.
+//
+// BUG-AG-HLD-002: there is NO fallback that reads the credential store and
+// uses its value as the API key. readDecryptedKey() only strips Layer 2 —
+// its return value is still Layer-1 ciphertext (agent-credential-store.ts:4-10:
+// "the agent never sees the plaintext API key"). If resolvedApiKey is absent,
+// buildAgentEnv() throws instead of injecting a value that is certainly wrong.
+// readDecryptedKey() is still called below, but only to distinguish "no
+// credential at all" from "credential exists but Orca Server forgot to
+// resolve+forward resolvedApiKey" in the error message.
 ```

**2. Thay nhánh `else if` (dòng 231-246) — bỏ hẳn việc gán ciphertext vào `base`, thay bằng throw:**

```diff
   // Inject API key for all providers (forward resolvedApiKey to all known env vars)
   // so that multi-provider agents can use whichever they need.
   if (resolvedApiKey) {
     base['ANTHROPIC_API_KEY'] = resolvedApiKey
     base['OPENAI_API_KEY']    = resolvedApiKey
     base['GEMINI_API_KEY']    = resolvedApiKey
-  } else if (spec.apiKeyEnvVar && accountId) {
-    // Fallback: read Layer 1 encryptedBlob from store
-    const logFn = log ?? {
-      info:  () => {},
-      warn:  () => {},
-      error: () => {},
-      debug: () => {},
-    }
-    const blob = await readDecryptedKey(accountId, config, logFn as AgentLogger, parentSpanId)
-    if (blob) {
-      base[spec.apiKeyEnvVar] = blob
-      logFn.warn?.(`buildAgentEnv: injecting Layer1 blob for ${spec.apiKeyEnvVar} — agent may fail auth if key not plaintext`)
-    } else {
-      logFn.warn?.(`buildAgentEnv: no credential found for accountId=${accountId} — agent will fail authentication`)
-    }
+  } else if (spec.apiKeyEnvVar && accountId) {
+    // No resolvedApiKey from Orca Server. NEVER fall back to the Layer-1
+    // ciphertext (readDecryptedKey() only strips Layer 2) — a ciphertext
+    // "API key" fails auth silently and confusingly downstream. Fail fast
+    // here instead, with a message that distinguishes the two real causes.
+    const logFn = log ?? {
+      info:  () => {},
+      warn:  () => {},
+      error: () => {},
+      debug: () => {},
+    }
+    const blob = await readDecryptedKey(accountId, config, logFn as AgentLogger, parentSpanId)
+    const err = new Error(
+      blob
+        ? `buildAgentEnv: a credential exists for accountId=${accountId} but no plaintext ` +
+          `resolvedApiKey was provided. The Dev Server agent cannot decrypt the Layer-1 ` +
+          `(browser-encrypted) credential blob itself — Orca Server must resolve it and pass ` +
+          `"resolvedApiKey" in the agent.spawn RPC params.`
+        : `buildAgentEnv: no credential found for accountId=${accountId} and no resolvedApiKey ` +
+          `provided. Configure an AI provider account in Orca settings, or ensure Orca Server ` +
+          `passes "resolvedApiKey" when spawning this agent.`
+    )
+    Object.assign(err, { agentErrorCode: AgentErrorCode.PermissionDenied })
+    logFn.warn?.(err.message)
+    throw err
+  }
```

**3. Không cần đổi `handleAgentSpawn()`** — nhánh `try { ... await buildAgentEnv(...) ... } catch (err) { ... }` (dòng 331-450) đã bọc sẵn lời gọi `buildAgentEnv()` (dòng 338-345); khi hàm throw, code đã có tại dòng 440-449 sẽ:
   - `spawner.transition('error')`
   - `span.fail(err, ...)` / `orchSpan.fail(err, ...)`
   - gửi `errResp` JSON-RPC error với `code: AgentErrorCode.ServerError` về client qua `ws.send(...)`

   → hành vi lỗi hiện có (transition state machine, trace span, WS response) tái sử dụng nguyên vẹn, không cần sửa thêm gì ở đây.

   **Lưu ý nhỏ:** vì `err` throw ra từ `buildAgentEnv()` giờ có `agentErrorCode: PermissionDenied`, có thể cân nhắc (không bắt buộc trong fix này) đổi dòng 447 từ `code: AgentErrorCode.ServerError` sang đọc `(err as { agentErrorCode?: number }).agentErrorCode ?? AgentErrorCode.ServerError` để trả đúng mã lỗi — pattern này đã có sẵn ở `errorResponse()` trong `agent-credential-store.ts:95-99`. Để scope fix này gọn, gợi ý áp dụng cùng pattern trong lần sửa tiếp theo nếu cần phân biệt lỗi credential vs lỗi server khác ở phía client.

---

## Verification

```bash
cd agent
npx vitest run src/relay/__tests__/sub-agent-spawner.test.ts
```

Thêm test case mới vào `agent/src/relay/__tests__/sub-agent-spawner.test.ts`:

```typescript
it('buildAgentEnv throws (not injects ciphertext) when resolvedApiKey is absent and a credential exists', async () => {
  // mock readDecryptedKey to return a fake Layer-1 blob
  vi.doMock('../agent-credential-store', () => ({
    readDecryptedKey: vi.fn().mockResolvedValue('layer1-ciphertext-blob'),
  }))
  await expect(
    buildAgentEnv({ accountId: 'acct-1', userId: 'u1', taskId: 't1', cwd: '/tmp' }, claudeSpec, config, null)
  ).rejects.toThrow('no plaintext resolvedApiKey was provided')
})

it('buildAgentEnv throws a "no credential found" message when nothing is stored either', async () => {
  vi.doMock('../agent-credential-store', () => ({
    readDecryptedKey: vi.fn().mockResolvedValue(null),
  }))
  await expect(
    buildAgentEnv({ accountId: 'acct-1', userId: 'u1', taskId: 't1', cwd: '/tmp' }, claudeSpec, config, null)
  ).rejects.toThrow('no credential found for accountId=acct-1')
})

it('never assigns the Layer-1 blob value to ANTHROPIC_API_KEY/OPENAI_API_KEY/GEMINI_API_KEY', async () => {
  vi.doMock('../agent-credential-store', () => ({
    readDecryptedKey: vi.fn().mockResolvedValue('layer1-ciphertext-blob'),
  }))
  await expect(
    buildAgentEnv({ accountId: 'acct-1', userId: 'u1', taskId: 't1', cwd: '/tmp' }, claudeSpec, config, null)
  ).rejects.toThrow()
  // assert no env object with the ciphertext value was ever returned/observed
})
```

Xoá/điều chỉnh test cũ (nếu có) đang assert hành vi "fallback set Layer-1 blob vào env" — kiểm tra bằng:

```bash
grep -n "injecting Layer1\|Layer1 blob\|no credential found for accountId" agent/src/relay/__tests__/sub-agent-spawner.test.ts
```

`gitnexus detect_changes({scope: "compare", base_ref: "main"})` sau khi sửa — kỳ vọng chỉ 2 symbol thay đổi (`buildAgentEnv`, comment) và affected surface đúng như impact analysis ở trên (`handleAgentSpawn` là caller duy nhất, không lan ra module khác).

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/agent-spawner.ts` | `buildAgentEnv()` — sửa chính |
| `agent/src/relay/agent-credential-store.ts` | `readDecryptedKey()` — dùng lại để check tồn tại, không lấy giá trị làm key |
| `agent/src/relay/__tests__/sub-agent-spawner.test.ts` | Test cần bổ sung/điều chỉnh |
| `agent/src/shared/agent-wire-protocol.ts` | `AgentErrorCode.PermissionDenied` — mã lỗi dùng cho throw mới |
| `specs/agent/bugs/hld-v1/solutions/SOL-AG-HLD-001-ai-complete-resolvedapikey-fallback.md` | Áp dụng cùng quyết định kiến trúc (không giải mã Layer 1 trong `agent/`) cho `ai.complete` |
| `specs/agent/tdd/v5/09-ai-credential-relay.md §1` | Kiến trúc Layer 1/Layer 2 — căn cứ chính cho quyết định fail-fast |
| `specs/agent/tdd/v5/12-agent-spawner.md §7` | Pattern `resolvedApiKey` ưu tiên — phần đã đúng, giữ nguyên |
