# CR-CV-080 — Chiến lược index cho worktree của agent và tự làm mới khi agent xong

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-080 |
| **Tên** | Chọn chiến lược index cho worktree do agent sửa (index riêng / `sync`+`analyze` nền / overlay theo diff), phân loại `indexScope` trung thực (`exact\|repo_root\|stale\|none`), tự kích hoạt làm mới khi agent xong lượt (backend nghe sự kiện, có debounce, hạn mức, chống chạy trùng), hiển thị "kết luận dựa trên index nào" (`IndexBasis`) |
| **Loại** | Feature (vận hành index; sửa mặc định O3, hiện thực hoá O10) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | [CR-CV-001](../agent-codeintel/CR-CV-001-codeintel-agent-foundation.md), [CR-CV-004](../agent-codeintel/CR-CV-004-codeintel-reindex-and-index-notifications.md), [CR-CV-012](../code-intel-service-foundation/CR-CV-012-project-worktree-to-repo-binding.md); sự kiện từ `infra-fleet-service` ([CR-CV-023](../code-intel-graph-pipeline/CR-CV-023-infra-fleet-codeintel-transport.md), [CR-CV-024](../code-intel-graph-pipeline/CR-CV-024-event-distribution.md)); quota/quyền ở [CR-CV-013](../code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md) |
| **Mở khoá** | [CR-CV-081](./CR-CV-081-quality-runner-on-agent.md) (ghi `indexCommit` của lần chạy), [CR-CV-082](./CR-CV-082-quality-finding-model-and-parsers.md) (`quality_runs.index_basis`), CR-CV-085 (cổng nêu "dựa trên index nào"), CR-CV-087 (chip), độ chính xác của CR-CV-036/037 |
| **Tác động** | Agent: `codeintel-method-table.ts`/`codeintel-status.ts` (mở rộng `codeintel.status`), `codeintel-reindex-job.ts` (tham số mới) — **không sửa file của CR-CV-001/004 trong CR này**, mục 2.8 nêu chỗ cần sửa. Backend: `code-intel-service` (use case mới `auto_refresh_index.go`, bộ nghe sự kiện, proto `IndexBasis`), `infra-fleet-service` (thêm trường vào payload sự kiện). Frontend: chip index ở CR-CV-051/087 (chỉ nêu hợp đồng) |

---

## 1. Bối cảnh và vấn đề

Đọc/chạy chỉ-đọc ngày 2026-10-06 trên máy khảo sát (`/opt/repos/orca`, GitNexus 1.6.9, CodeGraph 1.4.1). "chưa kiểm chứng" nghĩa là CR này không chạy lệnh ghi (`analyze`, `sync`, `index`).

### 1.1 Index luôn chậm hơn code vừa sinh, và worktree của agent không có index riêng

1. `gitnexus status` trong repo: `Indexed commit: d819812`, `Current commit: 1b0c760`, `Status: stale`. `meta.json` ghi `lastCommit d8198127…`, `indexedAt 2026-10-05T05:55:17Z`.
2. `git worktree list`: `/opt/repos/orca` (main), `/opt/repos/orca-deploy` (detached), `/opt/repos/orca/.claude/worktrees/dev-process-9beda3` (worktree liên kết; thư mục này bị loại bởi `.git/info/exclude:11`, cấu hình cục bộ của clone, không phải của repo). Worktree liên kết **không có** `.gitnexus/` (`ls` báo không tồn tại); `codegraph status <worktree> -j` trả `projectPath: /opt/repos/orca`, `worktreeMismatch: {worktreeRoot: …/dev-process-9beda3, indexRoot: /opt/repos/orca}`, và `pendingChanges: {0,0,0}`. Với thư mục không có index (`/opt/repos/orca-deploy`) CodeGraph trả `{"initialized":false,…}`.
3. **`pendingChanges` của CodeGraph ở worktree liên kết là của checkout chính, không phải của worktree.** Hệ quả trực tiếp: `pendingChanges = 0` không chứng minh index phản ánh code agent vừa sửa trong worktree. Điều này lật lại giả định ở research 11 §2 và CR-CV-001 F8 ("CodeGraph dựa `pendingChanges`") khi áp cho worktree liên kết.
4. Dung lượng index Orca (`du -sh`, đọc): `.gitnexus` 2,7 GB (`lbug` 1,6 GB, `parse-cache` 357 MB, `parsedfile-cache` 739 MB) + `.codegraph` 1,3 GB ≈ **4 GB mỗi bản index**. Máy khảo sát: 32 lõi, 31 GB RAM, 12 GB đang dùng. Index riêng cho từng worktree (phương án a) nhân con số này với số worktree agent (chưa đo thời gian `analyze`, xem 6).
5. Tệp trong `.gitnexus/`: `lbug.wal.missing-shadow.*` (4 tệp, đã ghi ở CR-CV-004 1.4), `parse-cache/` và `parsedfile-cache/` với tên tệp là chuỗi hex 64 ký tự (giống SHA-256). Gợi ý cache theo nội dung; **khoá cache thật (nội dung hay đường dẫn+mtime) chưa kiểm chứng**.

### 1.2 Hai công cụ cho phép gì (từ `--help`, không chạy)

| Lệnh | Điều đọc được | Hệ quả |
|---|---|---|
| `gitnexus analyze [path]` | `-f/--force`: "Force full re-index even if up to date" (nghĩa là bình thường bỏ qua khi đã mới); **không có cờ `--incremental`**; `--index-only` (không chèn `AGENTS.md`/`CLAUDE.md`/skills); `--branch <name>`: "per-branch index slot (multi-branch indexing)… without this flag, analyze always updates the workspace index, which follows the checked-out working tree"; `--skip-git` ("Treat the provided path/cwd as the index root and skip parent git-root discovery"); `--name <alias>`, `--allow-duplicate-name`; `--workers`, `GITNEXUS_WORKER_POOL_SIZE` | Có "khe theo nhánh" nhưng chưa rõ nó nằm ở `.gitnexus/` của repo nào và đăng ký registry ra sao; `analyze` luôn là lệnh toàn repo, không có "chỉ các tệp đổi" |
| `gitnexus detect-changes` | `--scope unstaged\|staged\|all\|compare`, `--base-ref`, `--branch`, `-r`, `-l` | Ánh xạ diff → symbol **dựa trên index đã có**; symbol mới (chưa có trong index) không thể ánh xạ |
| `codegraph sync [path]` | "Sync changes since last index", `-q` | Đồng bộ theo **gốc index**. Với worktree liên kết, gốc index là checkout chính (1.1.2); `sync` có đọc cây của worktree hay của checkout chính: **chưa kiểm chứng** |
| `codegraph status -j` | `pendingChanges{added,modified,removed}`, `worktreeMismatch{worktreeRoot,indexRoot}`, `index.state`, `lastIndexed` | Không có commit; `lastIndexed` không đổi khi daemon tự đồng bộ (CR-CV-004 1.5) |
| `codegraph affected [files…]` | "Find test files affected by changed source files" (`-j`, `--stdin`, `-d`) | Dùng được cho phạm vi `changed` của test (CR-CV-081), nhưng cũng dựa trên index |
| `.codegraph/daemon.*` | `daemon.pid`, `daemon.sock`, `daemon.log` | Daemon tự đồng bộ (đã thấy dòng `Auto-synced` ở CR-CV-004): với checkout chính CodeGraph gần như luôn tươi; **không** với worktree liên kết |

### 1.3 Tín hiệu "agent xong" thật sự có ở đâu

README v7 O10 và research 11 A2 giả định `agent.hook`/`agent-status`. Kiểm tra thực tế:

| Tín hiệu | Nơi có | Tới được `code-intel-service` không |
|---|---|---|
| `agent.hook` (JSON-RPC notification) | Phát bởi `RelayAgentHookServer` (`agent/src/relay/agent-hook-server.ts:112`), chỉ được dựng ở `relay.ts:554` (Part B) và `wsl-agent-hook-relay.ts:49`; **không** có trong Part A (grep `RelayAgentHookServer`, `'agent.hook'` ở `agent/src/relay/agent-session*.ts`, `agent-rpc-dispatch*.ts`: không có). Go định tuyến nó (`devserveragent/session.go:438`) nhưng chỉ giải mã `worktreeId`, `ptyId`, `providerSession` (`session.go:540-552`), **không có `state`** | Không dùng làm tín hiệu "xong" ở direct-websocket |
| Trạng thái phiên agent do backend khởi (`StartAgentSession`) | `agent_sessions.status` ∈ `spawning\|idle\|running\|waiting\|completed\|error\|stopped` (`infrafleet.proto:1569-1582`), tính bởi `AgentOutputClassifier` (`usecase/agent_output_classifier.go`) từ luồng PTY (OSC 133) | **Có**: sự kiện NATS `orca.infra.agent.statusChanged` `{session_id,status}` (`adapter/eventbus/agent_status_publisher.go:24-30`), publish **trực tiếp, không qua outbox** (ghi chú đầu tệp) nên at-most-once. Payload **không có** `worktree_id`/`dev_server_id` |
| PTY của agent thoát | `orca.infra.terminal_session.agent_completed` `{pty_id, connection_id, agent_kind, exit_code, user_ids}` (`adapter/eventbus/publisher.go:25-37`, phát từ `usecase/attach_pty.go:220`) | Có, nhưng chỉ khi tiến trình thoát, không phải "xong một lượt" của phiên tương tác |
| `agentStatus.state` ∈ `working\|blocked\|waiting\|done` | Frontend (`frontend/src/shared/agent-status-types.ts:15`, `store/slices/agent-status.ts` `setAgentStatus` có `routing.worktreeId`), nguồn: OSC 9999/IPC hook phía renderer | Chỉ ở trình duyệt/Electron, **backend không thấy** |
| Không có RPC lấy `AgentSession` theo `session_id` | `infrafleet.proto:330` `ListAgentSessions` chỉ lọc theo `origin_*`, `active_only`; không có `GetAgentSession` | Muốn ánh xạ `session_id` → `worktree_id` phải thêm trường vào payload hoặc thêm RPC (mục 2.2, 2.8) |

Kết luận: tín hiệu khả dụng sạch nhất là `orca.infra.agent.statusChanged`; phiên agent tương tác do người dùng mở thẳng trong terminal (không qua `StartAgentSession`) **không** có tín hiệu ở backend ngoài việc UI báo.

### 1.4 Vấn đề cần giải

1. Chọn chiến lược index cho worktree của agent dựa trên chi phí đo được và những gì công cụ thật sự làm (chưa đo → phải có phép thử và cổng quyết định).
2. Tự làm mới khi agent xong, không phụ thuộc người bấm, mà **không** tạo bão `analyze`, không giết job dở, không chạy trùng.
3. Luôn nói rõ index đang dùng: commit, phạm vi (`exact|repo_root|stale|none`), có chứa thay đổi chưa commit của agent hay không.

## 2. Giải pháp đề xuất

### 2.1 Ba phương án và quyết định

| | (a) Index riêng từng worktree | (b) `codegraph sync` + `gitnexus analyze --index-only` ở nền | (c) Overlay: index nền ở merge-base + phân tích nhanh phần đổi |
|---|---|---|---|
| Ý tưởng | `analyze`/`init` ngay trong thư mục worktree | Làm mới index hiện có sau mỗi lượt | Giữ index của checkout chính; phủ kết quả `detectChanges`/diff lên |
| Chi phí đĩa | ~4 GB mỗi worktree (1.1.4) | 0 thêm | 0 thêm |
| Chi phí thời gian | Toàn repo mỗi worktree; số đo chưa có; lần đầu chắc chắn lâu | `sync`: chưa đo; `analyze` lặp lại: chưa đo (cache có thể giúp, 1.1.5) | Rẻ (một lần `git diff` + truy vấn đọc) |
| Đúng cho worktree liên kết | Có (nếu CLI tự đăng ký đúng, chưa kiểm chứng) | **Không chứng minh được**: gốc index là checkout chính (1.1.2, 1.2) | Một phần: symbol đã có ở index nền ánh xạ được; **symbol mới của agent thì không** |
| Đúng cho agent làm trong checkout chính | Không cần | **Có** (gốc index = nơi agent sửa) | Không cần |
| Rủi ro | Registry khoá theo tên (CR-CV-012 6), va chạm tên; đĩa; nhiều `lbug` mở | Khoá DB khi đọc đồng thời (CR-CV-004 6); `analyze` làm bẩn cây nếu thiếu `--index-only` | Kết luận thiếu cho code mới; phải gắn nhãn |
| Trạng thái kiểm chứng | Chưa chạy | Chưa chạy | CR-CV-005 đã mô tả phần diff → symbol |

**Quyết định (thay đổi cách đọc O10, nêu ở "Điều chỉnh hợp đồng")**

1. **Mặc định `index_policy = auto_in_place`**: tự làm mới **chỉ khi gốc index trùng `workspaceRoot`** (agent sửa trong checkout chính, hoặc worktree đã có index riêng): tier 1 `codegraph sync`, tier 2 `gitnexus analyze --index-only` (mục 2.3). Đây đúng ý O10 (b).
2. **Với worktree liên kết chưa có index riêng (trường hợp phổ biến của agent)**, (b) không làm mới thứ người review nhìn. Hệ thống **không giả vờ**: dùng overlay (c) làm nền (CR-CV-005/036), đặt `indexScope = repo_root` kèm `IndexBasis.changedFilesNotInIndex`, và **không** chạy `analyze` ở checkout chính chỉ vì worktree đổi (chạy sẽ không phản ánh code agent, chỉ tốn tài nguyên).
3. **(a) là chế độ chọn bật** (`index_policy = per_worktree`), mặc định tắt, chỉ mở sau khi các phép thử M1-M6 (2.7) cho kết quả chấp nhận được (thời gian, đĩa, đăng ký registry sạch). Khớp O10 "chỉ làm sau khi đo".
4. `index_policy = off` để tắt tự làm mới theo tenant/project (quay về thủ công, như O3).

Cấu hình `index_policy` đặt ở đâu: cột mới `repo_bindings.index_policy` hoặc `tenant_settings` — chưa chốt (Q2); tạm đề xuất theo tenant ở `tenant_settings` (CR-CV-011) vì CR-CV-073 đã sở hữu cờ theo tenant.

### 2.2 Ai kích hoạt: backend nghe sự kiện, không phải agent tự chạy

| Phương án | Ưu | Nhược | Chọn |
|---|---|---|---|
| Agent tự chạy `analyze` khi phát hiện PTY agent xong | Ít hop | Agent không biết tenant/quyền/hạn mức (CR-CV-013), không có audit, khó dedupe nhiều agent trên cùng máy, agent không thấy trạng thái `agent_sessions` | Không |
| **`code-intel-service` nghe sự kiện, quyết định, gọi `codeintel.reindex` qua `RelayByDevServer`** | Có quyền/quota/audit/cờ tenant (O8), một nơi dedupe (`reindex_jobs.active_key`), agent vẫn hẹp (D5) | Cần sự kiện có `worktree_id`; `statusChanged` at-most-once | **Có** |
| UI báo khi `agentStatus=done` | Bao phủ cả terminal tương tác | Chỉ khi có người mở UI; cần kênh mới | Phụ (P1, mục 2.2.3) |

#### 2.2.1 Nguồn sự kiện và điều kiện "xong"

Consumer bền mới `auto_refresh_index_consumer.go` (mẫu consumer `orca.project.worktree.deleted` ở CR-CV-012 2.6; dedup bằng `processed_events`):

| Sự kiện | Điều kiện kích hoạt | Ghi chú |
|---|---|---|
| `orca.infra.agent.statusChanged` | `status` chuyển từ `running` sang `idle`, `completed`, `waiting`, `error` hoặc `stopped` | Cần `worktree_id`, `dev_server_id` trong payload: **đề xuất thêm hai trường tuỳ chọn, tương thích ngược**, nếu chưa có thì tra `ListAgentSessions{active_only:false}` rồi lọc theo `id` (không có bộ lọc theo id; chỉ là phương án tạm, tối đa 500 dòng/lần) |
| `orca.infra.terminal_session.agent_completed` / `agent_error` | PTY agent thoát | Dự phòng cho trường hợp `statusChanged` mất; ánh xạ `connection_id` → dev server, **không có worktree**, nên chỉ dùng để đánh dấu "cần kiểm tra lại index của dev server này" (kích hoạt `codeintel.status` cho các binding đang mở) |
| `codeintel.indexChanged {tool:"git", reason:"head"}` (CR-CV-004) | HEAD đổi | Chỉ cập nhật `stale`, **không** kích hoạt `analyze` |
| UI gọi `codeIntel.hintAgentTurnFinished {projectId, worktreeRef}` (kênh mới, P1) | Renderer thấy `agentStatus.state === 'done'` | Thêm một kênh vào 26 kênh của CR-CV-040 (Điều chỉnh hợp đồng); bỏ qua nếu đã có refresh trong cửa sổ debounce |

`agent.hook` **không** được dùng ở MVP (1.3). Nếu sau này Part A dựng `RelayAgentHookServer`, Go đã định tuyến sẵn; khi đó cần thêm `state` vào `agentHookNotificationParams` (ngoài phạm vi).

#### 2.2.2 Quy trình quyết định (use case `AutoRefreshIndex.Handle`)

```
sự kiện ─▶ [1] cờ tenant code_intel_enabled và index_policy != off
       ─▶ [2] ResolveTarget(worktree) (CR-CV-012) → binding; dev server online & approved
       ─▶ [3] debounce theo binding: chờ CODEINTEL_AUTOREFRESH_QUIET (mặc định 20 s) không có sự kiện mới; tối đa chờ 120 s
       ─▶ [4] codeintel.status → IndexBasis hiện tại (2.4)
       ─▶ [5] RefreshPlan:
              - scope == exact và freshness == fresh                       → không làm gì
              - gốc index == workspaceRoot (exact|stale)                    → tier 1 (codegraph sync), rồi tier 2 nếu thoả (dưới)
              - gốc index != workspaceRoot (repo_root) và policy auto_in_place → KHÔNG chạy lệnh; ghi basis=repo_root, phát chip
              - policy per_worktree                                          → tier 3 (tạo/làm mới index worktree, 2.3.3)
       ─▶ [6] kiểm hạn mức/tải (dưới) ─▶ ReindexJobRepository.Create (active_key) ─▶ RelayByDevServer codeintel.reindex {trigger:"agent_done", …}
```

Điều kiện tier 2 (`gitnexus analyze --index-only`): số tệp đổi kể từ `indexedCommit` ≥ `CODEINTEL_AUTOANALYZE_MIN_FILES` (mặc định 1, chỉnh được) **và** không có `analyze` nào của repo đang chạy/đã chạy trong `CODEINTEL_AUTOANALYZE_MIN_INTERVAL` (mặc định 10 phút) **và** tải máy cho phép (dưới). Nếu chưa thoả, ghi `refresh_state="deferred"` và đặt hẹn lại một lần sau khoảng đó. Con số mặc định là giả định, chưa đo (xem 6).

Hạn mức và chống chạy trùng:

| Cơ chế | Quy tắc |
|---|---|
| Một job mỗi repo | `reindex_jobs.active_key` UNIQUE (CR-CV-011); vi phạm → coi như đã có job, gắn vào `jobId` đó, không tạo job mới |
| Khoá theo dấu vết | `dedupe_key = sha256(binding_id ‖ headCommit ‖ dirtyFingerprint)`; cùng khoá đã xử lý trong 5 phút thì bỏ (idempotent với at-least-once của consumer và at-most-once của `statusChanged`) |
| Mỗi dev server | tối đa 1 job nặng (`codeintel.reindex` toàn tool hoặc `quality.run`, dùng chung cổng nặng ở agent, CR-CV-081 2.9) |
| Mỗi tenant | Tối đa `CODEINTEL_AUTOREFRESH_PER_HOUR` job tự động (mặc định 12) dùng cơ chế hạn mức của CR-CV-013 (mã `CODEINTEL_RATE_LIMITED`); vượt thì bỏ và ghi metric, **không** xếp hàng vô hạn |
| Tải máy | Agent trả `loadavg1`, số lõi trong `codeintel.status` (trường mới `host`); backend hoãn tier 2 nếu `loadavg1 > 0,7 × cores` hoặc đang có `quality.run` |
| Sự kiện bão | Tối đa 1 kế hoạch/binding/`QUIET` nhờ debounce; sự kiện đến khi job đang chạy chỉ ghi `pendingRefresh=true`, chạy lại **một lần** sau khi job kết thúc nếu `IndexBasis` còn lệch |

Quyền: job tự động chạy với **tác nhân hệ thống**. CR-CV-011 đặt `reindex_jobs.requested_by NOT NULL` và CR-CV-013 kiểm quyền ghi theo người dùng; cần mở rộng (mục 2.8): `requested_by` cho phép NULL (như `tenant_settings.updated_by`), cột `trigger` (`manual|agent_done|head_change|schedule`), `trigger_event_id`; audit ghi `actor_type=system`. Chính sách `index_policy` là sự cho phép của tenant admin thay cho quyền từng người.

#### 2.2.3 Huỷ khi agent chạy tiếp

Sự kiện `statusChanged → running` cho cùng worktree trong lúc có job tự động:

| Trạng thái job | Hành động |
|---|---|
| Chưa gửi tới agent (đang debounce/hàng đợi backend) | Huỷ kế hoạch, đặt hẹn lại sau lượt tiếp |
| `codegraph.sync` đang chạy | Cho chạy hết (ngắn, thao tác lặp được), rồi `verify` |
| `gitnexus.analyze` đang chạy | **Không kill** mặc định (`CODEINTEL_AUTOANALYZE_CANCEL_ON_RESUME=false`): kill giữa chừng có thể để lại `lbug.wal.missing-shadow.*` hoặc DB dở (CR-CV-004 2.5, 6; chưa kiểm chứng). Khi kết thúc, `verify` so sánh với `headCommit` + `dirtyFingerprint` mới nhất; lệch thì đánh dấu `freshness=stale` và lập kế hoạch lại sau khi agent xong |
| Bật `CANCEL_ON_RESUME=true` | Gọi `codeintel.reindexCancel`, bắt buộc `verify` sau huỷ như CR-CV-004 2.5; chỉ nên bật khi M6 (2.7) cho kết quả an toàn |

### 2.3 Hợp đồng agent (mở rộng `codeintel.status` và `codeintel.reindex`)

Đây là phần **đề xuất cho CR-CV-001/004 sửa theo**; mục 2.8 liệt kê chính xác chỗ.

#### 2.3.1 `codeintel.status` thêm khối `indexBasis` cho mỗi công cụ

```jsonc
// result.data.indexes.gitnexus (thêm các trường, giữ nguyên trường cũ của CR-CV-001 2.2)
{ "state": "ready", "indexedCommit": "d8198127b6…", "indexedAt": "2026-10-05T05:55:17.289Z",
  "indexRoot": "/opt/repos/orca",          // đường dẫn đã đăng ký (registry) hoặc projectPath (CodeGraph)
  "indexScope": "repo_root",               // exact | repo_root | stale | none (2.4)
  "headCommit": "1b0c760935…", "mergeBase": "d8198127b6…",   // merge-base(HEAD, base mặc định O7)
  "dirtySinceIndex": true,                 // có tệp đổi (đã commit sau indexedCommit, hoặc chưa commit) có mtime/commit mới hơn index
  "changedFilesNotInIndex": 42,            // số tệp đổi so với indexedCommit (tính bằng git, không bằng index)
  "pendingChanges": null }                 // CodeGraph: chỉ có nghĩa khi indexRoot == workspaceRoot (1.1.3), ngược lại null
// result.data.host
{ "platform": "linux", "cores": 32, "loadavg1": 3.2, "freeMemBytes": 7700000000 }
```

Cách tính (agent, chỉ đọc, ghi vào `codeintel-index-basis.ts`, **mới**):

1. `indexRoot`: GitNexus = `registeredPath` khớp theo CR-CV-001 2.4; CodeGraph = `projectPath` từ `status -j`. `rootMatches = realpath(indexRoot) === realpath(workspaceRoot)`.
2. `headCommit = git rev-parse HEAD`. `mergeBase = git merge-base HEAD <baseRef>` với `baseRef` do backend truyền (O7) hoặc `origin/HEAD`; không tính được thì `null`.
3. `changedFilesNotInIndex`: `git diff --name-only <indexedCommit>..HEAD` (commit) **cộng** `git status --porcelain=v1 -z` (chưa commit; `-z` có từ Git 1.7, nằm dưới baseline 2.25 của `git-compatibility.md`). `indexedCommit` không còn trong object store (đã gc) → `null` và `freshness` suy ra `stale`.
4. `dirtySinceIndex`: tồn tại tệp trong tập ở bước 3 có `mtime > indexedAt`, hoặc `changedFilesNotInIndex > 0`. Lý do không dựa `lastCommit`: GitNexus index theo **cây làm việc** lúc `analyze` ("follows the checked-out working tree", `--branch` help), nên `indexedCommit == HEAD` vẫn có thể lệch nếu sau đó có sửa chưa commit (agent thường chưa commit).
5. Chi phí: vài lệnh git (tens ms trên repo Orca là **ước lượng**, chưa đo); cache 5 s như `headCommit` của CR-CV-001 2.6.

#### 2.3.2 `codeintel.reindex` tham số mới (không phá hợp đồng)

| Tham số | Kiểu | Ý nghĩa |
|---|---|---|
| `trigger` | `"manual"\|"agent_done"\|"head_change"` | Ghi vào nhật ký job và `indexChanged`; `agent_done` đổi hành vi worktree liên kết (dưới) |
| `ifStale` | boolean (mặc định `false`) | `true`: nếu `IndexBasis.freshness === "fresh"` thì trả `outcome:"already_up_to_date"` ngay, không spawn |
| `expectHead` | string? | Nếu `HEAD` hiện tại khác thì trả `outcome:"superseded"`, không chạy (chống làm việc lỗi thời khi agent đã commit tiếp) |
| `tiers` | `("sync"\|"analyze")[]` | Thay cho `mode`/`tools` khi cần chọn tier; `sync` ↔ `codegraph sync -q`, `analyze` ↔ `gitnexus analyze --index-only` (đúng bảng lệnh CR-CV-004 2.2, không thêm cờ) |

Với `workspaceRoot` là worktree liên kết, `trigger:"agent_done"`: **không** trả `CODEINTEL_PATH_NOT_ALLOWED (reindex_linked_worktree_unsupported)` như CR-CV-004 2.2 hiện nay (lỗi sẽ chảy thành nhiễu mỗi lượt agent). Trả thành công với `outcome:"skipped_scope_repo_root"`, `skipped:[{tool, reason:"index_root_is_main_checkout"}]`. Với `trigger:"manual"` giữ nguyên hành vi lỗi cho tới khi `per_worktree` được bật.

`codeintel.indexChanged` thêm `indexScope`, `mergeBase`, `trigger`.

#### 2.3.3 Chế độ `per_worktree` (tắt mặc định)

Chỉ sau khi M1-M6 đạt. Dự kiến: chạy trong thư mục worktree `gitnexus analyze --index-only --skip-git?` hoặc `--branch <slug>` (chưa chọn, tuỳ M3/M4) và `codegraph init`/`index <worktree>`; đăng ký registry với tên duy nhất (`--name <repo>@<wt-slug>`); giới hạn số index worktree đồng thời và tổng đĩa (`CODEINTEL_PERWT_MAX_INDEXES`, `…_MAX_BYTES`); dọn khi `orca.project.worktree.deleted` (CR-CV-012 2.6) **bằng một lệnh xoá do CR khác mô tả**, vì CR-CV-001 cấm `clean`/`remove` (mục 6). Đây là rủi ro lớn nhất của (a): quy tắc "không bao giờ xoá `.gitnexus/`" mâu thuẫn với nhu cầu dọn; nên tách thành CR riêng khi M-tests đạt.

### 2.4 `indexScope` và `IndexBasis`

Hàm thuần `classifyIndexBasis(input) → {indexScope, freshness}` (agent, `codeintel-index-basis.ts`, kèm bảng test), đánh giá theo thứ tự, điều kiện đầu đúng thì dừng:

| # | Điều kiện | `indexScope` | `freshness` |
|---|---|---|---|
| 1 | Không có index của công cụ này (`state=missing`, hoặc không khớp registry) | `none` | `unknown` |
| 2 | Công cụ không dùng được (`incompatible`, binary vắng) | `none` | `unknown` |
| 3 | `rootMatches` và `indexedCommit === headCommit` và `!dirtySinceIndex` và (CodeGraph: `pendingChanges` toàn 0) | `exact` | `fresh` |
| 4 | `rootMatches` và (`indexedCommit !== headCommit` hoặc `dirtySinceIndex` hoặc `pendingChanges` > 0) | `stale` | `stale` |
| 5 | `!rootMatches` (worktree liên kết dùng index của checkout chính) và `indexedCommit === mergeBase` (index đúng nền của nhánh) | `repo_root` | `fresh_base` |
| 6 | `!rootMatches` và `indexedCommit !== mergeBase` | `stale` | `stale` |

Tên giá trị khớp README 3.10 (`exact|repo_root|stale|none`). Quy ước đã chốt để tránh va chạm với CR-CV-012: **`indexScope` ở kết quả chất lượng là giá trị phái sinh đầy đủ ở trên**; cột `repo_bindings.index_scope` (CR-CV-011: `exact|repo_root|unresolved`) vẫn lưu **vị trí** gốc index (`exact` khi `rootMatches`, `repo_root` khi không, `unresolved` ↔ `none`), còn độ tươi nằm trong `last_status` JSON. Không đổi CHECK của CR-CV-011.

Message proto `IndexBasis` (`orca.codeintel.v1`, `codeintel_index_basis.proto`, **mới**; CR này sở hữu, dùng lại bởi `QualityRun` ở CR-CV-082):

```proto
message IndexBasis {
  string tool = 1;                       // "gitnexus" | "codegraph"
  string index_scope = 2;                // exact | repo_root | stale | none
  string freshness = 3;                  // fresh | fresh_base | stale | unknown
  string indexed_commit = 4;
  string head_commit = 5;
  string merge_base = 6;
  google.protobuf.Timestamp indexed_at = 7;
  bool dirty_since_index = 8;
  int32 changed_files_not_in_index = 9;
  string refresh_state = 10;             // idle | queued | running | deferred | failed | skipped
  string trigger = 11;                   // manual | agent_done | head_change
  string tool_version = 12;
  string index_policy = 13;              // auto_in_place | per_worktree | off
}
```

**Tín hiệu nào phụ thuộc index, tín hiệu nào không** (để hiển thị đúng, không gắn nhãn thừa):

| Tín hiệu | Dùng index? | Hiển thị `IndexBasis`? |
|---|---|---|
| Kết quả lint, typecheck, test, vet, buf, opa, script `check-*` (CR-CV-081/082) | **Không** (chạy trên cây làm việc) | Không; chỉ ghi `headCommit` + `dirtyFingerprint` của lần chạy |
| Test gap, ảnh hưởng, vi phạm lớp, hotspot, dead code, luồng bị ảnh hưởng (CR-CV-036/037) | Có | Có, **bắt buộc** |
| `diff coverage` (CR-CV-083) | Không (dựa dòng đã đổi) | Không |
| Suy luận "test phủ symbol" khi thiếu coverage | Có | Có, nhãn "ước lượng" |

Quy tắc nêu kết luận (mẫu câu cho UI; chuỗi i18n do CR-CV-087 sở hữu):

| `indexScope` | Câu hiển thị |
|---|---|
| `exact` | "Dựa trên index tại `<indexedCommit7>` (khớp worktree)" |
| `repo_root` | "Dựa trên index của nhánh gốc tại `<indexedCommit7>`; `<n>` tệp thay đổi của worktree chưa có trong index" |
| `stale` | "Index cũ (`<indexedCommit7>`, HEAD `<head7>`): kết luận phụ thuộc đồ thị có thể thiếu; đang làm mới / chưa làm mới" |
| `none` | "Chưa có index: bỏ qua các kiểm tra dựa trên đồ thị" |

Cổng chất lượng (CR-CV-085) khi một kiểm tra **cần index** mà `indexScope ∈ {stale, none}`: kết luận phần đó là `unknown`, không `pass` (khớp README 3.10 "`unknown` khi thiếu dữ liệu, không bao giờ suy diễn thành `pass`").

### 2.5 Lưu và truyền

- `quality_runs` (CR-CV-082 2.7) có `index_commit` (README) và cột `index_basis` JSON (≤ 4 KiB) là ảnh chụp `IndexBasis[]` tại lúc kết thúc lần chạy; `QualityGate.basedOn.indexCommit/stale` (README 3.10) lấy từ đó.
- `IndexStatus` (CR-CV-012 2.5) thêm `indexBasis[]`; `overall` thêm giá trị `OVERLAY` cho `indexScope=repo_root` + `fresh_base` (Điều chỉnh: hiện bảng 2.5 dòng 8 xếp `repo_root` vào `STALE`, gây báo "cũ" cho mọi worktree liên kết dù index nền đúng).
- Đẩy UI: `codeIntel.changed` (README 3.7) mang `indexScope`/`freshness` mới; chip index ở CR-CV-051 hiển thị theo bảng câu ở 2.4.

### 2.6 Số đo và metric (CR-CV-071)

`orca_codeintel_auto_refresh_total{outcome=skipped|deduped|rate_limited|ran|failed|superseded}`, `orca_codeintel_auto_refresh_seconds{tier}` (từ sự kiện agent xong đến `IndexBasis.freshness` đạt), `orca_codeintel_index_basis_total{scope}`; không gắn nhãn đường dẫn/tên repo.

### 2.7 Phép thử bắt buộc trước khi chốt (chạy trên **bản sao nhỏ**, không trên Orca)

CR này không chạy chúng. Người triển khai ghi kết quả vào PR kèm số đo; mỗi dòng có tiêu chí quyết định:

| # | Phép thử | Câu hỏi | Kết quả quyết định |
|---|---|---|---|
| M1 | Repo mẫu 2 worktree: sửa tệp ở worktree liên kết, chạy `codegraph sync <worktree>`, rồi `codegraph status -j` ở cả hai | `sync` đọc cây nào? `pendingChanges` đổi ở đâu? | Nếu không phản ánh worktree: xác nhận 2.1 điểm 2; nếu có: (b) dùng được cho worktree liên kết, bỏ nhánh `repo_root` cho CodeGraph |
| M2 | `gitnexus analyze --index-only` lần 2 không đổi gì; rồi sau khi sửa 1 tệp | Có bỏ qua khi "up to date" không? Thời gian lần sửa 1 tệp so với lần đầu (cache `parse-cache` hiệu quả không)? | Nếu lần sửa 1 tệp ≪ lần đầu: tier 2 tự động hợp lý; nếu ≈ lần đầu: tier 2 chỉ chạy khi `MIN_INTERVAL` dài, hoặc chỉ thủ công |
| M3 | `analyze --index-only` ngay trong worktree liên kết | Có tạo `.gitnexus/` trong worktree không? Registry thêm mục gì (tên, đường dẫn)? Xung đột tên với checkout chính? Có ghi `AGENTS.md`/`CLAUDE.md` (phải không, nhờ `--index-only`)? | Quyết định `per_worktree` khả thi hay không |
| M4 | `analyze --branch <slug>` ở checkout chính | Khe nhánh nằm ở đâu, `-r`/`detect-changes --branch` dùng ra sao, tốn bao nhiêu đĩa | Phương án thay thế của M3 |
| M5 | Truy vấn `gitnexus cypher` liên tục trong khi `analyze` chạy | Đọc có lỗi/ treo/ hỏng không | Giữ hay nới quy tắc "chặn đọc khi analyze" (CR-CV-004 Q3) |
| M6 | Kill (`SIGTERM`) `analyze` giữa chừng, rồi `gitnexus status` và một truy vấn | Có sinh `lbug.wal.missing-shadow.*`? Index còn dùng được? | Có bật `CANCEL_ON_RESUME` hay không |
| M7 | Thời gian/RAM `codegraph sync` khi 1 tệp, 100 tệp đổi; có xung đột khoá với daemon (`daemon.sock`) | Ngưỡng tier 1 | Chỉnh `QUIET` và hạn mức |

### 2.8 Sửa/mở rộng CR-CV-004, CR-CV-012 và CR khác (đề xuất; **không sửa file của họ**)

| CR | Mục | Hiện tại | Đề xuất |
|---|---|---|---|
| CR-CV-004 | 2.2 (từ chối worktree liên kết) | Trả `CODEINTEL_PATH_NOT_ALLOWED` với `hint=reindex_linked_worktree_unsupported` | Giữ cho `trigger:"manual"`; với `trigger:"agent_done"` trả thành công `outcome:"skipped_scope_repo_root"` |
| CR-CV-004 | 2.2 tham số | `workspaceRoot, mode, tools` | Thêm `trigger`, `ifStale`, `expectHead`, `tiers` (2.3.2) |
| CR-CV-004 | 2.5 huỷ | Huỷ khi người dùng gọi | Thêm quy tắc "agent chạy tiếp" ở 2.2.3; mặc định không kill `analyze` |
| CR-CV-004 | 2.6 `indexChanged` | `{workspaceRoot, tool, commit, indexedAt, reason, headCommit, stale}` | Thêm `indexScope`, `mergeBase`, `trigger` |
| CR-CV-004 | 3 mặc định "người dùng bấm" | O3 | Thay bằng O10 theo 2.1; nút thủ công vẫn còn |
| CR-CV-001 | 2.2 `codeintel.status` | `indexes.<tool>.state` ∈ `missing\|building\|ready\|stale\|unknown` | Thêm `indexRoot`, `indexScope`, `freshness`, `dirtySinceIndex`, `changedFilesNotInIndex`, `mergeBase`; khối `host` (2.3.1) |
| CR-CV-001 | 2.6 `stale` | `indexedCommit ≠ HEAD` hoặc `worktreeMismatch` | Thay bằng `freshness` ở 2.4; `pendingChanges` chỉ hợp lệ khi `rootMatches` |
| CR-CV-003 | 2.1 probe | `pendingChanges` dùng cho độ tươi | Chỉ khi `rootMatches` (1.1.3) |
| CR-CV-012 | 2.5 bảng `overall` dòng 8 | `repo_root` → `STALE` + `scopeMismatch` | Tách: `repo_root`+`fresh_base` → `OVERLAY`; `stale` giữ `STALE` |
| CR-CV-012 | 2.5 | `indexScope: exact\|repo_root\|none` từ agent | Nhận thêm `stale`; lưu vị trí vào `index_scope`, độ tươi vào `last_status` (2.4) |
| CR-CV-011 | `reindex_jobs` | `requested_by NOT NULL`, không có `trigger` | `requested_by` NULL cho hệ thống; thêm `trigger`, `trigger_event_id` |
| CR-CV-013 | quyền reindex | Theo người dùng | Thêm tác nhân hệ thống theo `index_policy`; hạn mức job tự động |
| CR-CV-023/024 | payload sự kiện | `statusChanged {session_id,status}` | Thêm `worktree_id`, `dev_server_id` (tuỳ chọn, tương thích ngược); consumer mới ở `code-intel-service` |
| CR-CV-040 | danh sách kênh | 26 kênh | Thêm `codeIntel.hintAgentTurnFinished` (P1) |
| README v7 | O3, O10; mục 3.2 | "Người dùng bấm" / "tự kích hoạt khi agent xong (`agent.hook`)" | Theo 2.1-2.2 và mục "Điều chỉnh hợp đồng" của báo cáo |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Backend kích hoạt, agent không tự chạy `analyze` | Quyền, hạn mức, audit, dedupe đa agent (CR-CV-013, D5); agent vẫn hẹp |
| Dùng `orca.infra.agent.statusChanged`, không `agent.hook` | `agent.hook` không có ở Part A và Go không giải mã `state` (1.3) |
| Không `analyze` checkout chính chỉ vì worktree đổi | Kết quả không phản ánh code agent; tốn tài nguyên (1.1.2, 1.2) |
| `indexScope` phái sinh từ (vị trí gốc index, commit, dirty), không từ `pendingChanges` | `pendingChanges` ở worktree liên kết là của checkout chính (1.1.3) |
| `dirtySinceIndex` dùng mtime/diff, không chỉ `lastCommit` | GitNexus index theo cây làm việc; agent thường chưa commit |
| Không kill `analyze` khi agent chạy tiếp (mặc định) | Có thể hỏng WAL/DB (chưa kiểm chứng); thà đánh dấu `stale` rồi làm lại |
| Debounce 20 s, tối đa 120 s, một job/binding | Lượt agent liên tiếp; tránh bão `analyze` |
| `per_worktree` tắt mặc định, chờ M1-M6 | Tốn ~4 GB/worktree, rủi ro registry, mâu thuẫn quy tắc "không xoá" |
| Hiển thị `IndexBasis` chỉ cho tín hiệu phụ thuộc đồ thị | Tránh gắn nhãn "index cũ" lên kết quả lint/test vốn không dùng index |
| `unknown` thay vì `pass` khi cần index mà `stale\|none` | README 3.10 |

## 4. Tiêu chí chấp nhận

- [ ] `codeintel.status` trả `indexRoot`, `indexScope`, `freshness`, `dirtySinceIndex`, `changedFilesNotInIndex`, `mergeBase` cho mỗi công cụ; bảng test `classifyIndexBasis` phủ đủ 6 dòng ở 2.4 và các ca: checkout chính sạch, checkout chính có sửa chưa commit sau `analyze`, worktree liên kết có index nền đúng/sai merge-base, không có index, `indexedCommit` đã bị gc.
- [ ] Ở worktree liên kết, `pendingChanges` của CodeGraph **không** được dùng để kết luận `fresh`; test với dữ liệu giống 1.1.2 (`worktreeMismatch` có, `pendingChanges` 0) cho `indexScope=repo_root` hoặc `stale`, không bao giờ `exact`.
- [ ] Sự kiện `statusChanged running→idle` cho worktree có binding và `index_policy=auto_in_place` tạo đúng một `reindex_jobs` (`trigger=agent_done`) sau debounce; 10 sự kiện trong 20 s chỉ tạo một.
- [ ] Sự kiện trùng (cùng `event_id`, hoặc cùng `dedupe_key` trong 5 phút) không tạo job thứ hai; có job đang chạy thì sự kiện mới chỉ đặt `pendingRefresh`, và chạy lại đúng một lần sau khi xong nếu `IndexBasis` còn lệch.
- [ ] Worktree liên kết không có index riêng: không có lệnh `analyze`/`sync` nào được gửi cho agent (`trigger=agent_done`); `IndexBasis.indexScope=repo_root`, `changedFilesNotInIndex` đúng; `codeintel.reindex` với `trigger=agent_done` trả `skipped_scope_repo_root`, không lỗi.
- [ ] Tier 1 chạy trước tier 2; tier 2 bị hoãn khi `loadavg1 > 0,7 × cores`, khi đang có `quality.run`, hoặc trong `MIN_INTERVAL`; hoãn được ghi `refresh_state=deferred`.
- [ ] Agent chạy tiếp (`→ running`) trong lúc `analyze` đang chạy: job **không** bị kill mặc định; sau khi kết thúc `IndexBasis.freshness=stale` nếu HEAD/dirty đã đổi, và có kế hoạch lại sau khi agent xong.
- [ ] Vượt `CODEINTEL_AUTOREFRESH_PER_HOUR`: sự kiện bị bỏ với `CODEINTEL_RATE_LIMITED` và metric, không xếp hàng.
- [ ] Mọi `analyze` do tự động sinh có `--index-only` (test snapshot của CR-CV-004 vẫn xanh); sau một job `git status --porcelain` không có thay đổi mới ở `AGENTS.md`, `CLAUDE.md`, `.claude/`.
- [ ] Job tự động có `requested_by IS NULL`, `trigger=agent_done`, audit `actor_type=system`; tenant tắt `code_intel_enabled` hoặc `index_policy=off` không tạo job nào.
- [ ] Kết quả chất lượng phụ thuộc đồ thị mang `IndexBasis`; kết quả lint/test không mang. Cổng nhận `stale|none` cho kiểm tra cần index trả `unknown` (CR-CV-085).
- [ ] `quality_runs.index_commit` và `index_basis` được điền khi kết thúc lần chạy.
- [ ] Windows: agent trả `unsupported_platform` như CR-CV-001, backend không tạo job cho dev server `win32`.
- [ ] Không có tên tệp `helpers/utils/common/misc`; không có `max-lines` disable mới.

## 5. Kiểm thử

Chưa chạy bất kỳ test nào ở thời điểm viết CR.

| Tầng | Nội dung |
|---|---|
| Agent unit (Vitest) | `codeintel-index-basis.test.ts`: bảng `classifyIndexBasis`; repo git thật trong thư mục tạm có `git worktree add`; mock `git status --porcelain=v1 -z`; `indexedCommit` không tồn tại; `host.loadavg1` giả |
| Agent | `codeintel-reindex-job.test.ts` mở rộng: `ifStale`, `expectHead`, `trigger=agent_done` ở worktree liên kết, `tiers` |
| Go unit | `auto_refresh_index_test.go`: debounce (đồng hồ giả), `dedupe_key`, hạn mức/giờ, hoãn do tải, tác nhân hệ thống, thứ tự tier; consumer idempotent (cùng `event_id` hai lần) |
| Go integration | Hai dialect: `reindex_jobs` với `requested_by` NULL, `active_key` chống job thứ hai; NATS testcontainer (`common/testutil/nats.go`) phát `orca.infra.agent.statusChanged` |
| Hợp đồng | Fixture `codeintel.status` mới (mở rộng CR-CV-070 `status-*.json`): dùng lại cơ chế `MANIFEST.json`, không thêm cơ chế chụp mới |
| Thủ công / benchmark | M1-M7 ở 2.7 trên bản sao nhỏ; ghi số vào PR |

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa biết `codegraph sync <worktree>` đọc cây nào** (M1) và **`gitnexus analyze` lặp lại có thật sự gia tăng không** (M2). Hai điều này quyết định giá trị của O10; số liệu trong 2.2 (`QUIET` 20 s, `MIN_INTERVAL` 10 phút, 12 job/giờ, ngưỡng tải 0,7) đều là giả định.
- **Thời gian `analyze` Orca chưa đo** (như CR-CV-004 1.6); với repo 20 174 tệp và 247 556 nút nó có thể chiếm nhiều phút CPU lúc agent vừa xong, đúng lúc người review cần máy.
- **`statusChanged` at-most-once** (publish trực tiếp, `agent_status_publisher.go` đầu tệp): mất sự kiện thì không có refresh. Giảm thiểu: `agent_completed`/HEAD đổi/UI hint làm đường dự phòng; nút thủ công vẫn còn.
- **Phiên agent tương tác mở thẳng trong terminal không qua `StartAgentSession`** không có `agent_sessions` nên không có tín hiệu backend; chỉ UI hint bao phủ. Tỷ lệ phiên như vậy chưa đo.
- **Payload sự kiện thiếu `worktree_id`**: nếu CR-CV-023 không thêm, phương án tra `ListAgentSessions` bị giới hạn 500 dòng và không có bộ lọc id; nên coi việc thêm trường là điều kiện của CR này.
- **`agent.hook` ở Part A**: kết luận "không có" dựa trên grep trong `agent/src/relay`; build Part B nằm ở `desktop/src/relay/` (CR-CV-006), chưa đọc hết `desktop/src/relay` để loại trừ đường khác.
- **`indexedCommit` có thể không phải commit mà cây làm việc đã được index** (GitNexus index cây làm việc, `lastCommit` chỉ ghi HEAD lúc đó): `dirtySinceIndex` dùng mtime có thể báo thừa (chạm tệp không đổi nội dung) hoặc thiếu (mtime bị giữ khi checkout). Chấp nhận thiên về `stale`.
- **`per_worktree`** mâu thuẫn quy tắc "không xoá `.gitnexus/`" (CR-CV-001 F9) khi dọn worktree đã xoá; chưa có cách dọn an toàn (cần `gitnexus remove`/`clean`, đang bị cấm).
- **Registry GitNexus khoá theo tên** (CR-CV-012 6): worktree trùng tên cơ sở có thể va chạm.
- **macOS/Windows**: `mtime`, `realpath`, `loadavg` (Windows trả 0) chưa kiểm chứng; MVP chỉ Linux (CR-CV-001).
- **SSH (`relay-ssh`)**: Part A tự có method mới qua `--stdio` (CR-CV-001 1.8), nhưng sự kiện `statusChanged` chỉ có cho phiên do backend khởi; chưa kiểm chứng cho `relay-ssh`.
- **Git**: chỉ dùng `rev-parse`, `merge-base`, `diff --name-only`, `status --porcelain=v1 -z` (đều dưới 2.25); `merge-base` với `origin/HEAD` có thể không tồn tại ở clone cục bộ: phải có nhánh dự phòng `null`.

## 7. Câu hỏi mở

- **Q1.** Chấp nhận `repo_root` + overlay làm trải nghiệm mặc định cho worktree liên kết, hay đầu tư ngay M3/M4 để có `per_worktree`? Mặc định: chấp nhận overlay, đo M1-M7 trước.
- **Q2.** `index_policy` nằm ở `tenant_settings` hay `repo_bindings`? Mặc định: `tenant_settings`, ghi đè theo project là ngoài phạm vi.
- **Q3.** Có cho `CANCEL_ON_RESUME` sau M6 không? Mặc định: không.
- **Q4.** Thêm `worktree_id`/`dev_server_id` vào `orca.infra.agent.statusChanged` (đổi payload nhẹ, thuộc `infra-fleet-service`) hay thêm `GetAgentSession`? Đề xuất: thêm trường (rẻ, không cần RPC).
- **Q5.** Có thêm kênh `codeIntel.hintAgentTurnFinished` (UI báo) ở MVP không? Đề xuất: P1, sau khi đo tỷ lệ phiên tương tác ngoài `StartAgentSession`.
- **Q6.** Dev server có nhiều người dùng chạy agent song song: hạn mức "một job nặng/dev server" có làm chậm đáng kể không? Cần số đo hàng đợi.
- **Q7.** Có cần nút "Làm mới ngay" gắn nhãn chi phí (ước tính phút) khi `analyze` thủ công? Cần M2.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 2 O3, O10; mục 3.2, 3.5, 3.10; mục 8 điểm 18)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` §2 (A1-A3), §7
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md` (2.2, 2.4, 2.6), `CR-CV-004-codeintel-reindex-and-index-notifications.md` (2.2, 2.5, 2.6, 6), `CR-CV-003-codegraph-extraction.md`
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md` (`reindex_jobs`, `repo_bindings`), `CR-CV-012-project-worktree-to-repo-binding.md` (2.5, 2.6), `CR-CV-013-authorization-audit-and-quotas.md`
- `/opt/repos/orca/docs/crs/v7/quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md`
- `/opt/repos/orca/agent/src/relay/agent-hook-server.ts` (`:112`), `relay.ts` (`:554`), `wsl-agent-hook-relay.ts` (`:49`)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go` (`:428-452`, `:526-575`), `internal/adapter/eventbus/agent_status_publisher.go`, `internal/adapter/eventbus/publisher.go`, `internal/usecase/agent_output_classifier.go`, `internal/usecase/attach_pty.go` (`:220`), `internal/domain/agent_session.go`
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`AgentSession` `:1569`, `ListAgentSessions` `:330`, `:1584`)
- `/opt/repos/orca/frontend/src/shared/agent-status-types.ts`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/agent-status.ts`
- Lệnh chỉ-đọc đã chạy ngày 2026-10-06: `gitnexus analyze --help`, `gitnexus detect-changes --help`, `gitnexus status`, `codegraph sync|index|affected|files --help`, `codegraph status -j` (checkout chính và worktree liên kết), `git worktree list`, `git merge-base`, `du -sh .gitnexus .codegraph`, đọc `.gitnexus/meta.json`, `~/.gitnexus/registry.json`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
