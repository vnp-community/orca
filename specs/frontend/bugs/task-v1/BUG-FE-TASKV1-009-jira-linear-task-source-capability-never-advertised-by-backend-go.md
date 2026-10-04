# BUG-FE-TASKV1-009 — Jira/Linear task source luôn hiện "unavailable" trên web/backend-go — `task-source-context.v1` capability chưa từng được backend-go advertise

## Mức độ: 🔴 HIGH

## Trạng thái: ✅ Root cause CONFIRMED chính xác đến từng dòng code (2026-09-15) — chưa fix.

## Root Cause — CONFIRMED CHÍNH XÁC ĐẾN TỪNG DÒNG (2026-09-15)

Tìm thấy đúng nơi duy nhất `capabilities` được backend-go gửi cho frontend:

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:741-754` (`registerStatusChannels`, RPC `status.get`):
```go
r.Register("status.get", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
    return map[string]any{
        ...
        "capabilities": []string{"browser.screencast.v1"},
        ...
    }, nil
})
```

Đây là **danh sách hardcode 1 phần tử duy nhất** — trong khi `frontend/src/shared/protocol-version.ts`'s `RUNTIME_CAPABILITIES` liệt kê **15 capability** mà hệ Node.js cũ advertise (`task-source-context.v1`, `project-host-setup.v1`, `workspace-run-context.v1`, `aiVault.v1`, `terminal.multiplex.v1`, v.v.). `TaskPage.tsx`'s `host.capabilities.includes(TASK_SOURCE_CONTEXT_RUNTIME_CAPABILITY)` luôn `false` vì backend-go chỉ bao giờ trả về `["browser.screencast.v1"]`.

**Xác nhận qua UI thật**: user gặp đúng chuỗi `"Jira source unavailable: Orca Session server update needed for task sources"` — khớp chính xác với `task-source-context-summary.ts:249-250`'s mapping cho `reason: 'missing-task-source-capability'`.

**Phạm vi thật RỘNG HƠN chỉ Jira/Linear**: cùng gap này khả năng ảnh hưởng cả 14 capability còn thiếu khác (một số — như `aiVault.v1` — comment ghi rõ "STATIC, advertised unconditionally for every build", nghĩa là backend-go ĐÁNG LẼ phải luôn trả về nhưng không làm) — không mở rộng điều tra các capability khác trong bug này, chỉ ghi nhận.

## Tóm tắt

Sau khi kết nối Jira thành công (verify bằng token thật, `200 OK`), chọn nguồn "Jira" trong Task page vẫn hiện **unavailable/disabled** — hoàn toàn không liên quan tới trạng thái connect Jira.

## Root Cause — CONFIRMED

`TaskPage.tsx`'s `getTaskSourceHostAvailabilityForHost` (dùng cho Jira/Linear — các provider "account-backed", khác GitHub/GitLab "repo-backed"):

```ts
if (host.kind === 'runtime') {
  if (!host.capabilities) {
    return { hostId, reason: 'checking-task-source-capability' }
  }
  if (!host.capabilities.includes(TASK_SOURCE_CONTEXT_RUNTIME_CAPABILITY)) {
    return { hostId, reason: 'missing-task-source-capability' }   // ← luôn rơi vào đây
  }
}
```

`TASK_SOURCE_CONTEXT_RUNTIME_CAPABILITY = 'task-source-context.v1'` (`frontend/src/shared/protocol-version.ts`). Grep xác nhận:
```
frontend/src/shared/protocol-version.ts  → định nghĩa hằng số
backend/src/shared/protocol-version.ts   → có (hệ Node.js/Electron cũ)
desktop/src/shared/protocol-version.ts   → có
agent/src/shared/protocol-version.ts     → có
backend-go/                              → 0 kết quả — KHÔNG tồn tại ở đâu cả
```

**`backend-go` (toàn bộ 17 service chạy trên `b15.openledger.vn`) chưa từng implement/advertise capability `task-source-context.v1`** — tính năng "account-backed task source" (Jira/Linear browse trong Task page) được xây cho hệ runtime cũ (Node.js `backend`/`desktop`), **chưa được port sang backend-go**. Vì "runtime" host (web/backend-go) không bao giờ khai báo capability này, `getTaskSourceHostAvailabilityForHost` luôn trả `missing-task-source-capability` — độc lập hoàn toàn với việc Jira đã connect hay chưa.

## Ảnh hưởng

Toàn bộ luồng "browse Jira/Linear issue trong Task page, tạo worktree từ issue" (đã điều tra/giải thích nhiều lần trong session này) **không dùng được trên web/backend-go deployment**, bất kể backend Jira integration (issue-tracking-service) đã hoạt động đúng — đây là gap ở tầng "capability advertisement" của runtime, không phải ở Jira adapter.

## Cùng họ với

- [BUG-FE-PW-005](../project-workspace/BUG-FE-PW-005-agent-panel-window-api-undefined-on-web.md) — `AgentPanel`'s `window.api.agentOrchestration` cũng chưa port sang web/backend-go. Cả 2 bug đều là "tính năng xây cho hệ Node.js/Electron cũ, chưa port sang backend-go" — pattern lặp lại thứ 2 trong cùng 1 session.

## Fix direction (chưa làm — cần xác nhận phạm vi trước)

1. Xác định "runtime capabilities" advertisement thật sự hoạt động thế nào trên backend-go hiện tại (chỗ nào trả về `host.capabilities` cho 1 "runtime" host trong web mode) — chưa điều tra trong phiên này.
2. Thêm `task-source-context.v1` vào danh sách capability backend-go quảng bá, VÀ đảm bảo `issue-tracking-service`'s API (đã hoạt động, verify bằng token thật) thực sự đáp ứng đúng "hợp đồng" mà `task-source-context.v1` client-side mong đợi (không chỉ thêm cờ giả).
3. Cần review thêm: `RuntimeEnvironmentsPane.tsx`, `AutomationsPage.tsx` cũng check cùng capability này — sửa 1 chỗ có khả năng ảnh hưởng cả 2 màn hình đó.

## Liên quan

- Phát hiện trong lúc điều tra "vẫn hiện jira source unavailable" sau khi user connect Jira thành công (CR-JIRA-001/BUG-013 đã fix xong, xác nhận hoạt động bằng token thật).
