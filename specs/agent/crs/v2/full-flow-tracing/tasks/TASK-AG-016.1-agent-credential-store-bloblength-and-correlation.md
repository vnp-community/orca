# TASK-AG-016.1: Add blobLength field and parentSpanId correlation to agent-credential-store.ts

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-016](../solutions/SOL-AG-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Precondition:** Phase 0 (`Tracer.start(fields?, resume?)`)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — Implemented as specified, no drift from current source. `blobLength` added to `handleWriteCredential()` start/fail fields; `parentSpanId` threaded through `handleReadCredential()` and `readDecryptedKey()` as an optional trailing param. `handleHealthCheck()`/`handleDeleteCredential()` left untouched. `pnpm run typecheck:node` clean for this file (one pre-existing unused-import error in `agent-credential-store.test.ts` — confirmed via `git status` not touched by this task, addressed in TASK-AG-016.3). `gitnexus_impact` on all 3 symbols returned LOW risk.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "handleWriteCredential"
codegraph explore "handleReadCredential"
codegraph explore "readDecryptedKey"
```

Cả 3 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis cho từng symbol:

```
gitnexus_impact({ target: "handleWriteCredential", direction: "upstream" })
gitnexus_impact({ target: "handleReadCredential", direction: "upstream" })
gitnexus_impact({ target: "readDecryptedKey", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp — chú ý `readDecryptedKey` được gọi từ `agent-spawner.ts::buildAgentEnv()`, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

`agent-credential-store.ts` ĐÃ CÓ tracer `agent:credential` (`credTracer`) bọc đầy đủ cả 4 RPC handler (`handleWriteCredential`, `handleReadCredential`, `handleHealthCheck`, `handleDeleteCredential`). Đây là tracer pre-existing chưa được liệt kê trong CR-TRACE-000 GAP-3 — KHÔNG tạo tracer trùng tên/chức năng. Phạm vi task này: (1) bổ sung field `blobLength` còn thiếu theo spec CR-TRACE-016 §4, (2) thêm cơ chế correlation field `parentSpanId` cho điểm "read-credential-for-spawn" (khi `agent.spawn` tự đọc lại credential fallback).

**BL-AIP-02 (resolution cascade):** không có và không cần counterpart phía agent — `ProviderResolver.resolve()` hoàn toàn in-process ở backend, không băng qua boundary.

**Ràng buộc bảo mật (đã tuân thủ trong code hiện có, PHẢI giữ nguyên):** không `span.start()/step()/ok()/fail()` nào được truyền `encryptedBlob`, `iv`, hay `apiKey` vào fields — chỉ `accountId`, `method`, `provider`, `latencyMs`, `note`, `deleted`, và field mới `blobLength`/`parentSpanId`.

## File: `src/relay/agent-credential-store.ts` [MODIFY]

### `handleWriteCredential()` — thêm `blobLength`

```typescript
// src/relay/agent-credential-store.ts

export async function handleWriteCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId     = typeof params.accountId     === 'string' ? params.accountId     : ''
  const encryptedBlob = typeof params.encryptedBlob === 'string' ? params.encryptedBlob : ''
  const iv            = typeof params.iv            === 'string' ? params.iv            : ''
  const algorithm     = typeof params.algorithm     === 'string' ? params.algorithm     : 'AES-GCM'
  // CR-TRACE-016 §4 BL-AIP-01 + §1 security constraint: blobLength (metadata,
  // KHÔNG PHẢI giá trị blob) giúp phân biệt "blob rỗng/bất thường" vs "lỗi khác"
  // khi debug "Add Provider Account bị treo/lỗi" — encryptedBlob/iv/apiKey KHÔNG
  // BAO GIỜ được đưa vào TraceFields ở bất kỳ dòng nào trong file này.
  const span = credTracer.start({
    method: 'ai.provider.writeCredential',
    accountId,
    blobLength: encryptedBlob.length,
  })

  if (!accountId || !encryptedBlob || !iv) {
    span.fail('missing required params', { accountId, blobLength: encryptedBlob.length })
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required params: accountId, encryptedBlob, iv' },
    }
  }

  try {
    const masterKey = getCredentialKey()
    const plaintext = JSON.stringify({ encryptedBlob, iv, algorithm })
    const encrypted = encryptPayload(masterKey, plaintext)
    const stored: StoredCredential = { version: FILE_VERSION, ...encrypted }

    mkdirSync(config.credentialDir, { recursive: true, mode: 0o700 })

    const filePath = credentialFilePath(config.credentialDir, accountId)
    writeFileSync(filePath, JSON.stringify(stored), { mode: 0o600 })

    log.info(`ai.provider.writeCredential: stored accountId=${accountId}`)
    span.ok({ accountId })
    return { jsonrpc: '2.0', id, result: { ok: true } }

  } catch (err: unknown) {
    log.error(`ai.provider.writeCredential failed: ${err instanceof Error ? err.message : String(err)}`)
    span.fail(err, { accountId })
    return errorResponse(id, err)
  }
}
```

Không thêm `step()` cho bước mkdir/writeFileSync — thao tác filesystem in-process, nhanh, không phải network hop (CR-TRACE-000 §5).

### `handleReadCredential()` + `readDecryptedKey()` — correlation field `parentSpanId`

```typescript
// src/relay/agent-credential-store.ts — handleReadCredential() + readDecryptedKey()

export async function handleReadCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId    = typeof params.accountId    === 'string' ? params.accountId    : ''
  // CR-TRACE-016 §5 (mô hình parentTraceId của CR-TRACE-017): field nghiệp vụ
  // thuần tuý để nối span này với span agent:spawn đã gọi nó (qua
  // buildAgentEnv() → readDecryptedKey()) — KHÔNG phải Tracer.start() `resume`
  // (CR-TRACE-000 §3.1), vì core API đó chưa ship.
  const parentSpanId = typeof params.parentSpanId === 'string' ? params.parentSpanId : undefined
  const span = credTracer.start({ method: 'ai.provider.readCredential', accountId, parentSpanId })

  if (!accountId) {
    span.fail('missing accountId', { method: 'ai.provider.readCredential' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: accountId' } }
  }

  try {
    const masterKey = getCredentialKey()
    const filePath  = credentialFilePath(config.credentialDir, accountId)

    if (!existsSync(filePath)) {
      span.fail('credential not found', { accountId })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `Credential not found: ${accountId}` } }
    }

    const stored: StoredCredential = JSON.parse(readFileSync(filePath, 'utf8'))
    if (stored.version !== FILE_VERSION) {
      span.fail('unknown version', { accountId, version: stored.version })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `Unknown credential version: ${stored.version}` } }
    }

    // NOTE: decrypted payload contains the Layer-1 encryptedBlob (still opaque
    // to the agent) — never touches span fields, only the JSON-RPC result body.
    const plaintext = decryptPayload(masterKey, stored)
    const payload   = JSON.parse(plaintext) as { encryptedBlob: string; iv: string; algorithm: string }
    span.ok({ accountId })
    return {
      jsonrpc: '2.0', id,
      result: { accountId, encryptedBlob: payload.encryptedBlob, iv: payload.iv, algorithm: payload.algorithm },
    }

  } catch (err: unknown) {
    log.error(`ai.provider.readCredential failed: ${err instanceof Error ? err.message : String(err)}`)
    span.fail(err, { accountId })
    return errorResponse(id, err)
  }
}

// ─── readDecryptedKey (used by agent-spawner.ts) ─────────────────────────────

export async function readDecryptedKey(
  accountId: string,
  config:    AgentConfig,
  log:       AgentLogger,
  parentSpanId?: string,   // NEW — forwarded from buildAgentEnv()'s caller span
): Promise<string | null> {
  const result = await handleReadCredential(null, { accountId, parentSpanId }, config, log) as {
    result?: { encryptedBlob: string }; error?: unknown
  }
  if (result.error || !result.result) return null
  return result.result.encryptedBlob
}
```

`handleHealthCheck()`/`handleDeleteCredential()` — giữ nguyên KHÔNG đổi, đã đủ theo CR-TRACE-016 BL-AIP-03.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-credential-store" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `handleWriteCredential()` span `start` chứa `blobLength: encryptedBlob.length`
- [ ] Không có bất kỳ trace event nào (start/step/ok/fail) trong file này chứa field `apiKey`, `encryptedBlob`, hoặc `iv`
- [ ] `readDecryptedKey()`/`handleReadCredential()` nhận và forward `parentSpanId` optional, backward-compatible với call site cũ (không truyền vẫn hoạt động)
- [ ] `handleHealthCheck()`/`handleDeleteCredential()` giữ nguyên KHÔNG đổi
- [ ] `pnpm run typecheck:node` pass
