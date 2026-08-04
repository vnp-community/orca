# TASK-AG-014.2: Add agent:preflight tracer to handlePreflightCheck in fs-agent-extensions.ts

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-014](../solutions/SOL-AG-TRACE-014-remote-integration.md)
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Precondition:** Phase 0 (`Tracer.start(fields?, resume?)`)
**Estimated time:** 0.5h
**Status:** ✅ Done (2026-08-03) — implemented as specified; new local `createTracer('agent:preflight')` added alongside existing `fsTracer`. No drift — `handlePreflightCheck` matched the task's code sample. Note: `fs-agent-extensions.ts` also carries unrelated pre-existing uncommitted changes (an `fs.watch`/`fs.unwatch` feature from concurrent work) — confirmed via `git diff` that only the `preflightTracer` constant and `handlePreflightCheck` body were touched by this task. `gitnexus_impact` on `handlePreflightCheck` reported LOW risk (0 direct callers — dispatched dynamically). typecheck:node clean for this file.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "handlePreflightCheck"
```

`handlePreflightCheck` là symbol MODIFY (đã tồn tại — task này chỉ thêm tracer `agent:preflight` mới, không đổi logic check service) — chạy thêm

```
gitnexus_impact({ target: "handlePreflightCheck", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

Ngoài `external-api-connector.ts` (phạm vi chính CR-014), điều tra phát hiện thêm **một implementation preflight thứ ba** chưa được CR-TRACE-014 đề cập: `handlePreflightCheck()` trong `src/relay/fs-agent-extensions.ts`, gọi qua Agent WS JSON-RPC method `preflight.check`. Khác với `PreflightHandler.checkFullPreflight()` (`preflight-handler.ts`, mode `relay-ssh`, CR-TRACE-014 đã loại trừ rõ) — `handlePreflightCheck()` chạy cùng tiến trình/kênh Agent WS JSON-RPC nên nằm trong phạm vi.

## File: `src/relay/fs-agent-extensions.ts` [MODIFY]

Thêm tracer mới cạnh `fsTracer` đã có ở đầu file:

```typescript
// src/relay/fs-agent-extensions.ts
// (thêm tracer mới cạnh fsTracer đã có ở đầu file)

const preflightTracer = createTracer('agent:preflight')

export async function handlePreflightCheck(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const services = Array.isArray(params.services) ? params.services.map(String) : []
  const results: Record<string, boolean> = {}
  const span = preflightTracer.start({ services: services.join(',') || '(empty)' })

  await Promise.all(services.map(async (service) => {
    try {
      switch (service) {
        case 'github-cli':
          results[service] = await checkBinaryAvailable('gh', config)
          break
        case 'ripgrep':
          results[service] = await checkRgAvailable()
          break
        case 'docker':
          results[service] = await checkBinaryAvailable('docker', config)
          break
        case 'claude':
          results[service] = await checkBinaryAvailable('claude', config)
          break
        default:
          results[service] = false
      }
    } catch {
      results[service] = false
    }
  }))

  const failedServices = Object.entries(results).filter(([, ok]) => !ok).map(([svc]) => svc)
  if (failedServices.length > 0) {
    span.fail(`unavailable: ${failedServices.join(',')}`, { failedCount: failedServices.length })
  } else {
    span.ok({ checkedCount: services.length })
  }
  return { jsonrpc: '2.0', id, result: results }
}
```

Đặt tên `agent:preflight` (không dùng `remoteIntegration:preflight` — namespace đó thuộc backend/Main process; agent-side theo convention `agent:xxx` cục bộ). `span.fail()` ở đây dùng cho "có service không khả dụng" (business-level fail, không phải exception) — phân biệt rõ trạng thái fail thay vì chỉ dựa vào exception, đúng yêu cầu CR-TRACE-014.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "fs-agent-extensions" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `agent:preflight` tracer mới thêm — KHÔNG trùng tên/chức năng với `agent:fs` (đã có trong cùng file, dùng cho `fs.*`)
- [ ] `handlePreflightCheck` có `fail()` khi có service không khả dụng — không chỉ dựa vào exception
- [ ] `span.ok({checkedCount})` khi tất cả services khả dụng
- [ ] `remoteIntegration:credentialDecrypt`/`remoteIntegration:preflight` KHÔNG xuất hiện trong file này — các tracer đó chỉ tồn tại ở Main process
- [ ] `pnpm run typecheck:node` pass
