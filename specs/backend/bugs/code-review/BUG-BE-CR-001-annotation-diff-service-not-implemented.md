# BUG-BE-CR-001: `AnnotationService` và `DiffService` chưa được implement — Code Review domain thiếu core components

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-CR-001  
**Note:** code-review/AnnotationDiffService.ts: unified diff parser via relay  

## Mức độ: 🔴 HIGH (Feature Missing)

## Tóm tắt

HLD (BL-CR-01 → BL-CR-05) mô tả:
```
BL-CR-01: DiffService.load() — git diff HEAD, git diff --cached
BL-CR-02: AnnotationService.create() — INSERT annotations { id, worktreeId, file, line, text }
BL-CR-03: AgentManager.sendFeedback() — Daemon.writeToPty(sessionId, feedbackMessage)
BL-CR-04: GitService.generateCommitMessage() — inject to agent PTY → parse response
BL-CR-05: GitHubService.createPR() — git push + GitHub REST API POST /pulls
```

Grep toàn bộ `src/` không tìm thấy:
```
AnnotationService              → No results
DiffService                    → No results
annotation.create              → No results
diff.load                      → No results
generateCommitMessage          → No results (RPC method)
sendFeedback                   → No results
orca_annotations table         → No results
```

## Phân tích

- `git.ts` và `github.ts` trong RPC methods có implement một số git/GitHub operations.
- Nhưng các operations đặc thù Code Review (annotation, diff view, AI commit message, PR submit từ worktree) chưa có.

## Ảnh hưởng

1. **BL-CR-01**: Diff viewer data source không có `DiffService.load()`.
2. **BL-CR-02**: Line-level annotations không có store/retrieve.
3. **BL-CR-04**: AI commit message generation không có RPC handler.
4. **BL-CR-05**: PR creation với AI-generated description không có flow.

## Files không tồn tại

- `src/main/runtime/rpc/methods/` — không có `code-review.ts` hoặc `annotations.ts`
- DB migration: `orca_annotations` table — chưa tạo

## Liên quan đến luồng

- **BL-CR-01**: Diff load — DiffService missing.
- **BL-CR-02**: Line annotations — AnnotationService missing.
- **BL-CR-04**: AI commit message — RPC handler missing.
