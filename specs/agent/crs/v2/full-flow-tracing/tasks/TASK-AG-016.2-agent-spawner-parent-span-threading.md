# TASK-AG-016.2: Thread parentSpanId from agent:spawn span through buildAgentEnv()

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-016](../solutions/SOL-AG-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Precondition:** Phase 0 + [TASK-AG-016.1](./TASK-AG-016.1-agent-credential-store-bloblength-and-correlation.md) (needs `readDecryptedKey(..., parentSpanId?)` signature) + [TASK-AG-002.3](./TASK-AG-002.3-agent-spawner-orchestration-spans.md) (touches the same `handleAgentSpawn()` call site — apply on top of `orchSpan` code already present)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — Implemented as specified. Current source already had `orchSpan` (from TASK-AG-002.3) alongside `span` (spawnerTracer) exactly as documented; added `parentSpanId?: string` as the 6th optional param on `buildAgentEnv()`, forwarded it to `readDecryptedKey()` in the fallback branch, and passed `span.id` (not `orchSpan.id`) at the `handleAgentSpawn()` call site. `pnpm run typecheck:node` clean for this file.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "buildAgentEnv"
codegraph explore "handleAgentSpawn"
```

Cả 2 đều là symbol MODIFY (đã tồn tại — `handleAgentSpawn` vừa được TASK-AG-002.3 thêm `orchSpan`, task này chỉ thêm tham số `span.id` vào call site `buildAgentEnv(...)` đã có, không đè logic đó) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "buildAgentEnv", direction: "upstream" })
gitnexus_impact({ target: "handleAgentSpawn", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

Khi `agent.spawn` không nhận được `resolvedApiKey` từ backend (fallback path), nó gọi `readDecryptedKey()` → tạo ra một span `agent:credential` độc lập, id ngẫu nhiên riêng, không cách nào trong TracePanel biết span đó thuộc về lần `agent.spawn` nào. Core API `resume` (CR-TRACE-000 §3.1) chưa ship — giải pháp khả dụng ngay là field nghiệp vụ thuần tuý `parentSpanId`, không phải cơ chế resume chuẩn.

**Lưu ý merge với TASK-AG-002.3:** hàm `handleAgentSpawn()` đã được TASK-AG-002.3 sửa để thêm biến `orchSpan` (`agentOrch:spawn`/`resume`). Task này thêm tham số `span.id` (biến `span` = `spawnerTracer`, KHÁC với `orchSpan`) vào lời gọi `buildAgentEnv(...)` đã tồn tại — áp dụng trên code đã có `orchSpan`, không xoá/đè logic đó.

## File: `src/relay/agent-spawner.ts` [MODIFY]

### `buildAgentEnv()` — thêm tham số `parentSpanId`

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

### `handleAgentSpawn()` — call site, truyền `span.id`

```typescript
// src/relay/agent-spawner.ts — handleAgentSpawn(), call site
// Base logic (bao gồm `orchSpan` từ TASK-AG-002.3) giữ nguyên — CHỈ sửa tham số
// thứ 6 truyền vào buildAgentEnv().

const span = spawnerTracer.start({ method: 'agent.spawn', taskId: req.taskId, modelId: req.modelId })
// ...validation unchanged (bao gồm orchSpan.fail(...) từ TASK-AG-002.3 nếu missing/unknown model)...
const envBase = await buildAgentEnv(
  req,
  spec,
  config,
  resolvedApiKey ?? null,
  log,
  span.id,   // NEW — CR-TRACE-016 correlation field (spawnerTracer span, KHÔNG phải orchSpan)
)
```

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-spawner" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `buildAgentEnv()` nhận tham số `parentSpanId?: string` optional cuối cùng, backward-compatible với call site cũ
- [ ] `buildAgentEnv()` truyền `parentSpanId` xuống `readDecryptedKey()` tại nhánh fallback (không có `resolvedApiKey`)
- [ ] `handleAgentSpawn()` truyền `span.id` (biến `spawnerTracer`, KHÔNG phải `orchSpan.id`) vào `buildAgentEnv(...)`
- [ ] Code `orchSpan` (từ TASK-AG-002.3) KHÔNG bị xoá/đè — vẫn hoạt động song song
- [ ] `pnpm run typecheck:node` pass
