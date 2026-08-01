# BUG-CR-001: Code Review (BL-CR-01 đến BL-CR-05) — Không có dual-path cho Remote Dev Server

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-CR-001  
**Note:** AnnotationDiffService.ts: path normalization + relay git.exec  

## Mức độ: 🔴 HIGH

## Tóm tắt

Tất cả các luồng trong `code-review.md` (BL-CR-01 → BL-CR-05) mô tả:
- Local: `child_process.execFile('git', ...)` → Git CLI local
- Remote: `relay.call('git.*')` → Dev Server

Thực tế khi grep code:
```bash
grep -rn "DiffService\|diff\.load\|git\.diff\|git\.generateCommitMessage" src/main --include="*.ts"
→ No results for DiffService
```

**`DiffService` không tồn tại trong `src/main`.**

Grep renderer:
```bash
grep -rn "diff.load\|DiffViewer\|diff.*worktree\|git.*diff" src/renderer/src --include="*.tsx" --include="*.ts"
```

Code review (diff viewer, annotation, PR creation) dùng Electron IPC nhưng **Main Process side không có** `DiffService`, `AnnotationService`, hay `GitHubService` (theo nghĩa BL-CR flows).

## Thực tế

Code review flow trong Orca thực tế có thể được implement ở renderer side trực tiếp (gọi `git diff` qua Electron IPC `shell.exec`) hoặc chưa implement.

Cần verify riêng phần renderer để xác định luồng thực tế.

## Ảnh hưởng

1. BL-CR-01 (View Diff): Nếu dùng remote worktree → cần `relay.call('git.diff')` nhưng Main Process không có handler
2. BL-CR-04 (Commit Message AI): Inject prompt vào remote agent không thể thực hiện (BUG-AG-ORCH-001)
3. BL-CR-05 (PR Creation): `git push` trên remote worktree cần `relay.call('git.push')` — không có route

## Files liên quan

- `src/main/`: Thiếu DiffService, AnnotationService, GitHubService cho remote path
- Cần kiểm tra `src/renderer/src/` để xác định implementation thực tế
