# TASK-V2-033: E2E: spec `--repro add-project-worktree` xanh + kịch bản host ngắt giữa chừng, kênh tắt không `devServerId`, duyệt thư mục lớn

**From Solution:** SOL-V2-002 (mục 6.3) · **Resolves:** tiêu chí cuối của TASK-FE-E2E-015
**Priority:** P1 · **Service/Area:** tests/stages/e2e
**Files:**
- tests/stages/e2e/orca-flows/ui-live/add-project-worktree-repro.web.e2e.ts, live-add-project-repro-steps.ts, run_ui_flow.py (11, 37-38, 61-75)
- tests/stages/e2e/orca-flows/ui-live/add-project-host-down.web.e2e.ts (mới), suite Python mới `suite_repo_host_binding.py` (đăng ký trong `run_acceptance.py`)
- backend-go/services/request-service/e2e (mẫu) hoặc `services/git-gateway-service/internal/usecase` (kịch bản có agent giả)
**Depends on:** TASK-FE-E2E-016 đến 018, TASK-V2-028, 029, 030
**Status:** `[~]` MỘT PHẦN (2026-10-11): spec `--repro add-project-worktree` xanh; kịch bản dev server ngắt giữa chừng và repo thêm qua kênh tắt chưa viết e2e.

---

## Việc cần làm

- [ ] `run_ui_flow.py --repro add-project-worktree` XANH sau khi sửa (host `dev-local-mac`, repo tạm `/private/tmp/e2e-ui-repro-*`, tự dọn). Giữ `BUG_PATTERN` làm phát hiện hồi quy.
- [ ] Kịch bản A (Python/WS, project và repo tạm): `repo.add` KHÔNG `devServerId` vào project KHÔNG host phải trả `PROJECT_REPO_HOST_REQUIRED`; vào project CÓ host phải lưu host của project; có `devServerId` thì lưu đúng.
- [ ] Kịch bản B: host ngắt khi đang tạo worktree thì UI hiện lỗi `host offline` (không `chdir`), có Retry; nối lại rồi Retry thành công. Cách ngắt: chạy agent phụ do chính kịch bản khởi động trên cổng riêng (một dev server tạm), ngắt NÓ, không bao giờ `dev-local-mac` của người dùng; nếu môi trường không cho, chỉ chạy bản có agent giả ở lớp Go.
- [ ] Kịch bản C: duyệt thư mục tạm có >2000 mục (tạo trong `/tmp` của kịch bản) trả `truncated`, agent giữ `connected`.
- [ ] Dọn dữ liệu: `report_orphan_requests.py` và `report_unbound_repos.py` báo 0 mục còn lại; không đụng repo `codex`, `probe-proj`, `Demo (this Mac)`.
- [ ] Không hồi quy: `run_ui_flow.py --real all` 6/6 và `run_acceptance.py` (103 PASS như README v2).

## Tiêu chí hoàn thành

Cả ba kịch bản và spec repro xanh; báo cáo ghi thời gian, 0 vi phạm an toàn, 0 mục mồ côi.

## Rủi ro · Ước lượng

MEDIUM (ngắt dev server phụ cần cẩn thận để không ảnh hưởng stack). 1.5 ngày.
