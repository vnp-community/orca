# TASK-TDD-002-T0-03: Đưa worktree pilot về trạng thái sạch sau spike

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 4 (runbook)
**Checklist:** [T0.3](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** pilot (vnp-qa-platform)
**Files:** `vnp-qa-platform/.claude/worktrees/tdd-agent-bl-h01-h02/{.tdd-agent/,docs/tdd/,openspec/,.mcp.json}`; baseline `~/.cache/orca-pilot/vnp-qa-platform/baseline-1/`
**Depends on:** không
**Status:** `[ ]` TODO

---

## Context

Lần spike `tdd-agent change` (SOL-TDD-001 mục 2.2b) để lại trong worktree: KG đã bị ghi thêm, thư mục `.tdd-agent/changes/`, `.tdd-agent/staging/`, `.tdd-agent/runs/change-*`, `openspec/` (bộ khung do `change` tạo), `docs/tdd/TDD-NEW-*.md`, `docs/tdd/TDD-TMS.md` bị sửa, và `[approvals] mode = "auto"` trong `.tdd-agent/config.toml`. Baseline KG (915 phần tử, 1544 cạnh, 246 gap) đã được sao lưu bằng `sqlite3 .backup` trước spike.
Dùng `bash -c` cho lệnh dọn: zsh dừng cả chuỗi `&&` khi một glob không khớp.

## Việc cần làm

1. Chạy trong `bash -c`: chép lại `knowledge.db` và `operational.db` từ baseline-1 đè lên `.tdd-agent/`; xoá `.tdd-agent/*.db-wal` và `*.db-shm` bằng `find ... -delete`.
2. Xoá `.tdd-agent/changes`, `.tdd-agent/staging`, `.tdd-agent/runs/change-*`, `openspec/`, `docs/tdd/TDD-NEW-FR-SYS-*.md`.
3. `git checkout -- docs/tdd/TDD-TMS.md .tdd-agent/config.toml docs/crs/v5` để về bản đã commit (`18d68d01`), bỏ khối `[approvals] mode = "auto"`.
4. Chạy `tdd-agent -C <worktree> mcp enable --read-only` (ghi `.mcp.json` với đường dẫn tuyệt đối và khối `[mcp]` vào config); thêm `.mcp.json` vào `.git/info/exclude` vì nó chứa đường dẫn máy; ghi lại `git diff .tdd-agent/config.toml`: nếu chỉ khác khối `[mcp]`, giữ lại làm thay đổi có chủ đích hoặc hoàn tác sau khi dùng xong.
5. Chạy `tdd-agent -C <worktree> status` và `change list`.

## Tiêu chí hoàn thành

- [ ] `git status` của worktree chỉ còn thay đổi có chủ đích (khối `[mcp]` trong config nếu giữ).
- [ ] `tdd-agent -C <worktree> status` báo 915 phần tử, 1544 quan hệ, 246 gap, 0 xung đột; `change list` báo "No change requests yet".
- [ ] Thư mục chính của vnp-qa-platform không bị đụng (so `stat` các tệp `.tdd-agent/config.toml`, `docs/tdd/TDD-TMS.md` trước và sau).

## Kiểm thử

- Thủ công: các lệnh ở mục "Tiêu chí hoàn thành" và so sánh `git status` trước/sau.

## Rủi ro và ghi chú

- Mọi lệnh `tdd-agent` phải có `-C` tuyệt đối: worktree lồng trong checkout chính, thiếu `-C` sẽ dùng `.tdd-agent/` của thư mục cha.
