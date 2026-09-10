# TASK-EMU-008: Thêm `mobileEmulatorAgentId` vào project

**Solution:** [SOL-EMU-006](../solutions/SOL-EMU-006-project-binding-and-routing.md)
**Priority:** P2
**Depends on:** TASK-EMU-007
**Status:** `[x]` DONE

## Việc đã làm

1. `backend-go/proto/orca/project/v1/project.proto`: thêm
   `string mobile_emulator_agent_id = 11;` trên `Project` (field 10 cuối
   cùng trước đó là `updated_at`, xác nhận trước khi thêm) và
   `string mobile_emulator_agent_id = 6;` trên `UpdateProjectRequest`
   (rỗng = không đổi, cùng convention với `name`/`description`/…).
2. **Quyết định thiết kế khác với `devServerId`:** `devServerId` chỉ đổi
   được qua `RebindDevServer` (có active-execution guard riêng — xem
   `project-service.md` §3). `mobileEmulatorAgentId` KHÔNG đi qua guard đó —
   CR-DS-009 §3.2 nói rõ hai binding này "độc lập", không có active-execution
   concern nào gắn với việc đổi máy chạy emulator. Nên
   `mobileEmulatorAgentId` được thêm thẳng vào `UpdateProjectRequest`/
   `UpdateProject` usecase như một field cập nhật thông thường (cùng nhóm
   với `name`/`description`/`defaultBranch`/`visibility`), không có RPC
   riêng kiểu `RebindDevServer`.
3. `domain.Project`: thêm field `MobileEmulatorAgentID string`.
   `domain.ProjectUpdatePatch`: thêm field cùng tên (khác với
   `DevServerID` — cố tình không có trong patch, xem điểm 2).
4. `usecase.UpdateProjectInput`/`UpdateProject.Execute`: nối field qua patch.
5. `internal/adapter/postgres/repository.go`: cột `mobile_emulator_agent_id`
   (UUID nullable, không FK thật — cùng convention `dev_server_id`, logical
   FK sang infra-fleet-service) thêm vào `projectColumns`, `scanProject`, và
   `UpdateProject`'s SQL (`COALESCE(NULLIF($7,'')::uuid, mobile_emulator_agent_id)`
   — cast `::uuid` tường minh trên riêng `NULLIF` trước khi `COALESCE`, cùng
   pattern `infra-fleet-service`'s `AssignGroup` dùng cho cột UUID nullable
   khác, tránh nhập nhằng type-resolution của Postgres khi trộn
   `COALESCE(text, uuid_column)`).
6. Migration `backend-go/services/project-service/migrations/0015_project_mobile_emulator_agent.{up,down}.sql`.
7. `internal/adapter/grpc/server.go`: `toProtoProject` trả
   `MobileEmulatorAgentId`; `UpdateProject` handler nối
   `req.GetMobileEmulatorAgentId()`.
8. `internal/usecase/fakes_test.go`'s `fakeProjectRepository.UpdateProject`
   cập nhật để áp field mới (test double, không phải test case).
9. Ngoài phạm vi mô tả gốc (nhưng cần để field thực sự "dùng được" từ
   frontend, đúng tinh thần "UI Settings cho phép chọn" của task): nối
   `mobileEmulatorAgentId` vào `api-gateway`'s wscompat —
   `channels_tenant_project.go`'s `projectView`/`toProjectView`/
   `project.update` channel args. Đây là điều kiện tiên quyết để
   TASK-EMU-009's `resolveEmulatorConnectionID` có field thật để đọc
   (`GetProject().GetMobileEmulatorAgentId()`).

## Verify (chạy thật trong pass này, có bằng chứng)

```
$ cd backend-go/proto && buf generate && go build ./...
(exit 0, no output)

$ for svc in api-gateway git-gateway-service project-service; do
    (cd backend-go/services/$svc && go build ./... && go vet ./... && go test ./...)
  done
```
Kết quả: build/vet sạch cho cả 3 service (những service import
`projectv1` — `grep -rl projectv1 backend-go/services --include=*.go`).
`go test ./...`:
- `project-service`: `domain` ok, `usecase` ok (bao gồm
  `TestUpdateProject_*` hiện có, không sửa nội dung, vẫn xanh).
- `git-gateway-service`: mọi package `ok` (field mới không đụng gì nó dùng).
- `api-gateway`: mọi package `ok`, bao gồm `internal/adapter/wscompat`
  (24.5s, có test mới của TASK-EMU-009).

## Chưa làm (ngoài phạm vi backend-go này)

UI Settings thật (chọn 1 `DevServer` có `kind=AGENT_KIND_MOBILE_EMULATOR`
làm giá trị `mobileEmulatorAgentId`) — Phase 5 của CR-DS-009, chưa bắt đầu.
Field đã có đường dây đầy đủ (proto → domain → SQL → gRPC → wscompat
`project.update`/`project.get`), chỉ thiếu UI component gọi nó.
