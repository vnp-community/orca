# AG-CV-TASK-072-06: Phân giải repo: worktree lồng, registry trùng, tên gần giống, stderr không rò

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/security-repo-resolution.test.ts` (mới)
**Depends on:** 072-01, 072-05; AG-CV-SOL-001
**Status:** [x] DONE

## Context

Agent-rpc §9.2: khớp registry theo **đường dẫn chính xác** sau `realpath`; trùng → `indexedAt` mới nhất + cảnh báo (CR-072 nói "từ chối": hợp đồng thắng); hỏng → `TOOL_FAILED reason="registry_unreadable"`; không khớp → `REPO_NOT_REGISTERED`/`INDEX_MISSING`.
CR-072: worktree lồng (`/…/orca/.claude/worktrees/x`) không được nhận chỉ mục repo cha; lỗi thiếu `-r` có stack trace liệt kê tên repo khác (cần `TestToolStderrNeverForwarded`).
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. Dựng `HOME` tạm + `~/.gitnexus/registry.json` giả + `git init` thật (git ≥ 2.25 như baseline).
2. Test: nested worktree; trùng đường dẫn; tên gần giống; `-r` lấy từ registry không từ tham số; registry hỏng; registry đổi giữa hai lần gọi (ghi hành vi thật; chưa chốt, solution câu hỏi 7); CodeGraph `-p` realpath.
3. `TestToolStderrNeverForwarded`: CLI giả in stack trace có `Available: orca, vnp-…` → `stderrTail` đã che, ≤ 2 KiB, không có tên repo khác, không `$HOME`.

## Kiểm thử

- Như mục 2.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/security-repo-resolution.test.ts` (5 passed).

## Tiêu chí hoàn thành

- [x] Không dữ liệu repo cha; cảnh báo trùng có mặt; stderr sạch.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Cache 30 s theo `workspaceRoot` có thể che thay đổi registry (hợp đồng chưa nói).
