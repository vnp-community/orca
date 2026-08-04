# TASK-AG-016.3: Add ai-providers tracing tests (agent-credential-store, agent-spawner)

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-016](../solutions/SOL-AG-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Precondition:** Phase 0 + [TASK-AG-016.1](./TASK-AG-016.1-agent-credential-store-bloblength-and-correlation.md) + [TASK-AG-016.2](./TASK-AG-016.2-agent-spawner-parent-span-threading.md)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — Added all 4 cases to `agent-credential-store.test.ts` (blobLength, 2× parentSpanId, 1× SECURITY, plus removed an unrelated pre-existing unused `statSync` import while touching this file) and 1 case to `agent-spawner.test.ts` (`vi.mock('../agent-credential-store', ...)` + `vi.hoisted` spy — the file's existing `claudeSpec`/`ollamaSpec` fixtures use a mismatched `apiKeyEnv` field name (pre-existing bug, not touched here) so the new test builds a local spec literal with the correct `apiKeyEnvVar` field to actually exercise the fallback branch). `pnpm vitest run` on both files → 91/91 passed. Pre-existing unrelated `apiKeyEnv`/`AgentConfig` TS errors in this file (from earlier TASK-AG-002.4 fixtures) are untouched by this diff — confirmed via `git diff`, ignored per task rules.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "handleWriteCredential"
codegraph explore "handleReadCredential"
codegraph explore "buildAgentEnv"
```

Đây đều là symbol MODIFY (đã tồn tại, vừa được TASK-AG-016.1/016.2 thêm field/tham số) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "handleWriteCredential", direction: "upstream" })
gitnexus_impact({ target: "handleReadCredential", direction: "upstream" })
gitnexus_impact({ target: "buildAgentEnv", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/agent-credential-store.test.ts` [MODIFY]

Mở rộng file test hiện có — tái sử dụng `mockLog`, `makeConfig()`, `MASTER_KEY` fixture đã có.

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

## File: `src/relay/__tests__/agent-spawner.test.ts` [MODIFY]

Mở rộng suite `buildAgentEnv` sẵn có — thêm case:

```typescript
it('threads parentSpanId through to readDecryptedKey (mocked)', async () => {
  // mock readDecryptedKey via vi.mock('../agent-credential-store')
  // assert nó được gọi với đúng span.id truyền xuống
})
```

## Verification

```bash
pnpm vitest run src/relay/__tests__/agent-credential-store.test.ts src/relay/__tests__/agent-spawner.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `agent-credential-store.test.ts` có thêm 4 test case theo trên (blobLength, 2× parentSpanId, 1× SECURITY)
- [ ] Test SECURITY xác nhận KHÔNG có field `apiKey`/`encryptedBlob`/`iv` VÀ không có giá trị secret nào trong `JSON.stringify(fields)` của bất kỳ event nào (write/read/healthCheck/delete)
- [ ] `agent-spawner.test.ts` có thêm test "threads parentSpanId through to readDecryptedKey"
- [ ] `pnpm vitest run src/relay/__tests__/agent-credential-store.test.ts src/relay/__tests__/agent-spawner.test.ts` pass
