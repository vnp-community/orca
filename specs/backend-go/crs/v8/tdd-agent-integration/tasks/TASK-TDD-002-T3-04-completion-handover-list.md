# TASK-TDD-002-T3-04: Bàn giao khi Request hoàn tất: liệt kê việc còn lại cho người

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.5
**Checklist:** [T3.4](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** request-service + orca-cli
**Files:** `request-service` (view Request), `api-gateway`, `orca-cli` (`request show`), test đi kèm
**Depends on:** T3.2, T3.3, T2.12
**Status:** `[ ]` TODO

---

## Context

Sau `completed`, phần việc còn lại thuộc về người (đề xuất, chờ xác nhận): hợp nhất code của các nhánh task, xem nhánh proposal, chạy `tdd-agent generate` nếu tài liệu đã đổi, quyết định áp các đề xuất ChangeSet. Orca không tự làm các việc này (không merge, không đụng KG).

## Việc cần làm

1. Tính danh sách bàn giao khi đọc Request `completed`: các nhánh/worktree task chưa hợp nhất (từ git-gateway), nhánh proposal và trạng thái archive, tài liệu đã đổi (từ `writeback`), đề xuất ChangeSet chưa áp, xung đột Decision/Constraint chưa được xử lý, trạng thái đồng bộ `tasks.md`.
2. Sinh các lệnh gợi ý (không chạy): `git merge …`, `tdd-agent -C <repo> generate`, `tdd-agent kg apply <id> …` (nếu có T3.1).
3. `orca request show` in danh sách; `--json` cho script.
4. Thông báo trong CLI khi Request `completed` mà còn mục chưa xử lý.

## Tiêu chí hoàn thành

- [ ] Một Request `completed` có bàn giao đúng và đầy đủ; Request không có gì còn lại thì ghi rõ.
- [ ] Không có hành động tự động nào.

## Kiểm thử

- Test tính toán danh sách với dữ liệu giả; test render CLI.

## Rủi ro và ghi chú

- Danh sách này là gợi ý, không phải gate.
