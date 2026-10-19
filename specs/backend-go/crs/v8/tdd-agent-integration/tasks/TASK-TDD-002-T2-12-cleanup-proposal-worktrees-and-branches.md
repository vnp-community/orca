# TASK-TDD-002-T2-12: Dọn worktree và nhánh proposal

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 2, 7)
**Checklist:** [T2.12](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** request-service + script e2e
**Files:** `request-service/internal/usecase/` (consumer sự kiện Request kết thúc, sweeper), `internal/adapter/eventbus/openspec_consumers.go`, `tests/stages/e2e/orca-flows/report_orphan_worktrees.py` (mới)
**Depends on:** T2.3, T2.6
**Status:** `[ ]` TODO

---

## Context

Mỗi run `openspec` tạo worktree và nhánh `request/<số>-proposal`. CR-REQ-026: `cancelled` ⇒ trạng thái gói `abandoned`, giữ nhánh; không tự merge hay mở PR. Run thất bại giữ worktree để điều tra (T2.4), nên cần thời hạn dọn. `report_orphan_requests.py` chỉ kiểm Request, không kiểm worktree.

## Việc cần làm

1. Consumer sự kiện Request `cancelled`/`completed`: gỡ worktree (`Remove`), giữ nhánh; cập nhật trạng thái gói (`abandoned` hoặc để T3.3 xử lý archive).
2. Sweeper: worktree của run `failed` giữ `proposal_retention` (cấu hình, mặc định 7 ngày) rồi gỡ; không bao giờ gỡ worktree của run đang `running`.
3. Lệnh/RPC quản trị liệt kê và gỡ tay (`orca` CLI: `orca request cleanup <id>`), chỉ cho chủ Request/admin.
4. Script `report_orphan_worktrees.py`: liệt kê `git worktree list` và nhánh `request/*-proposal` của repo cấu hình, đối chiếu với Request còn hoạt động; in phần mồ côi và thoát khác 0 nếu có (dùng ở T4.8, T5.1).

## Tiêu chí hoàn thành

- [ ] Sau khi Request kết thúc không còn worktree proposal nào ngoài nhánh được giữ.
- [ ] Run lỗi bị gỡ sau thời hạn; run đang chạy không bị đụng.
- [ ] Script báo 0 mồ côi sau một chu kỳ e2e sạch.

## Kiểm thử

- Test consumer và sweeper với đồng hồ giả và workspace giả; test script trên repo tạm có worktree mồ côi.

## Rủi ro và ghi chú

- Xoá worktree là thao tác phá huỷ: chỉ gỡ đúng worktree do chính run này tạo (kiểm tên nhánh và `workspace_id`).
