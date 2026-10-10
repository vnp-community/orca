# TASK-V2-031: Công cụ quản trị phát hiện repo không host và gắn lại (dry-run mặc định); cải thiện `RebindRepoDevServer`

**From Solution:** SOL-V2-002 (mục 4, 5) · **Resolves:** TASK-FE-E2E-015 việc 3 (dữ liệu cũ, ví dụ `codex`)
**Priority:** P1 · **Service/Area:** tooling (tests/stages/e2e/orca-flows) + project-service
**Files:**
- tests/stages/e2e/orca-flows/report_unbound_repos.py (mới, theo mẫu `report_orphan_requests.py`: `OrcaSession`, `load_config`, `ws.call`)
- backend-go/services/project-service/internal/usecase/rebind_repo_dev_server.go (62-97: từ chối tháo host cho repo ngoài gốc local; khi `PROJECT_ADD_REPO_VERIFY_HOST` bật thì kiểm host mới tới được và đường dẫn có thật)
- backend-go/services/project-service/cmd/server/main.go (dòng log khởi động đếm repo không host)
**Depends on:** TASK-V2-028 (biến cấu hình) cho phần Go; phần công cụ độc lập
**Status:** `[x]` XONG qua TASK-V2-035 (2026-10-11), đã kiểm live; các mục dự kiến bị đơn giản hoá theo quyết định của người dùng (không có chế độ warn/legacy) ghi ở TASK-V2-035.
**Triển khai thực tế (2026-10-10, chưa commit, chưa redeploy):** [TASK-V2-035](./TASK-V2-035-repo-host-binding-and-no-silent-local.md) — công cụ `deploy/dev/scripts/rebind-unbound-repos.sh` (dry-run mặc định); phần cải thiện `RebindRepoDevServer` chỉ gồm chặn tháo host

---

## Việc cần làm

- [ ] `report_unbound_repos.py`: qua gateway `project.list` rồi `repo.list {projectId}`; liệt kê repo `devServerId == ""` (cột: project, repo id, url, host của project, dưới gốc local hay không). Mặc định CHỈ ĐỌC. Không cần SQL.
- [ ] `--apply` bắt buộc kèm ánh xạ tường minh (`--repo <id>=<tên host>` hoặc `--host <tên> --repo <id>...`); in kế hoạch trước; từ chối host `disconnected`; một lần `repo.rebindDevServer` mỗi repo; dừng ở lỗi đầu tiên có thể phục hồi; in kết quả từng repo. KHÔNG bao giờ suy host; gợi ý (không tự áp) từ host của project nếu đang kết nối.
- [ ] Xác minh thư mục trên host mới bằng `devServer.browseDir` thư mục cha (có `--no-verify`); cần TASK-V2-030 cho thư mục cha lớn.
- [ ] `RebindRepoDevServer`: các cải thiện của SOL mục 5 sau cờ, mặc định hành vi cũ.
- [ ] Dòng log khởi động project-service: số repo `dev_server_id IS NULL` không dưới gốc local (một truy vấn, Postgres `project.repos`, MySQL `repos`).

## Tiêu chí hoàn thành

- Chạy dry-run trên stack hiện có liệt kê đúng các repo không host (kể cả `codex`) mà KHÔNG thay đổi gì; `--apply` chỉ chạy khi người dùng cho phép và chỉ trên repo họ chỉ định (không bao giờ tự chạy trong e2e).
- Test Go cho các nhánh rebind mới; test Python cho phân tích đối số và từ chối khi thiếu ánh xạ.

## Kiểm thử

Go: `rebind_repo_dev_server_test.go` (mở rộng 11 test); tích hợp hai dialect không thay đổi lược đồ. Python: test đơn vị đối số (`--apply` không ánh xạ thì thoát mã lỗi, không gọi RPC).

## Rủi ro · Ước lượng

LOW (đọc là chính; ghi chỉ khi người dùng xác nhận). 1 ngày.
