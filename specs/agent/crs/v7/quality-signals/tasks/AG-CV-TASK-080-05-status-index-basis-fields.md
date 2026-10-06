# AG-CV-TASK-080-05: Gắn trường `indexScope`/`freshness`/… và `host` vào `codeintel.status`

**From Solution:** [AG-CV-SOL-080-index-basis-and-reindex-triggers](../solutions/AG-CV-SOL-080-index-basis-and-reindex-triggers.md) mục 5.2-5.5
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-status.ts` (sửa; do AG-CV-SOL-001 tạo), `agent/src/relay/codeintel-status.test.ts` (sửa/mới), fixture `agent/src/relay/__fixtures__/index-basis/` (task 01)
**Depends on:** AG-CV-TASK-080-02, 080-03, 080-04; AG-CV-SOL-001 (task tạo `codeintel-status.ts`), AG-CV-SOL-002/003 (probe chỉ mục)
**Status:** [ ] TODO

## Context

Hợp đồng §4.1 là hình dạng chuẩn trên dây (PQ-19). Task này nối ba mô-đun thuần ở trên vào handler. Nếu `codeintel-status.ts` chưa tồn tại khi làm task này thì task **bị chặn** (không tạo bản thay thế).

## Việc cần làm

1. Với mỗi công cụ có chỉ mục: `indexRoot` (GitNexus: đường dẫn registry; CodeGraph: `projectPath`), `rootMatches = realpath(indexRoot) === realpath(workspaceRoot)`; gọi `probeIndexBasis` rồi `classifyIndexBasis`; gắn `indexScope`, `freshness`, `headCommit`, `mergeBase`, `dirtySinceIndex`, `changedFilesNotInIndex`.
2. `pendingChanges`: CodeGraph chỉ có nghĩa khi `rootMatches`; ngược lại ép `null`. `rootMismatch: {worktreeRoot, indexRoot} | null` (đổi tên từ `worktreeMismatch`, PQ-19); `binding.worktreeMismatch` là boolean.
3. `data.host = readHostSnapshot()`.
4. Phong bì: `stale = OR(indexedCommit !== headCommit hoặc rootMismatch/worktreeMismatch hoặc pendingChanges > 0)` (§2.2); `warnings` thêm `index_commit_differs_from_head` khi cần.
5. `baseRef?` của `codeintel.status`: validate theo task 03, đưa vào probe; sai định dạng → `CODEINTEL_INVALID_PARAMS field="baseRef"`.
6. `codeintel.status` vẫn **luôn thành công** (không ném `CODEINTEL_*` khi thiếu công cụ/chỉ mục).

## Kiểm thử

`codeintel-status.test.ts` với runner giả đọc fixture task 01: checkout chính sạch → `exact/fresh` (GitNexus); worktree liên kết (`worktreeMismatch` có, `pendingChanges 0`) → `repo_root/…`, `pendingChanges:null`, `rootMismatch` đối tượng; không có chỉ mục → `none/unknown`; `baseRef` xấu → INVALID_PARAMS. Mở rộng fixture `status-*.json` của CR-070 bằng cơ chế sẵn (không thêm cơ chế chụp mới). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-status.test.ts`.

## Tiêu chí hoàn thành

- [ ] Đủ trường theo hợp đồng §4.1; `stale` đúng; `host` có.
- [ ] Đường dẫn tuyệt đối (`indexRoot`, `repoRoot`) chỉ ở `data`, không vào `warnings`/lỗi.

## Rủi ro

- Nếu SOL-002/003 chốt tên trường probe khác, chỉ sửa lớp nối ở file này. Thời gian status tăng thêm vài lệnh git (chưa đo).
