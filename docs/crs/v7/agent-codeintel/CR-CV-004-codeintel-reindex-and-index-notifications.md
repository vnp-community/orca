# CR-CV-004 — Làm mới index (`codeintel.reindex`) chạy nền có tiến trình và thông báo `codeintel.indexChanged`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-004 |
| **Tên** | Job làm mới chỉ mục GitNexus/CodeGraph chạy nền trên agent (tiến trình, chặn đồng thời, huỷ, an toàn khoá DB), cùng bộ thăm dò thay đổi chỉ mục phát `codeintel.indexChanged` |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-001, CR-CV-002 (đọc registry/probe), CR-CV-003 (probe CodeGraph) |
| **Mở khoá** | CR-CV-023 (consumer thông báo ở Go), CR-CV-024 (huỷ cache khi đổi chỉ mục), CR-CV-012 (trạng thái reindex), UI "Làm mới index" (CR-CV-051) |
| **Tác động** | `agent/src/relay/codeintel-method-table.ts` (thêm dòng), `agent/src/relay/agent-session.ts` (thêm một lời gọi dọn dẹp trong `stop()`), `agent/src/relay/agent-tool-registry.ts` (tuỳ chọn `signal`, CR-CV-001), các file mới `agent/src/relay/codeintel-reindex-*.ts`, `agent/src/relay/codeintel-index-watcher.ts` |

---

## 1. Bối cảnh và vấn đề

1. **Index thường cũ.** Trên máy khảo sát: GitNexus `Indexed commit d819812`, `Current commit 1b0c760`, `⚠️ stale` (`gitnexus status`). Ghi chú `Index stale? Run ... analyze` đã nằm trong chỉ dẫn của repo. Mặc định O3 (README v7): người dùng bấm làm mới qua `codeintel.reindex`, không chạy tự động.
2. **`gitnexus analyze` không chỉ ghi chỉ mục.** Từ `gitnexus analyze --help`: không có `--index-only` thì lệnh còn cập nhật khối GitNexus trong `AGENTS.md`/`CLAUDE.md` và cài `.claude/skills/gitnexus/`; `--skip-agents-md`, `--skip-skills`, `--index-only` (bỏ mọi chèn tệp) là cách tắt. Repo khảo sát đang có `M AGENTS.md`, `M CLAUDE.md` (nguyên nhân chưa xác minh; gợi ý: khối GitNexus trong hai tệp ghi `248146 symbols`, trong khi registry và `meta.json` ghi `247556`, nên có vẻ một lần `analyze` hoặc hook đã ghi hai tệp này). Nếu agent chạy `analyze` không có `--index-only`, mỗi lần làm mới sẽ **làm bẩn working tree** của repo đang được review. Đây là lý do chính để cấm `args` tự do (CR-CV-001 2.9).
3. **Cờ `analyze` đáng chú ý** (đọc từ `--help`): `-f/--force` (re-index đầy đủ dù đã mới), `--branch <name>` ("per-branch index slot"; không có cờ thì `analyze` luôn cập nhật chỉ mục workspace, đi theo working tree đang checkout), `--workers <n>` (mặc định lõi-1, tối đa 16), `--worker-timeout`, `--max-file-size` (512 KB mặc định), `--embeddings` (tắt mặc định, tốn tài nguyên), `--skip-git`, `--name`, `--wal-checkpoint-threshold`. Tiến trình `analyze` in tiến độ ra terminal; **định dạng chưa kiểm chứng** vì CR này không chạy `analyze`.
4. **Rủi ro khoá DB có bằng chứng gián tiếp.** `.gitnexus/` của Orca chứa 4 tệp `lbug.wal.missing-shadow.*` (ngày 10/08, 17/08, 29/08, 08/09; kích thước 2575, 308, 330, 257 904 byte) cạnh `lbug` 1,64 GB; tên gợi ý WAL của LadybugDB thiếu tệp "shadow" tại một thời điểm đóng/ghi. Nguyên nhân chưa được xác minh (không đọc mã GitNexus). Cùng lúc, ít nhất hai cặp tiến trình `gitnexus mcp` và `codegraph serve --mcp` (PID khởi chạy 09:00 và từ hôm trước) đang giữ chỉ mục mở (kết quả `ps`), tức là thực tế có nhiều bên đọc/ghi cùng một DB. `analyze` đồng thời với các bên đọc khác có thể gây xung đột; tác động chưa được thử.
5. **CodeGraph tự đồng bộ.** Tiến trình `codegraph serve --mcp` ghi `daemon.log` các dòng `Auto-synced 1 file(s) in ~1100ms`; `codegraph.db` có mtime 19:33 trong khi `lastIndexed` trong `status -j` là 12:33:43. Nghĩa là **`lastIndexed` không đổi khi tự đồng bộ**, nên không dùng làm tín hiệu thay đổi. `codegraph sync [path]` ("Sync changes since last index", `-q`), `codegraph index [path]` ("Rebuild the full index from scratch", `-f`, `-q`, `-v`), `codegraph unlock [path]` ("Remove a stale lock file that is blocking indexing") đều có; cơ chế khoá giữa daemon và `sync` thủ công chưa kiểm chứng.
6. **Thời gian `analyze` Orca: chưa đo.** Nghiên cứu (`02-local-mcp-interaction.md`) ghi "mất nhiều phút"; số liệu cần để thiết kế timeout: repo 20 174 tệp, 247 556 nút, DB 1,64 GB. Không chạy `analyze` trong quá trình soạn CR (cấm lệnh ghi).
7. **Kênh thông báo.** Agent đẩy thông báo bằng `makeNotifier(ws, state)` (`agent-rpc-dispatch.ts:280`), gửi khung JSON-RPC không có `id` (cách `fs.changed` đang làm, `fs-agent-watch-extensions.ts`). Bộ theo dõi `fs.watch` nằm trong một `Map` toàn module và bị `cleanupAgentWatches()` xoá **mỗi khi một phiên WS dừng** (`agent-session.ts:253`). Riêng PTY của `agent.spawn` được giữ qua mất kết nối bằng thời gian ân hạn (CR-STORAGE-008(b), `agent-session.ts` `stop()`). Phía Go hiện chỉ có consumer cho `pty.data`, `fs.changed`…; consumer cho `codeintel.*` là CR-CV-023.
8. **Không có "tiến trình" từ công cụ ở dạng cấu trúc.** README v7 (3.2) định nghĩa `codeintel.reindexProgress {jobId, stage, percent, message}`; với công cụ chưa biết định dạng tiến độ, `percent` có thể không tính được (xem 2.4).

## 2. Giải pháp đề xuất

### 2.1 File mới (`agent/src/relay/`)

| File | Nội dung |
|---|---|
| `codeintel-reindex-job.ts` | `startReindex(binding, params, deps)`, trạng thái job, máy trạng thái, huỷ |
| `codeintel-reindex-commands.ts` | **Nơi duy nhất** sinh argv của `analyze`/`sync`/`index` (kiểu `ReindexCommand`); không xuất qua `codeintel-command-whitelist.ts` (CR-CV-001 2.3) |
| `codeintel-reindex-progress.ts` | Phân tích dòng đầu ra để lấy `message`, `percent` (nếu nhận ra), giới hạn tần suất |
| `codeintel-reindex-journal.ts` | Ghi/đọc nhật ký job nhỏ trên đĩa để biết job `interrupted` sau khi agent khởi động lại |
| `codeintel-index-watcher.ts` | Bộ thăm dò thay đổi chỉ mục và HEAD, phát `codeintel.indexChanged` |
| `codeintel-notification-sink.ts` | Giữ tham chiếu notifier của kết nối gần nhất; phát thông báo an toàn khi ws đóng |

Test (mới): `codeintel-reindex-job.test.ts`, `codeintel-reindex-commands.test.ts`, `codeintel-reindex-progress.test.ts`, `codeintel-reindex-journal.test.ts`, `codeintel-index-watcher.test.ts`, `codeintel-notification-sink.test.ts`.

### 2.2 Method

**`codeintel.reindex`**

| Tham số | Kiểu | Mặc định | Ghi chú |
|---|---|---|---|
| `workspaceRoot` | string | bắt buộc | như CR-CV-001 |
| `mode` | `"incremental"` \| `"full"` | `"incremental"` | xem bảng lệnh dưới |
| `tools` | `("gitnexus"\|"codegraph")[]` | mọi công cụ khả dụng và hỗ trợ | thứ tự thực hiện cố định: `codegraph` rồi `gitnexus` (nhanh trước) |

Bảng lệnh (do `codeintel-reindex-commands.ts` dựng, không nhận gì từ client ngoài `mode`):

| Công cụ | `incremental` | `full` |
|---|---|---|
| GitNexus | `gitnexus analyze --index-only <repoRoot>` | `gitnexus analyze --index-only --force <repoRoot>` |
| CodeGraph | `codegraph sync <projectPath> -q` | `codegraph index <projectPath> -q` |

- `--index-only` là **bắt buộc** (2). Test snapshot kiểm tra không có lệnh `analyze` nào thiếu cờ này, và không có `--embeddings`, `--skills`, `--name`, `--branch`, `--drop-embeddings`.
- Thư mục thực hiện `<repoRoot>` = đường dẫn trong registry (CR-CV-001 2.4). Với **worktree liên kết** (`worktreeMismatch`) `analyze` trên checkout chính sẽ làm mới chỉ mục của checkout chính, **không** phản ánh worktree đang review; xem 2.7 và Q1. MVP: với worktree liên kết, trả `CODEINTEL_PATH_NOT_ALLOWED (data.hint = "reindex_linked_worktree_unsupported")`, không chạy lệnh.
- Biến môi trường thêm cho tiến trình con: `GITNEXUS_WORKER_POOL_SIZE` đặt từ `ORCA_CODEINTEL_ANALYZE_WORKERS` (mặc định `max(1, min(4, floor(cpuCount / 2)))` để không chiếm hết CPU của dev server, vốn chạy PTY của người dùng; giá trị là giả định, chưa đo). Không đặt nhúng embedding.
- Trả ngay (không chờ xong):

```jsonc
// result.data
{ "jobId": "ri_01J9ZK3Q8M2X",           // ULID, sinh ở agent
  "state": "running",                    // queued|running
  "workspaceRoot": "/opt/repos/orca", "repoRoot": "/opt/repos/orca",
  "mode": "incremental", "tools": ["codegraph", "gitnexus"],
  "startedAt": "2026-10-05T14:00:00.000Z",
  "estimate": null }                     // chưa có số đo thật nên null
```

**`codeintel.reindexStatus`** (**mới, đề xuất bổ sung hợp đồng**) `{workspaceRoot, jobId?}`: trả trạng thái job đang chạy hoặc gần nhất của repo (hoặc đúng `jobId`). Cần vì thông báo có thể mất khi mất kết nối và backend (`code-intel-service`) cần truy vấn lại sau khi nối lại. Trả `{job: {jobId, state, stage, percent, message, startedAt, finishedAt?, error?}}`; không có job nào → `{job: null}`.

**`codeintel.reindexCancel`** (**mới, đề xuất**) `{workspaceRoot, jobId}`: yêu cầu huỷ (2.5). Idempotent: job đã kết thúc → trả trạng thái cuối, không lỗi.

**`codeintel.watch`** (**mới, đề xuất**) `{workspaceRoot, enabled: boolean}`: bật/tắt thăm dò `indexChanged` cho repo (2.6). Idempotent; backend gọi lại sau mỗi lần agent nối lại.

Trạng thái job: `queued → running → (succeeded | failed | canceled)`, thêm `canceling` (trung gian) và `interrupted` (chỉ do `journal` sau khi agent khởi động lại).

### 2.3 Chặn chạy đồng thời

- **Mỗi repo một job**: khoá theo `repoRoot` (đường dẫn registry, đã `realpath`). Gọi `codeintel.reindex` khi đã có job `queued|running|canceling` của repo → `CODEINTEL_REINDEX_IN_PROGRESS` với `data = {jobId, state, stage}`, `retryable: true` (backend dùng `jobId` để gắn vào tiến trình đang chạy, không tạo job thứ hai).
- **Toàn agent**: tối đa 1 job `analyze`/`index` đang chạy cùng lúc (cấu hình `ORCA_CODEINTEL_MAX_REINDEX`, mặc định 1, tối đa 2). Job thứ hai của repo khác vào hàng đợi (`state: "queued"`, tối đa 4, quá thì `CODEINTEL_REINDEX_IN_PROGRESS` với `data.reason = "queue_full"`).
- **Đọc trong lúc làm mới (bảo thủ):** khi một giai đoạn GitNexus của repo đang chạy, mọi method đọc phụ thuộc GitNexus trả `CODEINTEL_REINDEX_IN_PROGRESS (data.jobId)` thay vì truy cập `lbug`; khi giai đoạn CodeGraph đang chạy, đóng kết nối SQLite (CR-CV-003 2.4) và đọc CodeGraph qua CLI hoặc trả cùng lỗi (CLI cũng mở DB); method chỉ cần phần công cụ còn lại vẫn chạy được. Lý do: bằng chứng ở mục 1.4 và việc đọc đồng thời với `analyze` chưa được kiểm chứng. Nếu thử nghiệm sau này chứng minh an toàn, nới quy tắc này (Q3).
- **Bên ngoài agent**: không phát hiện được `analyze` do người dùng/hook tự chạy hoặc `gitnexus mcp`/`codegraph serve --mcp` đang giữ DB. Agent chỉ kiểm tra trước khi bắt đầu: nếu có tệp `.gitnexus/lbug.lock`/tương tự (tên chưa kiểm chứng) thì không có gì để kiểm; ghi `warnings: ["other_processes_may_hold_index"]` và log, **không** kill tiến trình của người khác.

### 2.4 Tiến trình và thông báo

`codeintel.reindexProgress` (thông báo agent → backend, không có `id`):

```jsonc
{ "jsonrpc": "2.0", "method": "codeintel.reindexProgress",
  "params": { "jobId": "ri_01J9ZK3Q8M2X", "workspaceRoot": "/opt/repos/orca",
              "state": "running", "tool": "gitnexus", "stage": "gitnexus.analyze",
              "percent": null, "message": "Parsing files…", "at": "2026-10-05T14:01:07.220Z" } }
```

- Giai đoạn (`stage`): `preflight` → `codegraph.sync` | `codegraph.index` → `gitnexus.analyze` → `verify` → `done`; kết thúc bằng một thông báo với `state` ∈ `succeeded|failed|canceled` kèm `errorCode` (nếu có).
- `percent`: trung thực. Chỉ phát số khi một dòng đầu ra khớp mẫu `(\d{1,3})%` trong khoảng 0..100 (**định dạng của `analyze` chưa kiểm chứng**, nên mẫu này là giả định cần thử); nếu không thì `null`. Không suy diễn phần trăm theo thời gian. Phần UI hiển thị "đang chạy" không thanh phần trăm khi `null`. Đề xuất README v7 cho phép `percent: null` (Điều chỉnh hợp đồng).
- `message`: dòng stdout/stderr cuối, bỏ mã ANSI, ≤ 200 ký tự, **đã loại đường dẫn `$HOME`**; tần suất ≤ 1 thông báo/giây/job (gộp, giữ cái mới nhất); luôn phát thông báo khi đổi `stage` hoặc `state`.
- **Đầu ra của `analyze`/`sync`/`index`** đi qua pipe (dự kiến nhỏ, tiến độ dòng); nhớ rằng `gitnexus` cắt cụt stdout lớn qua pipe (CR-CV-001 1.9): với tiến độ chỉ cần dòng cuối, mất phần đầu chấp nhận được, nhưng **không** dùng stdout của `analyze` làm bằng chứng thành công. Thành công xác định bằng `exit code 0` **và** bước `verify` (2.4 cuối).
- Giới hạn: giữ tối đa 64 KiB stdout/stderr cuối trong bộ nhớ (vòng) cho chẩn đoán; không gửi toàn bộ qua RPC.
- Timeout job: mặc định 45 phút/công cụ (`ORCA_CODEINTEL_REINDEX_TIMEOUT_MS`, **giả định vì chưa đo thời gian analyze**); quá hạn → huỷ (2.5) và `failed (CODEINTEL_TIMEOUT)`.
- **`verify`**: sau mỗi công cụ, gọi lại probe chỉ mục (CR-CV-002 2.5, CR-CV-003 2.1) để xác nhận `indexedAt`/`lastCommit` đã đổi hoặc `pendingChanges` về 0; không đổi → `failed (CODEINTEL_TOOL_FAILED, reason = "index_not_updated")` kèm cảnh báo (có thể "đã mới sẵn": `analyze` không có `--force` bỏ qua khi chỉ mục đã mới; khi đó `state: succeeded`, `outcome: "already_up_to_date"` nếu `indexedCommit === headCommit`).

### 2.5 Huỷ và dọn dẹp

`codeintel.reindexCancel` → `state: canceling` → `SIGTERM` cho **nhóm tiến trình** (spawn với `detached: true` trên POSIX, `process.kill(-pid, 'SIGTERM')`; trên Windows `taskkill /T /PID` — chưa kiểm chứng, MVP chặn Windows như CR-CV-001) → sau 10 s nếu chưa thoát `SIGKILL` → `state: canceled`. Cần `runToolCommand` hỗ trợ `signal`/`killGraceMs` (CR-CV-001 2.5) và `detached`. Sau huỷ, bắt buộc chạy `verify`: nếu DB ở trạng thái không rõ (probe lỗi mở/parse), job kết thúc `canceled` kèm `indexHealth: "unknown"` và `codeintel.status` trả `state: "unknown"` cho tới khi có lần làm mới thành công (UI nên đề nghị `mode: "full"`). **Việc huỷ giữa chừng `analyze` có thể để lại tệp kiểu `lbug.wal.missing-shadow.*` hoặc DB dở dang: chưa kiểm chứng**, cần thử trên bản sao repo (CR-CV-070). Agent **không bao giờ** xoá tệp trong `.gitnexus/`/`.codegraph/` và không chạy `codegraph unlock` hay `gitnexus clean`.

Khi session WS dừng (`agent-session.ts` `stop()`), job **không** bị huỷ (làm mới dài hơn một lần chớp mạng; cùng triết lý với PTY của `agent.spawn`). Chỉ bộ thăm dò `watch` bị dừng (không có ai nhận), thêm một dòng `cleanupCodeIntelWatchers()` cạnh `cleanupAgentWatches()` (`agent-session.ts:253`). Khi agent thoát có chủ đích, kill nhóm tiến trình `analyze`.

Nhật ký job: `~/.orca/codeintel/jobs/<jobId>.json` (thư mục `0700`; `~/.orca` đã tồn tại do `credentialDir = ~/.orca/credentials`) ghi lúc bắt đầu và lúc kết thúc (không ghi `message`); khi agent khởi động, các job còn `running` trong nhật ký được đánh dấu `interrupted` (không tự chạy lại). Giữ 50 nhật ký gần nhất.

### 2.6 `codeintel.indexChanged`: bộ thăm dò

`codeintel.watch {enabled: true}` đăng ký `repoRoot` vào `codeintel-index-watcher.ts` (chung một bộ định thời, tối đa 8 repo, mỗi repo một mục). **Thăm dò (polling), không dùng `fs.watch`**: tránh giới hạn inotify (bộ theo dõi sẵn có đã giới hạn `MAX_LINUX_WATCH_DIRS = 4000`), `meta.json` 2,8 MB bị ghi lại cả tệp, và hệ tệp từ xa/NFS.

| Tín hiệu | Cách đo | Chu kỳ | Phát khi |
|---|---|---|---|
| GitNexus | `fs.stat` trên `.gitnexus/meta.json` (`mtimeMs`, `size`); khi đổi, đọc phần tử registry (`lastCommit`, `indexedAt`) | 10 s | `mtimeMs` đổi (debounce 2 s sau lần đổi cuối) |
| CodeGraph | `fs.stat` trên `.codegraph/codegraph.db` và `.codegraph/codegraph.db-wal` (lấy `max(mtimeMs)`) | 10 s | thay đổi, nhưng tối đa 1 thông báo/30 s/repo (daemon tự đồng bộ nhiều lần, 1.5) |
| HEAD | `git rev-parse HEAD` tại `workspaceRoot` (vài ms) | 15 s | commit đổi |

Thông báo (README v7: `{workspaceRoot, tool, commit, indexedAt}`; thêm trường tuỳ chọn):

```jsonc
{ "jsonrpc": "2.0", "method": "codeintel.indexChanged",
  "params": { "workspaceRoot": "/opt/repos/orca", "tool": "gitnexus",     // "gitnexus" | "codegraph" | "git"
              "commit": "d8198127b6bd14a3bf02dda1762ee13b7f3848ed", "indexedAt": "2026-10-05T05:55:17.289Z",
              "reason": "index",                                           // index | head | reindex
              "headCommit": "1b0c760935…", "stale": true } }
```

- `tool: "git"` (`reason: "head"`) báo HEAD đổi để backend cập nhật `stale` mà không cần chỉ mục đổi; trường này **không có** trong README v7 (Điều chỉnh hợp đồng).
- Sau mỗi job thành công, phát `reason: "reindex"` ngay (không chờ chu kỳ), và huỷ cache ngắn hạn (CR-CV-002 2.8).
- Lần đầu `watch` chỉ ghi nhớ trạng thái hiện tại, **không** phát ngay (tránh bão khi bật).
- Khi ws chưa mở (đang mất kết nối): bỏ thông báo (không xếp hàng vô hạn); sau khi nối lại, backend gọi `codeintel.watch` + `codeintel.status` để đồng bộ.
- Hỏng một phép đo (tệp không tồn tại) → bỏ qua chu kỳ đó, không lỗi.

`codeintel-notification-sink.ts`: mỗi lần `dispatchCodeIntelRpc` được gọi, lưu `makeNotifier(ws, state)` làm notifier hiện hành (kèm `ws` để kiểm `readyState === 1`); thông báo của job/watcher đi qua notifier hiện hành. Hệ quả: sau khi ws đổi, thông báo chỉ tới nơi khi backend gọi bất kỳ `codeintel.*` nào (ví dụ `codeintel.watch` lúc nối lại). Ghi rõ vì sao: thông báo gắn với `WireState` của từng kết nối (`encodeDataFrame(state, ...)`), không dùng được `ws` cũ.

### 2.7 Worktree liên kết

Chỉ mục của checkout chính không phản ánh worktree đang review (CR-CV-001 1.5). Hai hướng, chưa chọn (Q1): (a) `analyze` ngay trong thư mục worktree (GitNexus có thể tự đăng ký một mục registry mới; **chưa kiểm chứng**, và lại bị `analyze` làm bẩn registry/`.gitnexus` trong worktree), (b) `analyze --branch <tên>` (khe chỉ mục theo nhánh, chưa kiểm chứng). Cho tới khi chốt, `reindex` từ chối worktree liên kết như ở 2.2.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Luôn `--index-only` | Không làm bẩn working tree (2); đây là điểm ngăn lỗi lớn nhất |
| Thăm dò thay vì `fs.watch` | inotify, tệp lớn, hệ tệp từ xa; chi phí thấp (một `stat`) |
| `percent` có thể `null` | Không có định dạng tiến độ đã kiểm chứng; không bịa số |
| Job tồn tại qua mất kết nối | Làm mới dài hơn thời gian chớp mạng; theo tiền lệ `agent.spawn` PTY |
| Một job mỗi repo, một job toàn agent (mặc định) | `analyze` nặng CPU/đĩa, có rủi ro khoá DB |
| Chặn đọc GitNexus khi `analyze` chạy | Bảo thủ do bằng chứng `lbug.wal.missing-shadow.*` |
| Không bao giờ xoá/sửa tệp trong `.gitnexus/`/`.codegraph/` | Tránh hỏng chỉ mục của bên khác |
| Từ chối `reindex` worktree liên kết ở MVP | Kết quả sẽ gây hiểu lầm (làm mới chỉ mục của checkout khác) |
| Thêm `reindexStatus`, `reindexCancel`, `watch` | README v7 chỉ có `reindex`; thông báo có thể mất, cần truy vấn lại và huỷ; cần cách bật thăm dò |
| Nhật ký job trên đĩa nhỏ | Biết `interrupted` sau khởi động lại agent |

## 4. Tiêu chí chấp nhận

- [ ] `codeintel.reindex` trả `jobId` trong < 1 s; job chạy nền, các method `codeintel.*` khác vẫn đáp ứng.
- [ ] Mọi tiến trình `analyze` sinh ra có `--index-only`; sau một job thành công `git status --porcelain` của repo không có thay đổi mới ở `AGENTS.md`, `CLAUDE.md`, `.claude/`.
- [ ] Gọi `reindex` lần hai khi job đang chạy trả `CODEINTEL_REINDEX_IN_PROGRESS` với đúng `jobId`; không spawn tiến trình thứ hai.
- [ ] Trong lúc GitNexus `analyze` chạy, `codeintel.overview` trả `CODEINTEL_REINDEX_IN_PROGRESS`; method chỉ cần CodeGraph vẫn chạy (khi giai đoạn CodeGraph không chạy).
- [ ] `codeintel.reindexProgress` đủ các giai đoạn, `percent` là `null` khi không có dòng khớp mẫu, ≤ 1 thông báo/giây/job, `message` không chứa `$HOME`.
- [ ] Huỷ: tiến trình con và con của nó biến mất sau ≤ 15 s; `state: canceled`; `codeintel.reindexCancel` lặp lại không lỗi; sau huỷ chạy `verify` và nếu thất bại `indexHealth: "unknown"`.
- [ ] Mất kết nối ws giữa chừng: job vẫn chạy; sau nối lại `codeintel.reindexStatus` trả đúng trạng thái; thông báo kết thúc không mất (được phát khi có notifier hiện hành, nếu không backend phát hiện qua `reindexStatus`).
- [ ] Khởi động lại agent: job còn `running` trong nhật ký thành `interrupted`.
- [ ] Sau `reindex` thành công, `codeintel.indexChanged (reason: "reindex")` được phát và `status` phản ánh chỉ mục mới (`stale: false` nếu HEAD khớp).
- [ ] Sửa tay `meta.json` (hoặc `touch`) → `indexChanged (tool: gitnexus)` trong ≤ 15 s; commit mới → `indexChanged (tool: git, reason: head)` trong ≤ 20 s; `watch` bật lần đầu không phát thông báo.
- [ ] CodeGraph tự đồng bộ nhiều lần/phút chỉ phát ≤ 1 thông báo/30 s/repo.
- [ ] Worktree liên kết bị từ chối với `hint = reindex_linked_worktree_unsupported`.
- [ ] Không file nào tên `helpers/utils/common/misc`; không `max-lines` disable; `pnpm lint`, `pnpm test` trong `agent/` xanh.

## 5. Kiểm thử

- **Unit (Vitest):** các file ở 2.1. Dùng binary giả (script Node trong `PATH` tạm) để mô phỏng `analyze`: ghi dòng tiến độ, thoát 0/1, treo (để thử huỷ và timeout), bỏ qua `SIGTERM` (để thử `SIGKILL`), sinh tiến trình con (thử kill nhóm). Đồng hồ giả (`vi.useFakeTimers`) cho thăm dò và giới hạn tần suất.
- **Tích hợp trên bản sao repo nhỏ (không phải repo Orca):** `gitnexus analyze --index-only` thật và `codegraph index` thật trên một repo fixture vài chục tệp, kiểm `git status`, thời gian, định dạng đầu ra tiến độ; đây là bước **bắt buộc để chốt mẫu `percent`** (chưa làm).
- **Thử huỷ giữa chừng trên bản sao**: ghi lại có sinh `lbug.wal.missing-shadow.*` không, `status` sau đó ra sao (CR-CV-070).
- Tệp test: `agent/src/relay/codeintel-reindex-job.test.ts`, `codeintel-reindex-commands.test.ts`, `codeintel-reindex-progress.test.ts`, `codeintel-reindex-journal.test.ts`, `codeintel-index-watcher.test.ts`, `codeintel-notification-sink.test.ts`; sửa `agent/src/relay/__tests__/agent-session.test.ts` (kiểm `stop()` gọi dọn dẹp watcher nhưng không huỷ job). Chưa chạy gì ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Khoá/hỏng DB khi `analyze` chạy cùng các tiến trình `gitnexus mcp`/`codegraph serve --mcp` đã mở chỉ mục** (có ít nhất hai cặp trên máy khảo sát): chưa thử, chưa biết GitNexus xử lý ra sao. Nguy cơ mất chỉ mục dùng chung của nhiều agent trên cùng dev server. Giảm thiểu hiện tại: một job tại một thời điểm, chặn đọc từ agent, `--index-only`; **không thể** kiểm soát tiến trình ngoài.
- **Thời gian `analyze` Orca chưa đo** (mục 1.6); timeout 45 phút và `--workers` là giả định. Làm mới `full` có thể chiếm CPU/đĩa của dev server và làm chậm PTY.
- **Định dạng tiến độ** và liệu `analyze` có in `%` hay không: chưa biết; `percent` có thể luôn `null` trên thực tế.
- **Nguyên nhân `lbug.wal.missing-shadow.*`** chưa rõ; không nên coi là bình thường. `indicators` trong `status` chỉ để theo dõi.
- **Chỉ mục của worktree liên kết** chưa giải quyết (2.7); tính năng làm mới có thể vô dụng cho trường hợp review chính.
- `codegraph sync` thủ công đồng thời với daemon tự đồng bộ: khoá không rõ; `codegraph unlock` không được dùng.
- Thăm dò cách nhau 10-15 s tạo độ trễ phát hiện; chấp nhận được cho "index đã cũ".
- Nhật ký trong `~/.orca/codeintel/jobs/` phụ thuộc quyền ghi `HOME`; nếu không ghi được, bỏ qua (không lỗi) và mất khả năng báo `interrupted`.
- Windows chưa hỗ trợ (CR-CV-001); `detached` + kill nhóm trên Windows chưa thử.

## 7. Câu hỏi mở

- **Q1.** Cách làm mới chỉ mục cho worktree liên kết: `analyze` trong worktree hay `--branch`? Cần thử trên bản sao; ảnh hưởng thiết kế O4 và CR-CV-012.
- **Q2.** Có cho `reindex` đăng ký repo chưa có trong registry không (CR-CV-001 Q2)? Hiện không.
- **Q3.** Có nới quy tắc "chặn đọc GitNexus khi `analyze` chạy" sau khi thử nghiệm không?
- **Q4.** `ORCA_CODEINTEL_ANALYZE_WORKERS` mặc định hợp lý cho dev server của người dùng là bao nhiêu? Cần số đo thật.
- **Q5.** Quyền: ai được bấm làm mới? Agent không có khái niệm người dùng; backend (CR-CV-013) phải kiểm quyền ghi trước khi gọi `codeintel.reindex`. Có cần cờ môi trường `ORCA_CODEINTEL_REINDEX=off` trên agent để dev server tự cấm không? Đề xuất: có (`off` → `CODEINTEL_TOOL_UNAVAILABLE (reason = "reindex_disabled")`).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 2 O3, D4; mục 3.2)
- `/opt/repos/orca/docs/research/view-code/02-local-mcp-interaction.md` §4, `06-gaps-risks-roadmap.md` §2
- `gitnexus analyze --help`, `codegraph sync|index|unlock --help` (chạy ngày 2026-10-05)
- `/opt/repos/orca/.gitnexus/` (có `lbug.wal.missing-shadow.*`), `/opt/repos/orca/.codegraph/daemon.log`
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch.ts` (`makeNotifier` `:280`), `/opt/repos/orca/agent/src/relay/agent-session.ts` (`stop()` `:253`), `/opt/repos/orca/agent/src/relay/fs-agent-watch-extensions.ts`
- `/opt/repos/orca/agent/src/relay/agent-tool-registry.ts` (`runToolCommand`)
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md`, `CR-CV-002-gitnexus-extraction.md`, `CR-CV-003-codegraph-extraction.md`
