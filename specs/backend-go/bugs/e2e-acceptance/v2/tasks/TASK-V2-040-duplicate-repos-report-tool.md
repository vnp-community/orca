# TASK-V2-040: Công cụ báo cáo repo trùng hiện có (dry-run, chỉ liệt kê)

**From:** [SOL-V2-003](../solutions/SOL-V2-003-project-own-project-and-repo-dedupe.md) mục 2.8 · **Priority:** P2 · **Service/Area:** deploy/dev
**Files (dự kiến):** `deploy/dev/scripts/report-duplicate-repos.sh`, `deploy/dev/scripts/report_duplicate_repos.py`, test `deploy/dev/scripts/test_report_duplicate_repos.py` (chuẩn hoá và gom nhóm, không gọi mạng)
**Depends on:** TASK-V2-037 (cùng quy tắc chuẩn hoá) · **Status:** `[x]` XONG (2026-10-11): mã, test (đơn vị, hợp đồng, tích hợp hai dialect, đua) và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

## Việc cần làm

- [x] Theo khuôn `rebind-unbound-repos.sh`/`rebind_unbound_repos.py` (cùng cách đăng nhập/kênh WS, cùng cờ `--url`, `--token` qua môi trường, không in bí mật).
- [x] Liệt kê project (`project.list`) và repo (`repo.list`), gom theo (`devServerId`, đường dẫn chuẩn hoá y hệt `domain.NormalizeRepoLocation`), in mỗi nhóm có từ hai repo trở lên: dev server, đường dẫn, từng (project id, tên, repo id, vị trí).
- [x] Chỉ ĐỌC: không có cờ ghi/xoá/gắn lại; thoát mã 0 khi không có trùng, mã 1 khi có (để dùng trong CI), `--json` tuỳ chọn.
- [x] Test chuẩn hoá cùng bảng ca với Go (POSIX, Windows, URL, hoa/thường) và gom nhóm.

## Kết quả thực thi

- `deploy/dev/scripts/report-duplicate-repos.sh` + `report_duplicate_repos.py`: chỉ gọi `project.list` và `repo.list` (không có cờ ghi/xoá/gắn lại); in nhóm có từ hai repo trở lên theo (dev server không phân biệt hoa/thường, đường dẫn chuẩn hoá); `--json`; thoát 1 khi có trùng, 0 khi không, 2 khi đăng nhập hỏng. Chỉ thấy project mà người dùng đăng nhập là hội viên (ghi trong `--help`).
- Test offline `test_report_duplicate_repos.py`: `python3 -m unittest test_report_duplicate_repos` → 4 test OK (20 ca chuẩn hoá giống bản Go + gom nhóm).
- Việc cho người điều phối: chạy `deploy/dev/scripts/report-duplicate-repos.sh` để biết số bản trùng sẵn có (kỳ vọng thấy repo `codex`).
