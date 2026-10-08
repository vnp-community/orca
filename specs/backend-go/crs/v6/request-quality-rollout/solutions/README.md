# request-quality-rollout: solutions (backend-go, v6)

> Cập nhật 2026-10-08: SOL-024 ✅ 8/8 task; SOL-025 🚧 4/8 task (xem `../tasks/README.md` và `../IMPLEMENTATION-NOTES.md`). CR nguồn: [`docs/crs/v6/request-quality-rollout/`](../../../../../../docs/crs/v6/request-quality-rollout/README.md). Hợp đồng: [`docs/crs/v6/README.md`](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3).
> CR nguồn: [`docs/crs/v6/request-quality-rollout/`](../../../../../../docs/crs/v6/request-quality-rollout/README.md). Hợp đồng: [`docs/crs/v6/README.md`](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3).

## CR → Solution

| CR | Solution | Nội dung | Service | Priority |
|---|---|---|---|---|
| CR-REQ-024 | [BE-REQ-SOL-024](./BE-REQ-SOL-024-jira-status-sync-audit-observability.md) | `issue-status-sync` nghe sự kiện Request, bảng `request_sync_state`, audit `AppendDetailed`, metric, alert, `LookupRequestBySource` | `issue-status-sync`, `request-service`, `common/auditclient` | P1 |
| CR-REQ-025 | [BE-REQ-SOL-025](./BE-REQ-SOL-025-e2e-feature-flag-rollout.md) | Cờ `request_flow_enabled` (bảng, RPC, interceptor), e2e T1/T2/T3, script kiểm đăng ký, rollout, runbook | `request-service`, `ci`, `.github`, `tests/request`, `docs` | P0 |

## Thứ tự phụ thuộc

```
lõi (CR-REQ-001..016) ổn định
        │
        ├─▶ BE-REQ-SOL-025 phần cờ (task 01, 02)  ◀── CR-016, 017 đã trỏ tới request.flowStatus
        │              │
        │              ▼
        ├─▶ BE-REQ-SOL-024 ─────────────┐   (LookupRequestBySource đọc cờ)
        │                                ▼
        └─▶ BE-REQ-SOL-025 phần e2e, wiring, T2, tài liệu (task 03..08)
                                         ▼
                                  rollout giai đoạn 0 → 4
```

Phần cờ của SOL-025 vào sớm; phần e2e và tài liệu làm sau cùng.

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| Q1 | Issue có Request chưa kết thúc thì chỉ Request đổi Jira; đồng bộ worktree, PR bị bỏ qua | tránh "Done" sớm; một nguồn ghi |
| Q2 | Chỉ Jira (`syncableProviders`) | giữ ràng buộc hiện có |
| Q3 | Không đóng, huỷ issue khi Request backlog hoặc huỷ | tránh mất dữ liệu tracker |
| Q4 | Audit qua `auth-service` (best effort), metric trên `/metrics` | mẫu repo |
| Q5 | Cờ hai tầng, `request-service` thi hành bằng interceptor có bảng phân loại RPC | một điểm, RPC mới không lọt |
| Q6 | Rollback là tắt cờ, không `down` migration | không mất dữ liệu `plan|phase` |
| Q7 | `traceparent` trong payload sự kiện, không sửa `common/eventbus` | tránh chạm mọi service |
| Q8 | T1 chặn PR; T2, T3 không | AI thật và stack đầy đủ không ổn định |

## Phát hiện đáng chú ý (đã đối chiếu code)

- Sự kiện PR hiện không có `ActorUserID` và `LinkedIssueSite`; `canSync` đã bỏ qua chúng. Rủi ro "Done sớm" của CR-024 chưa xảy ra với PR cho tới khi scm-integration-service phát actor.
- Handler hiện nuốt lỗi và `MarkSeen`; CR-024 cần handler trả lỗi để Nak: phải thêm bộ đếm giao lại (consumer không thấy giới hạn `MaxDeliver`).
- `AppendAuditEntryRequest` đã có `actor_type`, `target_type`, `target_id`, `metadata_json`; chỉ client chưa dùng.
- Stub AI cho T2 không phải `ai-provider-service` mà là dev server agent (relay `ai.complete`); chưa có trong repo.
- `/metrics` chưa có ở `issue-status-sync`; `notification-service` có mẫu `healthAndMetricsMux`.
