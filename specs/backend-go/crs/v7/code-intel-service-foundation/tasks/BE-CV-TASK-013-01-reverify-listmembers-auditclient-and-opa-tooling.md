# BE-CV-TASK-013-01: Re-verify `ListMembers`, `x-orca-role`, `auditclient`, công cụ `opa`

**From Solution:** BE-CV-SOL-013-authorization-flags-and-audit
**Priority:** P0
**Service:** (chỉ đọc) `project-service`, `auth-service`, `api-gateway`, `common`
**File:** không sửa file; kết quả ghi vào PR
**Depends on:** không
**Status:** [x] DONE

---

## Context

SOL-013-authorization mục 1 và 6 liệt kê điểm chưa kiểm chứng ảnh hưởng thiết kế `ProjectAuthorizer`: quyền của `ListMembers`; `x-orca-role` có đủ ở mọi đường gateway; giới hạn `audit_log.target`; `opa` CLI cho `make opa-test`.

## Việc cần làm

1. Đọc `project-service/internal/usecase/list_members.go`: gọi `requireProjectAccess` với action nào (`any_member` hay `owner_only`)? Nếu `owner_only`, `member` không gọi được → phải thay bằng cách khác (đề nghị RPC "vai trò của tôi" hoặc suy từ `GetProject`); ghi quyết định.
2. Đọc `api-gateway` nơi `AttachIdentity` (`internal/adapter/grpc/dial.go`) và các đường auth (cookie/session, JWT, MCP): xác nhận `x-orca-role` có ở đường WS `/ws` của UI.
3. Đọc `auth-service` `append_audit_entry.go`/`domain/audit.go`: giới hạn độ dài `action`, `target`, `outcome` hợp lệ (`allowed|denied`), `actor_type` mặc định.
4. Kiểm `Makefile` `opa-test` và CI workflow có `opa` CLI không (`grep -rn "opa" .github/workflows`).
5. Đọc `task-service/internal/adapter/opaclient` để lấy mẫu cổng OPA.
6. Ghi bảng "đã kiểm/lệch"; cập nhật SOL-013-authorization mục 1/6 nếu lệch.

## Kiểm thử

- Chỉ đọc; không chạy test/build.

## Tiêu chí hoàn thành

- [x] Kết luận cho bốn điểm (ListMembers, role, audit, opa) hoặc ghi "chưa kiểm chứng".

## Rủi ro và lưu ý

- Nếu `ListMembers` bị hạn chế owner, task 013-04 đổi thiết kế (không thể bắt đầu trước).
