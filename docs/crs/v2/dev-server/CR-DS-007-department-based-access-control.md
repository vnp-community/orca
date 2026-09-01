# CR-DS-007 — Department-Based Dev Server Access Control

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-007 |
| **Tên** | Map Dev Server Groups → Departments/Teams + OPA Policy |
| **Loại** | Feature — Multi-tenant Access Control (Phase 2) |
| **Priority** | P1 — High |
| **Phiên bản** | v6.1 |
| **Ngày tạo** | 2026-08-28 |
| **Trạng thái** | ✅ Hoàn tất (backend + frontend) — usecase + Postgres + wscompat (BE-SOL-003), quyết định §3 dùng defaults đã ghi (OR logic, ungrouped=admin-only, inherit xuống cây con); OPA rego KHÔNG viết riêng; Admin UI quản lý group/grant đã triển khai (FE-SOL-001), deployed b15.openledger.vn |
| **Phụ thuộc** | CR-DS-006, tenant-service's Department/Team model |
| **Tác động HLD** | security.md (Trust Boundary — tenant org data used for authorization) |

---

## 1. Bối cảnh

CR-DS-006 tạo `DevServerGroup` nhưng không tự nó phân quyền ai được thấy nhóm nào — mọi user trong tenant vẫn thấy mọi group. CR này thêm lớp phân quyền: **một `DevServerGroup` chỉ hiển thị cho user thuộc Department/Team đã được gán quyền vào group đó.**

Rà soát trước khi thiết kế (đã xác nhận, xem CR-DS-006 §2 và research trước đó):
- Department: **1 user — 1 department** (không multi), phẳng (không phân cấp), tồn tại ở `tenant.user_profiles.department_id`.
- Team: **N:M** (`tenant.team_members`), có `priority`.
- Chưa có bảng join nào giữa (Department|Team) và bất kỳ resource nào ngoài tenant-service.
- Có sẵn mẫu OPA level-matrix (`task_grant.rego`) để tham khảo cấu trúc policy.

## 2. Giải pháp

### 2.1 Bảng mapping mới (infra-fleet-service, KHÔNG sửa schema tenant-service)
```sql
CREATE TABLE infra.dev_server_group_grants (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  dev_server_group_id TEXT NOT NULL REFERENCES infra.dev_server_groups(id),
  grantee_kind TEXT NOT NULL,      -- 'department' | 'team'
  grantee_id TEXT NOT NULL,        -- department_id hoặc team_id (logical FK, tenant-service sở hữu)
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (dev_server_group_id, grantee_kind, grantee_id)
);
```
Logical FK sang `tenant.departments`/`tenant.teams` (khác service, khác database — giống cách `project-service` đã validate `tenant_id` qua gRPC call tới `tenant-service.ValidateTenant`, không FK vật lý xuyên service).

### 2.2 Usecase mới
- `GrantDevServerGroupAccess(groupId, granteeKind, granteeId)` / `RevokeDevServerGroupAccess(...)`.
- `ListDevServersForUser(ctx)` — thay thế `ListDevServers` ở nơi user thường gọi (không phải admin): resolve user's `department_id` (gọi `tenant-service.GetResolvedProfile`) + user's team memberships, rồi trả về dev server thuộc group nào có grant khớp. **Global admin luôn thấy tất cả** (giữ nguyên `callerGlobalRole` override pattern đã có ở `project-service/internal/usecase/authorization.go`).

### 2.3 OPA policy mới — `dev_server_access.rego`
Theo khuôn mẫu `task_grant.rego`'s level-matrix, nhưng khoá theo `(department_id ∪ team_ids)` thay vì task-ownership chain:
```rego
package orca.authz.dev_server_access

default allow = false

allow {
  input.actor.global_role == "admin"
}

allow {
  some grant in input.group_grants
  grant.grantee_kind == "department"
  grant.grantee_id == input.actor.department_id
}

allow {
  some grant in input.group_grants
  grant.grantee_kind == "team"
  grant.grantee_id == input.actor.team_ids[_]
}
```

## 3. Quyết định (đã áp dụng khi triển khai — dùng defaults đề xuất ở trên, chưa có xác nhận riêng của người dùng cho từng điểm, ghi lại rõ ràng để dễ đảo ngược nếu cần)

1. **Dev server chưa gán group** → chỉ admin thấy (KHÔNG bao giờ trả về trong `ListDevServersForUser`). Đã implement + test (`TestListDevServersForUser_UngroupedNeverReturned`).
2. **Grant ở group cha kế thừa xuống group con** → có, đã implement bằng ancestor-walk có cycle-guard trong `ListDevServersForUser`, test `TestListDevServersForUser_InheritsGrantFromParentGroup`.
3. **Department vs Team xung đột** → OR (1 trong 2 đủ), đã implement + test (`TestListDevServersForUser_DirectDepartmentGrantMatches`, `TestListDevServersForUser_TeamGrantMatches`).

## 4. Acceptance Criteria

- [x] Migration `infra.dev_server_group_grants` chạy sạch (integration test qua Testcontainers).
- [x] `ListDevServersForUser` trả đúng tập theo department + team; **admin KHÔNG dùng RPC này** — admin dùng `ListDevServers` (không lọc) như trước giờ, không cần override trong `ListDevServersForUser`.
- [ ] `dev_server_access.rego` — **KHÔNG viết** (quyết định triển khai): logic gate được implement trực tiếp trong Go usecase (`ListDevServersForUser`) thay vì OPA, để tránh trùng lặp logic ở 2 nơi (Go + Rego) có nguy cơ lệch nhau. Test coverage tương đương đã có ở tầng Go (xem `list_dev_servers_for_user_test.go`).
- [x] 3 điểm quyết định ở mục 3 đã áp dụng theo defaults đề xuất, ghi lại rõ ràng ở trên (chưa phải xác nhận riêng của người dùng cho từng điểm — có thể đảo ngược dễ dàng nếu cần, logic tập trung ở 1 hàm `groupGrantsAccess` trong `list_dev_servers_for_user.go`).

**Known gap ghi nhận khi triển khai:** `ListDevServersForUser`'s `team_ids` luôn rỗng — tenant-service chưa có RPC "list teams for user" (chỉ có `ListTeams(company)`/`ListTeamMembers(team)`, N+1 nếu tự ghép). Department-based grant hoạt động đầy đủ; team-based grant cần RPC mới ở tenant-service mới thực sự khớp được. Xem `channels_dev_server_access_control.go`'s `devServer.listForUser` doc comment.
