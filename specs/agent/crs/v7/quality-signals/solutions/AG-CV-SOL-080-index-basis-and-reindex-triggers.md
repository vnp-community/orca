# AG-CV-SOL-080: Phân loại `indexScope`/`freshness`, khối `host`, tham số `trigger|ifStale|expectHead` của `codeintel.reindex`

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi dòng "đã đọc" là đọc code/CR, chưa chạy gì. Phần "(mới)" là đề xuất của solution này.

**CR:** [CR-CV-080](../../../../../../docs/crs/v7/quality-signals/CR-CV-080-agent-worktree-index-strategy-and-auto-refresh.md) mục 2.3, 2.4, 2.5 (phía agent). Phần backend (consumer `statusChanged`, debounce, hạn mức, `reindex_jobs.trigger`) thuộc `BE-CV-SOL-080-auto-refresh-index`, không có ở đây.
**Khu vực:** `agent/` (Dev Server Agent), thư mục `agent/src/relay/`.
**Feature:** `quality-signals`.
**TDD/Spec tham chiếu:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [TDD-AG-04 mục 7](../../../../tdd/v5/04-handshake-session.md) (capabilities), [TDD-AG-07 mục 9.1](../../../../tdd/v5/07-jsonrpc-dispatch.md) (`makeNotifier`), [api/agent-rpc-catalog-git-fs.md](../../../../api/agent-rpc-catalog-git-fs.md) (git whitelist Part A), [api/gaps-and-findings.md](../../../../api/gaps-and-findings.md).
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/`.

## 1. Hợp đồng áp dụng

| Mục hợp đồng | Áp vào đâu |
|---|---|
| [`CONTRACT-codeintel-agent-rpc.md` §4.1](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md) | Hình dạng `codeintel.status` mở rộng (`indexRoot`, `indexScope`, `freshness`, `headCommit`, `mergeBase`, `dirtySinceIndex`, `changedFilesNotInIndex`, `pendingChanges`, `rootMismatch`, `host`); bảng 6 dòng của `classifyIndexBasis` |
| §4.10 | `codeintel.reindex` thêm `trigger`, `ifStale`, `expectHead`; `outcome ∈ already_up_to_date|superseded|skipped_scope_repo_root`; worktree liên kết |
| §6.1 | `codeintel.indexChanged` thêm `indexScope`, `mergeBase`, `trigger` (tuỳ chọn) |
| §2.2 | `stale` ở phong bì = OR theo công cụ (`rootMismatch` hoặc `pendingChanges > 0` hoặc `indexedCommit !== headCommit`) |
| §2.3, §9.2 | Cache `headCommit` 5 s; resolve repo qua `resolveCodeIntelRepo` (AG-CV-SOL-001) |
| PQ-16 | **Bỏ `tiers`**; giữ `mode`, `tools`; thêm `trigger`, `ifStale`, `expectHead`; trạng thái job; `percent` số hoặc `null`; chính tả `cancelled` |
| PQ-19 | `binding.worktreeMismatch` là boolean; `indexes.<tool>.rootMismatch: null \| {worktreeRoot, indexRoot}` (đổi tên); ImpactGraph không cạnh (không liên quan ở đây) |
| PQ-20, PQ-21 | `workspaceRoot` bắt buộc, tham số lạ bị từ chối |
| §8.3 (1), (5) | Trích PQ; không có tham số `command|argv|args|env|cwd|timeout` |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-080 nói | Hợp đồng nói | Solution theo |
|---|---|---|---|
| 1 | `codeintel.reindex` có `tiers: ("sync"\|"analyze")[]` thay `mode/tools` (2.3.2) | PQ-16: **bỏ `tiers`**, giữ `mode`, `tools` | Hợp đồng. Không cài `tiers` |
| 2 | `indexChanged` thêm `indexScope`, `mergeBase`, `trigger` (2.3.2) | §6.1 có đúng ba trường này, đánh dấu tuỳ chọn | Hợp đồng (tuỳ chọn, không bắt buộc ở backend) |
| 3 | CodeGraph `worktreeMismatch` là đối tượng | PQ-19: tên `rootMismatch` ở `indexes.<tool>`; `worktreeMismatch` ở `binding` là boolean | Hợp đồng |
| 4 | `trigger:"agent_done"` ở worktree liên kết trả thành công `skipped_scope_repo_root` | §4.10 giống, thêm `skipped:[{tool, reason:"index_root_is_main_checkout"}]` | Hợp đồng |
| 5 | Bảng 2.4 hàng 3 và 5 dùng `indexedCommit === headCommit`/`=== mergeBase` cho **cả hai** công cụ | §2.2: `sources[].commit` của CodeGraph **luôn `null`** (không có commit trong `status`) | **Hợp đồng thiếu quy tắc cho CodeGraph.** Solution đề xuất quy tắc ở mục 2.3 (chưa kiểm chứng, cần xác nhận khi sửa hợp đồng) |
| 6 | `host` đặt trong `codeintel.status` (2.3.1) | §4.1 đặt `host` cấp `data` (ngoài `indexes`), thêm `limits` | Hợp đồng; `limits` thuộc AG-CV-SOL-001 |
| 7 | Hạn mức, debounce, `QUIET`, `MIN_INTERVAL`, `CANCEL_ON_RESUME` | Chỉ nằm ở backend (PQ-16, CR-080 2.2) | Không có ở agent; agent chỉ cung cấp `host` và tham số `reindex` |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-001-codeintel-agent-foundation` (`codeintel-status.ts`, `codeintel-method-table.ts`, `codeintel-repo-resolution.ts`, `codeintel-result-envelope.ts`) | **Điều kiện trước.** Các file này **chưa tồn tại** (đã `ls agent/src/relay`: không có `codeintel*`). Solution này chỉ thêm module thuần và nối vào chúng |
| `AG-CV-SOL-002`, `AG-CV-SOL-003` (probe chỉ mục GitNexus/CodeGraph: `indexedCommit`, `indexedAt`, `pendingChanges`, `indexRoot`) | Cung cấp đầu vào của `classifyIndexBasis` |
| `AG-CV-SOL-004-reindex-and-index-notifications` (`codeintel-reindex-job.ts`, `codeintel-reindex-commands.ts`, `codeintel-index-watcher.ts`, `codeintel-notification-sink.ts`) | Điều kiện trước của task 06 và 07 (sửa tham số và payload của chính các file đó) |
| `BE-CV-SOL-080-auto-refresh-index` | Gọi `codeintel.status` và `codeintel.reindex {trigger:"agent_done", ifStale:true, expectHead}`; đọc `host.loadavg1`/`host.cores` để hoãn |
| `BE-CV-SOL-012-index-status-aggregation` | Nhận `indexScope=stale`, ánh xạ `OVERLAY` (CR-080 2.8) |
| `BE-CV-SOL-070-collector-golden-contract`, `AG-CV-SOL-070-golden-fixtures-and-parsers` | Fixture `status-*.json` mở rộng (task 05) dùng cơ chế `MANIFEST.json` của CR-070 |
| Thứ tự (hợp đồng §7.2): `004 → 080-AG`. Đợt 7 (§7.3) |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc: `agent/src/relay/agent-session-capabilities.ts`, `agent-session-handshake.ts`, `agent-session.ts`, `agent-rpc-dispatch.ts` (`route`, `makeNotifier :280`, `makeError :408`), `agent-config.ts`, `agent/src/shared/git-capability-cache.ts`, `agent/src/relay/agent-git-handler-extended.ts` (mẫu `['-c','core.quotePath=false','diff',…]` ở dòng 106, 110), `agent/vitest.config.ts`, và CR/hợp đồng liệt kê ở mục 1.

| Điểm | Hiện trạng thật | Hệ quả |
|---|---|---|
| Mã `codeintel*` ở agent | Không có file nào tên `codeintel*`, `quality*`, `gitnexus*`, `codegraph*` trong `agent/src/relay/` | Mọi "sửa" lên `codeintel-status.ts`... là sửa file do solution khác tạo; task ghi "Depends on" tương ứng |
| Git trong agent | `execFile('git', args, …)` (promisify) và mẫu `-c core.quotePath=false` đã có; `GitCapabilityCache` dùng cho `worktree list -z`, `rev-parse --path-format`, `merge-tree` | Lệnh solution dùng (`rev-parse HEAD`, `merge-base`, `diff --name-only`, `status --porcelain=v1 -z`, `cat-file -e`) đều có từ Git < 2.25: **không cần** `GitCapabilityCache`, ghi trong task 03 |
| Phiên bản/handshake | `buildCapabilities` trả mảng chuỗi tự do; không đụng ở solution này | `codeintel*` do SOL-001 thêm |
| Test | `agent/vitest.config.ts` include `src/**/*.test.ts`, môi trường `node`; CI **không** chạy test của `agent/` (hợp đồng §10) | Lệnh test ghi cụ thể; job CI là của CR-CV-070 |
| `oxlint max-lines` | `.oxlintrc.json:89-101` đặt 300 dòng cho `.ts` (bỏ trống/chú thích), 800 cho test | Mỗi file mới < 300 dòng; không `max-lines` disable |

| Bảng "Correction relative to CR" | |
|---|---|
| CR-080 2.3.1 nói cache `headCommit` 5 s "như CR-CV-001 2.6" | Hợp đồng §2.3 xác nhận 5 s. Đúng |
| CR-080 viết `git status --porcelain=v1 -z` "có từ Git 1.7" | Đúng về `-z`; nhưng `--porcelain=v1` (có số phiên bản) có từ Git 2.11. Vẫn dưới baseline 2.25, không đổi kết luận |
| CR-080 nói `pendingChanges` CodeGraph ở worktree liên kết là của checkout chính | Chưa chạy lại; solution tin vào quan sát của CR (1.1.2) và thiết kế để **không** dựa vào nó khi `!rootMatches` |

## 5. Giải pháp

### 5.1 Cây file (mới, phẳng trong `agent/src/relay/`)

```
agent/src/relay/
  codeintel-index-basis.ts                 (mới) classifyIndexBasis (hàm thuần) + kiểu IndexBasisInput/IndexBasisResult
  codeintel-index-basis.test.ts            (mới) bảng 6 hàng + ca CodeGraph
  codeintel-index-basis-probe.ts           (mới) mergeBase, changedFilesNotInIndex, dirtySinceIndex (git + stat)
  codeintel-index-basis-probe.test.ts      (mới) repo git thật trong thư mục tạm, git worktree add
  codeintel-host-snapshot.ts               (mới) {platform, cores, loadavg1, freeMemBytes}
  codeintel-host-snapshot.test.ts          (mới)
  __fixtures__/index-basis/                (mới) đầu ra thật của `codegraph status -j`, `gitnexus status`, meta.json (task 01)
  codeintel-status.ts                      (sửa; do AG-CV-SOL-001 tạo) gắn các trường mới
  codeintel-reindex-job.ts                 (sửa; do AG-CV-SOL-004 tạo) tham số + outcome mới
  codeintel-notification-sink.ts / -index-watcher.ts (sửa; AG-CV-SOL-004) payload indexChanged
```

### 5.2 `classifyIndexBasis` (hàm thuần, đúng bảng §4.1 của hợp đồng)

```ts
export type IndexScope = 'exact' | 'repo_root' | 'stale' | 'none'
export type IndexFreshness = 'fresh' | 'fresh_base' | 'stale' | 'unknown'

export type IndexBasisInput = {
  tool: 'gitnexus' | 'codegraph'
  toolUsable: boolean               // false khi thiếu binary hoặc ngoài dải phiên bản (hàng 2)
  indexExists: boolean              // false khi state=missing hoặc không khớp registry (hàng 1)
  rootMatches: boolean              // realpath(indexRoot) === realpath(workspaceRoot)
  indexedCommit: string | null      // GitNexus: meta.lastCommit; CodeGraph: luôn null
  indexedAtMs: number | null
  headCommit: string | null
  headCommitTimeMs: number | null   // (mới) thời điểm commit HEAD; chỉ dùng cho CodeGraph
  mergeBase: string | null
  mergeBaseCommitTimeMs: number | null  // (mới)
  dirtySinceIndex: boolean
  pendingChanges: { added: number; modified: number; removed: number } | null  // chỉ có nghĩa khi rootMatches
}
export type IndexBasisResult = { indexScope: IndexScope; freshness: IndexFreshness }
export function classifyIndexBasis(input: IndexBasisInput): IndexBasisResult
```

Dừng ở dòng đúng đầu tiên (bảng của hợp đồng, nguyên văn): (1) `!indexExists` → `none/unknown`; (2) `!toolUsable` → `none/unknown`; (3) `rootMatches ∧ indexedCommit==headCommit ∧ !dirtySinceIndex` (CodeGraph: `pendingChanges` toàn 0) → `exact/fresh`; (4) `rootMatches ∧ (commit khác ∨ dirty ∨ pending>0)` → `stale/stale`; (5) `!rootMatches ∧ indexedCommit==mergeBase` → `repo_root/fresh_base`; (6) `!rootMatches ∧ commit khác` → `stale/stale`.

### 5.3 Quy tắc cho CodeGraph khi `commit = null` (mới; hợp đồng thiếu)

Hợp đồng §2.2 chốt `commit` của CodeGraph luôn `null`, nên "`indexedCommit == headCommit`" và "`== mergeBase`" không tính được. Đề xuất (chưa kiểm chứng, ghi vào "Câu hỏi mở" 1 để sửa hợp đồng):

| `rootMatches` | Điều kiện | Kết quả |
|---|---|---|
| true | `pendingChanges` toàn 0 ∧ `!dirtySinceIndex` ∧ `indexedAtMs >= headCommitTimeMs` | `exact/fresh` |
| true | còn lại (có pending, có sửa chưa commit, hoặc HEAD mới hơn lần index) | `stale/stale` |
| false | luôn | `repo_root/unknown` (không có commit để so với `mergeBase`; **cổng coi là `unknown`**, không `fresh_base`) |

Lý do: thà báo `unknown` còn hơn giả `fresh_base` (README quality-signals F4 "thiếu dữ liệu → unknown"). `pendingChanges` luôn `null` khi `!rootMatches` (CR-080 1.1.3).

### 5.4 Probe git (chỉ đọc, `codeintel-index-basis-probe.ts`)

| Giá trị | Lệnh | Ghi chú |
|---|---|---|
| `headCommit` | `git rev-parse HEAD` | cache 5 s (dùng cache của AG-CV-SOL-001; không tạo cache thứ hai) |
| `mergeBase` | `git merge-base HEAD <baseRef>` | `baseRef` từ tham số `codeintel.status {baseRef?}` (mặc định `origin/HEAD`); không tính được → `null`, không lỗi |
| `changedFilesNotInIndex` | `git diff --name-only -z <indexedCommit> HEAD` **cộng** `git status --porcelain=v1 -z` | `indexedCommit` không còn trong object store (`git cat-file -e <commit>^{commit}` lỗi) → `null` và `freshness` suy ra `stale` |
| `dirtySinceIndex` | tệp trong tập trên có `mtime > indexedAt`, hoặc số tệp > 0 | `fs.stat` tối đa 5 000 mục; vượt thì `true` (thiên về `stale`, theo CR-080 mục 6) |

Mọi lệnh dùng `execFile('git', ['-c','core.quotePath=false', …], { cwd: workspaceRoot, timeout: 5000, maxBuffer: 8 MiB })`, không shell. Cache kết quả theo `(workspaceRoot, indexedAt, headCommit)` 5 s.

### 5.5 Khối `host`

```ts
export type HostSnapshot = { platform: NodeJS.Platform; cores: number; loadavg1: number; freeMemBytes: number }
export function readHostSnapshot(): HostSnapshot   // os.platform(), os.availableParallelism(), os.loadavg()[0], os.freemem()
```
Windows: `os.loadavg()` luôn `[0,0,0]` (ghi trong test); MVP không hỗ trợ Windows nên không ảnh hưởng (hợp đồng §1.1). Dùng chung bởi `quality.listProfiles` (AG-CV-SOL-081-catalog task 17).

### 5.6 `codeintel.reindex`: tham số và kết quả (sửa file của SOL-004)

| Tham số | Kiểm | Hành vi |
|---|---|---|
| `trigger` | `manual\|agent_done\|head_change`, mặc định `manual` | ghi vào journal và `indexChanged.trigger`; `agent_done` đổi nhánh worktree liên kết |
| `ifStale` | boolean, mặc định `false` | `true` và `freshness==="fresh"` → trả ngay `state:"succeeded"`, `outcome:"already_up_to_date"`, **không spawn** |
| `expectHead` | `^[0-9a-f]{7,64}$` | HEAD hiện tại khác → `outcome:"superseded"`, không chạy |
| (không có `tiers`) | — | `tiers` hoặc tham số lạ → `CODEINTEL_INVALID_PARAMS` (`data.field`) |

Worktree liên kết (`linkedWorktree:true`): `trigger:"manual"` → `CODEINTEL_PATH_NOT_ALLOWED hint="reindex_linked_worktree_unsupported"`; `trigger:"agent_done"` → **thành công** `outcome:"skipped_scope_repo_root"`, `skipped:[{tool, reason:"index_root_is_main_checkout"}]`, **không chạy** `analyze/sync`; `trigger:"head_change"` xử lý như `agent_done`. Agent không biết hạn mức/debounce (việc của backend). Không đụng quy tắc "không kill `analyze`" (không có `reindexCancel` do backend gọi ở v7, O-17).

### 5.7 `codeintel.indexChanged` (sửa payload của SOL-004)

Thêm vào `params` (tuỳ chọn): `indexScope`, `mergeBase`, `trigger`. Không thêm `freshness` (hợp đồng §6.1 không có). Phát khi `reindex` xong (`reason:"reindex"`) và khi watcher phát hiện đổi chỉ mục/HEAD (giá trị `indexScope` tính lại bằng `classifyIndexBasis`).

## 6. Quyết định thiết kế

| # | Quyết định | Lý do | Bỏ |
|---|---|---|---|
| 1 | `classifyIndexBasis` là hàm thuần, không đọc đĩa/git | Bảng test đầy đủ không cần repo thật | Gộp probe vào classify |
| 2 | Probe tách file riêng | `codeintel-status.ts` (SOL-001) dưới 300 dòng | Đặt trong `codeintel-status.ts` |
| 3 | CodeGraph không có commit → dùng `indexedAt` so với thời điểm commit HEAD (5.3) | Hợp đồng chốt `commit:null` | Giả định commit từ `daemon.log` (chưa kiểm chứng) |
| 4 | `!rootMatches` của CodeGraph → `repo_root/unknown` | Không giả `fresh_base` | `fresh_base` theo `lastIndexed` |
| 5 | Không cài `tiers` | PQ-16 | Theo CR-080 |
| 6 | `ifStale` dùng `freshness` mới tính, không dùng `stale` của phong bì | `stale` của phong bì chung hai công cụ | — |
| 7 | Không thêm cờ chặn `reindex` theo tải ở agent | Hợp đồng: backend hoãn theo `host` | Chặn ở agent |

## 7. Tiêu chí chấp nhận

- [ ] `classifyIndexBasis` có bảng test đủ 6 hàng của hợp đồng và các ca CodeGraph (5.3); không bao giờ trả `exact` khi `!rootMatches`.
- [ ] Dữ liệu giống CR-080 1.1.2 (`worktreeMismatch` có, `pendingChanges` 0, worktree liên kết) cho `indexScope ∈ {repo_root, stale}`, không bao giờ `exact`.
- [ ] `codeintel.status` trả `indexRoot`, `indexScope`, `freshness`, `headCommit`, `mergeBase`, `dirtySinceIndex`, `changedFilesNotInIndex`, `pendingChanges` (null khi `!rootMatches`), `rootMismatch` (PQ-19), `host`; `stale` ở phong bì = OR như §2.2.
- [ ] `indexedCommit` đã bị gc → `changedFilesNotInIndex:null`, `freshness:"stale"`.
- [ ] `codeintel.reindex {ifStale:true}` khi `fresh` không spawn tiến trình nào (test khẳng định `spawn` không được gọi); `expectHead` lệch → `superseded`.
- [ ] `trigger:"agent_done"` ở worktree liên kết trả `skipped_scope_repo_root`, `skipped[]` đúng; `trigger:"manual"` trả `CODEINTEL_PATH_NOT_ALLOWED`.
- [ ] `tiers` hoặc khoá lạ bị từ chối `CODEINTEL_INVALID_PARAMS`.
- [ ] Không file mới nào vượt 300 dòng; không `max-lines` disable; không tên `helpers/utils/common/misc`.

## 8. Kiểm thử

| File test (mới) | Nội dung |
|---|---|
| `codeintel-index-basis.test.ts` | bảng 6 hàng; CodeGraph `commit:null`; `pendingChanges` khi `!rootMatches` |
| `codeintel-index-basis-probe.test.ts` | repo git tạm: commit, sửa chưa commit, tệp mới, `git worktree add`, `indexedCommit` giả không tồn tại; baseline Git 2.25 (theo mẫu `git-handler-worktree-git-capabilities.test.ts`) |
| `codeintel-host-snapshot.test.ts` | giá trị hữu hạn, `cores ≥ 1` |
| `codeintel-status.test.ts` (sửa file do SOL-001 tạo) | tích hợp trường mới với fixture `index-basis/` |
| `codeintel-reindex-job.test.ts` (sửa file SOL-004) | `ifStale`, `expectHead`, `trigger`, worktree liên kết, `tiers` bị từ chối |

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-index-basis.test.ts src/relay/codeintel-index-basis-probe.test.ts src/relay/codeintel-host-snapshot.test.ts`; sau cùng `pnpm test`.

## 9. Rủi ro và chưa kiểm chứng

- Định dạng thật của `codegraph status -j` ở worktree liên kết và `gitnexus status`/`meta.json` chỉ lấy từ CR (chưa chạy lại): task 01 thu fixture thật trước khi viết hàm.
- Quy tắc 5.3 là đề xuất; nếu hợp đồng chốt cách khác thì chỉ đổi `classifyIndexBasis` và bảng test.
- `mtime` có thể báo thừa/thiếu (chạm tệp không đổi nội dung; `git checkout` giữ mtime): chấp nhận thiên về `stale` (CR-080 mục 6).
- macOS/Windows: `realpath` (hoa/thường), `loadavg` chưa kiểm chứng; MVP Linux.
- SSH: `--stdio` chạy cùng Part A (đã đọc `agent-connection-stdio.ts` dùng `createSession`), nên cùng hành vi; độ trễ git 50–200 ms qua SSH chưa đo.

## 10. Câu hỏi mở

1. Sửa hợp đồng §4.1 để nêu quy tắc CodeGraph khi `commit:null` (5.3)? Mặc định: theo 5.3.
2. `host` có thêm `diskFreeBytes` của `os.tmpdir()` (phục vụ `tmp_space_low`) không? Mặc định: không (hợp đồng không có).
3. `changedFilesNotInIndex` có cần giới hạn trên (vd 100 000) để tránh `diff --name-only` quá lớn? Mặc định: cắt ở 5 000 tệp và báo `warnings:["changed_files_capped"]` (chưa có trong hợp đồng, cần thêm).
