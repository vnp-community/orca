# AG-CV-TASK-004-07: Bộ thăm dò `indexChanged`

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.5
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-index-watcher.ts`, `codeintel-index-watcher.test.ts` (mới)
**Depends on:** [001](./AG-CV-TASK-004-01-codeintel-notification-sink.md), [AG-CV-TASK-001-08](./AG-CV-TASK-001-08-result-envelope-and-head-commit.md)
**Status:** [ ] TODO

## Context
Contract §4.13, §6.1. Thăm dò, không `fs.watch` (giới hạn inotify `MAX_LINUX_WATCH_DIRS=4000`, `meta.json` 2,8 MB, NFS). `lastIndexed` của CodeGraph không là tín hiệu.

## Việc cần làm
1. `enableWatch(binding)`/`disableWatch(root)`, một bộ định thời chung, ≤ 8 repo.
2. GitNexus `stat meta.json` 10 s debounce 2 s -> đọc registry `lastCommit/indexedAt`; CodeGraph `max(mtime(db, db-wal))` 10 s, ≤ 1/30 s/repo; HEAD 15 s.
3. Phát `codeintel.indexChanged {workspaceRoot,tool,commit,indexedAt,reason,headCommit,stale,(indexScope,mergeBase,trigger)}`; lần đầu chỉ ghi nhớ; đo hỏng -> bỏ chu kỳ.
4. `cleanupCodeIntelWatchers()` dừng thăm dò (không huỷ job).

## Kiểm thử
`vi.useFakeTimers`: sửa `meta.json` -> phát ≤ 15 s; commit mới -> `tool:'git'`; CodeGraph tự sync nhiều lần chỉ 1/30 s; bật lần đầu không phát; 9 repo -> `watch_limit`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-index-watcher.test.ts`

## Tiêu chí hoàn thành
- [ ] Không bão thông báo; không rò timer sau `cleanup`.

## Rủi ro
- Trễ phát hiện 10–15 s (chấp nhận).
