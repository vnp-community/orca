# SOL-AG-TRACE-016: AI Provider Management — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**TDD Ref:** TDD-AG-09 (AI Credential Store, v5.0)
**File(s):** `src/relay/agent-credential-store.ts` [MODIFY], `src/relay/agent-spawner.ts` [MODIFY]
**Mức độ:** 🟡 trung bình
**Thời gian ước tính:** 2h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

Đây là CR **agent-domain thực sự** — `AiCredStore` (tên trong TDD-AG-09; trong code hiện tại là tập hàm `handleWriteCredential`/`handleReadCredential`/`handleHealthCheck`/`handleDeleteCredential`/`readDecryptedKey` trong `src/relay/agent-credential-store.ts`) sống hoàn toàn trên Dev Server Agent. Đã đọc trực tiếp file này và xác nhận một điều quan trọng làm thay đổi phạm vi solution so với đề bài gốc:

**File đã có tracer `agent:credential` (`credTracer`, dòng 20) bọc đầy đủ cả 4 RPC handler** — `handleWriteCredential` (dòng 113), `handleReadCredential` (dòng 153), `handleHealthCheck` (dòng 204), `handleDeleteCredential` (dòng 288) đều đã `span.start()/ok()/fail()`. Đây là tracer **pre-existing không nằm trong danh sách "11 tracer đã tồn tại" của CR-TRACE-000 GAP-3** (bảng đó chỉ liệt kê `relay:agentCall`, `agent:rpc`, `wsSession:route`, `session:spawn`, `agentToken:register`, `agent:tokenManager` — thiếu `agent:credential`, `agent:spawn`, `agent:session`, `agent:connection`, `agent:ext-api`, `agent:fs`, `agent:git` mà code thực tế đã có). Đây là doc/code drift bổ sung cần ghi nhận, tương tự tinh thần mục 8 CR-TRACE-000 — không phải lỗi của solution này, nhưng **thay đổi hoàn toàn base case**: agent side không "hoàn toàn không có tracer nào" như BL-AIP-01/03 §1 của CR-TRACE-016 mô tả (mô tả đó đúng cho phía backend `AIProviderService.ts`/`ProviderHealthChecker.ts`, KHÔNG đúng cho phía agent).

Ánh xạ 3 sub-flow của CR-TRACE-016 sang thực tế agent-side:

| Sub-flow CR | Thành phần backend (đã có tracer đề xuất riêng, ngoài phạm vi) | Thành phần agent-side (đã có tracer, phạm vi solution này) |
|---|---|---|
| BL-AIP-01 (ghi credential) | `AIProviderService.writeCredentialToDevServer()` — lookup account, `relayPool.getOrConnect()` | `handleWriteCredential()` — nhận `ai.provider.writeCredential`, đã có `credTracer` |
| BL-AIP-02 (resolution cascade) | `ProviderResolver.resolve()` — **hoàn toàn in-process, không gọi relay/agent** (xác nhận qua CR §2: "n/a — không băng qua boundary") | **KHÔNG có** — BL-AIP-02 không bao giờ chạm tới Dev Server |
| BL-AIP-03 (health check cron) | `ProviderHealthChecker` — vòng lặp gọi `testConnection()` cho N account | `handleHealthCheck()` — nhận `ai.provider.healthCheck`, đã có `credTracer` |
| (bổ sung, ngoài 3 sub-flow chính) "read-credential-for-spawn" — điểm agent tự đọc lại credential khi backend không truyền sẵn plaintext key | — | `readDecryptedKey()` (gọi bởi `buildAgentEnv()` trong `agent-spawner.ts:225`) → tái sử dụng `handleReadCredential()`, đã có `credTracer` |

Vì vậy phạm vi solution này **không phải "thêm tracer mới"** mà là: (1) bổ sung field `blobLength` đúng theo spec CR-TRACE-016 §4 mà code hiện tại còn thiếu, (2) thêm cơ chế correlation field cho điểm "read-credential-for-spawn" để nối với span `agent:spawn` đang gọi nó, và (3) khẳng định + bảo vệ bằng test ràng buộc bảo mật (không log `apiKey`/`encryptedBlob`/`iv`) mà CR-TRACE-016 §1 yêu cầu nghiêm ngặt.

## 2. Gap hiện tại

**Gap 1 — thiếu `blobLength`:** CR-TRACE-016 §4 BL-AIP-01 yêu cầu `start` fields gồm `accountId, blobLength: encryptedBlob.length`. Code thực tế tại `agent-credential-store.ts:113`:

```typescript
const span = credTracer.start({ method: 'ai.provider.writeCredential', accountId })
```

— thiếu `blobLength`. Không vi phạm ràng buộc bảo mật (không log giá trị blob), nhưng thiếu tín hiệu debug hữu ích mà CR yêu cầu tường minh ("Add Provider Account bị treo/lỗi" cần phân biệt blob rỗng/bất thường vs. lỗi mạng).

**Gap 2 — không có correlation giữa `agent:spawn` và `agent:credential` khi spawn cần đọc lại credential:** Khi `agent.spawn` không nhận được `resolvedApiKey` từ backend (fallback path, `agent-spawner.ts:217-231`), nó gọi `readDecryptedKey()` → tạo ra một span `agent:credential` **độc lập, id ngẫu nhiên riêng**, không có cách nào trong TracePanel biết span đó thuộc về lần `agent.spawn` nào ngoài suy luận theo `accountId` + timestamp gần nhau. Core API `resume` (CR-TRACE-000 §3.1, `Tracer.start(fields, resume)`) **chưa được triển khai** — đã kiểm tra `src/shared/trace/index.ts:46-48,150-152`: `Tracer.start()` chỉ nhận `fields`, không có tham số `resume` nào tồn tại trong code hiện tại. Vì vậy không thể literally "resume" span; giải pháp khả dụng ngay là field nghiệp vụ thuần tuý (giống mô hình `parentTraceId` mà CR-TRACE-017 đề xuất), không phải cơ chế resume chuẩn.

**Ràng buộc bảo mật (đã verify tuân thủ, cần bảo vệ bằng test):** đọc toàn bộ 4 RPC handler trong `agent-credential-store.ts`, xác nhận không có `span.start()/step()/ok()/fail()` nào truyền `encryptedBlob`, `iv`, hay `apiKey` vào fields — chỉ `accountId`, `method`, `provider`, `latencyMs`, `note`, `deleted`. Solution này giữ nguyên tính chất đó và bổ sung comment + test để tránh regression trong tương lai.

## 3. Full Implementation

### 3.1. Thêm `blobLength` vào `handleWriteCredential`

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

Không thêm `step()` cho bước mkdir/writeFileSync — đúng nguyên tắc §5 CR-TRACE-000 (thao tác filesystem in-process, nhanh, không phải network hop, không cần breakdown riêng).

### 3.2. Correlation field cho "read-credential-for-spawn"

```typescript
// src/relay/agent-credential-store.ts — handleReadCredential() + readDecryptedKey()

export async function handleReadCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId    = typeof params.accountId    === 'string' ? params.accountId    : ''
  // CR-TRACE-016 §5 (áp dụng theo mô hình parentTraceId của CR-TRACE-017 §4):
  // field nghiệp vụ thuần tuý để nối span này với span agent:spawn đã gọi nó
  // (qua buildAgentEnv() → readDecryptedKey()) — KHÔNG phải Tracer.start() `resume`
  // (CR-TRACE-000 §3.1), vì core API đó chưa ship (Tracer.start() hiện chỉ nhận `fields`).
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

### 3.3. Thread `parentSpanId` từ `agent:spawn` xuống `buildAgentEnv()`

```typescript
// src/relay/agent-spawner.ts

export async function buildAgentEnv(
  req:            AgentEnvRequest | AgentSpawnRequest,
  spec:           AgentBinarySpec,
  config:         AgentConfig,
  resolvedApiKey: string | null,
  log?:           AgentLogger,
  parentSpanId?:  string,   // NEW — CR-TRACE-016: correlates the fallback
                            // credential-read span with the agent:spawn span
                            // that triggered it (see agent-credential-store.ts)
): Promise<Record<string, string>> {
  // ...unchanged normalisation of accountId/userId/taskId/projectId/cwd...
  // ...unchanged base env object...

  if (resolvedApiKey) {
    base['ANTHROPIC_API_KEY'] = resolvedApiKey
    base['OPENAI_API_KEY']    = resolvedApiKey
    base['GEMINI_API_KEY']    = resolvedApiKey
  } else if (spec.apiKeyEnvVar && accountId) {
    const logFn = log ?? { info: () => {}, warn: () => {}, error: () => {}, debug: () => {} }
    const blob = await readDecryptedKey(accountId, config, logFn as AgentLogger, parentSpanId)
    if (blob) {
      base[spec.apiKeyEnvVar] = blob
      logFn.warn?.(`buildAgentEnv: injecting Layer1 blob for ${spec.apiKeyEnvVar} — agent may fail auth if key not plaintext`)
    } else {
      logFn.warn?.(`buildAgentEnv: no credential found for accountId=${accountId} — agent will fail authentication`)
    }
  }

  // ...unchanged localInference / extraEnv merge...
}
```

```typescript
// src/relay/agent-spawner.ts — handleAgentSpawn(), call site

const span = spawnerTracer.start({ method: 'agent.spawn', taskId: req.taskId, modelId: req.modelId })
// ...validation unchanged...
const envBase = await buildAgentEnv(
  req,
  spec,
  config,
  resolvedApiKey ?? null,
  log,
  span.id,   // NEW — CR-TRACE-016 correlation field
)
```

## 4. Test Plan (Vitest)

File: `src/relay/__tests__/agent-credential-store.test.ts` (mở rộng file test hiện có — tái sử dụng `mockLog`, `makeConfig()`, `MASTER_KEY` fixture đã có).

```typescript
import { registerTraceSink, type TraceEvent } from '../../shared/trace'

describe('handleWriteCredential — blobLength (CR-TRACE-016)', () => {
  it('start event includes blobLength = encryptedBlob.length', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    await handleWriteCredential(1, { accountId: 'acc-1', encryptedBlob: 'x'.repeat(128), iv: 'iv1' }, makeConfig(), mockLog)
    unregister()
    const start = events.find(e => e.flow === 'agent:credential' && e.level === 'start')!
    expect(start.fields.blobLength).toBe(128)
  })
})

describe('handleReadCredential — parentSpanId correlation (CR-TRACE-016)', () => {
  it('forwards parentSpanId into the span fields when present', async () => { /* pass params.parentSpanId, assert start.fields.parentSpanId */ })
  it('omits parentSpanId cleanly when absent (existing RPC callers unaffected)', async () => { /* no parentSpanId param, assert field undefined/not-serialized */ })
})

describe('SECURITY — no trace event ever contains raw credential material (CR-TRACE-016 §1)', () => {
  it('write/read/healthCheck/delete: no TraceEvent.fields value equals or contains the plaintext encryptedBlob/iv used in the test', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const SECRET_BLOB = 'super-secret-blob-value-should-never-appear-in-trace'
    const SECRET_IV    = 'super-secret-iv-value'
    await handleWriteCredential(1, { accountId: 'acc-sec', encryptedBlob: SECRET_BLOB, iv: SECRET_IV }, makeConfig(), mockLog)
    await handleReadCredential(2, { accountId: 'acc-sec' }, makeConfig(), mockLog)
    unregister()
    for (const e of events) {
      const serialized = JSON.stringify(e.fields)
      expect(serialized).not.toContain(SECRET_BLOB)
      expect(serialized).not.toContain(SECRET_IV)
      expect(Object.keys(e.fields)).not.toContain('apiKey')
      expect(Object.keys(e.fields)).not.toContain('encryptedBlob')
      expect(Object.keys(e.fields)).not.toContain('iv')
    }
  })
})
```

Test cho `buildAgentEnv()` (`src/relay/__tests__/agent-spawner.test.ts`, mở rộng suite `buildAgentEnv` sẵn có): thêm case `'threads parentSpanId through to readDecryptedKey (mocked)'` — mock `readDecryptedKey` bằng `vi.mock('../agent-credential-store')` và assert nó được gọi với đúng `span.id` truyền xuống.

## 5. Acceptance Criteria

- [ ] `handleWriteCredential()` span `start` chứa `blobLength: encryptedBlob.length` — khớp đúng spec CR-TRACE-016 §4 BL-AIP-01
- [ ] Không có bất kỳ trace event nào (start/step/ok/fail) trong `agent-credential-store.ts` chứa field `apiKey`, `encryptedBlob`, hoặc `iv` — verify bằng test SECURITY ở mục 4, không chỉ bằng code review thủ công
- [ ] `readDecryptedKey()`/`handleReadCredential()` nhận và forward `parentSpanId` optional, backward-compatible với mọi call site hiện có (không truyền `parentSpanId` vẫn hoạt động bình thường)
- [ ] `buildAgentEnv()` truyền `span.id` của `agent:spawn` xuống `readDecryptedKey()` tại nhánh fallback (không có `resolvedApiKey` từ backend)
- [ ] Ghi nhận rõ trong tài liệu: BL-AIP-02 (`ProviderResolver.resolve()`) không có và không cần counterpart phía agent — xác nhận qua CR-TRACE-016 §2 ("n/a — không băng qua boundary")
- [ ] Ghi nhận rõ: `agent:credential` là tracer pre-existing chưa được liệt kê trong CR-TRACE-000 GAP-3 — không tạo tracer trùng tên/trùng chức năng
- [ ] `credTracer.start()` cho `handleHealthCheck()`/`handleDeleteCredential()` giữ nguyên không đổi (đã đủ theo CR-TRACE-016 BL-AIP-03, không cần thêm field)
