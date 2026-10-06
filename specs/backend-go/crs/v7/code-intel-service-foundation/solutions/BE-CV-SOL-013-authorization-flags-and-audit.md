# BE-CV-SOL-013-authorization-flags-and-audit: Bảo vệ RPC chỉ-gateway, cờ tính năng theo tenant, quyền OPA theo project, audit

> **📋 Proposed.** Chưa chạy build/test/`opa test` nào. Phần thứ nhất của CR-CV-013; hạn mức, cổng agent, che bí mật ở [`BE-CV-SOL-013-agent-call-gate-and-quotas`](./BE-CV-SOL-013-agent-call-gate-and-quotas.md).

**CR:** [CR-CV-013](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md) (mục 2.1–2.4, 2.7)
**Service:** `code-intel-service` (`internal/usecase`, `internal/adapter/{grpc,policyengine,grpcclient}`) · `backend-go/policy/orca-authz/code_intel.rego` (+ test)
**TDD tham chiếu:** [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (mục "AuthZ", "Multi-tenancy isolation", "Audit logging", "Service-to-service transport security"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "gRPC conventions"), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`services/project-service`](../../../../tdd/services/project-service.md), [`services/api-gateway`](../../../../tdd/services/api-gateway.md)

---

## Hợp đồng áp dụng

| Mục | Áp dụng |
|---|---|
| §3 (đầu mục) | Chuỗi 9 bước mọi RPC: (1) `internalcaller.Guard`/`StreamGuard` với `CODEINTEL_INTERNAL_CALLER_TOKEN` (rỗng = chặn hết), (2) tenant+user từ metadata, (3) cờ, (4) quyền OPA, (5) phân giải `selector`, (6) cổng agent, (7) quyền **trước** cache, (8) che bí mật, (9) audit |
| PQ-01 | Cờ `code_intel_enabled` hiệu lực tắt → **`CODEINTEL_DISABLED`** (`FailedPrecondition`); `CODEINTEL_FEATURE_DISABLED` bị bỏ; `QualityGateService` tắt → `CODEINTEL_QUALITY_GATE_DISABLED`; AI → `CODEINTEL_AI_REVIEW_DISABLED` |
| PQ-24 | Cache cờ **5 s**; ngoại lệ vẫn chạy khi tắt: `GetSettings`, `SetSettings` (admin), `GetReindexJob`; hiệu lực = công tắc env ∧ cờ tenant; lỗi đọc cờ = tắt |
| PQ-02 | `status.Message` bắt đầu `CODEINTEL_X: `; dữ liệu máy đọc là hậu tố `" \| {json}"` (≤ 2 KiB); không `status.Details` |
| PQ-03 | `CODEINTEL_NOT_AUTHORIZED` cho quyền dự án (không thành viên/project không tồn tại/tenant khác); `CODEINTEL_NOT_FOUND` cho id tài nguyên con; `CODEINTEL_AUTHZ_UNAVAILABLE` khi hạ tầng quyền lỗi |
| §6.3 | OPA `code_intel.rego`: `read`, `read_source`, `review_write`, `reindex`, `c4_write`, `quality_read`, `quality_waive`, `quality_profile_write`; chạy `GetProject` → `ListMembers` → OPA, cache 10 s; lỗi tra cứu = từ chối |
| §3.1/§3.2 | Cột "Action OPA" của từng RPC |
| §6.1 | Bảng cờ (cột `tenant_settings`), hiệu lực |
| §5 / H8 | Audit không chứa mã nguồn/bí mật; log không payload |
| UI-API U8, PQ-03 (7) | Từ chối phiên thiết bị (`DeviceID`) là việc **gateway** (service chỉ thấy bốn khoá metadata, không `DeviceID`) |

## Lệch giữa CR và hợp đồng

| # | CR-CV-013 nói | Hợp đồng | Xử lý (hợp đồng thắng) |
|---|---|---|---|
| L1 | `CODEINTEL_FEATURE_DISABLED` | PQ-01 | `CODEINTEL_DISABLED`; thêm hai mã cờ phụ cho Quality/AI |
| L2 | Cache cờ 30 s | PQ-24: 5 s | 5 s (`CODEINTEL_FLAG_CACHE_TTL`) |
| L3 | Chỉ một cờ `code_intel_enabled` | §6.1 nhiều cờ, hiệu lực env ∧ tenant | `FlagReader` trả `EffectiveFlags{CodeIntel, QualityGate, SecurityScan, AIReview}` |
| L4 | Năm action `read`, `read_source`, `review_write`, `reindex`, `c4_write` | §6.3 thêm `quality_read`, `quality_waive`, `quality_profile_write` | `code_intel.rego` có đủ 8; `quality_waive`: `member` tối đa 7 ngày, không miễn `check`/`error` là **luật nghiệp vụ** của SOL-085 (Rego chỉ cho phép action; ràng buộc 7 ngày nằm ở use case) |
| L5 | `CODEINTEL_NOT_AUTHORIZED` `PermissionDenied`; id con lạ → ? | PQ-03 (5): id con (job, run, waiver, turn) lạ = `CODEINTEL_NOT_FOUND` | Pipeline mapping tách hai loại |
| L6 | `retry_after_seconds=N` trong message | PQ-02 (3): hậu tố JSON | `" \| {"retryAfterSeconds":N}"` (SOL kia) |
| L7 | "không sửa `common/apperrors`" | PQ-03 (8): CR-010 thêm Kind | Dùng `KindResourceExhausted`/`KindUnavailable` |
| L8 | `reindex` cho `member` (Q4) | O-18: `member` được `reindex` với hạn mức | Giữ `member` được `reindex` |
| L9 | Audit qua `auditclient.Append` ("`Append` đồng bộ nuốt lỗi") | Hợp đồng §3.3 ghi `Append` thiếu `actor_type`/`target_type` | Giữ `Append` bọc async (không sửa `common/auditclient`); `metadata_json` hoãn (Q3 ở CR) |

## Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-010-scaffold-code-intel-service` | Trước: `buildServerOptions`, `KindResourceExhausted/Unavailable`, `FlagCacheTTL` |
| BE | `BE-CV-SOL-011-repositories-and-maintenance` | Trước: `TenantSettingsRepository` |
| BE | `BE-CV-SOL-012-target-resolution-and-bindings` | Trước: client `ProjectDirectory` (`GetProject`, `ListMembers`); `ResolveTarget` chạy **sau** quyền |
| BE | `BE-CV-SOL-013-agent-call-gate-and-quotas` | Cùng CR; pipeline gọi `L1`; `AuditRecorder` dùng ở đó |
| BE | `BE-CV-SOL-073-settings-flag-and-rollout` | Sau: `GetSettings/SetSettings` dùng `FlagReader`/`TenantSettingsRepository`; audit đổi cờ |
| BE | `BE-CV-SOL-085-*`, `-082`, `-089…093` | Dùng pipeline cho `QualityGateService` (`CODEINTEL_QUALITY_GATE_DISABLED`) |
| BE | `BE-CV-SOL-040-codeintel-channel-foundation` | Gateway gắn `x-orca-internal-token` bằng `ClientInterceptor`/`StreamClientInterceptor`, bóc mã `CODEINTEL_*`, từ chối phiên thiết bị |
| BE | `BE-CV-SOL-072-security-tests-service-gateway` | Test chống IDOR/cô lập |
| AG / FE | — | Không có |

---

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: `common/internalcaller/internalcaller.go` (`Guard`/`StreamGuard` chỉ chặn các `FullMethod` **được liệt kê**; token rỗng chặn mọi lời gọi **đã liệt kê**; `MetadataKey = "x-orca-internal-token"`; `ClientInterceptor`/`StreamClientInterceptor`), `mcp-service/cmd/server/main.go` (dòng 222–227: chỉ thêm `Guard` khi token khác rỗng, log WARN nếu rỗng), `common/policy/evaluator.go` (`NewEvaluator`, `Decision`, `Value`, `Warm`), `task-service/cmd/server/main.go` (dòng 293–298 `Warm` và thoát khi lỗi nạp), `policy/orca-authz/project.rego` (mẫu: `action_roles`, `default allow := false`, admin toàn cục luôn qua; `input.{caller_project_role, caller_global_role, action}`), danh sách `policy/orca-authz/*.rego` (không có `code_intel.rego`), `project-service/internal/usecase/{authorization.go,get_project.go}`, `proto/orca/project/v1/project.proto` (`Member{user_id, role}`; `ListMembersRequest{project_id}`), `common/auditclient/client.go` (`Append(ctx, tenantID, actorID, action, target, outcome, ip)` nuốt lỗi), `common/tenant/tenant.go` (`Role`, `ClientIP`, `RequireTenantID`), `common/grpcmw/grpcmw.go`.

### Correction relative to CR-CV-013

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | "`Guard` rỗng token từ chối mọi RPC" | Đúng **chỉ cho method trong danh sách**; method không liệt kê **đi qua** (`internalcaller.go`: `if !guarded[info.FullMethod] { return handler }`). Ở `mcp-service` còn bỏ hẳn interceptor khi token rỗng | Danh sách **bắt buộc sinh từ `ServiceDesc`** (unary + stream của `CodeIntelService` **và** `QualityGateService`); luôn thêm interceptor kể cả token rỗng (để mọi RPC bị chặn); test phản chiếu chứng minh không method nào lọt (task 013-03) |
| C2 | Chuỗi kiểm tra đặt rate L1 (bước 2) trước cờ | Hợp đồng §3: guard → identity → cờ → OPA → … (không nêu L1) | Giữ L1 sau identity, trước cờ (rẻ, bộ nhớ); ghi trong SOL kia |
| C3 | `GetProject` thành công ⇒ tenant khớp + thành viên | Đã xác nhận (`get_project.go`: `requireProjectAccess` rồi `repo.Get(ctx, tenantID, id)`) | Dùng làm cổng; admin toàn cục tenant khác bị `repo.Get` từ chối |
| C4 | `ListMembers` lấy role | `Member{user_id, role}` có; chưa đọc `ListMembers` usecase | Chưa kiểm chứng việc `ListMembers` yêu cầu quyền gì; nếu cần owner thì member không gọi được → task 013-01 kiểm |
| C5 | `tenant.Role` fail closed | Có `Role(ctx) (string, bool)`; `callerGlobalRole` dùng `""` khi vắng | Cùng quy ước |
| C6 | `common/auditclient` nuốt lỗi, `outcome` sai bị nuốt | Đúng; `Append` chỉ 6 trường | Async wrapper + `Outcome` kiểu hằng |
| C7 | `OPA_BUNDLE_PATH=/policy/orca-authz` | `task-service` dùng `cfg.OPABundlePath` + `Warm(ctx, "data.orca.authz.task.allow")` | Cùng cách: `Warm(ctx, "data.orca.authz.codeintel.allow")`, lỗi nạp → không khởi động |

Chưa kiểm chứng: `ListMembers` chính sách quyền; kích thước thành viên; `x-orca-role` có đủ ở mọi đường gateway (CR nêu chỉ cookie/session + JWT mới); giới hạn độ dài `target` của `audit_log`.

## 2. Giải pháp

### A. Rego `backend-go/policy/orca-authz/code_intel.rego` (mới)

```rego
package orca.authz.codeintel
import rego.v1
# input: {caller_project_role, caller_global_role, action}
action_roles := {
  "read": {"owner","member"}, "read_source": {"owner","member"},
  "review_write": {"owner","member"}, "reindex": {"owner","member"},
  "c4_write": {"owner"}, "quality_read": {"owner","member"},
  "quality_waive": {"owner","member"}, "quality_profile_write": {"owner"},
}
default allow := false
allow if input.caller_global_role == "admin"
allow if { roles := action_roles[input.action]; input.caller_project_role in roles }
```

Test `code_intel_test.rego`: bảng vai trò × 8 action, admin toàn cục, vai trò rỗng, action lạ → false. `make opa-test` (cần CLI `opa`; chưa kiểm chứng có sẵn trên CI).

### B. Cổng quyền `ProjectAuthorizer` (`internal/usecase/project_authorizer.go`, mới)

`Require(ctx, projectID, action) error`: (1) `ProjectDirectory.GetProject` (cổng tenant+thành viên; lỗi `NotFound`/`PermissionDenied` → `CODEINTEL_NOT_AUTHORIZED`, không lộ khác biệt); (2) `ListMembers` tìm `role` của `tenant.UserID` (admin không có dòng: `""`); (3) OPA `data.orca.authz.codeintel.allow` với `{caller_project_role, caller_global_role (tenant.Role), action}`; (4) cache quyết định `(tenant, user, project, action)` TTL `CODEINTEL_AUTHZ_CACHE_TTL` = 10 s (đồng hồ tiêm). Lỗi hạ tầng (project-service/OPA) → `CODEINTEL_AUTHZ_UNAVAILABLE` (`KindUnavailable`), **không** cho qua. Cache chỉ lưu allow/deny rõ ràng, không lưu lỗi. Mọi từ chối OPA sinh audit `codeintel.access` (denied).

Bảng `action` theo RPC lấy **nguyên văn** từ `CONTRACT-codeintel-proto-and-data-map.md` §3.1/§3.2 (cột "Action OPA"); đặt trong `internal/adapter/grpc/rpc_policy_table.go` (mới): `map[fullMethod]rpcPolicy{Action, FlagScope, ProjectFrom, AllowWhenDisabled, AuditAction}`; test kiểm mọi method của hai `ServiceDesc` có dòng (không thiếu, không thừa).

### C. Cờ tính năng (`internal/usecase/flag_reader.go`, mới)

```go
type EffectiveFlags struct{ CodeIntel, QualityGate, SecurityScan, AIReview bool }
type FlagReader interface{ Effective(ctx context.Context) (EffectiveFlags, error) } // lỗi đọc → trả all-false + lỗi (đóng)
```

`EffectiveFlags.CodeIntel = env.Enabled ∧ tenant.code_intel_enabled`; `QualityGate = … ∧ env.QualityGate ∧ tenant.quality_gate_enabled`; `SecurityScan = QualityGate ∧ tenant.quality_security_scan_enabled`; `AIReview = CodeIntel ∧ env.AIReview ∧ tenant.ai_review_level ≠ off`. Dòng thiếu → `GetOrCreate` với giá trị `TenantDefault*` (config). Cache 5 s theo tenant (đồng hồ tiêm); lỗi đọc → tắt (fail closed). Cờ tắt khi job reindex đang chạy: job chạy tới hết, RPC mới bị từ chối, `codeintel.indexChanged` vẫn xử lý (huỷ cache), dữ liệu giữ nguyên.

### D. Pipeline (`internal/adapter/grpc/policy_interceptor.go`, mới)

Interceptor unary + stream đặt trong `buildServerOptions` (SOL-010 task 07) sau `internalcaller` và `tenant extraction`:

1. `internalcaller.Guard/StreamGuard(token, <mọi FullMethod từ ServiceDesc>)` **luôn** được thêm (kể cả token rỗng → mọi RPC bị chặn, log WARN).
2. `tenant.RequireTenantID` + `tenant.UserID` → `CODEINTEL_NO_TENANT` (`Unauthenticated`).
3. `L1` giới hạn thô `(tenant,user)` (SOL kia).
4. Cờ theo `rpcPolicy.FlagScope`: tắt → `CODEINTEL_DISABLED` (hoặc `CODEINTEL_QUALITY_GATE_DISABLED`/`CODEINTEL_AI_REVIEW_DISABLED`), trừ `AllowWhenDisabled` (`GetSettings`, `SetSettings`, `GetReindexJob`).
5. `ProjectAuthorizer.Require(projectID, action)`; `projectID` từ `selector.project_id` hoặc `project_id` (mỗi request proto cung cấp `GetProjectId()` qua interface `projectScoped`; RPC thiếu → lỗi lập trình bị bắt bởi test bảng).
6. Gọi handler; handler tự `ResolveTarget`, cổng agent, quyền **trước** tra cache (đã đảm bảo vì bước 5 chạy trước handler).
7. Audit bất đồng bộ theo bảng mục F.
8. `toStatus(err)` (`status_mapping.go`): `apperrors.ToGRPCStatus` + hậu tố `" | {json}"` khi lỗi mang `Data` (kiểu `domain.CodedData`, ≤ 2 KiB, hợp lệ hoá bằng bỏ phần tử); mã `NOT_AUTHORIZED` `PermissionDenied`; `DISABLED`/`QUALITY_GATE_DISABLED`/`AI_REVIEW_DISABLED` `FailedPrecondition`; `AUTHZ_UNAVAILABLE` `Unavailable`; `NOT_FOUND` `NotFound`; hạn mức `ResourceExhausted` (SOL kia).

### E. `internal/adapter/policyengine/opa.go` (mới)

Bọc `policy.Evaluator`: `Allowed(ctx, input) (bool, error)`; `NewEvaluator(cfg.OPABundlePath)` + `Warm` lúc khởi động (lỗi → thoát).

### F. Audit (`internal/usecase/audit_recorder.go`, `internal/adapter/grpcclient/audit_recorder.go`, mới)

```go
type AuditOutcome string
const (AuditAllowed AuditOutcome = "allowed"; AuditDenied AuditOutcome = "denied")
type AuditEntry struct{ Action, Target string; Outcome AuditOutcome }  // Target ≤ 200 ký tự, dạng loại:id
type AuditRecorder interface{ Record(ctx context.Context, e AuditEntry) }
```

Hàng đợi có giới hạn (256 mục, 2 worker, hạn chót 2 giây mỗi lời gọi `auditclient.Client.Append`), đầy thì bỏ và tăng `codeintel_audit_dropped_total` (nối SOL-071; tạm bộ đếm nội bộ), xả tối đa 5 giây khi tắt. `actor_id = tenant.UserID`, `ip = tenant.ClientIP`. Bảng hành động: `codeintel.access` (denied, `project:<id>`), `codeintel.bind`, `codeintel.symbol.read`, `codeintel.reindex.request`, `codeintel.reindex.limit` (denied), `codeintel.review.save`, `codeintel.finding.dismiss|restore`, `codeintel.c4.save`; các CR sau thêm (`quality.waive`, `settings.set`…) theo `AuditAction` trong `rpc_policy_table`. Không audit đọc thường; chi tiết thêm vào log `slog` (`audit=true`, `trace_id`), không vào `audit_log`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Hỏi `project-service` mỗi lần (cache 10 s) | Gỡ thành viên có hiệu lực nhanh; một nguồn sự thật |
| D2 | Danh sách guard sinh từ `ServiceDesc` + test | `Guard` bỏ qua method không liệt kê (C1) |
| D3 | Bảng chính sách theo RPC tập trung | Thêm RPC mới thiếu dòng làm test đỏ |
| D4 | Cờ kiểm trước quyền | Tenant chưa bật không gây tải lên project-service |
| D5 | Audit async + hằng `Outcome` | `Append` đồng bộ nuốt lỗi; `outcome` sai bị nuốt |
| D6 | Fail closed ở mọi lỗi tra cứu | Dữ liệu là mã nguồn |
| D7 | Không sửa `common/auditclient`, `auth-service` | Ngoài phạm vi; audit giàu chi tiết là Q3 |

## 4. Tiêu chí chấp nhận

- [ ] Mọi method của `CodeIntelService_ServiceDesc` và `QualityGateService_ServiceDesc` (kể cả stream) bị `internalcaller` bảo vệ; thêm RPC thiếu dòng chính sách/guard làm test đỏ; token rỗng → mọi RPC `INTERNAL_CALLER_REQUIRED`.
- [ ] Không phải thành viên / project của tenant khác / project không tồn tại: cùng `CODEINTEL_NOT_AUTHORIZED`, **không** có `RelayByDevServer` nào (fake đếm).
- [ ] Bảng role × action đúng (test Rego và test Go dùng bó Rego thật); `member` không `c4_write`/`quality_profile_write`; admin toàn cục tenant khác vẫn bị `GetProject` từ chối.
- [ ] `project-service`/OPA lỗi → `CODEINTEL_AUTHZ_UNAVAILABLE`, không cho qua; bó Rego thiếu → service không khởi động.
- [ ] Cờ tắt hoặc lỗi đọc cờ → `CODEINTEL_DISABLED` **trước** mọi lời gọi `project-service`; `GetSettings`/`SetSettings`/`GetReindexJob` vẫn chạy khi tắt; quality tắt → `CODEINTEL_QUALITY_GATE_DISABLED`.
- [ ] Gỡ thành viên: sau TTL 10 s bị từ chối (đồng hồ giả); cờ đổi hiệu lực ≤ 5 s.
- [ ] Audit: mỗi dòng bảng một test; `Outcome` ngoài hai hằng không dựng được; hàng đợi đầy tăng bộ đếm, không chặn RPC; `auth-service` giả chậm 5 s không làm chậm RPC.
- [ ] Cô lập tenant: ctx tenant A + id tenant B không đọc được gì (cache/DB/trạng thái).
- [ ] `go vet`, `make lint`, `make opa-test` xanh; không `max-lines` disable.

## 5. Kiểm thử

- **Rego:** `code_intel_test.rego` (role × action, admin, rỗng, action lạ).
- **Unit:** pipeline với giả (project, OPA, cổng, audit, flag): thứ tự bước, bỏ qua khi lỗi; cache TTL; ánh xạ `toStatus` (hậu tố JSON hợp lệ hoá ≤ 2 KiB); bảng chính sách đủ method.
- **An toàn:** quét `ServiceDesc` (guard phủ mọi method); "RPC không có tenant"; chống IDOR.
- **Integration:** `FlagReader` + `tenant_settings` hai dialect.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- `ListMembers` quy mô/quyền chưa kiểm; có thể cần RPC "vai trò của tôi" ở project-service (ngoài phạm vi).
- `x-orca-role` vắng ở đường không cookie/JWT mới → admin toàn cục không qua (fail closed).
- Danh tính là metadata tin cậy theo mạng; token nội bộ chỉ là lớp nông.
- Audit có thể mất khi `auth-service` lỗi; độ dài `target` chưa kiểm chứng.
- Chưa có sự kiện thành viên để huỷ cache sớm (≤ 10 s).
- `opa` CLI cho `make opa-test` chưa kiểm chứng trên CI.
- `mcp.rego` từ chối mặc định rủi ro không rõ nên mở MCP sau (CR-041) không mở ngầm (chưa kiểm).

## 7. Câu hỏi mở

- **Q1.** Siết `read_source`/`reindex` theo vai trò chức năng trên repo (`repo_members`)? Mặc định: không (O-18).
- **Q2.** Member không có quyền nhóm dev server có được xem code-intel? Mặc định: có (ngang `files.*`).
- **Q3.** Audit bền/giàu chi tiết (`AppendDetailed` hoặc outbox + consumer ở `auth-service`): ngoài phạm vi series.
- **Q4.** Chủ sở hữu bộ che/`pathsafety` dùng chung với gateway (O-16): xem solution kia.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` PQ-01–03, PQ-24; §3, §3.1, §3.2, §6.1, §6.3
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` §1 U8, §2.3, §2.5
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md`
- `/opt/repos/orca/backend-go/common/{internalcaller,policy,auditclient,tenant,grpcmw}`, `backend-go/policy/orca-authz/project.rego`, `backend-go/services/{mcp-service,task-service,project-service}/…`
