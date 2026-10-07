# AG-CV-SOL-004: `codeintel.reindex*`, `watch` và thông báo `indexChanged`/`reindexProgress`

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 004-01 đến 004-09, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-004](../../../../../../docs/crs/v7/agent-codeintel/CR-CV-004-codeintel-reindex-and-index-notifications.md)
**Service:** `agent/src/relay/` (Part A)
**TDD tham chiếu:** [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md) mục 9.1 (`makeNotifier`), 9.3 (`fs.watch`); [v5/03-connection-modes](../../../../tdd/v5/03-connection-modes.md) mục 3 (`createSession`), 5; [v5/04-handshake-session](../../../../tdd/v5/04-handshake-session.md) mục 8 (cleanup `stop()`)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md`

## 0. Hợp đồng áp dụng
| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §2.5 (timeout 25 s, reindex trả ngay), §3.2 (`REINDEX_IN_PROGRESS`, `PATH_NOT_ALLOWED hint`), §4.10–4.13 (`reindex`, `reindexStatus`, `reindexCancel`, `watch`), §6.1–6.2 (thông báo), §7.2 (ví dụ), §9 mục 1, 6, §10 |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-16 (hợp đồng reindex: `trigger`, `ifStale`, `expectHead`; bỏ `tiers`; trạng thái, `outcome`, `cancelled`), PQ-17 (`workspaceRoot` mọi thông báo; chuyển qua infra-fleet), PQ-18, §9 O-6, O-17 |

## 1. Trạng thái hiện tại (re-verify)
Đã đọc (2026-10-06): CR-CV-004 toàn bộ; `agent-rpc-dispatch.ts:280-290` (`makeNotifier`: bỏ nếu `readyState !== 1`); `agent-session.ts:226-254` (`stop()` gọi `cleanupAgentWatches()`, `scheduleAgentSpawnGracePeriod`); `agent-connection-stdio.ts:262-280` (daemon `--detach`: mỗi socket một `createSession`, Map mức module sống qua nhiều kết nối); `context.ts`; thư mục `.gitnexus/` có 4 tệp `lbug.wal.missing-shadow.*`.
| Điểm | Hiện trạng |
|---|---|
| Notifier | gắn `WireState` từng kết nối; không dùng được `ws` cũ |
| `stop()` | chỉ dọn watch; PTY có ân hạn; chưa có hook cho codeintel |
| Công cụ ghi | chưa có mã nào chạy `analyze/sync/index`; whitelist đọc cấm chúng |
| Journal | `~/.orca` đã dùng (`credentialDir`), chưa có `~/.orca/codeintel/` |

### Correction relative to CR
| # | CR nói | Quyết định |
|---|---|---|
| 1 | `canceled` | `cancelled` (PQ-16) |
| 2 | `reindex` chỉ `mode`,`tools` | thêm `trigger`, `ifStale`, `expectHead`; bỏ `tiers`; trả `outcome`, `skipped[]` |
| 3 | worktree liên kết luôn từ chối | `trigger:'manual'` -> `PATH_NOT_ALLOWED hint='reindex_linked_worktree_unsupported'`; `trigger:'agent_done'` -> thành công `outcome:'skipped_scope_repo_root'` không chạy lệnh (contract §4.10) |
| 4 | `indexChanged {workspaceRoot,tool,commit,indexedAt,reason,headCommit,stale}` | thêm tuỳ chọn `indexScope`, `mergeBase`, `trigger` (CR-080) |
| 5 | `reindexProgress` không có `outcome/errorCode` | thêm khi kết thúc (§6.2) |
| 6 | `reindexStatus/Cancel/watch` cần `workspaceRoot` | bắt buộc (§2.1) |

### Lệch giữa CR và hợp đồng
`cancelled`; thêm `ifStale/expectHead/trigger/outcome`; `workspaceRoot` trong mọi thông báo kể cả `quality.*` (PQ-17: sink phải dùng được cho `quality.progress|finished` của AG-CV-SOL-081); `state` hợp đồng gồm `cancelling`,`interrupted`.

### Phụ thuộc chéo khu vực
BE: `BE-CV-SOL-023-infra-fleet-codeintel-transport` (nhận `codeintel.indexChanged|reindexProgress|quality.*`, `StreamCodeIntelEvents`), `BE-CV-SOL-024-event-distribution`, `BE-CV-SOL-021-agent-collector` (gọi `reindex`, gọi lại `watch`+`status` sau nối lại), `BE-CV-SOL-012-index-status-aggregation`, `BE-CV-SOL-073-settings-flag-and-rollout` (cờ), `BE-CV-SOL-080-auto-refresh-index`. AG: `AG-CV-SOL-001/002/003` (probe, cache, SQLite), `AG-CV-SOL-080-index-basis-and-reindex-triggers` (mở rộng trigger/basis), `AG-CV-SOL-081` (dùng sink), `AG-CV-SOL-073` (kill-switch). FE: `FE-CV-SOL-051-review-workspace-shell` (nút "Làm mới index").

## 2. Giải pháp
### 2.1 Cây file
```
agent/src/relay/
  codeintel-notification-sink.ts    (mới) setCodeIntelNotifier, emitCodeIntelNotification
  codeintel-reindex-commands.ts     (mới) ReindexCommand: nơi DUY NHẤT sinh argv analyze/sync/index
  codeintel-reindex-progress.ts     (mới) message, percent, giới hạn tần suất
  codeintel-reindex-journal.ts      (mới) ~/.orca/codeintel/jobs/<jobId>.json
  codeintel-reindex-job.ts          (mới) hàng đợi, khoá, máy trạng thái, outcome
  codeintel-reindex-runner.ts       (mới) spawn nhóm tiến trình, stage, verify, huỷ, timeout
  codeintel-reindex-read-guard.ts   (mới) REINDEX_IN_PROGRESS cho đọc GitNexus
  codeintel-index-watcher.ts        (mới) thăm dò thay đổi chỉ mục/HEAD
  codeintel-reindex-methods.ts      (mới) reindex, reindexStatus, reindexCancel, watch
  codeintel-method-table.ts (sửa +4)  agent-session.ts (sửa +1 dòng cleanupCodeIntelWatchers)
```
### 2.2 Lệnh (chỉ ở `codeintel-reindex-commands.ts`)
GitNexus incremental `gitnexus analyze --index-only <repoRoot>`, full thêm `--force`; CodeGraph incremental `codegraph sync <projectPath> -q`, full `codegraph index <projectPath> -q`. **`--index-only` bắt buộc**; cấm `--embeddings, --skills, --name, --branch, --drop-embeddings`. Env con `GITNEXUS_WORKER_POOL_SIZE` ← `ORCA_CODEINTEL_ANALYZE_WORKERS` (mặc định `max(1,min(4,floor(cpu/2)))`, giả định). Thứ tự `codegraph` rồi `gitnexus`. `ORCA_CODEINTEL_REINDEX=off` -> `TOOL_UNAVAILABLE reason='reindex_disabled'`.
### 2.3 Job
Một job/repo (khoá `realpath(repoRoot)`); toàn agent `ORCA_CODEINTEL_MAX_REINDEX` (1, tối đa 2); hàng đợi ≤ 4 (`queue_full`); trùng repo -> `REINDEX_IN_PROGRESS {jobId,state,stage}`. Trạng thái `queued→running→(succeeded|failed|cancelled)`, `cancelling`, `interrupted` (từ journal). `stage ∈ preflight|codegraph.sync|codegraph.index|gitnexus.analyze|verify|done`. Timeout 45 phút/công cụ (`ORCA_CODEINTEL_REINDEX_TIMEOUT_MS`) -> `failed` `CODEINTEL_TIMEOUT`. Thành công = exit 0 **và** `verify` (probe `indexedAt/lastCommit` đổi hoặc `pendingChanges→0`; `indexedCommit===headCommit` -> `outcome:'already_up_to_date'`; không -> `TOOL_FAILED reason='index_not_updated'`). `ifStale:true` mà `freshness==='fresh'` -> `already_up_to_date` không spawn; `expectHead` khác HEAD -> `superseded`. Huỷ: `detached:true`, `process.kill(-pid,'SIGTERM')`, 10 s rồi `SIGKILL`, luôn `verify`; DB không rõ -> `indexHealth:'unknown'`; **không bao giờ** xoá/sửa tệp trong `.gitnexus/`, `.codegraph/`, không `unlock/clean`. Session WS dừng **không** huỷ job. Journal 50 bản, thư mục `0700`, không ghi `message`.
### 2.4 Đọc khi đang làm mới
`codeintel-reindex-read-guard.ts`: giai đoạn GitNexus -> mọi đọc GitNexus trả `REINDEX_IN_PROGRESS`; giai đoạn CodeGraph -> `closeCodeGraphDb` và đọc CodeGraph qua CLI hoặc cùng lỗi; method dùng công cụ kia vẫn chạy. Không cho `quality.run` heavy cùng lúc (cổng AG-CV-SOL-081).
### 2.5 Thông báo
Sink: `codeintel-notification-sink.ts` giữ notifier của lần gọi `codeintel.*`/`quality.*` gần nhất; `ws` đóng -> bỏ (không xếp hàng); Part B dùng `dispatcher.notify` (SOL-006). `reindexProgress`: ≤ 1/s/job, `percent` chỉ khi dòng khớp `/(\d{1,3})%/` (giả định), `message` ≤ 200 ký tự bỏ ANSI loại `$HOME`. `indexChanged`: thăm dò (không `fs.watch`) GitNexus `stat meta.json` 10 s (debounce 2 s), CodeGraph `max(mtime(db, db-wal))` 10 s (≤ 1/30 s/repo), HEAD `git rev-parse HEAD` 15 s (`tool:'git'`, `reason:'head'` chỉ cập nhật `stale`); lần đầu `watch` chỉ ghi nhớ; ≤ 8 repo (vượt -> `INVALID_PARAMS reason='watch_limit'`); sau reindex thành công phát `reason:'reindex'` ngay và huỷ cache (`invalidateShortLivedCache`, `invalidateRepoBindings`, `invalidateHeadCommit`).

## 3. Quyết định thiết kế
| # | Quyết định | Lý do |
|---|---|---|
| 1 | Luôn `--index-only` | không làm bẩn working tree |
| 2 | Thăm dò thay `fs.watch` | inotify, tệp lớn, NFS |
| 3 | `percent:null` khi không biết | không bịa số |
| 4 | Job qua mất kết nối | như PTY `agent.spawn` |
| 5 | Chặn đọc GitNexus khi analyze | bằng chứng `lbug.wal.missing-shadow.*` |
| 6 | Tách job/runner | test độc lập, max-lines |

## 4. Thứ tự task
```
01 ──────────────► 08 ◄── 07
02 ─► 04 ─► 05 ─► 06 ─► 08 ─► 09
03 ─► 04
```
01 sink; 02 lệnh + tiến độ; 03 journal; 04 job lõi cần 02, 03; 05 runner cần 04, SOL-001-04; 06 read-guard cần 05; 07 watcher cần 01; 08 method + nối `stop()` cần 01, 05, 06, 07; 09 tích hợp thủ công trên bản sao cần 08.

## 5. Tiêu chí chấp nhận
- [x] `reindex` trả `jobId` < 1 s; mọi `analyze` có `--index-only`; sau job `git status --porcelain` không đổi `AGENTS.md`, `CLAUDE.md`, `.claude/`.
- [x] Gọi lần hai -> `REINDEX_IN_PROGRESS` đúng `jobId`, không spawn thứ hai; hàng đợi đầy -> `queue_full`.
- [x] Đang analyze: `overview` -> `REINDEX_IN_PROGRESS`; method CodeGraph vẫn chạy khi giai đoạn CodeGraph không chạy.
- [x] Tiến độ ≤ 1/s, `percent:null` khi không khớp, không `$HOME`.
- [x] Huỷ: cả cây biến mất ≤ 15 s; lặp lại không lỗi; `verify` sau huỷ.
- [x] Mất WS: job chạy tiếp; `reindexStatus` đúng; khởi động lại -> `interrupted`.
- [x] Sau reindex: `indexChanged reason:'reindex'`, `status` mới; chạm `meta.json` -> `indexChanged` ≤ 15 s; commit mới -> `tool:'git'` ≤ 20 s; `watch` lần đầu không phát.
- [x] Worktree liên kết đúng hai nhánh (`manual` từ chối; `agent_done` `skipped_scope_repo_root`).
- [x] Không file `helpers/utils/common/misc`; không `max-lines` disable.

## 6. Kiểm thử
`/opt/repos/orca/agent`: `pnpm exec vitest run src/relay/codeintel-notification-sink.test.ts src/relay/codeintel-reindex-commands.test.ts src/relay/codeintel-reindex-progress.test.ts src/relay/codeintel-reindex-journal.test.ts src/relay/codeintel-reindex-job.test.ts src/relay/codeintel-reindex-runner.test.ts src/relay/codeintel-reindex-read-guard.test.ts src/relay/codeintel-index-watcher.test.ts src/relay/codeintel-reindex-methods.test.ts src/relay/__tests__/agent-session.test.ts`. Binary giả (Node script): ghi dòng tiến độ, thoát 0/1, treo, bỏ qua SIGTERM, sinh con. `vi.useFakeTimers` cho thăm dò. Tích hợp thật trên **bản sao nhỏ** (task 09), không phải Orca.

## 7. Rủi ro và điểm chưa kiểm chứng
Khoá/hỏng DB khi analyze cùng `gitnexus mcp`/`codegraph serve --mcp` đang mở (không kiểm soát được); thời gian analyze Orca chưa đo (45 phút, workers giả định); định dạng tiến độ chưa biết (`percent` có thể luôn null); nguyên nhân `lbug.wal.missing-shadow.*` chưa rõ; huỷ giữa chừng có thể sinh tệp dở dang; chỉ mục worktree liên kết (O-6) chưa chọn; `HOME` không ghi được -> mất journal; Windows chưa hỗ trợ; nhiều phiên trong daemon `--detach`: thông báo chỉ tới phiên gọi gần nhất.

## 8. Câu hỏi mở
1. Cách làm mới worktree liên kết (CR Q1; O-6). 2. Nới chặn đọc sau thử nghiệm (Q3). 3. Workers mặc định (Q4). 4. Quyền bấm làm mới ở backend (CR-013); `ORCA_CODEINTEL_REINDEX=off` đã có. 5. `reindexCancel` không có kênh backend (O-17).

## 9. Tham chiếu
CR-CV-004; contract §4.10–4.13, §6; `agent-rpc-dispatch.ts:280`, `agent-session.ts:226-254`, `agent-connection-stdio.ts:262-280`.
