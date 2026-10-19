# TASK-V2-034: Cấu hình triển khai, quan sát, tài liệu và rollout `warn` → `enforce` / `legacy` → `strict`

**From Solution:** SOL-V2-002 (mục 3.7, 6) · **Resolves:** "metric/log, tài liệu, rollback" của TASK-FE-E2E-015
**Priority:** P2 · **Service/Area:** deploy/dev, git-gateway-service, project-service, specs
**Files:**
- deploy/dev/docker-compose.yml (`x-go-common-env`: `ORCA_LOCAL_REPO_ROOT`; khối `project-service` và `git-gateway-service`: `PROJECT_REPO_HOST_POLICY`, `PROJECT_ADD_REPO_VERIFY_HOST`, `GIT_GATEWAY_HOST_DISPATCH_MODE`; cập nhật comment trước `git-gateway-service` nói về `/data/repos`), deploy/dev/README.md
- specs/backend-go/tdd/services/git-gateway-service.md, project-service.md (mã lỗi mới, chính sách host), specs/agent/api/agent-rpc-catalog-git-fs.md
- (tuỳ chọn) bộ đếm Prometheus theo mẫu `request-service/internal/adapter/metrics/request_metrics.go`, mount `/metrics` ở `common/health` (hiện git-gateway và project-service chỉ có health: `cmd/server/main.go:303`, `97`)
**Depends on:** TASK-V2-028, 029
**Status:** `[~]` MỘT PHẦN (2026-10-11): cờ cấu hình + công cụ + tài liệu có; chưa có metric/log `repo_host_dispatch`.

---

## Việc cần làm

- [ ] Cấu hình mặc định an toàn: mã mặc định `enforce`/`strict`; compose dev đặt chế độ chuyển tiếp ban đầu (`warn`/`legacy`) có ghi chú ngày bật chính thức.
- [ ] Tài liệu mã lỗi và quy tắc "host hiệu lực" ở hai spec service; ghi cách lùi (SOL mục 6.4).
- [ ] Log có cấu trúc đã đủ cho quan sát; bộ đếm Prometheus là tuỳ chọn (`orca_git_gateway_host_dispatch_total{target,outcome}`, nhãn không chứa tenant/đường dẫn).
- [ ] Danh sách kiểm rollout: (1) 030 bước 1, (2) frontend 016-018, (3) 028 `warn`, (4) báo cáo 031, (5) 029 `strict`, (6) 028 `enforce`, ghi ngày và kết quả từng bước vào README tasks.

## Tiêu chí hoàn thành

Cấu hình, tài liệu và checklist rollout có mặt; `docker compose config` hợp lệ (chạy không cần khởi động stack).

## Rủi ro · Ước lượng

LOW. 0.5 đến 1 ngày.
