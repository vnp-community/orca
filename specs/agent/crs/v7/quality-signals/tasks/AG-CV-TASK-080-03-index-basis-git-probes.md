# AG-CV-TASK-080-03: Probe git chỉ đọc: `mergeBase`, `changedFilesNotInIndex`, `dirtySinceIndex`

**From Solution:** [AG-CV-SOL-080-index-basis-and-reindex-triggers](../solutions/AG-CV-SOL-080-index-basis-and-reindex-triggers.md) mục 5.4
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-index-basis-probe.ts` (mới), `agent/src/relay/codeintel-index-basis-probe.test.ts` (mới)
**Depends on:** AG-CV-TASK-080-01
**Status:** [ ] TODO

## Context

CR-080 2.3.1 bước 2-4. Đã đọc: agent chạy git bằng `execFile('git', args, opts)` (`git-handler.ts:3,77`) và đã có mẫu `['-c','core.quotePath=false','diff',…]` (`agent-git-handler-extended.ts:106`). AGENTS.md (Git Binary Compatibility): mọi lệnh dưới đây có từ Git < 2.25 nên **không cần** `GitCapabilityCache`; phải ghi điều này trong comment ngắn và giữ cờ `-c` ở đầu lệnh.

## Việc cần làm

1. `export async function probeIndexBasis(workspaceRoot: string, input: { indexedCommit: string | null; indexedAtMs: number | null; baseRef: string }, deps?: { git?: GitRunner; stat?: StatFn; now?: () => number }): Promise<{ headCommit: string | null; headCommitTimeMs: number | null; mergeBase: string | null; mergeBaseCommitTimeMs: number | null; changedFilesNotInIndex: number | null; dirtySinceIndex: boolean }>`.
2. `headCommit`: `git rev-parse HEAD` (dùng cache 5 s của AG-CV-SOL-001 nếu đã có; nếu chưa có thì cache module 5 s theo `workspaceRoot`, ghi TODO hợp nhất). `headCommitTimeMs`: `git log -1 --format=%ct HEAD` × 1000.
3. `mergeBase`: `git merge-base HEAD <baseRef>`; `baseRef` đã kiểm `[A-Za-z0-9._/@^~{}+-]{1,256}`, không bắt đầu `-`; lỗi (không có `origin/HEAD`, unborn HEAD) → `null`, không ném.
4. `changedFilesNotInIndex`: nếu `indexedCommit` null → `null`. Kiểm `git cat-file -e <indexedCommit>^{commit}`; không còn → `null`. Ngược lại hợp nhất (Set) tên từ `git diff --name-only -z <indexedCommit> HEAD` và `git status --porcelain=v1 -z` (parse bản ghi `XY path\0` và đổi tên `R` có thêm một bản ghi đường dẫn gốc); đếm; cắt ở 5 000 (trả 5 000 và đặt cờ nội bộ `capped`).
5. `dirtySinceIndex`: `true` nếu `changedFilesNotInIndex > 0`, hoặc có tệp trong tập có `mtimeMs > indexedAtMs` (`fs.stat` tối đa 5 000); `capped` → `true`.
6. Mọi lệnh: `execFile` không shell, `timeout 5000`, `maxBuffer 8 MiB`, `cwd: workspaceRoot`; lỗi git bị nuốt thành `null` kèm `log.debug` (không stack).

## Kiểm thử

`codeintel-index-basis-probe.test.ts` dùng repo git thật trong `fs.mkdtemp` (không mock git): commit 1, commit 2; sửa chưa commit; tệp mới; đổi tên; `git worktree add`; `indexedCommit` giả `deadbeef…` không tồn tại → `null`; `baseRef` không tồn tại → `mergeBase:null`; tên tệp có khoảng trắng và Unicode; unborn HEAD (repo `git init` chưa commit). Có một ca chạy dưới Git baseline bằng cách kiểm không dùng cờ nào ngoài danh sách ở trên (test quét `args` đã gọi qua `git` giả).

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-index-basis-probe.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mọi ca trên xanh; không ném khi git thiếu/lỗi.
- [ ] Không có `git` nào dùng `--path-format`, `merge-tree`, `--merge-base` (test quét).
- [ ] `dirtySinceIndex` là `true` với sửa chưa commit sau thời điểm index.

## Rủi ro

- `mtime` không tin cậy sau `git checkout`/rsync (CR-080 mục 6): chấp nhận thiên về `stale`.
- `git status` trên repo rất lớn chậm; `timeout 5000` có thể cắt → `dirtySinceIndex:true` và `capped`.
