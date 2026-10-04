# CR-TSRC-001 — Port `task-source-context.v1` capability advertisement sang backend-go — Jira/Linear task source luôn "unavailable" trên web

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TSRC-001 |
| **Tên** | Task page's Jira/Linear source luôn hiện "unavailable" trên web/backend-go — capability `task-source-context.v1` chưa từng được backend-go quảng bá |
| **Loại** | Feature Gap |
| **Priority** | 🔴 P0 — chặn hoàn toàn tính năng browse Jira/Linear trong Task page trên web, độc lập với việc Jira/Linear đã connect đúng hay chưa |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | ✅ Root cause CONFIRMED, 🔲 chưa thiết kế fix |
| **Tác giả** | User report: kết nối Jira thành công (verify bằng token thật) nhưng nguồn "Jira" trong Task page vẫn hiện unavailable |
| **Tác động HLD** | `frontend/src/renderer/src/components/TaskPage.tsx`, `RuntimeEnvironmentsPane.tsx`, `AutomationsPage.tsx`; toàn bộ backend-go (chưa có nơi nào implement) |
| **Tác động Features** | Task page — nguồn Jira/Linear (account-backed task source), có thể ảnh hưởng cả Automations |

---

## Bối cảnh & Vấn đề gốc

`TaskPage.tsx`'s `getTaskSourceHostAvailabilityForHost` (cho Jira/Linear — provider "account-backed"):
```ts
if (host.kind === 'runtime' && !host.capabilities.includes(TASK_SOURCE_CONTEXT_RUNTIME_CAPABILITY)) {
  return { hostId, reason: 'missing-task-source-capability' }
}
```
`TASK_SOURCE_CONTEXT_RUNTIME_CAPABILITY = 'task-source-context.v1'`. Grep xác nhận: hằng số này tồn tại ở `frontend/src/shared/`, `backend/src/shared/`, `desktop/src/shared/`, `agent/src/shared/` (hệ Node.js/Electron cũ) — **0 kết quả trong toàn bộ `backend-go/`**. Backend-go (17 service chạy trên `b15.openledger.vn`) **chưa từng implement khái niệm "runtime capability advertisement" cho tính năng này** — tính năng được xây cho hệ chạy cũ, chưa port.

Vì "runtime" host (chính là kết nối web/backend-go) không bao giờ khai báo capability này, điều kiện trên **luôn đúng (luôn thiếu)**, độc lập hoàn toàn với trạng thái connect Jira/Linear thật — xác nhận bằng thực tế: user đã connect Jira thành công (verify `curl` với token thật trả `200 OK`), Jira source trong Task page **vẫn** hiện unavailable.

## Giải pháp đề xuất (chưa thiết kế chi tiết — cần điều tra thêm)

1. Xác định cơ chế "runtime capabilities" hiện tại của backend-go hoạt động ra sao (chỗ nào — nếu có — backend-go trả về danh sách capability cho 1 "runtime" host qua wscompat/handshake) — **chưa điều tra trong session này**.
2. Thêm `task-source-context.v1` vào danh sách capability backend-go quảng bá.
3. Xác nhận `issue-tracking-service`'s API contract (đã hoạt động — verify bằng token thật) khớp đúng những gì `task-source-context.v1` phía client mong đợi — không chỉ thêm cờ giả rồi vẫn lỗi ở tầng khác.
4. Review 2 nơi dùng chung capability này (`RuntimeEnvironmentsPane.tsx`, `AutomationsPage.tsx`) — có thể được unlock cùng lúc, cần xác nhận không có side-effect không mong muốn.

## Không thuộc phạm vi CR này

- Sửa `issue-tracking-service`'s adapter Jira/Linear (đã hoạt động đúng, verify bằng token thật — xem CR-JIRA-001) — CR này chỉ về tầng "capability advertisement", không phải backend Jira API.

## Liên quan

- [BUG-FE-TASKV1-009](../../../../specs/frontend/bugs/task-v1/BUG-FE-TASKV1-009-jira-linear-task-source-capability-never-advertised-by-backend-go.md)
- [BUG-FE-PW-005](../../../../specs/frontend/bugs/project-workspace/BUG-FE-PW-005-agent-panel-window-api-undefined-on-web.md) — cùng pattern gốc: tính năng xây cho hệ Node.js/Electron cũ, chưa port sang backend-go/web
- [CR-JIRA-001](../jira-integration/CR-JIRA-001-support-self-hosted-jira-server-data-center.md) — backend Jira adapter đã hoạt động đúng, CR này là lớp chặn KHÁC, phía trên
