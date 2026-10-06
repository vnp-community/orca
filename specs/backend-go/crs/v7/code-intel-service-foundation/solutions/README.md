# backend-go Solutions: Code Intel Service Foundation (v7)

**CRs:** [docs/crs/v7/code-intel-service-foundation](../../../../../../docs/crs/v7/code-intel-service-foundation/README.md)
**Hợp đồng chuẩn tắc:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md), [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md), [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md) (hợp đồng thắng CR khi khác nhau)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

> 📋 Proposed. Chưa triển khai, chưa chạy build/test/migration/`buf`/`opa` nào. Đọc lại code liên quan trước khi sửa.

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-CV-010](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-010-scaffold-code-intel-service.md) | [BE-CV-SOL-010-scaffold-code-intel-service](./BE-CV-SOL-010-scaffold-code-intel-service.md) | `code-intel-service` (mới), `common/apperrors`, `proto`, `go.work`, `deploy/*`, CI | Medium | `BE-CV-TASK-010-01` đến `-08` |
| [CR-CV-011](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md) | [BE-CV-SOL-011-data-model-and-migrations](./BE-CV-SOL-011-data-model-and-migrations.md) | migration `0002`, `internal/domain` | Medium | `BE-CV-TASK-011-01` đến `-05` |
| | [BE-CV-SOL-011-repositories-and-maintenance](./BE-CV-SOL-011-repositories-and-maintenance.md) | repository hai dialect, bảo trì | Medium | `BE-CV-TASK-011-06` đến `-13` |
| [CR-CV-012](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-012-project-worktree-to-repo-binding.md) | [BE-CV-SOL-012-target-resolution-and-bindings](./BE-CV-SOL-012-target-resolution-and-bindings.md) | `ResolveTarget`, `BindRepo`, `ListRepoBindings`, client hạ nguồn, consumer | Medium | `BE-CV-TASK-012-01` đến `-07` |
| | [BE-CV-SOL-012-index-status-aggregation](./BE-CV-SOL-012-index-status-aggregation.md) | `GetIndexStatus`, `overall`, cache | Medium | `BE-CV-TASK-012-08` đến `-11` |
| [CR-CV-013](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md) | [BE-CV-SOL-013-authorization-flags-and-audit](./BE-CV-SOL-013-authorization-flags-and-audit.md) | OPA `code_intel.rego`, `internalcaller`, cờ, audit, pipeline | Medium | `BE-CV-TASK-013-01` đến `-07` |
| | [BE-CV-SOL-013-agent-call-gate-and-quotas](./BE-CV-SOL-013-agent-call-gate-and-quotas.md) | cổng agent, hạn mức, admission reindex, che bí mật | Medium | `BE-CV-TASK-013-08` đến `-12` |

Tổng: 7 solution, 44 task (010: 8; 011: 13; 012: 11; 013: 12; số `NN` tăng liên tục trong từng CR).

## Re-verify trước khi thiết kế (tổng hợp các lệch đã phát hiện, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| `x-go-common-env` cần thêm bốn địa chỉ service | Đã có `PROJECT_SERVICE_ADDR`, `INFRA_FLEET_SERVICE_ADDR`, `GIT_GATEWAY_SERVICE_ADDR`, `AUTH_SERVICE_ADDR`, `NATS_URL` (`docker-compose.yml` dòng 46–60) | **Lệch** ⇒ SOL-010 C3 |
| RLS "như `task-service`" | `task-service` không `set_config`; `mcp-service` làm RLS thật | **Lệch** ⇒ theo `mcp-service` |
| Cờ `app.maintenance`/chính sách bảo trì có mẫu | Mẫu `reconcile_read` dùng `app.relay`, chỉ `SELECT`; không có `app.maintenance` ở mã hiện có | **Lệch** ⇒ cờ mới, chính sách hẹp (SOL-011-data D3) |
| "Không nơi nào gọi `rate.NewLimiter`" | `api-gateway/internal/usecase/rate_limit.go:46` gọi | **Lệch** ⇒ SOL-013-gate C1 |
| `internalcaller.Guard` với token rỗng chặn mọi RPC | Chỉ chặn method **được liệt kê**; `mcp-service` bỏ interceptor khi token rỗng | **Lệch** ⇒ danh sách sinh từ `ServiceDesc`, luôn thêm interceptor (SOL-013-authz C1) |
| `codeintel.status` trả `data.tools[]` | Hình dạng chuẩn là `data.tools{}` + `data.indexes.<tool>` + `data.binding` (PQ-19) | **Lệch** ⇒ SOL-012-status L1 |
| `common/apperrors` không sửa | PQ-03 (8): CR-010 thêm `KindResourceExhausted`, `KindUnavailable` | **Lệch** ⇒ SOL-010 task 02 |
| `CODEINTEL_FEATURE_DISABLED`, `INVALID_ARGUMENT`, `DEV_SERVER_OFFLINE` FailedPrecondition | PQ-01/03: `CODEINTEL_DISABLED`, `INVALID_PARAMS`, `Unavailable` | **Lệch** ⇒ áp hợp đồng |

## Thứ tự thực thi và phụ thuộc

```
SOL-010 (khung service, apperrors Kind, proto, 0001, outbox, main, deploy, CI)
   └─▶ SOL-011-data (0002, domain) ─▶ SOL-011-repos (repository hai dialect, bảo trì)
            └─▶ SOL-012-target (ResolveTarget, BindRepo, consumer)  ─▶ SOL-012-status (GetIndexStatus)
                       └─▶ SOL-013-authz (Rego, internalcaller, cờ, audit, pipeline) ─▶ SOL-013-gate (cổng agent, hạn mức, reindex, che)
```

Ngoài feature: `SOL-020` (G0: `codeintel_common.proto`) cần trước 012-02; `SOL-023` (G2) làm mã lỗi agent thật cho 012-status; `SOL-040` đặt token nội bộ, từ chối phiên thiết bị, bóc mã lỗi; `SOL-021` **phải** dùng `GatedAgentRelay`; `SOL-073` dùng `FlagReader`/`GetSettings`. Có thể song song: `013-02` (Rego) với mọi task; `011-02` với `011-03`; `012-01` với `010`.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Hai dialect từ đầu (`dbcapability.DetectDialectFromDSN`) | O1; hợp đồng H10 |
| F2 | Layout `notification-service`; outbox và RLS theo `mcp-service` | RLS `task-service` không chạy |
| F3 | Ghi outbox qua tham số `events` của phương thức repository | Một transaction; không `TxRunner` |
| F4 | `common/apperrors` chỉ thêm 2 Kind ở cuối | PQ-03 (8) |
| F5 | Không FK; `tenant_id` mọi bảng; `repo_id` cho tri thức repo (C4, dismissals) | §4.1, PQ-05 |
| F6 | Phân giải đích bằng `ListRepos`/`ListWorktrees` theo project, không `GetRepo`/`GetWorktree` theo id | IDOR (không lọc tenant) |
| F7 | Chuỗi RPC: guard → identity → cờ → OPA → selector → cổng agent → quyền trước cache → che → audit | §3 |
| F8 | Danh sách `internalcaller` sinh từ `ServiceDesc` + test | `Guard` bỏ qua method không liệt kê |
| F9 | Mọi lời gọi agent qua `GatedAgentRelay` (test AST) | Bảo vệ dev server dùng chung |
| F10 | Mã lỗi `CODEINTEL_X: msg \| {json}`; không `status.Details` | PQ-02 |
| F11 | Bảo trì: việc xuyên tenant hẹp, việc có mốc cấu hình chạy theo tenant; `RetentionTask` đăng ký được | Retention đổi bằng env |
| F12 | Số liệu hạn mức/TTL/kích thước là giả định chưa đo | O-15; SOL-071 thay bằng số đo |

## Điều còn mở (cần chủ sở hữu hợp đồng/CR)

- **Hợp đồng mâu thuẫn/thiếu:** (1) `IndexStatus.index_basis = 5` cần `codeintel_index_basis.proto` của CR-080 (đợt 7) nhưng `codeintel_binding.proto` ở đợt 2 → hoãn field 5; (2) §7.1 G0 "`codeintel.proto` rỗng-có-health" mơ hồ (chỉ HTTP health theo CR-010); (3) T0 `outbox_events` không có cột thứ tự `seq` (xem v6 SOL-001 D2); (4) cột `trigger` (T7) là từ dành riêng MySQL 8 theo hiểu biết, cần kiểm (SOL-011 task 01), có thể đề nghị đổi tên; (5) §3.1 không ghi rõ `GetIndexStatus` có `selector` (áp dụng quy tắc chung).
- **Phiên bản MySQL tối thiểu** (`CHECK` 8.0.16+) và TiDB chưa kiểm.
- **`ListMembers` quyền/quy mô** (013-01).
- **Chủ sở hữu bộ che/`pathsafety` dùng chung** với gateway (O-16).
- **Audit giàu chi tiết** (`metadata_json`): ngoài phạm vi series.
- **Trigger tự động và cooldown reindex** (chốt ở SOL-080).
