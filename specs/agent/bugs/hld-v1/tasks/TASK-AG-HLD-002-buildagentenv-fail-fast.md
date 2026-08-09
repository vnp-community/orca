# TASK-AG-HLD-002 — `buildAgentEnv` Fail-Fast Thay Vì Inject Ciphertext Layer-1

**Solution:** [SOL-AG-HLD-002](../solutions/SOL-AG-HLD-002-buildagentenv-fail-fast-no-resolvedapikey.md)
**Bug:** [BUG-AG-HLD-002](../BUG-AG-HLD-002-credential-fallback-injects-ciphertext.md)
**File:** `agent/src/relay/agent-spawner.ts`
**Phụ thuộc:** —
**Estimated:** 150 phút (~2-3 giờ theo SOL-AG-HLD-002)
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

Sửa `buildAgentEnv()` để khi thiếu `resolvedApiKey`, thay vì gán Layer-1 ciphertext (`readDecryptedKey()`) vào biến env API key, hàm throw lỗi rõ nghĩa và fail-fast.

---

## Context

Đọc trước:
- `agent/src/relay/agent-spawner.ts` — hàm `buildAgentEnv()` và caller `handleAgentSpawn()` (đoạn `try { ... await buildAgentEnv(...) ... } catch (err) { ... }`)
- `agent/src/relay/agent-credential-store.ts` — `readDecryptedKey()` (dùng lại, chỉ để phân biệt "không có credential" với "có credential nhưng thiếu resolvedApiKey", KHÔNG dùng giá trị trả về làm API key)
- `agent/src/shared/agent-wire-protocol.ts` — `AgentErrorCode.PermissionDenied`

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/agent-spawner.ts`

**1. Cập nhật comment kiến trúc phía trên `buildAgentEnv`**

**TÌM:**
```typescript
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
```

**THAY BẰNG:**
```typescript
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
//
// BUG-AG-HLD-002: there is NO fallback that reads the credential store and
// uses its value as the API key. readDecryptedKey() only strips Layer 2 —
// its return value is still Layer-1 ciphertext (agent-credential-store.ts:4-10:
// "the agent never sees the plaintext API key"). If resolvedApiKey is absent,
// buildAgentEnv() throws instead of injecting a value that is certainly wrong.
// readDecryptedKey() is still called below, but only to distinguish "no
// credential at all" from "credential exists but Orca Server forgot to
// resolve+forward resolvedApiKey" in the error message.
```

**2. Thay nhánh `else if` — bỏ gán ciphertext vào `base`, throw thay vào đó**

**TÌM:**
```typescript
  // Inject API key for all providers (forward resolvedApiKey to all known env vars)
  // so that multi-provider agents can use whichever they need.
  if (resolvedApiKey) {
    base['ANTHROPIC_API_KEY'] = resolvedApiKey
    base['OPENAI_API_KEY']    = resolvedApiKey
    base['GEMINI_API_KEY']    = resolvedApiKey
  } else if (spec.apiKeyEnvVar && accountId) {
    // Fallback: read Layer 1 encryptedBlob from store
    const logFn = log ?? {
      info:  () => {},
      warn:  () => {},
      error: () => {},
      debug: () => {},
    }
    const blob = await readDecryptedKey(accountId, config, logFn as AgentLogger, parentSpanId)
    if (blob) {
      base[spec.apiKeyEnvVar] = blob
      logFn.warn?.(`buildAgentEnv: injecting Layer1 blob for ${spec.apiKeyEnvVar} — agent may fail auth if key not plaintext`)
    } else {
      logFn.warn?.(`buildAgentEnv: no credential found for accountId=${accountId} — agent will fail authentication`)
    }
  }
```

**THAY BẰNG:**
```typescript
  // Inject API key for all providers (forward resolvedApiKey to all known env vars)
  // so that multi-provider agents can use whichever they need.
  if (resolvedApiKey) {
    base['ANTHROPIC_API_KEY'] = resolvedApiKey
    base['OPENAI_API_KEY']    = resolvedApiKey
    base['GEMINI_API_KEY']    = resolvedApiKey
  } else if (spec.apiKeyEnvVar && accountId) {
    // No resolvedApiKey from Orca Server. NEVER fall back to the Layer-1
    // ciphertext (readDecryptedKey() only strips Layer 2) — a ciphertext
    // "API key" fails auth silently and confusingly downstream. Fail fast
    // here instead, with a message that distinguishes the two real causes.
    const logFn = log ?? {
      info:  () => {},
      warn:  () => {},
      error: () => {},
      debug: () => {},
    }
    const blob = await readDecryptedKey(accountId, config, logFn as AgentLogger, parentSpanId)
    const err = new Error(
      blob
        ? `buildAgentEnv: a credential exists for accountId=${accountId} but no plaintext ` +
          `resolvedApiKey was provided. The Dev Server agent cannot decrypt the Layer-1 ` +
          `(browser-encrypted) credential blob itself — Orca Server must resolve it and pass ` +
          `"resolvedApiKey" in the agent.spawn RPC params.`
        : `buildAgentEnv: no credential found for accountId=${accountId} and no resolvedApiKey ` +
          `provided. Configure an AI provider account in Orca settings, or ensure Orca Server ` +
          `passes "resolvedApiKey" when spawning this agent.`
    )
    Object.assign(err, { agentErrorCode: AgentErrorCode.PermissionDenied })
    logFn.warn?.(err.message)
    throw err
  }
```

> [!IMPORTANT]
> KHÔNG cần sửa `handleAgentSpawn()`. Nó đã bọc `buildAgentEnv()` trong `try { ... } catch (err) { ... }` — khi hàm throw, code catch hiện có sẽ tự `spawner.transition('error')`, `span.fail(err, ...)`, `orchSpan.fail(err, ...)`, và gửi `errResp` JSON-RPC error (`code: AgentErrorCode.ServerError`) về client qua `ws.send(...)`. Hành vi lỗi hiện có (state machine, trace span, WS response) được tái sử dụng nguyên vẹn.

### Test bổ sung — `agent/src/relay/__tests__/sub-agent-spawner.test.ts`

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

Xoá/điều chỉnh test cũ (nếu có) đang assert hành vi "fallback set Layer-1 blob vào env":

```bash
grep -n "injecting Layer1\|Layer1 blob\|no credential found for accountId" agent/src/relay/__tests__/sub-agent-spawner.test.ts
```

---

## Verify

```bash
cd agent
npx vitest run src/relay/__tests__/sub-agent-spawner.test.ts
npm run typecheck
```

`gitnexus detect_changes({scope: "compare", base_ref: "main"})` sau khi sửa — kỳ vọng chỉ 2 symbol thay đổi (`buildAgentEnv` + comment liên quan), caller duy nhất `handleAgentSpawn` (cùng file), không lan ra module khác.

---

## Definition of Done

- [ ] Comment kiến trúc phía trên `buildAgentEnv` bổ sung đoạn giải thích BUG-AG-HLD-002 (không còn ngụ ý fallback Layer-1 là hợp lệ)
- [ ] Nhánh `else if (spec.apiKeyEnvVar && accountId)` không còn gán `base[spec.apiKeyEnvVar] = blob`
- [ ] Nhánh đó throw `Error` với message phân biệt rõ 2 trường hợp: "credential tồn tại nhưng thiếu resolvedApiKey" vs "không có credential nào"
- [ ] `err` throw ra có gắn `agentErrorCode: AgentErrorCode.PermissionDenied`
- [ ] KHÔNG sửa `handleAgentSpawn()` — dựa vào cơ chế catch/transition/span.fail/ws.send hiện có
- [ ] Test mới trong `sub-agent-spawner.test.ts` pass (throw đúng message, không có env nào chứa ciphertext)
- [ ] Test cũ (nếu có) assert hành vi fallback ciphertext đã được xoá/điều chỉnh
- [ ] `npx vitest run src/relay/__tests__/sub-agent-spawner.test.ts` pass
- [ ] `npm run typecheck` (trong `agent/`) pass
- [ ] `detect_changes({scope: "compare", base_ref: "main"})` chỉ show thay đổi trong `agent-spawner.ts` (và test file), không lan ra module khác

---

## Kết Quả Thực Thi (2026-08-09)

Đã sửa `buildAgentEnv()` trong `agent-spawner.ts`: nhánh thiếu `resolvedApiKey` giờ `throw` lỗi rõ nghĩa (gắn `agentErrorCode: AgentErrorCode.PermissionDenied`) thay vì gán ciphertext vào env. Không sửa `handleAgentSpawn()` — dựa vào try/catch có sẵn. ⚠️ Đây là thay đổi kiến trúc bảo mật — khuyến nghị review kỹ trước khi merge/deploy thật.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa.

**Cập nhật (sau khi `agent/vitest.config.ts` được bổ sung, cùng ngày):** chạy `npx vitest run` phát hiện 2 test cũ fail vì giả định hành vi cũ (inject ciphertext) — đã sửa:
- `agent-spawner.test.ts`: test "threads parentSpanId..." giờ `.catch(() => {})` vì call này cố ý trigger nhánh throw; đã thêm `describe('fails fast instead of injecting ciphertext (BUG-AG-HLD-002)')` với 4 test case mới (throw đúng message cho cả 2 nhánh credential-tồn-tại/không-tồn-tại, xác nhận không có env nào chứa ciphertext, xác nhận `resolvedApiKey` hợp lệ vẫn hoạt động bình thường).
- `sub-agent-spawner.test.ts`: test "HOME và PATH are set" đổi sang `accountId: ''` để không trigger nhánh credential-fallback (test này không quan tâm tới credential).

Kết quả cuối: `npx vitest run` trong `agent/` → 3620/3648 test pass, 2 lỗi còn lại (`pty-handler.test.ts`, `feature-interactions.test.ts`) không liên quan tới thay đổi này. `npx tsc --noEmit` vẫn 98 lỗi baseline không đổi. `node agent/build.mjs` build thành công.
