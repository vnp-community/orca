# Feature: request-quality-rollout — Đồng bộ Jira, audit, quan sát, kiểm thử và rollout

> **Trạng thái:** 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code ngày 2026-10-05, chưa chạy hệ thống.
> **Hợp đồng chung:** [`../README.md`](../README.md).

## 1. Mục tiêu

Làm cho luồng Request vận hành được: Jira phản ánh đúng tiến độ theo Request (không theo từng PR), mọi quyết định có vết audit, có số đo và cảnh báo, có bộ kiểm thử đầu cuối cho 11 loại, và bật dần bằng cờ `request_flow_enabled` với kế hoạch rollback.

## 2. Danh sách CR

| CR | Tên | Priority | Effort | Phụ thuộc | Mở khoá |
|---|---|---|---|---|---|
| [CR-REQ-024](./CR-REQ-024-jira-status-sync-audit-observability.md) | Đồng bộ trạng thái Jira theo Request, audit, observability | 🟠 P1 | Medium | CR-REQ-003, 004, 006, 013 | CR-REQ-025 (rollout dựa vào alert) |
| [CR-REQ-025](./CR-REQ-025-e2e-tests-feature-flag-rollout.md) | Kiểm thử đầu cuối, feature flag, rollout, tài liệu | 🔴 P0 | Medium | CR-REQ-001 đến 024 (đủ cho phần được bật) | GA |

## 3. Thứ tự thực thi

```
lõi (CR-REQ-001..016) ổn định ─▶ CR-REQ-024 ─▶ CR-REQ-025
```

Hai CR chạy gần song song: bảng cờ và RPC cờ của CR-025 (mục 2.2) nên vào sớm, vì CR-016 và CR-017 đã trỏ tới nó; phần e2e, tài liệu và rollout làm sau cùng.

## 4. Quyết định chung của feature

| # | Quyết định | Lý do |
|---|---|---|
| Q1 | Khi một issue Jira đã có Request đang xử lý, Request là bên duy nhất đổi trạng thái Jira; đồng bộ theo worktree và PR của `issue-status-sync` bị bỏ qua cho issue đó | Một PR merge không có nghĩa Request xong (còn Phase khác); hai bên ghi cùng issue gây Done sớm |
| Q2 | Chỉ Jira được đồng bộ trạng thái | Giữ nguyên `syncableProviders` (CR-TG-008): Linear cần UUID state, GitHub chưa có đường credential theo user |
| Q3 | Không bao giờ tự đóng hay huỷ issue Jira vì Request bị huỷ hay trả về backlog | Cùng bài học `worktree.deleted` ở CR-TG-008: mất dữ liệu trên tracker |
| Q4 | Audit đi qua `auth-service.AppendAuditEntry` (best effort); metric Prometheus trên cổng health `/metrics` | Mẫu có sẵn (`common/auditclient`, `mcp-service`, `notification-service`) |
| Q5 | Cờ `request_flow_enabled`: hai tầng (biến môi trường tổng và bảng cài đặt theo tenant), `request-service` là nơi duy nhất thi hành, mặc định tắt | Mẫu `MCP_ENABLED` + `tenant_settings` của MCP; một điểm thi hành |
| Q6 | Rollback là tắt cờ; dữ liệu giữ nguyên; không có migration phá huỷ trong v6 | Có thể đảo ngược không mất dữ liệu |

## 5. Phát hiện chung

- `backend-go/docker-compose.yml` chỉ chạy `postgres`, `vault`, `nats`; stack đầy đủ nằm ở `deploy/dev/docker-compose.yml` (có service, `migrate-*`, `MCP_ENABLED`). `deploy/dev/scripts/migrate.sh` từng quên `issuetracking` nên DB không bao giờ được migrate; CR-025 thêm script kiểm đăng ký `request-service` ở mọi chỗ.
- `backend-go/deploy/postgres-init-databases.sh` liệt kê cứng danh sách DB; CR-REQ-001 thêm `request`.
- `common/outbox` và `common/eventbus` không thấy truyền ngữ cảnh trace (grep `traceparent` rỗng), nên trace không liền mạch qua sự kiện cho tới khi sửa.
- `common/auditclient.Append` không mang `actor_type`, `target_type`, `metadata_json` dù proto `AppendAuditEntryRequest` có; cần thêm phương thức mới.
- README hợp đồng v6 còn thiếu RPC `LookupRequestBySource`, `GetRequestFlowSettings`, `SetRequestFlowSettings`, và bảng `tenant_settings`; xem mục "Câu hỏi mở" của hai CR.
