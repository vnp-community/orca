# BE-CV-TASK-012-01: Re-verify hành vi `project-service`, `infra-fleet-service`, `git-gateway-service` mà phân giải dựa vào

**From Solution:** BE-CV-SOL-012-target-resolution-and-bindings
**Priority:** P0
**Service:** (chỉ đọc) `project-service`, `infra-fleet-service`, `git-gateway-service`, `api-gateway`
**File:** không sửa file; kết quả ghi vào PR
**Depends on:** không
**Status:** [x] DONE

---

## Context

SOL-012 mục 1 liệt kê các điểm chưa kiểm chứng: tiền tố `repo:`; `DetectWorktrees` kiểm user hay chỉ tenant; `IsMainWorktree` = `Repo.url`; `ListWorktrees`/`ListRepos` không phân trang; `ListMembers` quy mô; `RelayByDevServer` xếp hàng chờ kết nối; dự án chia sẻ.

## Việc cần làm

1. `grep -rn '"repo:"' backend-go frontend/src/shared` để xác nhận nơi sinh/đọc tiền tố `repo:`; ghi kết quả (nếu không có, giữ chấp nhận theo PQ-04 nhưng đánh dấu không bắt buộc).
2. Đọc `git-gateway-service/internal/adapter/grpcclient/{tenant_forwarding.go,project_client.go}` và usecase `DetectWorktrees`: có `RequireTenantID` không, có kiểm user/membership không (quyết định có cần gọi sau `GetProject` hay không).
3. Đọc `project-service` `ListRepos`/`ListWorktrees`/`ListMembers` (usecase + repository): xác nhận lọc `project_id` + tenant, không phân trang, thứ tự trả về.
4. Đọc `infra-fleet-service` `list_dev_servers.go`: tenant filter; `DevServer.platform`, `kind`, `approval_status`, `mode` có trong response.
5. Đọc `devserveragent` `client.go` đường `IsConnected`/chờ nối lại để xác nhận `INFRA_DEV_SERVER_NOT_CONNECTED` trả ngay.
6. Ghi bảng "đã kiểm / lệch" và cập nhật SOL-012 mục 1 nếu lệch.

## Kiểm thử

- Chỉ đọc (`grep`, `Read`); không chạy test hay build.

## Tiêu chí hoàn thành

- [x] Mỗi điểm chưa kiểm chứng ở SOL-012 mục 6 có kết luận hoặc ghi rõ còn chưa kiểm chứng.

## Rủi ro và lưu ý

- Nếu `DetectWorktrees` không kiểm membership, **bắt buộc** gọi sau `GetProject` (đã là thứ tự mặc định).
