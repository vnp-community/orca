# CR-TG-003 — Task Access Control: Team-Scope Resolver thật, Revoke/Expiry, Public Share-Link

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-003 |
| **Tên** | Sửa `StubTeamScopeResolver` (trả `nil` cứng) + bổ sung revoke/expiry/share-link/notification cho grant system |
| **Loại** | Feature / Bugfix (Security-relevant) |
| **Priority** | P0 — grant scope=team hiện chết hoàn toàn trong production |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-TG-001](./CR-TG-001-orcatask-data-model-widening.md) (cần `owner_id` cho owner short-circuit) |
| **Áp dụng thiết kế** | [SOL-TG-03-task-access-control.md](../../../../specs/backend-go/bugs/logic-v1/solutions/SOL-TG-03-task-access-control.md) (427 dòng) |
| **Tác động** | `backend-go/services/task-service/internal/domain/grant.go`, `grant_resolution.go`, `internal/adapter/grpcclient/team_scope_resolver.go`, `internal/usecase/resolve_permission.go`, `internal/adapter/grpc/server.go`, `proto/orca/task/v1/task.proto` |

---

## 1. Vấn đề

Phần lõi resolve-grant là **thật và có test** — `domain.ResolveGrant`
(`internal/domain/grant_resolution.go:37-67`) là BFS ancestor-walk với
`apply_tree` inheritance thật, `ResolvePermission` usecase
(`internal/usecase/resolve_permission.go:51-91`) wire đúng vào OPA policy
check, fail-closed. Nhưng:

1. **`StubTeamScopeResolver.ResolveTeams` trả `nil, nil` cứng**
   (`internal/adapter/grpcclient/team_scope_resolver.go:11-26`), và được wire
   **thẳng vào production** (`cmd/server/main.go:86-89`, tự comment
   *"team-scope resolution ... still STUB"*). Hệ quả: **mọi grant
   `scope=team` không bao giờ match** — tính năng "share với cả team" trong
   spec (dòng 225-261) chết hoàn toàn dù code resolve grant phía trên đúng.
2. **`GrantLevel` là enum sai trục** — `domain/grant.go:11-19` định nghĩa
   `Owner/Admin/User/Team/Company` (đây là **loại đối tượng được cấp quyền**),
   trong khi spec yêu cầu 1 thang **hành động** thứ tự `view &lt; comment &lt; edit &lt;
   execute &lt; manage`. `Grant.Matches` (`grant.go:83-96`) xác nhận nhầm lẫn
   này — cần tách 2 khái niệm (`GranteeKind` vs `PermissionLevel`).
3. **Không có expiry** — không cột `expires_at` ở đâu (`grant.go:54-59`,
   `usecase/grant.go:11-16`, `postgres/grants.go:26-39`).
4. **Không có `Revoke`** — 0 symbol khớp, chỉ có ghi chú TODO trong README.
5. **Không có public share-link** — 0 kết quả `public_link`/`share_token`
   toàn service.
6. **Không có grant-received notification** — tạo grant mới không bắn event
   nào cho người được cấp quyền biết.
7. **`ResolvePermissionRequest` không có field `action`** — server hard-code
   `Action: "read"` cho MỌI call (`internal/adapter/grpc/server.go:117-132`),
   nghĩa là nhánh OPA check cho write/execute/manage-level **chỉ chạy trong
   unit test**, chưa từng được exercise qua RPC thật.
8. Không có `OwnerID` trên `Task` để làm short-circuit "owner luôn có full
   quyền" — cột này do CR-TG-001 bổ sung, CR này tiêu thụ.

## 2. Giải pháp đề xuất (theo SOL-TG-03)

### 2.1 `TeamScopeResolver` thật

```go
// internal/adapter/grpcclient/team_scope_resolver.go
type TeamScopeResolver struct {
    tenant tenantv1.TenantServiceClient // hoặc team-membership source thật đã có ở tenant-service
}

func (r *TeamScopeResolver) ResolveTeams(ctx context.Context, userID string) ([]string, error) {
    resp, err := r.tenant.ListUserTeams(ctx, &tenantv1.ListUserTeamsRequest{UserId: userID})
    if err != nil { return nil, fmt.Errorf("team_scope_resolver: %w", err) }
    return resp.GetTeamIds(), nil
}
```

Thay `StubTeamScopeResolver` ở `cmd/server/main.go:86-89` bằng implementation
thật gọi `tenant-service` (RPC `ListUserTeams` đã tồn tại hay cần thêm tuỳ vào
audit `tenant-service` hiện có — xác nhận trước khi code).

### 2.2 Tách `GranteeKind` (ai được cấp) khỏi `PermissionLevel` (được làm gì)

```go
// domain/grant.go
type GranteeKind int      // User | Team | Company
type PermissionLevel int  // View | Comment | Edit | Execute | Manage — ordered

type Grant struct {
    GranteeKind  GranteeKind
    GranteeID    string
    Permission   PermissionLevel
    ApplyToTree  bool
    ExpiresAt    *time.Time
    GrantedBy    string
    GrantedAt    time.Time
}
```

`ResolveGrant`'s BFS giữ nguyên logic ancestor-walk, chỉ đổi shape field —
không viết lại thuật toán.

### 2.3 Expiry filter + owner short-circuit

```go
// grant_resolution.go
func ResolveGrant(task Task, grants []Grant, userID string, teamIDs []string, now time.Time) *PermissionLevel {
    if task.OwnerID == userID {
        manage := PermissionManage
        return &manage // owner luôn có full quyền, bỏ qua toàn bộ grant list
    }
    active := filterExpired(grants, now) // bỏ grant đã expires_at < now
    // ...BFS ancestor-walk như cũ, trên `active`...
}
```

### 2.4 `RevokeGrant` / `ListGrants` — RPC public mới

```protobuf
rpc RevokeGrant(RevokeGrantRequest) returns (google.protobuf.Empty);
rpc ListGrants(ListGrantsRequest) returns (ListGrantsResponse); // public-facing, khác ListGrantsForAncestors nội bộ
```

### 2.5 Public share-link

```protobuf
message Task {
  // ...
  string share_token = 20; // random, đủ entropy — set khi bật share-link
}
rpc GenerateShareLink(GenerateShareLinkRequest) returns (GenerateShareLinkResponse);
rpc GetTaskByShareToken(GetTaskByShareTokenRequest) returns (GetTaskByShareTokenResponse); // view-only, không cần login
```

`GetTaskByShareToken` trả permission cố định `View`, không chạy qua
`ResolvePermission` thông thường (đây là public/anonymous access theo thiết
kế, không phải lỗi bảo mật — nhưng CHỈ trả field an toàn để hiển thị, không
trả `aiContext`/comment nội bộ).

### 2.6 `action` wire field + notification

```protobuf
message ResolvePermissionRequest {
  string task_id = 1;
  string user_id = 2;
  string action = 3; // MỚI — "read"|"write"|"execute"|"manage", server không còn hard-code "read"
}
```

Tạo `Grant` mới → publish event qua outbox (tái sử dụng cơ chế đã có ở
`SOL-PW-04`, không tự chế event bus riêng) để `notification-service` báo cho
grantee.

## 3. Rủi ro / Không thuộc phạm vi

- Đổi `GrantLevel` là **breaking schema change** cho data đã tồn tại — cần
  migration data (map `Owner→Manage`, `Admin→Manage`, `User→Edit`,
  `Team→Edit`, `Company→View` là gợi ý mặc định, cần team xác nhận mapping
  thật trước khi chạy migration, không tự quyết định 1 chiều trong CR).
- `GetTaskByShareToken` là access-control-relevant — implementation PHẢI đi
  qua security review trước khi merge (không tự ý coi "share-link" là tính
  năng ít rủi ro).
- Không tự thiết kế lại toàn bộ notification-service — chỉ publish 1 event
  loại mới vào outbox đã có.
- Không thuộc phạm vi: UI cho revoke/share-link — đó là CR-TG-007.

## Acceptance Criteria

- [ ] `TeamScopeResolver` thật thay `StubTeamScopeResolver` trong
      `cmd/server/main.go`; test: grant `scope=team` với user thuộc team đó
      match đúng.
- [ ] `PermissionLevel` là thang thứ tự `view&lt;comment&lt;edit&lt;execute&lt;manage`,
      tách khỏi `GranteeKind`; migration mapping dữ liệu cũ có review riêng.
- [ ] Grant đã `expires_at &lt; now` không match trong `ResolveGrant`.
- [ ] Owner (`task.OwnerID == userID`) luôn resolve `Manage` bất kể grant list.
- [ ] `RevokeGrant`/`ListGrants` hoạt động, `RevokeGrant` idempotent (revoke 2
      lần không lỗi).
- [ ] `GenerateShareLink`/`GetTaskByShareToken` hoạt động, response của
      `GetTaskByShareToken` KHÔNG chứa `aiContext`/comment nội bộ — có test
      xác nhận field bị lọc.
- [ ] `ResolvePermissionRequest.action` được server đọc thật (không còn
      hard-code `"read"`); test: RPC gọi với `action="manage"` bị OPA từ chối
      đúng khi user chỉ có `Edit`.
- [ ] Tạo grant mới → notification xuất hiện ở outbox table, đúng
      `grantee_id`.
