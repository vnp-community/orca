# TASK-TDD-002-T4-05: Kiểm mã đã sinh và phạm vi thay đổi

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 5 (tiêu chí 3, 6)
**Checklist:** [T4.5](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** pilot (vnp-qa-platform)
**Files:** các worktree task, nhánh proposal, thư mục chính
**Depends on:** T4.4
**Status:** `[ ]` TODO

---

## Context

Kiểm chứng độc lập (không dựa vào lời của agent) rằng kết quả đúng và không vượt phạm vi.

## Việc cần làm

1. Với mỗi worktree task: `git diff --stat <nền>..HEAD`; chạy test của dự án cho phần đã đổi (lệnh `Verify` của task và `go test` của package tương ứng); ghi kết quả.
2. Kiểm không có thay đổi ngoài phạm vi: đặc biệt `services/infrastructure/integration-hub/` (thư mục bảo vệ), `openspec/`, `docs/tdd/`, `.tdd-agent/`.
3. Kiểm luật dự án: `gitnexus detect_changes` trên các thay đổi (nếu có công cụ), không sửa symbol rủi ro cao mà không báo.
4. `pilot_guard.py compare`: thư mục chính không đổi; chỉ có worktree/nhánh do run tạo.
5. `git push --dry-run` vẫn thất bại; không có `refs/remotes` mới.
6. `report_orphan_requests.py`, `report_orphan_worktrees.py`: 0 mồ côi.
7. Ghi báo cáo kèm diff tóm tắt.

## Tiêu chí hoàn thành

- [ ] Test của dự án cho phần đã làm xanh trong từng worktree task.
- [ ] Không đổi ngoài phạm vi; không push; thư mục chính nguyên vẹn.

## Kiểm thử

- Chính các lệnh trên.

## Rủi ro và ghi chú

- Nếu test đỏ: không sửa tay trong worktree task rồi tuyên bố xanh; ghi là lỗi của luồng.
