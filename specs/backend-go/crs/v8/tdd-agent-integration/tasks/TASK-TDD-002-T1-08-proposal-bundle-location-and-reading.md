# TASK-TDD-002-T1-08: Chốt nơi lưu và cách agent task đọc gói OpenSpec

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4.6, 3.6 (Q1)
**Checklist:** [T1.8](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** spike + thiết kế
**Files:** `task-service` (cách tạo worktree task), `request-service/internal/adapter/grpcclient/proposal_workspace.go`, `evidence/T1.8-bundle-location.md` (mới)
**Depends on:** T1.5
**Status:** `[ ]` TODO

---

## Context

CR-REQ-026 đặt gói trên nhánh `request/<số>-proposal` (cục bộ, chặn push). Agent task chạy trong worktree riêng của task, và cần đọc `tasks.md`/`spec.md`. Chưa rõ worktree task được tạo từ nhánh nào (`base_ref`): nếu từ nhánh mặc định thì sẽ **không** thấy gói.

## Việc cần làm

1. Đọc code `task-service` nơi tạo worktree cho task (`CreateWorktree` với `base_ref`/`branch`): xác định `base_ref` mặc định và có tham số nào để chọn nhánh proposal không; ghi đường dẫn file và hàm.
2. Thử trên repo thử: tạo nhánh `request/9001-proposal`, commit gói mẫu (từ T1.5) vào đó; tạo worktree task theo đúng cách `task-service` làm; kiểm `openspec/changes/<id>/tasks.md` có hiện trong worktree không.
3. Nếu không hiện: thử (1) `git checkout request/9001-proposal -- openspec/changes/<id>` (hoặc `git restore --source`) làm bước chuẩn bị của task; (2) đưa nội dung spec của task vào `prompt_template` (giới hạn kích thước); chọn phương án và ghi lý do.
4. Xác nhận agent task đọc được spec bằng một lượt `claude --print` trong worktree task thử.
5. Ghi quyết định, lệnh chính xác và việc phải làm ở T2.6/T2.11 vào `evidence/T1.8-bundle-location.md`.

## Tiêu chí hoàn thành

- [ ] Có quyết định cụ thể (nhánh nền hoặc bước chuẩn bị hoặc nhúng nội dung) và lệnh chạy được trên repo thử.
- [ ] Agent task trong worktree thử đọc được `spec.md` của task.

## Kiểm thử

- Kiểm thủ công với repo thử; không đụng repo pilot.

## Rủi ro và ghi chú

- Cẩn thận không để worktree task thử còn sót (`git worktree remove`).
