# AG-CV-SOL-005: `codeintel.detectChanges` (diff theo merge-base → symbol → luồng)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi mục "đã đọc" là đọc code/CR, chưa chạy gì. Cypher `FILE_SYMBOLS_BATCH` (`MATCH (n)` + `IN`) **chưa chạy**; hình dạng `git diff` lấy từ CR (Git 2.43.0), chưa chạy lại.

**CR:** [CR-CV-005](../../../../../../docs/crs/v7/agent-codeintel/CR-CV-005-codeintel-detect-changes.md)
**Service:** `agent/src/relay/` (Part A)
**TDD tham chiếu:** [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md) mục 1, 5; [v5/10-git-handler-extension](../../../../tdd/v5/10-git-handler-extension.md) (git handler); `guides/reference/git-compatibility.md`
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md`

## 0. Hợp đồng áp dụng
| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §2.5 (55 s agent, 90 s Go), §3.2, §4.8 (`detectChanges`, whitelist git ≤ 2.24, giới hạn, enum, `warnings` ở phong bì), §10 (`head` không đặt = cây làm việc chưa kiểm chứng ở `branchCompare`) |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-13 (timeout), PQ-19 (`warnings` chỉ phong bì, `data.index`, `affectedClusters`), PQ-20, PQ-21, §9 O-13, §10 (dòng `git-handler-ops.ts` `branchCompare`) |

## 1. Trạng thái hiện tại (re-verify)
Đã đọc (2026-10-06): CR-CV-005; `agent/src/relay/git-handler-ops.ts:124-215` (`branchCompare`); `agent-git-handler-extended.ts:42-57` (`git()`), `git-handler-check-ignore.ts`; `desktop/src/relay/` (có `git-handler-ops.ts`, `git-handler-utils.ts`, không có `agent-git-handler-extended.ts`).
| Điểm | Hiện trạng |
|---|---|
| `branchCompare` | chuỗi `branch --show-current`, `rev-parse --verify HEAD`, `rev-parse --verify <baseRef>` (không `--end-of-options`/`^{commit}`), `merge-base`, `rev-list --count`; **HEAD unborn + base resolvable trả `status:'ready'` entries rỗng** (không phải `unborn-head`); không nhận `headRef` |
| Git helper | `git()` dùng `process.env` đầy đủ, không `signal`; không có trong `desktop/` |
| `git.baseRefDefault` | thử `ls-remote` (mạng) |

### Correction relative to CR
| # | CR nói | Quyết định |
|---|---|---|
| 1 | tái dùng `branchCompare`, có thể sửa nhận `headRef` | **không tái dùng, không sửa**: `codeintel-merge-base-resolution.ts` tự dùng `runGit` của SOL-001 (đúng `unborn_head`, `--end-of-options`, chạy được ở `desktop/`) |
| 2 | dùng `git()` của `agent-git-handler-extended.ts` | dùng `runGit` (`codeintel-git-exec.ts`) |
| 3 | `unborn-head` từ `branchCompare` | tự kiểm `rev-parse --verify HEAD` |

### Lệch giữa CR và hợp đồng
Hợp đồng §11/§10 liệt kê "sửa nhỏ `git-handler-ops.ts` `branchCompare` (cần `impact`)"; solution **không sửa** (lý do trên) nên mục đó của §11 không còn việc; ghi ở báo cáo. `warnings` chỉ ở phong bì (PQ-19), `data.warnings` bỏ. Mã lỗi `base_required|unresolved_ref|no_merge_base` đúng §4.8.

### Phụ thuộc chéo khu vực
BE: `BE-CV-SOL-036-change-overlay-pipeline` (người tiêu dùng; hợp nhất `ChangeOverlay`, gọi thêm `codegraph affected`), `BE-CV-SOL-023-infra-fleet-codeintel-transport` (timeout 90 s cho `detectChanges`; nếu chưa nâng, đặt `ORCA_CODEINTEL_DETECT_TIMEOUT_MS=25000`), `BE-CV-SOL-021-agent-collector`. AG: `AG-CV-SOL-001`, `AG-CV-SOL-002` (`FILE_SYMBOLS_BATCH`, `SYMBOL_FLOWS`, `MEMBER_CLUSTER`, `SymbolRef`, probe), `AG-CV-SOL-003` (không dùng trong method này), `AG-CV-SOL-070`. FE: `FE-CV-SOL-053-impact-lens-and-symbol-detail`, `FE-CV-SOL-051` (nhãn "ước lượng" khi `approximate`).

## 2. Giải pháp
### 2.1 Cây file
```
agent/src/relay/
  codeintel-merge-base-resolution.ts   (mới) base mặc định không mạng, headOid, merge-base
  codeintel-diff-hunk-parser.ts        (mới) parse --raw -z, --numstat -z, --unified=0
  codeintel-diff-collection.ts         (mới) chạy git, untracked, dirty, maxBuffer fallback
  codeintel-hunk-symbol-mapping.ts     (mới) hunk -> symbol, độ tin cậy theo tệp
  codeintel-detect-changes-flows.ts    (mới) SYMBOL_FLOWS, MEMBER_CLUSTER
  gitnexus-detect-changes-header.ts    (mới) 3 dòng đầu cho crossCheck
  codeintel-detect-changes.ts          (mới) handler, ngân sách thời gian
  codeintel-method-table.ts            (sửa) +1 dòng, timeout 55 s
```
### 2.2 Hợp đồng và lệnh
Tham số/biên/`data` đúng §4.8 (không lặp). Git (tất cả ≤ 2.24; `-c core.quotePath=false --no-color --no-ext-diff --no-textconv`): `diff --raw -z --no-abbrev -M <mb> [<head>]`, `diff --numstat -z -M …`, `diff --unified=0 -M …`, `diff --name-only -z HEAD`, `ls-files --others --exclude-standard -z`; không `--merge-base`, `merge-tree --write-tree`, `--path-format`; không mạng; `rev-parse --verify --quiet --end-of-options <ref>^{commit}`. Base mặc định: `symbolic-ref --quiet refs/remotes/origin/HEAD` -> `origin/main|origin/master|main|master` -> `INVALID_PARAMS reason='base_required'`. So với **merge-base**; không `head` = cây làm việc (`diff <mergeBase>` + untracked ≤ 200, ≤ 2 MiB/tệp, nhị phân -> bỏ ánh xạ).
Hunk `-U0`: `@@ -a[,b] +c[,d] @@`, `d=0` xoá thuần -> `[c,c]` `pureDeletion` (`c=0` -> 1); khớp tệp theo thứ tự `--raw`; lệch -> `hunk_file_order_mismatch`; tràn `maxBuffer` -> chỉ `--raw`+`--numstat` + `hunks_unavailable_diff_too_large`; tên có ký tự điều khiển -> `unmapped.unsafePathFiles`.
Ánh xạ: nhóm 100 đường dẫn `FILE_SYMBOLS_BATCH` song song ≤ 3; `s ≤ se ∧ e ≥ ss`; ưu tiên symbol con (`containers`); `value` chỉ khi không có hàm/lớp chứa; `Section`→`doc`; độ tin cậy: `exact` khi `indexedCommit==headOid` và tệp sạch, hoặc tệp không thuộc `git diff --name-only <indexedCommit> <headOid>` (khi `cat-file -e` được); tệp bẩn luôn `approximate`; `index_commit_unreachable`; tệp `D` -> symbol từ chỉ mục `deleted`/`approximate`; không bịa symbol cho mã mới. Giới hạn: 2 000 symbol, 1 000 tệp ánh xạ, 5 000 `changedFiles`, 200 luồng. Luồng: `SYMBOL_FLOWS` nhóm 300, mỗi luồng một lần. `crossCheck`: `gitnexus detect-changes` chỉ parse 3 dòng đầu bằng regex cố định -> `riskHint`, `crosscheck_mismatch` khi lệch > 20%. Ngân sách 55 s; hết giờ giữa chừng -> trả phần đã có `truncated:true` + `deadline_partial`; `TIMEOUT` chỉ khi chưa có `changedFiles`.

## 3. Quyết định thiết kế
| # | Quyết định | Lý do |
|---|---|---|
| 1 | Tự tính diff và ánh xạ; CLI chỉ đối chiếu | CLI cắt 15 symbol/10 luồng |
| 2 | merge-base | O7 |
| 3 | Không mạng, Git ≤ 2.24 | baseline 2.25 |
| 4 | Không sửa `branchCompare` | rủi ro hồi quy, thiếu ở `desktop/` |
| 5 | `-z` + `--raw` lấy tên tệp | tên bị trích dẫn ở dạng text |
| 6 | Trả một phần khi hết giờ | dùng được trong review |

## 4. Thứ tự task
```
01 ─► 03 ─► 04 ─► 07 ─► 08
02 ─► 03        05 ─► 07
06 ───────────────────► 07
```
01 merge-base; 02 parser (thuần); 03 thu thập diff cần 01, 02; 04 ánh xạ cần 03 và AG-CV-TASK-002-04/08; 05 luồng/cụm; 06 crossCheck header; 07 handler cần 04, 05, 06; 08 re-verify.

## 5. Tiêu chí chấp nhận
- [ ] Sửa một hàm TS đã index -> đúng symbol trong cùng (innermost), dòng 1-based; `exact` khi chỉ mục khớp HEAD, tệp bẩn -> `approximate`.
- [ ] `base=HEAD~30` (CLI báo 1 508 symbol) trả đến 2 000, không cắt ở 15; mỗi luồng một lần.
- [ ] Tệp untracked `status:'A'`, `untracked:true`; nhị phân/tên có xuống dòng không vỡ phản hồi; `R` có `oldPath`; `D` đúng quy tắc.
- [ ] `base` xấu/`-x`/không merge-base -> `INVALID_PARAMS` đúng `reason`, không spawn công cụ.
- [ ] `index_commit_differs_from_head`, `index_commit_unreachable`; `mappingConfidence ≠ exact` khi cũ.
- [ ] Không lệnh Git > 2.24, không mạng; diff vượt `maxBuffer` không lỗi; `crossCheck` chỉ thêm `riskHint`.
- [ ] Hết giờ -> `truncated:true`, `deadline_partial`.

## 6. Kiểm thử
`/opt/repos/orca/agent`: `pnpm exec vitest run src/relay/codeintel-merge-base-resolution.test.ts src/relay/codeintel-diff-hunk-parser.test.ts src/relay/codeintel-diff-collection.test.ts src/relay/codeintel-hunk-symbol-mapping.test.ts src/relay/codeintel-detect-changes-flows.test.ts src/relay/gitnexus-detect-changes-header.test.ts src/relay/codeintel-detect-changes.test.ts`. Repo git thật trong thư mục tạm; `gitnexus` giả phát lại fixture. Ma trận Git (2.25.5 / 2.38.1 / 2.49.1 theo `git-compatibility.md`, tên job CI chưa kiểm chứng) cho hai test parser/merge-base.

## 7. Rủi ro và điểm chưa kiểm chứng
Độ chính xác phụ thuộc chỉ mục (cũ, hoặc của checkout chính ở worktree liên kết); chưa biết `analyze` lập chỉ mục từ working tree hay commit; `handler` vô danh bị gộp; hiệu năng diff lớn chưa đo (ước 10–20 s); `diff -M` có thể bỏ dò đổi tên; `ls-files --others` chậm trên repo lớn; hành vi `detect-changes -s compare` chưa rõ; `MATCH (n)` + `IN` chưa chạy.

## 8. Câu hỏi mở
1. `head` mặc định = commit để so lượt (CR Q1; O-13). 2. Phân trang thay cắt 2 000 (Q2). 3. Gộp `codegraph affected` (Q3). 4. Tệp bẩn luôn `approximate` (Q4).

## 9. Tham chiếu
CR-CV-005; contract §4.8; `git-handler-ops.ts:124`, `agent-git-handler-extended.ts:42`; `guides/reference/git-compatibility.md`.
