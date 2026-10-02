# backend-go Solutions — MCP Governance & Safety (v5)

**CRs:** [docs/crs/v5/mcp-governance-safety](../../../../../../docs/crs/v5/mcp-governance-safety/README.md)
**Hợp đồng bắt buộc:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Quy ước chung & bảng T1..T8:** [crs/v5/README.md](../../README.md)
**TDD tham chiếu:** `arch/05` (outbox), `arch/07` (Authz/Audit), `arch/08` (events), `services/api-gateway.md`, `services/auth-service.md`, `services/notification-service.md`
**Phía frontend:** [specs/frontend/crs/v5/mcp-governance-safety/solutions](../../../../../frontend/crs/v5/mcp-governance-safety/solutions/README.md)

## Solutions

| Solution | CR | Service / Area | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-012](./BE-MCP-SOL-012-tool-policy-and-annotations.md) | CR-MCP-012 | `mcp-service`, `api-gateway` (`wscompat`, `mcpserver`), `policy/orca-authz`, `common/policy` | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [BE-MCP-SOL-013](./BE-MCP-SOL-013-approvals-audit-killswitch.md) | CR-MCP-013 | `mcp-service`, `api-gateway`, `auth-service`, `notification-service`, `common/jwtauth` | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |

## Re-verify: khẳng định của CR vs mã thật (khảo sát 2026-10-01)

| Khẳng định (CR / README v5) | Thực tế trong code | Lệch? | Hệ quả thiết kế |
|---|---|---|---|
| Mở rộng bundle `policy/orca-authz`; `ci/check-opa-bundle-in-images.sh` xanh | Bundle có 6 package + test; script chỉ lặp 4 service (`auth, task, annotation, project`), mỗi Dockerfile copy cả thư mục `policy` | Cần thêm `mcp-service` vào script (đếm động) | SOL-012 §I |
| "Publish qua `policypublisher` hiện có" | `FilePublisher` ghi **một file chung không phân tenant** (`data/<kind>/<name>.json`) vào bundle dùng chung ⇒ không chứa được policy theo tenant | **Lệch ý đồ** | Policy tenant đi qua `input` (DB + RLS), Rego tĩnh — SOL-012 QĐ1 |
| `common/policy` đủ để gọi | `Evaluator.Decision` chỉ trả `bool` | Thiếu | Thêm `Evaluator.Value` (additive) |
| OPA lỗi ⇒ fail-closed | Mọi call site Epic E hiện đã fail-closed khi `err != nil` | Khớp | Giữ; thêm lớp deny khi `Value` trả sai kiểu/undefined |
| README v5: `Identity.Role` chỉ ở nhánh cookie | `usecase/validate_identity.go`: nay cả nhánh Bearer JWT cũng điền `Role` nếu token có claim (CR-RBAC-002) | Cũ nhẹ | Fail-closed vẫn đúng; `mcp.admin.*` không phụ thuộc điều này vì chỉ cookie tới được |
| Kênh admin trả `INFRA_NOT_ADMIN`/`errNotAdmin` | `errors.New("caller is not an admin")` — **không có prefix `CODE:`** | Khác | `MCP_NOT_ADMIN:` riêng cho `mcp.admin.*` |
| FE tách lỗi theo `^([A-Z0-9_]+): ` (C4) | `writeDialectError` (`session_dialect.go:94`) gửi `err.Error()`; lỗi gRPC có dạng `rpc error: code = … desc = CODE: msg` ⇒ **không khớp regex** | **Lệch (lỗi tích hợp)** | Bộ chuyển `mcpWSError` dùng `status.Convert(err).Message()` (SOL-012 §G); mọi `channels_mcp_*.go` bắt buộc dùng |
| Tham số kênh trong CONTRACT (vd. `approvalId`, `{decision,…}`) | Mẫu hiện có: **một** object ở `args[0]` (`admin.deletePolicy`: `{policyId}`); FE `callRuntimeResult(method, params)` chỉ truyền một `params` | Mơ hồ trong CONTRACT | R-2 |
| `AppendAuditEntry` thêm `actor_type=agent` | Request không có `actor_type`/metadata; `audit_log` chưa có cột `actor_type`; `auditclient.Append` nuốt lỗi | Cần additive | Đường outbox → consumer durable (khuôn `audit_ingest.go`) + proto/migration additive — SOL-013 §E |
| `QueryAuditLog` phân trang | `ORDER BY id` (UUID v4 ngẫu nhiên), token = id cuối ⇒ **không theo thời gian** | **Drift ảnh hưởng UI** | Keyset `(occurred_at,id)` + `order` additive — SOL-013 §E (điều kiện của FE-MCP-SOL-010) |
| auth-service chỉ Postgres | Có thêm `adapter/mysql` + `migrations/mysql` | Ghi nhận | Migration/Query làm ở cả hai |
| T5: "`SubscribeEphemeral` là core-NATS" | `common/eventbus.SubscribeEphemeral` là **consumer JetStream ephemeral** mỗi replica (không phải core-NATS) | **Sai trong bảng T5** | Dùng nguyên cho approval/kill switch; core-NATS chỉ cho điều khiển phiên (BE-004) |
| Rate limit phân tán | Không có Redis trong `go.mod`/compose/helm; limiter gateway là in-memory per-tenant | Khác kỳ vọng T6 | Postgres sliding-window ở `mcp-service` (SOL-013 §G) |
| `mcp_depth` trong token | `jwtauth.Claims` chỉ có `TenantID, DeviceID, Role` | Chưa có | Thêm `McpDepth`, `McpRoot` (additive) |
| notification-service nhận sự kiện qua luật theo subject | Đúng (`Subjects` + `subjectRules`); type hiện snake_case, CONTRACT §4 dùng `mcp.approval` | Nhẹ | 1 binding + 1 luật; giữ `mcp.approval` theo CONTRACT |
| Tồn tại `mcp-service`, `mcp.rego`, bảng, kênh `mcp.*` | Không có gì (`ls backend-go/services`) | Đúng như CR | Toàn bộ là NEW |

Rego của SOL-012 đã được **chạy thật trong scratchpad** (không đưa vào repo): `opa test` 44/44; gộp với bundle hiện có 98/98; nạp qua `rego.New(...).PrepareForEval` (cùng đường `common/policy`) cho kết quả `map[string]any`.

## Thứ tự thực thi & phụ thuộc

```
BE-001 (khung mcp-service, outbox, proto) ─┬─▶ SOL-012 ──▶ SOL-013 ──▶ (cho phép bật tool ghi: CR-008)
BE-006 (scopes, Role, aud)  ───────────────┤      ▲            ▲
BE-007 (descriptor: risk/openWorld/…)  ────┘      │            ├─ BE-004 (sessions/SSE/cancel/ephemeral T5)
                                                  │            ├─ BE-005 (client/grant, thu hồi refresh token)
                                                  │            └─ BE-008/009 (Redactor, hủy tool dài, readUntrusted)
auth-service: migration 0011 + proto additive + consumer ──▶ trước khi SOL-013 phát `orca.mcp.audit.appended`
notification-service: 1 binding + 1 luật ───────────────────▶ trước khi bật push duyệt
notification-service: CR-NOTIF-002 (`DeliverPush`) ─────────▶ điều kiện để push thật sự được gửi (D5); chưa có thì chỉ có hộp thoại trong app
```
Trong SOL-013 nên đi: (1) auth-service additive (độc lập, chạy được ngay) → (2) journal `tool_calls` + audit → (3) approval + events → (4) kill switch → (5) limiter/depth/taint/wrapper → (6) red-team. SOL-012 chạy được độc lập sau BE-001/006/007 với `killswitch/session` mặc định "sạch" (chỉ dùng trong test, **không** bật tool ghi trước khi 013 xong).

## Yêu cầu thay đổi CONTRACT (R-1..R-6 đã được CONTRACT hiện hành tiếp nhận; giữ lại để truy vết)

| # | Đề xuất | Lý do |
|---|---|---|
| R-1 | Thêm mã lỗi: `MCP_INVALID_ARGUMENT` (validate chung: policy sai biên, `killSwitch` trong patch settings, filter audit sai), `MCP_INTERNAL`, `MCP_TIMEOUT` | BE cần mã cho lỗi validate/hạ tầng; FE cần giá trị cố định thay vì chuỗi `rpc error…`; FE đã xử lý mã lạ theo kiểu chung |
| R-2 | §2 mở đầu ghi rõ: *tham số của mọi kênh là MỘT object JSON ở `args[0]`; dấu phẩy trong cột "Tham số" liệt kê field của object đó* (vd. `mcp.approval.decide` = `{approvalId, decision, paramsHash, note?}`; `mcp.admin.policy.delete` = `{policyId}`) | Cột hiện mơ hồ (`approvalId, {…}`); khớp mẫu `admin.deletePolicy` và `callRuntimeResult(method, params)` |
| R-3 | (Tuỳ chọn, additive) `mcp.admin.killswitch.set` thêm `revokeTokens?: boolean` (mặc định false) để thu hồi cả PAT | SOL-013 QĐ2: mặc định chỉ chặn PAT, admin cần lựa chọn rõ cho sự cố rò rỉ |
| R-4 | (Tuỳ chọn, additive) `McpApproval.reasons?: string[]` (vd. `open_world_after_untrusted_read`) | Người duyệt nên biết *vì sao* cần duyệt (taint) |
| R-5 | *(đã vào CONTRACT; D6)* `McpServerInfo.enabled` = chỉ cờ môi trường `MCP_ENABLED`; `tenantEnabled?: boolean` = cài đặt tenant, tenant mới mặc định theo `MCP_TENANT_DEFAULT_ENABLED` (mặc định `true`) | Nếu `enabled` trộn cờ tenant, FE ẩn cả tab Settings và admin không bật lại được (CONTRACT §6) |
| R-6 | (Khuyến nghị, additive) `mcp.admin.killswitch.list` → `Array<{scope, targetId?, active, reason, at, setBy}>` | CONTRACT chỉ cho FE đọc kill switch cấp tenant (`serverInfo.killSwitch`); công tắc client/grant/session đã bật thì UI không thấy để gỡ (FE-MCP-SOL-008) |
