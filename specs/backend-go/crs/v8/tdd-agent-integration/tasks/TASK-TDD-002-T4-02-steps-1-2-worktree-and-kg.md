# TASK-TDD-002-T4-02: Bước 1 và 2: vào worktree TMS, dựng và kiểm KG

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 4 (bước 1, 2)
**Checklist:** [T4.2](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** pilot (vnp-qa-platform)
**Files:** worktree `vnp-qa-platform/.claude/worktrees/tdd-agent-bl-h01-h02`
**Depends on:** T0.3, T4.1
**Status:** `[ ]` TODO

---

## Context

Bước 1 và 2 của runbook người dùng: vào worktree của TMS, chạy pipeline tdd-agent để khởi tạo knowledge và sinh TDD. KG đã được dựng một lần (915 phần tử, 1544 cạnh, 246 gap); task này xác nhận trạng thái ngay trước khi chạy Orca, và dựng lại nếu tài liệu đổi.

## Việc cần làm

1. `cd` vào worktree; `git status -sb` sạch, nhánh `feat/tdd-agent-bl-h01-h02`; ghi `git rev-parse HEAD`.
2. `pilot_guard.py snapshot` cho thư mục chính của vnp-qa-platform và cho worktree pilot (T4.1).
3. `tdd-agent -C <worktree> doctor`, `status`: nguồn URD/SRS/SAD "up to date"; nếu không, `tdd-agent -C <worktree> generate` (không `--force` trừ khi cần), đối chiếu số phần tử với T0.1.
4. `tdd-agent -C <worktree> mcp enable --read-only`; `.mcp.json` không commit.
5. Ghi HEAD, snapshot KG, số phần tử/cạnh/gap vào báo cáo pilot.

## Tiêu chí hoàn thành

- [ ] KG "up to date" với số liệu khớp T0.1 (hoặc có giải thích khi khác).
- [ ] Không có thay đổi chưa commit ngoài khối `[mcp]` trong config.

## Kiểm thử

- Lệnh kiểm ở trên.

## Rủi ro và ghi chú

- Mọi lệnh `tdd-agent` có `-C`.
