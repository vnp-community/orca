# TASK-012: Sửa `src/relay/preflight-handler.ts` — Thêm `platform` vào `detectAgents` response

**Phase:** 1 — Remote Agent Detection  
**Solution:** [SOL-003](../solutions/SOL-003-remote-agent-detection.md) §2  
**Depends on:** (không — relay-side change)  
**Blocks:** TASK-013

---

## Mục tiêu

Sửa handler `preflight.detectAgents` trong relay để trả về `platform` (process.platform của dev server) cùng với danh sách agents.

---

## File cần sửa

**Path:** `src/relay/preflight-handler.ts`

---

## Thay đổi cần thực hiện

Tìm method `detectAgents` (hoặc handler cho `preflight.detectAgents`) và thêm `platform` vào response:

```typescript
private async detectAgents(params: Record<string, unknown>): Promise<{
  agents: string[]
  platform: NodeJS.Platform   // NEW — platform của dev server
}> {
  // ... existing detection logic giữ nguyên ...

  return {
    agents: [...foundAgentIds],   // existing
    platform: process.platform    // NEW — dev server platform
  }
}
```

---

## Acceptance Criteria

- [x] Response của `preflight.detectAgents` có field `platform: NodeJS.Platform`
- [x] `platform` chứa đúng `process.platform` của relay process (dev server)
- [x] Logic detect agents hiện có không bị thay đổi
- [x] Backward compatible: host side vẫn nhận được `agents` đúng
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Đọc `src/relay/preflight-handler.ts` để tìm đúng tên method và cấu trúc hiện tại
2. Không refactor logic detection — chỉ thêm `platform` vào return value
3. Kiểm tra type của return value để cập nhật nếu cần
