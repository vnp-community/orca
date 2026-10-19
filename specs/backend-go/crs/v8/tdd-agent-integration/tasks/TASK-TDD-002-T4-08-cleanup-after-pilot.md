# TASK-TDD-002-T4-08: Dọn sau pilot

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 5 (tiêu chí 6)
**Checklist:** [T4.8](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** orca + pilot
**Files:** stack local, worktree, project thử
**Depends on:** T4.5, T4.6, T4.7
**Status:** `[ ]` TODO

---

## Context

Không để lại Request, project, worktree hay cấu hình tạm của pilot.

## Việc cần làm

1. Huỷ/hoàn tất Request thử còn lại; `report_orphan_requests.py --cancel` nếu có mồ côi.
2. Gỡ project thử và repo của nó (`DELETE /v1/projects/{id}/repos/{repoId}`, `DELETE /v1/projects/{id}`): chỉ bookkeeping, không xoá thư mục.
3. Gỡ worktree task và worktree proposal theo T2.12; giữ nhánh proposal nếu người dùng muốn xem.
4. Khôi phục agent (`PushBlockedAgent`/`restore`), xoá file MCP tạm trong `workDir/tmp`.
5. `pilot_guard.py compare` lần cuối; xoá `.mcp.json` hoặc để trong `.git/info/exclude`.

## Tiêu chí hoàn thành

- [ ] 0 Request và 0 worktree mồ côi; thư mục chính của vnp-qa-platform không đổi; agent chạy lại bình thường.

## Kiểm thử

- Các script báo cáo.
