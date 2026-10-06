# Code Intel Service Foundation — Change Requests (v7)

> Dựng nền móng backend: service `code-intel-service`, mô hình dữ liệu, ánh xạ project/worktree → dev server → repo, và phân quyền, audit, hạn mức. Xem bối cảnh, quyết định D1 đến D7, mặc định O1 đến O8 và hợp đồng chung ở [README v7](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-CV-010](./CR-CV-010-scaffold-code-intel-service.md) | Chưa có service nào chuẩn hoá, cache, phân quyền dữ liệu code-intel; chưa có proto `orca.codeintel.v1` | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-011](./CR-CV-011-code-intel-data-model-and-repositories.md) | Chưa có bảng, entity, repository cho binding, snapshot, trạng thái review, ghi đè C4, job reindex, cờ tenant | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-012](./CR-CV-012-project-worktree-to-repo-binding.md) | Chưa có quy tắc từ `(project, worktree)` tới dev server, đường dẫn và repo index; chưa có trạng thái index tổng hợp | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-013](./CR-CV-013-authorization-audit-and-quotas.md) | Đọc mã trên dev server hiện chỉ giới hạn theo tenant; chưa có quyền theo project, audit, hạn mức, che bí mật | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-CV-010 (khung service, proto rỗng-có-chủ, outbox, wiring, CI)
    └──▶ CR-CV-011 (migration 0002, domain, repository hai dialect)
              └──▶ CR-CV-012 (ResolveTarget, BindRepo/ListRepoBindings/GetIndexStatus, consumer worktree.deleted)
                        └──▶ CR-CV-013 (OPA, cờ tenant, internalcaller, audit, AgentCallGate, che bí mật)
```

CR-CV-010 phải merge trước CR-CV-011 vì CR-CV-011 thêm migration và adapter vào module do CR-CV-010 tạo. CR-CV-012 cần schema của CR-CV-011. CR-CV-013 cần CR-CV-012 (dùng `ResolveTarget`, `GetProject`). Có thể làm song song phần Rego của CR-CV-013 ngay từ đầu. CR-CV-012 làm trước được với agent giả cho `codeintel.status`. Cả bốn là điều kiện tiên quyết của backend v7 (collector CR-CV-021, gateway CR-CV-040).

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Hai dialect (Postgres, MySQL) ngay từ đầu, chọn bằng `dbcapability.DetectDialectFromDSN` | O1; mẫu `notification-service` (`cmd/server/main.go` dòng 91 đến 159) |
| F2 | Layout theo `notification-service`; outbox ghi cùng transaction và RLS theo `mcp-service` | `task-service` có relay nhưng RLS vô hiệu (không bao giờ `set_config`) và `TxRunner` khác mẫu; `mcp-service` đúng hai điểm đó nhưng chỉ Postgres |
| F3 | Bảng nghiệp vụ ở CR-CV-011; CR-CV-010 chỉ tạo `outbox_events`, `processed_events` | Tách khung hạ tầng khỏi mô hình nghiệp vụ |
| F4 | Không FK chéo bảng hay chéo service; `project_id`, `repo_id`, `worktree_id`, `dev_server_id`, `tenant_id` là id tham chiếu; dọn bằng ứng dụng | README v7 mục 6 |
| F5 | Cô lập tenant: Postgres `set_config('app.tenant_id')` + `FORCE ROW LEVEL SECURITY`; MySQL lọc `tenant_id` ở mọi `WHERE`; test AST bắt phương thức quên phạm vi | `dbcapability` ghi `SupportsRLS=false` cho MySQL; compose dev dùng superuser nên RLS không chạy ở dev |
| F6 | Đồng hồ DB cho so sánh hết hạn | Mẫu F6 của v6 |
| F7 | `c4_overrides` và `finding_dismissals` khoá theo `repo_id`, không theo binding | Binding sống theo worktree; ghi đè và bỏ qua là tri thức của repo |
| F8 | Tên repo, đường dẫn, dev server không bao giờ đến từ client; lấy từ `project-service` bằng `ListRepos`/`ListWorktrees` có kiểm quyền | O4; `GetRepo`/`GetWorktree` theo id không lọc tenant |
| F9 | Mọi RPC qua `internalcaller.Guard`, kiểm cờ tenant, rồi OPA theo project; quyền luôn kiểm trước cache | Danh tính là metadata tin cậy theo mạng |
| F10 | Mọi lời gọi tới agent đi qua `AgentCallGate` | Bảo vệ dev server dùng chung với agent lập trình |

## Phạm vi ngoài feature này

Collector và điều phối truy vấn agent (CR-CV-021), cache snapshot (CR-CV-022), sự kiện (CR-CV-023, 024), chuẩn hoá đồ thị (CR-CV-020), kênh gateway (CR-CV-040), cờ rollout và RPC bật/tắt (CR-CV-073), đo hạn mức (CR-CV-071). Bốn CR ở đây chỉ dựng chỗ chứa, quy tắc phân giải, quyền và cổng hạn mức.

## Điểm lệch giữa README v7 (và v6) với code, đã phát hiện khi viết feature này

Chưa sửa README v7; danh sách để cập nhật khi duyệt (cũng ghi ở mục "Câu hỏi mở" của từng CR).

1. README v7 mục 3.5: bảng dữ liệu thiếu `tenant_settings` (cờ O8); `repo_bindings` cần thêm `repo_id`, `scope_key`, `path_hash`, `index_scope`, `last_status`, `last_status_at`, `version`; `graph_snapshots` thêm `schema_version`; `review_states` dùng `repo_binding_id` thay `worktree_id` (id worktree có ba dạng chuỗi, không chỉ UUID); `finding_dismissals`, `c4_overrides` khoá theo `repo_id`; `reindex_jobs` thêm `percent`, `requested_by`, `active_key`, `message`, `created_at`, `updated_at`, `version`; `processed_events` khoá `(tenant_id, event_id)`.
2. README v7 mục 1 (dòng "Handshake mang danh sách tool"): `HandshakeInfo` của Go **không giữ `tools[]`** (`devserveragent/session.go` dòng 47 đến 58); trạng thái công cụ phải lấy từ `codeintel.status`.
3. README v7 mục 3.2: câu "workspaceRoot đã nằm trong workspace root đăng ký": agent **không có** cơ chế đăng ký root (`RelayContext.registerRoot` no-op); "đăng ký" nghĩa là có trong `project-service`, và CR-CV-001 phải tự kiểm đường dẫn ở agent. Thêm yêu cầu `codeintel.status` không trả lỗi khi thiếu công cụ/chỉ mục/repo chưa đăng ký, mà trả `data.tools[]` (CR-CV-012 mục 2.5).
4. README v7 mục 3.3: mã lỗi agent do `error.data.code` đặt bị `RelayByDevServer` làm mất (bọc `INFRA_AGENT_EXEC_FAILED`, `ToGRPCStatus` bỏ nguyên nhân); cần CR-CV-023 chuyển mã. Thêm các mã phía Go: `CODEINTEL_NOT_AUTHORIZED`, `CODEINTEL_FEATURE_DISABLED`, `CODEINTEL_RATE_LIMITED`, `CODEINTEL_CONCURRENCY_LIMIT`, `CODEINTEL_REINDEX_COOLDOWN`, `CODEINTEL_AUTHZ_UNAVAILABLE`, `CODEINTEL_NO_TENANT`, `CODEINTEL_NOT_FOUND`, `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_ALREADY_EXISTS`, `CODEINTEL_PAYLOAD_TOO_LARGE`, `CODEINTEL_WORKTREE_NOT_FOUND`, `CODEINTEL_WORKTREE_REF_UNSUPPORTED`, `CODEINTEL_NO_DEV_SERVER`, `CODEINTEL_DEV_SERVER_NOT_APPROVED`, `CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED`, `CODEINTEL_DEV_SERVER_OFFLINE`.
5. README v7 mục 3.6/O4: `worktree_id` do UI/gateway gửi nên gọi là `worktree_ref` (UUID, `<repoId>::<path>`, repo trần); `BindRepo`/`GetIndexStatus` chỉ nhận `project_id` và `worktree_ref`.
6. README v7 mục 7: "backend xếp hàng ~20 giây" chưa khớp `RelayByDevServer`, vốn trả `INFRA_DEV_SERVER_NOT_CONNECTED` ngay khi không có phiên.
7. README v6 (và `CR-REQ-001`): đường dẫn đúng là `guides/STYLEGUIDE.md`, `guides/reference/git-compatibility.md` (có trong repo, kiểm 2026-10-05); bảng outbox `outbox_events`, bảng dedup `processed_events`, subject `orca.<service>.<entity>.<event>` vẫn đúng.
8. Điểm lệch của `CR-REQ-001` mà CR-CV-010 không sao lại: RLS "như `task-service`" vô hiệu; `TxRunner` của `task-service` không phải `InTx(ctx, fn(ctx))`; không service nào đăng ký gRPC health; `make proto-lint` không chặn (`|| true`); thiếu hai nơi wiring (`deploy/dev/docker/postgres/init-databases.sh`, `build-local.sh`); chưa có interceptor stream lấy tenant.
