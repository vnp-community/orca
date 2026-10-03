# BE-MCP-SOL-012: Cổng chính sách tool — OPA `orca.authz.mcp`, `tool_policies` có version, hard-deny, lọc `tools/list` + enforce lúc gọi

> **✅ Implemented (unit/integration tests) — see Gaps.** Điều kiện cứng trước khi bật bất kỳ tool ghi/exec/phá huỷ nào (cùng BE-MCP-SOL-013). Rego trong tài liệu này đã được **chạy thử trong scratchpad** (không nằm trong repo): `opa test` 44/44 PASS, 98/98 khi gộp với bundle `policy/orca-authz` hiện có; đã nạp và eval thành công qua `github.com/open-policy-agent/opa/rego` (cùng đường `common/policy.Evaluator` dùng), kết quả trả về là `map[string]any`.

**CR:** [CR-MCP-012](../../../../../../docs/crs/v5/mcp-governance-safety/CR-MCP-012-tool-policy-and-annotations.md)
**Service:** `mcp-service` (quyết định + dữ liệu), `api-gateway` (`adapter/mcpserver` enforce, `adapter/wscompat/channels_mcp_policy.go`), `backend-go/policy/orca-authz` (Rego), `backend-go/common/policy` (additive)
**Hợp đồng:** [CONTRACT §2.2](../../CONTRACT-mcp-ui-api.md) — `mcp.admin.settings.get/set`, `mcp.admin.policy.list/upsert/delete/explain`; lỗi `MCP_POLICY_HARD_DENY`, `MCP_POLICY_VERSION_CONFLICT`, `MCP_NOT_ADMIN`, `MCP_DISABLED`
**TDD tham chiếu:** `arch/07` (Authz: OPA in-process, `package orca.authz.<x>`, `default allow := false`, `*_test.rego`), `arch/05` (tenant/RLS/outbox), `arch/08` (deadline), `services/api-gateway.md` §1/§2/§6
**Phụ thuộc:** BE-MCP-SOL-001 (khung `mcp-service`, proto `McpService`, migration `0001`), BE-MCP-SOL-006 (token `scopes`, `Identity.Role`), BE-MCP-SOL-007 (descriptor: `risk`, `requiredScope`, `openWorld`, `spawnsProcess`, `readUntrusted`). **Cung cấp cho** BE-MCP-SOL-013 (input `killswitch`/`session`, nguồn của `Decision`).

---

## 1. Trạng thái hiện tại (re-verify trên code, 2026-10-01)

| Khẳng định của CR | Kết quả | Lệch? |
|---|---|---|
| Có bundle `policy/orca-authz`, chưa có gì ánh xạ tool MCP | Đúng: 6 package (`admin`, `annotation`, `project`, `repo`, `task_grant`, `tenant`) + `*_test.rego`; không có `mcp.rego`. `make opa-test` = `opa test policy/orca-authz/ -v` (`backend-go/Makefile:97`) | Không |
| `auth-service` có `opaclient` + `policypublisher` để "publish qua cơ chế hiện có" | `opaclient` chỉ query `data.orca.authz.admin.allow`. `policypublisher.FilePublisher` ghi **một file chung** `<bundle>/data/<kind>/<name>.json` (không có tenant) rồi `Evaluator` tự invalidate sau ≤2s | **Lệch ý đồ**: cơ chế này không chứa được policy *theo tenant* → xem Quyết định 1 |
| `common/policy.Evaluator` đủ dùng | `Decision()` chỉ trả `bool` (undefined/non-bool ⇒ deny hoặc error). Cần object `{decision,reasons,source}` ⇒ thiếu hàm | **Cần additive** `Evaluator.Value` (§2.C) |
| OPA lỗi ⇒ fail-closed | Mọi call site Epic E hiện đã fail-closed khi `err != nil` (doc `evaluator.go`) | Khớp |
| `Identity.Role` chỉ có ở nhánh cookie (README v5 §1) | **Đã cũ**: `usecase/validate_identity.go` ghi `Role` nay cũng được điền ở nhánh Bearer JWT (CR-RBAC-002) khi token có claim `role`; vẫn rỗng với JWT cũ | Lệch nhẹ; fail-closed vẫn đúng (rỗng ⇒ không admin) |
| `ci/check-opa-bundle-in-images.sh` | Chỉ lặp qua `auth-service task-service annotation-service project-service`, kiểm tar có `policy/orca-authz/*.rego`; `Dockerfile` mỗi service `COPY policy ./policy` + `COPY --from=build /src/policy/orca-authz /policy/orca-authz` | Phải **thêm `mcp-service`** (§2.I) |
| Kênh admin dùng `errNotAdmin` | `errors.New("caller is not an admin")` (`channels_tenant_project.go:37`) — **không** có prefix `CODE:` | Kênh `mcp.admin.*` dùng lỗi riêng `MCP_NOT_ADMIN` (C3/C4) |
| Lỗi gRPC → WS message | `writeDialectError` (`session_dialect.go:94`) gửi `err.Error()` nguyên văn; với lỗi gRPC đó là `rpc error: code = X desc = CODE: msg` ⇒ **regex FE `^([A-Z0-9_]+): ` sẽ không khớp** | Phải có bộ chuyển `mcpWSError` (§2.G) |
| `mcp-service`, `mcp.rego`, `tool_policies` | Không tồn tại (`ls backend-go/services` không có `mcp-service`) | Đúng như CR |

### Quyết định khác/thêm so với CR gốc

1. **Policy tenant truyền qua `input`, không qua `data/` của bundle.** CR nói publish bằng `policypublisher`; nhưng `FilePublisher` ghi file dùng chung, không phân tenant, và bundle là volume dùng chung nhiều replica. `mcp-service` đọc `tool_policies` của tenant từ DB (RLS), nhét vào `input.tenant_policies`. Rego là **tĩnh** (bundle chỉ chứa luật + hard-deny); đổi policy không cần ghi file ⇒ không có cửa sổ lệch giữa replica và không có rủi ro rò rỉ policy giữa tenant. Đổi hard-deny = đổi Rego = PR + `opa test` (đúng ý "không policy tenant nào mở được").
2. **PDP đặt ở `mcp-service` (in-process OPA), không ở api-gateway** (T1/T2): gateway chỉ gọi gRPC `EvaluateToolCall`/`FilterTools`. Một client OPA duy nhất (`common/policy`), không client thứ hai ⇒ không cần ADR.
3. **Strictest-wins, không "most-specific-wins".** Nhiều policy cùng khớp ⇒ lấy `deny > require_approval > allow` bất kể độ cụ thể (policy khớp *thay thế* mặc định theo risk, nên vẫn nới/siết được). Lý do: "most-specific" cho phép một `allow` theo tool đè một `deny` theo client đã bị chặn. Hệ quả chấp nhận: không tạo được ngoại lệ "namespace=require_approval trừ 1 tool=allow" — admin thấy qua `explain` (liệt kê mọi policy khớp).
4. **Sàn rủi ro (risk floor):** `exec`/`destructive` chỉ được nới thành `allow` bởi policy có `match.tool` chính xác; wildcard (namespace/risk/client) bị kẹp về `require_approval` ở runtime **và** bị từ chối lúc upsert. `admin` risk luôn cần `role=admin` kể cả có policy `allow`.
5. **Upsert từ chối cả `require_approval` chạm hard-deny** (CONTRACT chỉ nói `allow`): `require_approval` cũng "mở" tool. Runtime hard-deny vẫn thắng mọi thứ (phòng thủ chiều sâu).
6. **Không cache quyết định OPA; cache đầu vào** (policy/settings/client/kill-state, TTL 5s + vô hiệu bằng sự kiện). Eval OPA in-process chuẩn bị sẵn là micro/mili-giây (**chưa đo** — gate bằng benchmark `BenchmarkEvaluateToolCall`, p99 < 5ms; nếu trượt mới thêm cache quyết định khoá `sha256(input)` TTL 5s). Lý do: cache quyết định là nơi dễ phục vụ quyết định `allow` cũ sau khi siết policy/kill switch.
7. **Mặc định tenant mới: `enabled` = `MCP_TENANT_DEFAULT_ENABLED` (mặc định `true`) — Đã chốt (D6, 2026-10-01).** `mcp-service` đọc env khi tạo lười hàng `tenant_settings` của tenant mới (INSERT tường minh `enabled`, không dựa DEFAULT của DDL); admin vẫn tắt được trong UI (`mcp.admin.settings.set`). Công tắc tổng `MCP_ENABLED` (cấp tiến trình) vẫn mặc định `false` tới gate CR-015. **Cờ này chỉ quyết định `enabled`:** mặc định rủi ro (read/write cho phép, exec/destructive cần phê duyệt, admin bị chặn), hard-deny, scope và kill switch là mặc định bắt buộc, **không** thể bị cờ này thay đổi hay "mặc định đi"; có test `TestTenantDefaultEnabledTrue_DoesNotChangeRiskDefaults` (§Kiểm thử). `McpServerInfo.enabled` = chỉ cờ môi trường `MCP_ENABLED` (nếu trộn cả cờ tenant thì FE ẩn luôn tab Settings và admin không thể bật lại) — xem yêu cầu thay đổi CONTRACT R-5.

---

## 2. Giải pháp

### A. Mô hình quyết định (thứ tự đánh giá cố định, `else`-chain trong Rego)

```
1 killswitch.active (013)            → deny  source=kill_switch
2 hardDenied(channel)                → deny  source=hard_deny      (không gì mở được)
3 descriptor thiếu channel           → deny  source=tool_descriptor
4 tenant.enabled != true             → deny  source=tenant_settings
5 client.status != allowed           → deny  source=client
6 requiredScope ∉ token.scopes       → deny  source=scope
7 risk=admin ∧ role≠admin            → deny  source=role
8 spawnsProcess ∧ depth ≥ max_depth  → deny  source=recursion      (depth do 013 cấp)
9 pre = strictest(policy khớp) | risk_default      (allow | require_approval | deny)
10 sàn exec/destructive: pre=allow mà không có policy tool-chính-xác → require_approval
11 sàn open-world: allow ∧ openWorld ∧ session.untrusted_read → require_approval
```
Mặc định theo risk: `read`/`write_reversible`=allow, `exec`/`destructive`=require_approval, `admin`=deny; risk lạ ⇒ deny. **Mọi điều kiện deny viết dạng `not <vị-từ-dương>`** nên thiếu trường input ⇒ deny (undefined không bao giờ được rơi qua `else` thành allow); test `test_empty_input_denied`, `test_missing_*` khoá điều này.

Input schema (do `mcp-service` dựng; **mọi trường tool lấy từ catalog của gateway, không từ client**):

```json
{ "user": {"id": "uuid", "role": "admin|user"},
  "client": {"id": "string", "status": "allowed|blocked|pending"},
  "token": {"scopes": ["orca:read"]},
  "settings": {"enabled": true, "max_depth": 1},
  "killswitch": {"active": false},
  "session": {"depth": 0, "untrusted_read": false},
  "tenant_policies": [{"id": "uuid", "decision": "allow|require_approval|deny",
                       "match": {"tool": "task_create", "namespace": "task", "risk": "read", "clientId": "…", "roles": ["user"]}}],
  "tool": {"name": "task_create", "channel": "task.create", "namespace": "task", "risk": "write_reversible",
           "required_scope": "orca:write", "open_world": false, "spawns_process": false} }
```
`match` chỉ chứa các chiều **có đặt** (Go bỏ nil; JSON `null` sẽ làm `not m.tool` sai). Không đưa `args` vào input: policy không quyết định theo tham số (tránh tin chuỗi do LLM sinh); tham số chỉ vào `paramsHash`/preview ở 013.

### B. Rego — `backend-go/policy/orca-authz/mcp.rego` (NEW) + `mcp_test.rego` (NEW)

Output: `data.orca.authz.mcp.decision` = `{decision, source, reasons[]}`; `data.orca.authz.mcp.hard_denied` (bool, dùng cho catalog/upsert). Bản đầy đủ (đã `opa fmt`):

```rego
package orca.authz.mcp

import rego.v1

risk_default := {"read": "allow", "write_reversible": "allow", "exec": "require_approval", "destructive": "require_approval", "admin": "deny"}

strictness := {"allow": 0, "require_approval": 1, "deny": 2} # decision lạ xếp hạng 2 (deny) qua object.get

hard_deny_prefixes := {"credentials.", "auth.", "mcp."}

hard_deny_channels := {
	"admin.createUser", "admin.updateUserRole", "admin.deactivateUser", "admin.reactivateUser",
	"admin.forceRevokeSession", "admin.forceRevokeAllSessions",
	"admin.createPolicy", "admin.updatePolicy", "admin.deletePolicy", "admin.getPolicy", "admin.listPolicies",
	"aiProvider.writeCredential",
	"devServer.agentTokens.create", "devServer.agentTokens.revoke", "devServer.agentTokens.list",
	"team.create", "team.addMember", "team.removeMember",
	"profile.createCompany", "profile.updateCompany", "profile.updateUser",
}

hard_deny_patterns := {`\.start(Cli)?AuthLogin$`}

default hard_denied := false
hard_denied if { some prefix in hard_deny_prefixes; startswith(input.tool.channel, prefix) }
hard_denied if input.tool.channel in hard_deny_channels
hard_denied if { some pattern in hard_deny_patterns; regex.match(pattern, input.tool.channel) }

channel_known if input.tool.channel
tenant_enabled if input.settings.enabled == true
client_allowed if input.client.status == "allowed"
scope_ok if input.tool.required_scope in input.token.scopes
user_is_admin if input.user.role == "admin"
depth_ok if input.session.depth < input.settings.max_depth
killswitch_clear if input.killswitch.active == false
session_clean if input.session.untrusted_read == false
tool_closed_world if input.tool.open_world == false
tool_not_spawning if input.tool.spawns_process == false

# --- khớp policy tenant (rule rỗng không bao giờ khớp: has_dim) ---
has_dim(m) if m.tool
has_dim(m) if m.namespace
has_dim(m) if m.risk
has_dim(m) if m.clientId
has_dim(m) if m.roles
match_tool(m) if not m.tool
match_tool(m) if m.tool == input.tool.name
match_namespace(m) if not m.namespace
match_namespace(m) if m.namespace == input.tool.namespace
match_risk(m) if not m.risk
match_risk(m) if m.risk == input.tool.risk
match_client(m) if not m.clientId
match_client(m) if m.clientId == input.client.id
match_roles(m) if not m.roles
match_roles(m) if input.user.role in m.roles
policy_matches(m) if { has_dim(m); match_tool(m); match_namespace(m); match_risk(m); match_client(m); match_roles(m) }

matched := [p | some p in input.tenant_policies; policy_matches(p.match)]

policy_decision := d if {
	count(matched) > 0
	rank := max([object.get(strictness, p.decision, 2) | some p in matched])
	some d, r in strictness
	r == rank
}

base := object.get(risk_default, input.tool.risk, "deny")
pre := policy_decision if count(matched) > 0
pre := base if count(matched) == 0

exact_tool_allow if { some p in matched; p.match.tool == input.tool.name; p.decision == "allow" }
floor_exact_applies if { pre == "allow"; input.tool.risk in {"exec", "destructive"}; not exact_tool_allow }
step1 := "require_approval" if floor_exact_applies
step1 := pre if not floor_exact_applies
floor_openworld_applies if { step1 == "allow"; not tool_closed_world; not session_clean }
step2 := "require_approval" if floor_openworld_applies
step2 := step1 if not floor_openworld_applies
# … decision_source / policy_reasons / floor_reasons: xem file đầy đủ khi implement (cùng khuôn)

default decision := {"decision": "deny", "source": "default", "reasons": ["policy_undefined"]}

decision := {"decision": "deny", "source": "kill_switch", "reasons": ["kill_switch_active"]} if { not killswitch_clear }
else := {"decision": "deny", "source": "hard_deny", "reasons": ["hard_deny"]} if { hard_denied }
else := {"decision": "deny", "source": "tool_descriptor", "reasons": ["tool_descriptor_incomplete"]} if { not channel_known }
else := {"decision": "deny", "source": "tenant_settings", "reasons": ["mcp_disabled_for_tenant"]} if { not tenant_enabled }
else := {"decision": "deny", "source": "client", "reasons": ["client_not_allowed"]} if { not client_allowed }
else := {"decision": "deny", "source": "scope", "reasons": ["scope_missing"]} if { not scope_ok }
else := {"decision": "deny", "source": "role", "reasons": ["admin_role_required"]} if { input.tool.risk == "admin"; not user_is_admin }
else := {"decision": "deny", "source": "recursion", "reasons": ["depth_limit"]} if { not tool_not_spawning; not depth_ok }
else := {"decision": step2, "source": decision_source, "reasons": array.concat(policy_reasons, floor_reasons)}
```
> Lưu ý khi chép: các `;` trên một dòng ở trên chỉ để gọn tài liệu — file thật phải để mỗi biểu thức một dòng (`opa fmt -w`). Từ vựng `reasons` (admin đọc, FE-008 ánh xạ): `kill_switch_active`, `hard_deny`, `tool_descriptor_incomplete`, `mcp_disabled_for_tenant`, `client_not_allowed`, `scope_missing`, `admin_role_required`, `depth_limit`, `policy:<id>:<decision>`, `risk_default:<risk>=<decision>`, `risk_floor:exact_tool_policy_required`, `open_world_after_untrusted_read`, `policy_undefined`, `policy_unavailable` (Go, khi OPA lỗi).

**Hard-deny (CR §D) → danh sách cụ thể** (theo *channel*, không theo tên tool, vì tên là dẫn xuất `.`→`_`): mọi `credentials.*`, `auth.*`, `mcp.*` (gồm `mcp.approval.decide`, `mcp.token.*`, `mcp.admin.*`); tạo/sửa/khoá user (`admin.createUser|updateUserRole|deactivateUser|reactivateUser`), thu hồi phiên (`admin.forceRevoke*`), sửa/đọc access-policy (`admin.*Policy|getPolicy|listPolicies`), ghi credential AI (`aiProvider.writeCredential`), phát/thu hồi token dev-server (`devServer.agentTokens.*`), đổi nhóm/thành viên (`team.create|addMember|removeMember`), đổi công ty/hồ sơ user (`profile.createCompany|updateCompany|updateUser`), khởi OAuth login host (`*.startAuthLogin|startCliAuthLogin`). Kênh thật trích bằng `grep` `Register("…")` ở `wscompat/channels*.go`; **một số kênh đăng ký qua biến (vd. `registerCLIAuthStatusRelay(r, client, "github.checkAuthStatus", …)`) nên danh sách có thể thiếu — BE-007 phải xuất đầy đủ catalog và test dưới đây là chốt chặn**. "Gỡ audit": hiện không có kênh xoá audit (`admin.queryAuditLog` chỉ đọc) — giữ nguyên, thêm test chống kênh `*.deleteAudit*` xuất hiện.

**`mcp_test.rego` — bảng ca (44 ca đã PASS):** read/write allow; exec & destructive ⇒ approval; admin risk ⇒ deny (user) / allow (admin + policy); thiếu scope; thiếu `token`; tenant tắt; thiếu `settings`; client blocked/pending; kill switch; thiếu `killswitch`; input rỗng; thiếu `risk`; risk lạ; hard-deny credentials (kể cả có policy allow), `mcp.*`, **mọi** phần tử `hard_deny_channels` (`every`), **mọi** prefix (`every`), `startAuthLogin`, prefix không khớp chuỗi con (`task.mcp.note`); policy siết read→deny, nới write→approval, deny thắng allow bất kể độ cụ thể, approval thắng allow, policy client khác bị bỏ qua, chiều `roles`, `match:{}` không bao giờ khớp, decision lạ=deny; wildcard exec allow bị kẹp, exact-tool allow được giữ; open-world sạch/nhiễm/đóng-world/deny-giữ-nguyên/thiếu cờ=nhiễm; spawn bị chặn ở `depth>=max_depth`, tool không spawn không bị ảnh hưởng; thiếu `channel` ⇒ `tool_descriptor`.

### C. Go — `common/policy` (additive) + adapter OPA của `mcp-service`

```go
// backend-go/common/policy/evaluator.go (MODIFY, additive)
// Value returns the raw JSON-decoded result of query (undefined => nil, nil). Decision()
// only yields a bool; MCP needs {decision, reasons, source}. Same fail-closed contract:
// callers MUST treat (nil|error) as deny.
func (e *Evaluator) Value(ctx context.Context, query string, input any) (any, error) { /* preparedQuery + Eval, giống Decision */ }
```
```go
// services/mcp-service/internal/adapter/policyengine/opa_engine.go (NEW)
const (decisionQuery = "data.orca.authz.mcp.decision"; hardDeniedQuery = "data.orca.authz.mcp.hard_denied")
func (e *OPAEngine) Evaluate(ctx context.Context, in domain.PolicyInput) (domain.PolicyDecision, error) {
    ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond) // eval in-process; bound để policy xấu không treo tool call (override mặc định 5s)
    defer cancel()
    v, err := e.ev.Value(ctx, decisionQuery, in.AsMap())
    m, ok := v.(map[string]any)
    if err != nil || !ok { return domain.DenyUnavailable(), err } // fail-closed: decision=deny, reasons=[policy_unavailable]
    return domain.ParseDecision(m) // decision ∉ {allow,require_approval,deny} ⇒ deny
}
```
`cmd/server/main.go`: `ev := policy.NewEvaluator(cfg.OPABundlePath)`; `ev.Warm(ctx, decisionQuery, hardDeniedQuery)` — bundle hỏng ⇒ service **không khởi động** (cùng khuôn `auth-service/main.go:193`). Env: `OPA_BUNDLE_PATH` (mặc định `../../policy/orca-authz`), `MCP_MAX_DEPTH` (=1), `MCP_POLICY_CACHE_TTL` (=5s).

Fail-closed theo tầng: (1) OPA lỗi/undefined/kiểu sai ⇒ `Decision{deny, source:"policy_error"}` (không trả `error` để gateway không nhầm); (2) gRPC tới `mcp-service` lỗi ⇒ gateway coi là deny (`isError` trung tính "Orca cannot authorize this action right now"); (3) `tools/list` lỗi ⇒ JSON-RPC `-32603` (không trả danh sách rỗng im lặng); (4) cờ ngoại lệ `MCP_POLICY_FAIL_OPEN_READ` (mặc định **false**, chỉ cho `risk=read` và không bao giờ cho hard-deny, đếm `orca_mcp_policy_fail_open_total`) — **chưa bật**, cần ADR.

### D. Migration — `services/mcp-service/migrations/00NN_tool_policies.up.sql` (NN theo thứ tự merge; BE-001 giữ `0001`)

```sql
CREATE TABLE mcp.tenant_settings (
  tenant_id            UUID PRIMARY KEY,
  enabled              BOOLEAN NOT NULL DEFAULT false,   -- usecase luôn INSERT tường minh = MCP_TENANT_DEFAULT_ENABLED (D6); DEFAULT DDL chỉ là lưới an toàn
  dcr_enabled          BOOLEAN NOT NULL DEFAULT false,
  max_token_days       INT NOT NULL DEFAULT 30  CHECK (max_token_days BETWEEN 1 AND 90),      -- T4: PAT ≤ 90 ngày
  approval_ttl_seconds INT NOT NULL DEFAULT 300 CHECK (approval_ttl_seconds BETWEEN 30 AND 900),
  policy_epoch         BIGINT NOT NULL DEFAULT 0,    -- tăng trong CÙNG transaction mỗi lần đổi policy/settings
  updated_by UUID, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE mcp.tool_policies (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL,
  version INT NOT NULL DEFAULT 1,
  match_tool TEXT, match_namespace TEXT, match_client_id TEXT, match_roles TEXT[],
  match_risk TEXT CHECK (match_risk IN ('read','write_reversible','exec','destructive','admin')),
  decision TEXT NOT NULL CHECK (decision IN ('allow','require_approval','deny')),
  note TEXT CHECK (char_length(note) <= 500),
  created_by UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(match_tool, match_namespace, match_risk, match_client_id, match_roles) >= 1), -- cấm policy "khớp tất cả"
  CHECK (match_roles IS NULL OR match_roles <@ ARRAY['admin','user'])
);
CREATE INDEX idx_tool_policies_tenant ON mcp.tool_policies (tenant_id);
CREATE TABLE mcp.tool_policy_revisions (   -- append-only: ai đổi gì, lúc nào
  policy_id UUID NOT NULL, tenant_id UUID NOT NULL, version INT NOT NULL,
  op TEXT NOT NULL CHECK (op IN ('create','update','delete')),
  snapshot JSONB NOT NULL, changed_by UUID NOT NULL, changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (policy_id, version, op));
-- RLS: mỗi bảng ENABLE ROW LEVEL SECURITY + POLICY tenant_isolation USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
```
`.down.sql` drop theo thứ tự ngược; CI up→down→up (arch/04). Không FK xuyên DB (`match_client_id` là logical FK: kiểm tồn tại qua repo OAuth client của BE-005, thiếu thì từ chối `MCP_NOT_FOUND`; `created_by` là user id từ metadata).

### E. gRPC (additive vào `McpService`, `proto/orca/mcp/v1/governance.proto` — BE-001 sở hữu `service`)

```proto
message ToolRef { string name=1; string channel=2; string namespace=3; string risk=4; string required_scope=5;
                  bool open_world=6; bool spawns_process=7; bool read_untrusted=8; }   // gateway dựng từ catalog, không từ client
message CallContext { string client_id=1; repeated string scopes=2; string token_kind=3; string mcp_session_id=4; int32 depth=5; }
message PolicyDecision { string decision=1; string source=2; repeated string reasons=3; int64 policy_epoch=4; }
message EvaluateToolCallRequest { ToolRef tool=1; CallContext ctx=2; bool dry_run=3; }       // tenant/user/role từ gRPC metadata, KHÔNG từ body
message FilterToolsRequest { repeated ToolRef tools=1; CallContext ctx=2; }
message FilterToolsResponse { repeated ToolDecision decisions=1; int64 policy_epoch=2; }     // ToolDecision{name, PolicyDecision}
message ExplainPolicyRequest { string tool=1; string user_id=2; string user_role=3; string client_id=4; ToolRef tool_ref=5; }
message UpsertToolPolicyRequest { ToolPolicy policy=1; repeated ToolRef touched_tools=2; }   // gateway mở rộng match → touched_tools
message DeleteToolPolicyRequest { string policy_id=1; }
// + ListToolPolicies, Get/UpdateTenantSettings (partial: google.protobuf.FieldMask)
```
`buf lint` + `buf breaking` (CI contract). Mọi RPC lấy `tenant_id/user_id/role` từ metadata (`grpcmw.MetadataTenantID/UserID/Role`); RPC `admin` kiểm lại `role=="admin"` ở `mcp-service` (không chỉ tin gateway) ⇒ `MCP_NOT_ADMIN`.

### F. Usecase (`internal/usecase/`, mỗi cái một type `Execute`)

- `EvaluateToolCall`: nạp snapshot `{settings, policies, epoch}` (cache theo tenant), trạng thái client (BE-005), kill-state & taint session (013, qua port) → dựng `PolicyInput` → `engine.Evaluate` → trả `PolicyDecision`; luôn tăng metric. **Không có side effect** (013 mới tạo approval/audit/limit) — `dry_run` chỉ để `explain`.
- `FilterTools`: một lần nạp snapshot, lặp `Evaluate` cho N tool (không N lần RPC/DB); `deny` ⇒ loại khỏi `tools/list`; `require_approval` vẫn liệt kê (người dùng cần gọi mới tạo được approval). Dùng lại cho `mcp.admin.tool.list` (`effective`, `effectiveSource` với `role=user`, không client, đủ scope).
- `UpsertToolPolicy` (thứ tự kiểm — dừng ở lỗi đầu): `role==admin` → validate enum/`num_nonnulls≥1`/`roles⊂{admin,user}`/độ dài `note` (`MCP_INVALID_ARGUMENT`*) → với `decision≠deny`: với mỗi `touched_tools`, `hard_denied` (OPA) ⇒ **`MCP_POLICY_HARD_DENY`** (message liệt kê ≤5 tên) → `decision=allow` mà `touched_tools` có `exec|destructive` và `match.tool` rỗng ⇒ `MCP_INVALID_ARGUMENT`* → txn: tạo (`version=1`) hoặc `UPDATE … SET version=version+1 … WHERE id=$1 AND tenant_id=$2 AND version=$3`; 0 dòng ⇒ tồn tại thì **`MCP_POLICY_VERSION_CONFLICT`** (message kèm `current=<n>`) ngược lại `MCP_NOT_FOUND` → ghi `tool_policy_revisions` + `settings.policy_epoch+1` + outbox `orca.mcp.policy.changed` + sự kiện audit quản trị (013) **cùng transaction**. (*`MCP_INVALID_ARGUMENT` chưa có trong CONTRACT — xem R-1.)
- `DeleteToolPolicy`: cùng khuôn (không version — xoá policy đã bị người khác sửa vẫn thành công; ghi revision `delete`).
- `UpdateTenantSettings`: patch từng phần bằng một `UPDATE … SET col = COALESCE($n, col)` (atomic; **không** optimistic-lock vì CONTRACT không có `version` cho settings — last-write-wins có chủ đích); `killSwitch` trong patch ⇒ `MCP_INVALID_ARGUMENT`*. Hạ `maxTokenDays` không thu hồi token cũ (BE-006 kẹp `exp` khi verify). Bump `policy_epoch` + outbox.
- `GetTenantSettings`: gộp `killSwitch` qua port `KillSwitchReader` (013 hiện thực; trước đó là stub `{active:false}`).
- `ExplainPolicy`: **cùng** `EvaluateToolCall` với `dry_run=true`, không code đường thứ hai. Role của `userId`: gateway tra qua auth-service `ListUsers` (admin RPC có sẵn; **chưa xác minh** có lọc theo id — nếu không, quét ≤5 trang×200) hoặc rơi về `role=user` và thêm reason `role_unresolved`. Chỉ admin; reasons chứa policy id nên không bao giờ trả cho LLM.

### G. api-gateway

**`wscompat/channels_mcp_policy.go` (NEW)** — `registerMcpPolicyChannels(r, client mcpv1.McpServiceClient, catalog ToolCatalog)`; mỗi handler: `id.Role != "admin"` ⇒ `errors.New("MCP_NOT_ADMIN: admin role required")` → `AttachIdentity(ctx, usecase.Identity{TenantID, UserID, Role})` → `context.WithTimeout(ctx, rpcTimeout)` (8s sẵn có) → RPC → `mcpWSError(err)`. **Tham số luôn là MỘT object ở `args[0]`** (khuôn `admin.deletePolicy` hiện có: `decodeArg[struct{PolicyID string `json:"policyId"`}]`) — xem R-2. `policy.upsert` nhận `McpToolPolicy`; gateway tra `catalog` để tính `touched_tools` từ `match` (tool/namespace/risk; `match` chỉ có `clientId`/`roles` ⇒ toàn catalog).

**`wscompat/channels_mcp_errors.go` (NEW)**: `mcpWSError(err) error` — `status.Convert(err).Message()` (là `CODE: msg` do `apperrors.ToGRPCStatus`); không khớp `^[A-Z0-9_]+: ` ⇒ `errors.New("MCP_INTERNAL: internal error")`; `ctx.Err()` ⇒ `MCP_TIMEOUT: …`. Dùng chung cho mọi `channels_mcp_*.go` (013 dùng lại).

**`adapter/mcpserver/toolgate.go` (NEW, tên chỗ ở BE-003/008)**: `ToolGate.Authorize(ctx, claims, tool)` gọi `EvaluateToolCall`; `ToolGate.List(ctx, claims, tools)` gọi `FilterTools`. **Hai chốt enforce độc lập**: `tools/list` (ẩn) và `tools/call` (chặn — kiểm lại *mỗi lần gọi*, không tin `tools/list` trước đó, không tin tên tool client gửi: tra catalog theo tên, không có trong catalog ⇒ "unknown tool"). Thêm cầu chì cuối trong `ToolExecutor`: từ chối `Dispatch` mọi channel có prefix thuộc `neverDispatchPrefixes = {"mcp.","credentials.","auth."}` kể cả khi `Authorize` trả allow (lỗi cấu hình/bug) — test `TestNeverDispatchPrefixesMatchRego` eval `data.orca.authz.mcp.hard_deny_prefixes` bằng `common/policy` và so bằng với hằng Go, tránh lệch. Kết quả `deny` trả `isError:true` với văn bản trung tính từ bảng `reason→text` (không lộ policy id/hard-deny: "This action isn't permitted by your organization's policy."); chi tiết chỉ vào audit (013).

**Invalidation + `tools/list_changed`**: outbox `orca.mcp.policy.changed` `{tenant_id, policy_epoch, policy_id?, op}` (JetStream stream `MCP`) → mỗi replica `mcp-service` `SubscribeEphemeral` → xoá cache tenant ngay → phát ephemeral `orca.ephemeral.mcp.tenant.<tenantId>.tools_changed` (T5, BE-004) để gateway đẩy `notifications/tools/list_changed` tới phiên đang mở. Cận trên độ trễ hiệu lực: replica đã ghi = tức thì; replica khác = độ trễ sự kiện (thường < 1s), tối đa TTL cache 5s nếu mất sự kiện ⇒ khớp AC "≤5s".

### H. Ghi nhận quyết định cho audit (CR AC cuối)
Mọi `EvaluateToolCall` (không `dry_run`) trả `PolicyDecision` cho 013, nơi ghi audit/journal `tool_calls` (kể cả allow). Solution này không tự ghi audit tool-call; **có** ghi audit quản trị (`mcp.policy.upsert|delete`, `mcp.settings.update`, actor_type=`user`) qua cùng pipeline outbox của 013.

### I. CI
`ci/check-opa-bundle-in-images.sh`: thêm `mcp-service` vào vòng lặp và sửa dòng cuối thành đếm động (`"All ${#svcs[@]} images…"`); `services/mcp-service/deploy/Dockerfile` phải có đúng hai dòng `COPY policy ./policy` / `COPY --from=build /src/policy/orca-authz /policy/orca-authz` như `auth-service/deploy/Dockerfile:12,21`. `mcp.rego` nằm trong thư mục nên tự có mặt ở 4 image cũ (script vẫn xanh). `make opa-test` chạy cả `mcp_test.rego`.

---

## Hợp đồng với frontend

| Kênh | Request (một object ở `args[0]`) | Kết quả | Lỗi |
|---|---|---|---|
| `mcp.admin.settings.get` | — | `{enabled,dcrEnabled,maxTokenDays,approvalTtlSeconds,killSwitch}` | `MCP_NOT_ADMIN`, `MCP_DISABLED` |
| `mcp.admin.settings.set` | patch ≥1 trong `enabled,dcrEnabled,maxTokenDays,approvalTtlSeconds` | settings mới (đủ field) | `MCP_NOT_ADMIN`, `MCP_INVALID_ARGUMENT`* (ngoài biên 1–90 / 30–900, có `killSwitch`) |
| `mcp.admin.policy.list` | — | `McpToolPolicy[]` (`updatedBy` = userId) | `MCP_NOT_ADMIN` |
| `mcp.admin.policy.upsert` | `McpToolPolicy` (không `id` ⇒ tạo; có `id` ⇒ cần `version` hiện tại) | `McpToolPolicy` (`version` mới) | `MCP_POLICY_HARD_DENY`, `MCP_POLICY_VERSION_CONFLICT`, `MCP_INVALID_ARGUMENT`*, `MCP_NOT_FOUND` |
| `mcp.admin.policy.delete` | `{policyId}` | `{ok:true}` | `MCP_NOT_FOUND` |
| `mcp.admin.policy.explain` | `{tool, userId?, clientId?}` | `{decision, reasons[]}` | `MCP_NOT_FOUND` (tool lạ) |

`tools/list` của client MCP chỉ chứa `allow`+`require_approval`; `deny` không bao giờ lộ tên. Mã lỗi/field khớp CONTRACT; chỗ có dấu `*` cần R-1.

## Sửa TDD kèm theo
T1 (`api-gateway.md` §3/§6: `adapter/mcpserver` + `ToolGate` chỉ dịch giao thức + gọi gRPC); T2 (`ToolExecutor` luôn đi sau quyết định của `mcp-service`); T7 (`00-service-catalog.md`, `arch/02`: `mcp-service` giữ policy/settings); T8 (metric `orca_mcp_policy_decisions_total{decision,source,risk}`, `orca_mcp_policy_eval_seconds`, `orca_mcp_policy_errors_total`, `orca_mcp_policy_fail_open_total`; env `OPA_BUNDLE_PATH`, `MCP_MAX_DEPTH`, `MCP_POLICY_CACHE_TTL`, `MCP_POLICY_FAIL_OPEN_READ`). Ngoài bảng T#: `arch/07` mục Authz thêm package `orca.authz.mcp` + nguyên tắc "policy theo tenant đi qua input"; `arch/10` danh sách image có bundle thêm `mcp-service`.

## Kiểm thử
- Rego: `cd backend-go && make opa-test` (hoặc `opa test policy/orca-authz/ -v`) — `mcp_test.rego` ≥ 40 ca ở §B.
- Go unit: `cd backend-go/services/mcp-service && go test ./internal/usecase/... ./internal/adapter/policyengine/...` — `decision_table_test.go` bảng ≥40 tổ hợp (scope×role×policy×risk×client×taint×depth) chạy **Evaluator thật + bundle thật**; ca "OPA lỗi" (bundle path hỏng/engine fake trả error) ⇒ deny; `TestUpsertRejectsHardDenyTouch` (tool chính xác, namespace chứa tool hard-deny, `match` chỉ `clientId`, `require_approval`); `TestUpsertVersionConflict` (hai goroutine); `TestFilterToolsHidesDeny_CallStillBlocked` (AC: ẩn **và** chặn khi gọi trực tiếp).
- D6: `TestTenantDefaultEnabledTrue_DoesNotChangeRiskDefaults` — tenant mới với `MCP_TENANT_DEFAULT_ENABLED=true` (và `false`) cho cùng bảng quyết định mặc định: `read/write_reversible`=allow, `exec/destructive`=require_approval, `admin`=deny, hard-deny luôn deny, kill switch tắt/bật được; `enabled` là field duy nhất khác nhau. `TestTenantSettings_LazyCreateUsesConfigDefault`.
- Parity: `TestHardDenyCatalogParity` — với mọi tool trong catalog BE-007, `hardDenied` (flag descriptor) == `data.orca.authz.mcp.hard_denied`; danh sách vàng các channel nhạy cảm phải hard-denied.
- Integration (`//go:build integration`, testcontainers): migration up→down→up; RLS cross-tenant ⇒ not-found; `policy_epoch` tăng cùng txn; sự kiện `orca.mcp.policy.changed` làm vô hiệu cache ở replica thứ hai.
- Gateway: `cd backend-go/services/api-gateway && go test ./internal/adapter/wscompat/... -run Mcp` — non-admin ⇒ `MCP_NOT_ADMIN:`; lỗi gRPC `FailedPrecondition` ⇒ message đúng `^MCP_…: `; `TestNeverDispatchPrefixesMatchRego`.
- Benchmark: `go test -bench EvaluateToolCall -benchmem ./internal/adapter/policyengine/` — gate p99 < 5ms (**chưa đo**).
- CI: `backend-go/ci/check-opa-bundle-in-images.sh` (cần docker) xanh với 5 image; `buf lint && buf breaking --against '.git#branch=main'`.

## Impact analysis (gitnexus) — **chưa chạy**, chạy trước khi sửa
| Symbol | Lệnh | Rủi ro dự kiến |
|---|---|---|
| `Evaluator` (`common/policy`) | `impact({target:"Evaluator", direction:"upstream"})` | Phụ thuộc đồ thị (≥4 service dùng qua `opaclient`: auth, project, tenant, annotation); thay đổi chỉ **thêm** method ⇒ hành vi không đổi, nhưng công cụ có thể báo HIGH vì fan-in — báo người dùng trước khi sửa |
| `ChannelHandler`/`Registry.Register` (wscompat) | `impact({target:"Register", direction:"upstream"})` | LOW–MEDIUM (chỉ thêm file đăng ký mới) |
| `check-opa-bundle-in-images.sh` | n/a (script) | LOW |
Sau khi sửa: `detect_changes({scope:"compare", base_ref:"main"})`.

## Rủi ro & phụ thuộc
- **Danh sách hard-deny thiếu kênh đăng ký qua biến** → test parity + rà catalog BE-007; mặc định kênh không có descriptor ⇒ không có trong `tools/list` và `tools/call` ⇒ "unknown tool".
- Strictest-wins có thể làm admin bất ngờ → `explain` liệt kê mọi policy khớp; FE-008 nêu rõ quy tắc.
- `untrusted_read`/`depth`/`killswitch` do 013 cấp; trước 013 tất cả mặc định `false/0/false` nhưng **chỉ dùng được trong test** — không bật tool ghi trước khi 013 xong (mốc an toàn README v5).
- Lệch replica ≤5s khi mất sự kiện (đã nêu).

## Không thuộc phạm vi
Approval/audit tool-call/kill switch/rate limit/recursion counter/prompt-injection (SOL-013); UI (FE-MCP-SOL-008); descriptor & annotation sinh tự động (BE-007); quyết định theo *giá trị tham số* (cố ý không làm).

## Liên quan
[CR-MCP-012](../../../../../../docs/crs/v5/mcp-governance-safety/CR-MCP-012-tool-policy-and-annotations.md) · [BE-MCP-SOL-013](./BE-MCP-SOL-013-approvals-audit-killswitch.md) · `backend-go/common/policy/evaluator.go` · `backend-go/policy/orca-authz/` · `backend-go/services/auth-service/internal/adapter/policypublisher/publisher.go` · `backend-go/services/api-gateway/internal/adapter/wscompat/{registry.go,channels_admin_policies.go}` · FE: [FE-MCP-SOL-008](../../../../../frontend/crs/v5/mcp-governance-safety/solutions/FE-MCP-SOL-008-admin-policies-and-killswitch.md)
