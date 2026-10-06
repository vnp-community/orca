# CR-CV-005 — `codeintel.detectChanges`: ánh xạ diff → symbol và luồng bị ảnh hưởng

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-005 |
| **Tên** | Method `codeintel.detectChanges`: agent tự tính diff theo merge-base bằng Git, ánh xạ hunk sang symbol bằng Cypher của GitNexus, tìm luồng thực thi bị chạm; `gitnexus detect-changes` chỉ dùng để đối chiếu |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-001, CR-CV-002 (mẫu `FILE_SYMBOLS_BATCH`, `SYMBOL_FLOWS`, `MEMBER_CLUSTER`, `SymbolRef`); CR-CV-003 (tuỳ chọn: `affected` cho test) |
| **Mở khoá** | CR-CV-036 (change overlay, thứ tự đọc, test gap, chấm rủi ro), CR-CV-053 (lens Ảnh hưởng), CR-CV-021 |
| **Tác động** | `agent/src/relay/codeintel-method-table.ts` (thêm dòng, timeout riêng), các file mới `agent/src/relay/codeintel-detect-changes*.ts`, `codeintel-diff-hunk-parser.ts`, `codeintel-merge-base-resolution.ts`, `codeintel-hunk-symbol-mapping.ts`, `gitnexus-detect-changes-header.ts` |

---

## 1. Bối cảnh và vấn đề

Đã chạy thật (chỉ lệnh đọc) ngày 2026-10-05 trên `/opt/repos/orca` với `gitnexus 1.6.9`, Git 2.43.0.

1. **`gitnexus detect-changes` chỉ in văn bản và cắt cứng.** Dạng đầu ra: ba dòng đầu `Changes: N files, M symbols`, `Affected processes: K`, `Risk level: low|...|critical`, rồi `Changed symbols:` mỗi dòng `  Symbol <tên> → <tệp>` (không có kind, không có số dòng, không có uid), rồi `Affected execution flows:` mỗi dòng `  • A → B (n steps) — changed: x, y`. Với `-s compare -b HEAD~30`: `Changes: 446 files, 1508 symbols`, `Affected processes: 55`, `Risk level: critical`, nhưng **chỉ in 15 symbol** (`... and 1493 more`) và **10 luồng** (`... and 45 more`) dù thêm `-l 2000`. `-l` chỉ giảm được số hiển thị (`-l 5` in 5), không tăng quá 15. Các luồng lặp (cùng `Run → EventPublishingProcessor` 4 lần). Hệ quả: không thể dùng CLI để lấy danh sách đầy đủ ≤ 2 000 symbol mà README v7 yêu cầu.
2. **Phạm vi diff của CLI hạn chế:** `-s unstaged|staged|all|compare` và `-b <ref>` cho `compare`; không chọn được `head`, không rõ nó dùng `base...HEAD` hay hai chấm, và có gồm thay đổi chưa commit hay không (chưa kiểm chứng). Với Orca: `unstaged` → `2 files, 2 symbols` (`AGENTS.md`, `CLAUDE.md`, đúng `git status`); `compare -b HEAD~3` → `36 files`.
3. **Chỉ mục có thể cũ so với HEAD** (index `d819812`, HEAD `1b0c760` khi khảo sát) và GitNexus ánh xạ hunk theo chỉ mục: số dòng của symbol là của lúc `analyze`. Vì vậy ánh xạ hunk → symbol chỉ chính xác với các tệp chưa đổi từ commit được index.
4. **Số dòng GitNexus là 0-based** (CR-CV-002 1.7) trong khi diff của Git là 1-based.
5. **`DEFINES` bỏ sót phương thức** (CR-CV-002 2.4): phải truy vấn theo `n.filePath` (`MATCH (n) WHERE n.filePath IN [...]`; 100 đường dẫn của `agent/src/relay` → 827 hàng, 1,8 s, đã chạy).
6. **Agent đã có sẵn phần lớn hạ tầng Git cho "so với nhánh gốc":**
   - `agent/src/relay/git-handler-ops.ts:124` `branchCompare(git, worktreePath, baseRef, loadBranchChanges)`: `rev-parse --verify HEAD`, `rev-parse --verify <baseRef>`, `merge-base <baseOid> <headOid>`, `rev-list --count`, và trạng thái `ready | unborn-head | invalid-base | no-merge-base | error` (`:150-205`); `loadBranchChanges(mergeBase, headOid)` do bên gọi cung cấp.
   - `agent/src/relay/agent-git-handler-extended.ts:42` `git(args, cwd, opts)` (`execFile('git')`, `maxBuffer` 10 MiB, `:60` bản `gitBuffer`) và `:91` `handleGitBranchCompare` dùng `diff --name-status -M -C <mergeBase> <headOid>` kèm `-c core.quotePath=false`.
   - `agent/src/relay/agent-git-base-ref-handler.ts`: `git.baseRefDefault` thử `git symbolic-ref refs/remotes/origin/HEAD`, rồi **`git ls-remote --symref origin HEAD` (gọi mạng)**, rồi `git symbolic-ref HEAD`.
   - `agent/src/relay/git-exec-validator.ts` (whitelist `git.exec`, kể cả `merge-base`, `diff`).
7. **Git baseline 2.25** (`guides/reference/git-compatibility.md`): không dùng `git diff --merge-base` (2.30), `git merge-tree --write-tree` (2.38), `git rev-parse --path-format` (2.31). Các lệnh dưới đây chỉ dùng tính năng ≤ 2.24. `--end-of-options` có từ 2.24.
8. **Định dạng hunk `-U0`** (đã thấy): `@@ -83 +83 @@ <ngữ cảnh>`; số dòng bị bỏ khi bằng 1; số dòng 0 nghĩa là chèn/xoá thuần; với xoá thuần phía mới `+c,0` thì `c` là dòng **trước** vị trí xoá. `git diff --raw -z --no-abbrev -M` cho `:modes sha sha Status\0path\0[path2\0]`; với thay đổi working tree, sha phía mới là `0000…`.
9. **Mặc định `base`:** nhánh gốc của repo; `origin/HEAD` có (`git rev-parse --abbrev-ref origin/HEAD` → `origin/main`). Nhánh cục bộ `main` bằng `HEAD` khi khảo sát (merge-base = HEAD).
10. **Thay đổi chưa commit và tệp chưa theo dõi:** diff working tree so với một commit là `git diff <commit>` (tệp đã theo dõi, gồm staged + unstaged). Tệp chưa theo dõi lấy bằng `git ls-files --others --exclude-standard -z` (khảo sát: có nhiều thư mục `docs/crs/v7/...` chưa theo dõi). Đây là loại thay đổi phổ biến ngay sau khi agent viết code.

## 2. Giải pháp đề xuất

### 2.1 Hợp đồng

`codeintel.detectChanges`:

| Tham số | Kiểu | Mặc định | Ghi chú |
|---|---|---|---|
| `workspaceRoot` | string | bắt buộc | CR-CV-001 |
| `base` | string | nhánh gốc suy ra (2.2) | ref hoặc commit; 1..256 ký tự, không bắt đầu bằng `-`, chỉ gồm `[A-Za-z0-9._/@^~{}+-]` |
| `head` | string | không đặt = working tree | ref hoặc commit; đặt thì **không** tính thay đổi chưa commit |
| `includeUntracked` | bool | true (chỉ khi `head` không đặt) | tối đa 200 tệp chưa theo dõi |
| `withClusters` | bool | false | thêm `affectedClusters` (tốn thêm truy vấn) |
| `crossCheck` | bool | false | chạy `gitnexus detect-changes` để đối chiếu (2.6) |

Phản hồi (`data` = `ChangeOverlay` của README v7 mục 3.4, mở rộng bằng các trường dưới đây; `totalCount` = số symbol trước khi cắt; `truncated` khi đạt 2 000 symbol hoặc 1 000 tệp ánh xạ):

```jsonc
{ "base": { "ref": "origin/main", "oid": "1b0c760930b0…", "mergeBase": "1b0c760930b0…" },
  "head": { "ref": null, "oid": "1b0c760930b0…", "includesUncommitted": true, "dirtyFileCount": 2 },
  "changedFiles": [
    { "path": "agent/src/relay/context.ts", "oldPath": null, "status": "M", "additions": 3, "deletions": 1,
      "hunks": [ { "oldStart": 30, "oldLines": 1, "newStart": 30, "newLines": 3 } ],
      "untracked": false, "binary": false, "indexed": true, "driftedFromIndex": false } ],
  "changedSymbols": [
    { "symbol": { "key": "method:agent/src/relay/context.ts:RelayContext.registerRoot", "kind": "method", "name": "registerRoot",
                  "qualifiedName": "RelayContext.registerRoot", "filePath": "agent/src/relay/context.ts", "startLine": 31, "endLine": 33,
                  "gitnexusId": "Method:agent/src/relay/context.ts:RelayContext.registerRoot#1" },
      "change": "modified", "hunkCount": 1, "linesTouched": 3, "confidence": "exact", "containers": [ "type:agent/src/relay/context.ts:RelayContext" ] } ],
  "unmapped": { "filesWithoutSymbols": [ "docs/crs/v7/README.md" ], "filesNotIndexed": [], "filesBeyondCap": 0, "unsafePathFiles": [] },
  "affectedFlows": [
    { "flowId": "proc_0_checkspanel", "label": "ChecksPanel → TrimRuntimePathTrailingSlash", "stepCount": 9,
      "changedSymbolKeys": [ "function:frontend/src/renderer/src/components/right-sidebar/ChecksPanel.tsx:ChecksPanel" ], "earliestChangedStep": 1 } ],
  "affectedClusters": null,
  "riskHint": null,
  "index": { "commit": "d8198127b6bd…", "stale": true, "driftedFileCount": 412, "mappingConfidence": "mixed" },
  "warnings": [ "index_commit_differs_from_head" ] }
```

Quy ước: đường dẫn tương đối gốc repo (`workspaceRoot`); số dòng 1-based (đã +1 từ GitNexus); `change ∈ modified | added | deleted` (`added` chỉ khi toàn bộ tệp mới; `deleted` khi tệp bị xoá, lấy từ chỉ mục: 2.4 bước 6); không có `risk` kiểu số, chỉ có `riskHint` (do chấm rủi ro thuộc CR-CV-036). Giới hạn: ≤ 2 000 symbol; ≤ 1 000 tệp ánh xạ; ≤ 5 000 tệp trong `changedFiles` (phần còn lại chỉ đếm); ≤ 200 luồng.

### 2.2 Phân giải `base`, `head`, merge-base (`codeintel-merge-base-resolution.ts`)

Chạy tại `workspaceRoot` bằng helper `git()` đã xuất của `agent-git-handler-extended.ts:42` (không `shell`, `maxBuffer` 10 MiB). Thứ tự:

1. **Xác thực chuỗi** `base`/`head` (mẫu ở 2.1, không bắt đầu bằng `-`). Dùng `git rev-parse --verify --quiet --end-of-options <ref>^{commit}`; thất bại → `CODEINTEL_INVALID_PARAMS (field = "base", reason = "unresolved_ref")`.
2. **`base` mặc định**, **không gọi mạng** (khác `git.baseRefDefault` có `ls-remote`): (a) `git symbolic-ref --quiet refs/remotes/origin/HEAD` → `refs/remotes/origin/<tên>`; (b) thử lần lượt `refs/remotes/origin/main`, `origin/master`, `main`, `master` bằng `rev-parse --verify --quiet`; (c) không có → `CODEINTEL_INVALID_PARAMS (reason = "base_required")` kèm `hint`. Backend (CR-CV-012/036) nên truyền `base` tường minh (vốn đã biết nhánh gốc của worktree qua `git.baseRefDefault` có kiểm soát mạng); mặc định này chỉ để dùng độc lập.
3. **Tái dùng `branchCompare`** (`git-handler-ops.ts:124`) với `loadBranchChanges` là hàm của CR này (2.3): nhận lại `summary {baseOid, headOid, mergeBase, status, commitsAhead}`. Trạng thái `unborn-head` → trả `changedFiles: []` kèm `warnings: ["unborn_head"]`; `invalid-base` → `CODEINTEL_INVALID_PARAMS`; `no-merge-base` → `CODEINTEL_INVALID_PARAMS (reason = "no_merge_base")`; `error` → `CODEINTEL_TOOL_FAILED`. Khi `head` đặt, dùng `headOid` đã phân giải thay cho `HEAD` (cần mở rộng nhẹ `branchCompare` để nhận `headRef`; hoặc gọi trực tiếp `git merge-base <baseOid> <headOid>` trong file mới nếu không muốn sửa hàm có sẵn; chưa kiểm chứng ai khác gọi `branchCompare` với chữ ký hiện tại; chạy `gitnexus impact` trước khi sửa).
4. **Merge-base** là điểm so sánh (O7): thay đổi của worktree **so với chỗ nhánh tách khỏi nhánh gốc**, không phải so với đầu nhánh gốc hiện tại; nhờ vậy không tính thay đổi của người khác trên nhánh gốc.

### 2.3 Thu thập diff (`codeintel-diff-hunk-parser.ts`)

Lệnh (đều ≤ Git 2.24; thêm `-c core.quotePath=false`, `--no-color`, `--no-ext-diff`, `--no-textconv`):

| Mục đích | Lệnh |
|---|---|
| Danh sách tệp và trạng thái | `git diff --raw -z --no-abbrev -M <mergeBase> [<headOid>]` |
| Số thêm/xoá | `git diff --numstat -z -M <mergeBase> [<headOid>]` |
| Hunk | `git diff --unified=0 -M <mergeBase> [<headOid>]` |
| Tệp bẩn so với HEAD | `git diff --name-only -z HEAD` (khi `head` không đặt) |
| Tệp chưa theo dõi | `git ls-files --others --exclude-standard -z` |

- Khi `head` không đặt: so **`<mergeBase>` với working tree** (bỏ tham số `<headOid>`): gồm commit, staged và unstaged của tệp đã theo dõi trong một lần. Tệp chưa theo dõi là `added`, hunk giả `+1,N` với `N` = số dòng (đếm bằng đọc tệp, ≤ 2 MiB mỗi tệp, tệp có byte NUL trong 8 KiB đầu là `binary`, bỏ ánh xạ).
- Phân tích hunk: dòng bắt đầu `@@ -a[,b] +c[,d] @@`; `b`, `d` mặc định 1. Phía mới `[c, c+d-1]` nếu `d > 0`; `d = 0` (xoá thuần) → khoảng `[c, c]` đánh dấu `pureDeletion` (vị trí chèn nằm sau dòng `c`; coi như chạm symbol chứa dòng `c`, hoặc `c+1` nếu `c = 0`). Tên tệp lấy từ **`--raw -z`** (NUL, không bị trích dẫn); khớp các khối hunk theo thứ tự tệp (Git in cùng thứ tự); khi lệch (số tệp khác) → bỏ ánh xạ hunk, `warnings: ["hunk_file_order_mismatch"]` và vẫn trả `changedFiles`. Tệp nhị phân (`Binary files … differ` hoặc `numstat` `-\t-`) không có hunk.
- Đổi tên (`R`): dùng đường dẫn mới; nếu không có trong chỉ mục thì thử đường dẫn cũ (`oldPath`).
- Tệp bị xoá (`D`): không có hunk phía mới; xem 2.4 bước 6.
- Tràn `maxBuffer` (diff lớn): bắt lỗi (tệp `git-buffer-overflow.ts` đã có kiểu xử lý), chuyển sang chế độ chỉ `--raw` + `--numstat` (không hunk), `warnings: ["hunks_unavailable_diff_too_large"]`; mọi tệp vào `filesWithoutSymbols`.
- Tên tệp chứa ký tự điều khiển (kể cả xuống dòng) không thể đưa vào Cypher (CR-CV-002 2.2) → `unmapped.unsafePathFiles`.

### 2.4 Ánh xạ hunk → symbol (`codeintel-hunk-symbol-mapping.ts`)

1. Danh sách tệp cần ánh xạ = tệp có hunk, không nhị phân, không `unsafePath`, ≤ 1 000 (còn lại `filesBeyondCap`).
2. Chia nhóm 100 đường dẫn, chạy mẫu `FILE_SYMBOLS_BATCH` (CR-CV-002) song song tối đa 3 (do giới hạn đồng thời CR-CV-001): `MATCH (n) WHERE n.filePath IN {{paths}} RETURN n.filePath, n.id, label(n), n.startLine, n.endLine, n.name`. Đo: 100 tệp 1,8 s; 1 000 tệp ≈ 10 nhóm ≈ 6-7 s (ước tính). Chuẩn hoá dòng +1.
3. Loại nút `File` (không có dòng). Với từng hunk phía mới `[s, e]`, symbol `[ss, se]` được chọn khi `s ≤ se && e ≥ ss`.
4. **Chọn lọc lồng nhau**: nếu hunk nằm trọn trong một symbol con (method trong class) chỉ giữ symbol con và ghi lớp chứa vào `containers`; nếu hunk nằm trong thân class nhưng ngoài mọi method thì giữ class. Nút `Const`/`Variable`/`Property` chỉ giữ nếu không có symbol hàm/lớp nào chứa hunk (ví dụ hằng cấp tệp). `Section` (tài liệu markdown) giữ với `kind: doc`.
5. **Độ tin cậy theo tệp** (chống sai do chỉ mục cũ, 1.3): đọc `indexedCommit` (probe CR-CV-002). `confidence = "exact"` khi `indexedCommit === headOid` **và** tệp không có trong `dirtyFiles`; ngược lại tính `driftedFiles = git diff --name-only -z <indexedCommit> <headOid>` (chỉ khi `git cat-file -e <indexedCommit>^{commit}` thành công): tệp **không** thuộc `driftedFiles` và không bẩn → `exact`; còn lại `approximate`. Nếu `indexedCommit` không còn trong kho → mọi tệp `approximate`, `warnings: ["index_commit_unreachable"]`. `index.mappingConfidence` = `exact | mixed | approximate`. (Không biết chắc chỉ mục có phản ánh thay đổi chưa commit lúc `analyze` hay không; vì vậy tệp bẩn luôn là `approximate`.)
6. **Tệp xoá/ không có trong chỉ mục:** `D` → liệt kê symbol của tệp theo chỉ mục dưới `change: "deleted"` với `confidence: approximate` (chỉ khi chỉ mục còn dữ liệu của tệp); tệp `A` hoặc có hunk nhưng không tìm thấy nút nào → `unmapped.filesNotIndexed`; tệp có nút nhưng không hunk nào chạm → `unmapped.filesWithoutSymbols`. Không bịa symbol cho mã mới chưa được index.
7. Cắt tại 2 000 symbol (`truncated: true`; ưu tiên tệp theo thứ tự `additions + deletions` giảm dần).

### 2.5 Luồng bị ảnh hưởng, cụm, test

- **Luồng:** chia id symbol (`gitnexusId`) thành nhóm 300, chạy `SYMBOL_FLOWS` (CR-CV-002): `MATCH (s)-[r:CodeRelation {type:'STEP_IN_PROCESS'}]->(p:Process) WHERE s.id IN {{ids}} RETURN s.id, p.id, p.stepCount, r.step, p.label`. Gom theo `p.id`: `changedSymbolKeys`, `earliestChangedStep = min(step)`. Cắt 200 luồng theo số symbol bị chạm giảm dần. Khác với CLI (lặp luồng, chỉ 10 dòng), mỗi luồng xuất hiện **một lần**. Thời gian: tối đa 7 nhóm (2 000 symbol) song song 3 ≈ 5-6 s (ước tính).
- **Cụm** (chỉ khi `withClusters`): `MEMBER_CLUSTER` theo nhóm 300; `affectedClusters: [{id, label, changedSymbols}]`; nhãn cụm bổ sung từ `OV_CLUSTERS` không cần, dùng `c.id` (backend ghép nhãn từ `overview`).
- **Test:** không tính ở đây. CR-CV-036 sẽ gọi `codegraph affected` (CR-CV-003 2.6) với danh sách `changedFiles`; agent không làm trong method này để tránh kéo dài thời gian.

### 2.6 Đối chiếu với `gitnexus detect-changes` (tuỳ chọn)

Khi `crossCheck: true`, chạy một lần `gitnexus detect-changes -r <path> -s <scope> [-b <base>]` (qua `runCodeIntelTool` với động từ `detect-changes` đã nằm trong whitelist của CR-CV-001 chỉ cho mục đích này), `scope` = `unstaged` khi `head` không đặt và không có `base`; `compare` với `-b <base>` khi có (semantic chính xác của `compare` **chưa kiểm chứng**). Parse **chỉ ba dòng đầu** bằng biểu thức chính quy cố định (`^Changes: (\d+) files?, (\d+) symbols?$`, `^Affected processes: (\d+)$`, `^Risk level: (\w+)$`); bất kỳ lệch nào → bỏ qua kết quả (không lỗi). Trả `riskHint: {source: "gitnexus detect-changes", level, files, symbols, processes}` và, nếu số symbol/tệp khác số của agent quá 20%, thêm cảnh báo `crosscheck_mismatch` (không sửa kết quả). **Không** parse danh sách symbol/luồng của CLI. Lý do chỉ dùng như thông tin phụ: định dạng văn bản, cắt 15/10 dòng, không rõ phạm vi (1.1, 1.2).

### 2.7 Thời gian và giới hạn

Phần Git ~0,1-1 s (một vài lệnh, repo lớn có thể lâu hơn; chưa đo trên diff rất lớn); ánh xạ 1,8-7 s; luồng 2-6 s; cụm +2-6 s. Timeout riêng của method: **55 s** (đăng ký trong `codeintel-method-table.ts`), cao hơn mặc định 25 s của CR-CV-001, **với điều kiện** CR-CV-023 nâng timeout Go cho `codeintel.detectChanges` (hiện mọi method ngoài `agent.execPrompt` bị cắt ở 30 s: `client.go:412-417`). Nếu CR-CV-023 chưa nâng, đặt `ORCA_CODEINTEL_DETECT_TIMEOUT_MS=25000` và chấp nhận `truncated` sớm (bỏ `affectedClusters`, rồi cắt luồng). Khi hết ngân sách giữa chừng, trả những gì đã có với `truncated: true` và `warnings: ["deadline_partial"]` thay vì lỗi; chỉ trả `CODEINTEL_TIMEOUT` khi chưa có `changedFiles`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Agent tự tính diff và ánh xạ; CLI chỉ đối chiếu | CLI cắt 15 symbol/10 luồng, text, phạm vi không rõ (1.1, 1.2) |
| So với **merge-base** | O7; chỉ chứa thay đổi của nhánh đang review |
| Working tree = `diff <mergeBase>` + tệp chưa theo dõi | Phản ánh đúng thay đổi agent vừa viết, kể cả chưa commit |
| Không gọi mạng để tìm nhánh gốc | `git.baseRefDefault` có `ls-remote` (độ trễ SSH, thất bại offline) |
| Chỉ lệnh Git ≤ 2.24 | Baseline 2.25 (`git-compatibility.md`); không cần `GitCapabilityCache` mới |
| Dòng GitNexus +1 và độ tin cậy theo tệp | Số dòng chỉ mục là của lúc `analyze` |
| Không bịa symbol cho mã mới chưa index | Trung thực: báo `filesNotIndexed`, UI nói "chưa được index" |
| `-z` và `--raw` để lấy tên tệp | Tên có khoảng trắng/Unicode/dấu nháy bị Git trích dẫn trong dạng text |
| Trả một phần khi hết thời gian | Có kết quả dùng được trong review thay vì lỗi |

## 4. Tiêu chí chấp nhận

- [ ] Trên repo thử: sửa một hàm TypeScript đã index rồi gọi `codeintel.detectChanges` (không `head`) trả đúng symbol (innermost) với `startLine/endLine` 1-based, `confidence: exact` khi chỉ mục khớp HEAD và tệp bẩn → `approximate` (đúng quy tắc 2.4.5).
- [ ] Số symbol > 15 (ví dụ `base = HEAD~30` trên Orca: 1 508 symbol theo CLI) được trả đầy đủ đến 2 000, không bị cắt ở 15; `luồng` mỗi luồng xuất hiện một lần.
- [ ] Tệp chưa theo dõi xuất hiện với `status: "A"`, `untracked: true`; tệp nhị phân và tệp có tên chứa xuống dòng không làm vỡ phản hồi (`binary: true` / `unsafePathFiles`).
- [ ] Đổi tên (`R`) trả `oldPath`; xoá (`D`) trả `change: "deleted"` hoặc `filesNotIndexed` đúng quy tắc.
- [ ] Base không tồn tại, `base` bắt đầu bằng `-`, repo không có merge-base: lỗi `CODEINTEL_INVALID_PARAMS` với `reason` tương ứng; không spawn công cụ thứ hai.
- [ ] Chỉ mục cũ (`indexedCommit ≠ HEAD`) sinh `warnings: ["index_commit_differs_from_head"]` và `mappingConfidence ≠ exact`; chỉ mục không còn trong kho sinh `index_commit_unreachable`.
- [ ] Không có lệnh Git nào dùng tính năng > 2.24 (kiểm bằng test chạy với `git` baseline 2.25 nếu CI có ma trận, hoặc quét argv); không có lệnh gọi mạng.
- [ ] Diff vượt `maxBuffer` trả `changedFiles` không hunk kèm cảnh báo, không lỗi.
- [ ] `crossCheck: true` chỉ thêm `riskHint`; thay đổi định dạng dòng đầu của CLI chỉ làm `riskHint = null`, không làm hỏng method.
- [ ] Hết ngân sách thời gian trả kết quả một phần với `truncated: true`.
- [ ] Không có tên file `helpers/utils/common/misc`; không `max-lines` disable; `pnpm lint`, `pnpm test` trong `agent/` xanh.

## 5. Kiểm thử

| File test (mới) | Nội dung |
|---|---|
| `agent/src/relay/codeintel-diff-hunk-parser.test.ts` | `@@` các dạng (`-83 +83`, `-5,0 +6,3`, xoá thuần), `--raw -z` có đổi tên/chép, tên có khoảng trắng/Unicode/dấu nháy, nhị phân, thứ tự lệch |
| `agent/src/relay/codeintel-merge-base-resolution.test.ts` | Repo git thật trong thư mục tạm: mặc định `origin/HEAD`, `main`, `master`, không có; `no-merge-base`; `unborn-head`; `base` độc hại |
| `agent/src/relay/codeintel-hunk-symbol-mapping.test.ts` | Chọn symbol lồng nhau, hằng cấp tệp, `Section`, độ tin cậy theo tệp bẩn/lệch, cơ số dòng, cắt 2 000 |
| `agent/src/relay/codeintel-detect-changes.test.ts` | Toàn luồng với repo git tạm + binary `gitnexus` giả phát lại fixture của `FILE_SYMBOLS_BATCH`/`SYMBOL_FLOWS` (CR-CV-002); working tree bẩn, chưa theo dõi, hết ngân sách |
| `agent/src/relay/gitnexus-detect-changes-header.test.ts` | Ba dòng đầu thật (`Changes: 446 files, 1508 symbols`…), dòng lạ, rỗng |

Git baseline: nếu CI có job Git 2.25.5/2.38.1/2.49.1 của `git-compatibility.md`, đưa `codeintel-merge-base-resolution.test.ts` và `codeintel-diff-hunk-parser.test.ts` (chế độ integration với `git` thật) vào ma trận; chưa kiểm chứng tên job hiện có. Chưa chạy gì ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chính xác của ánh xạ phụ thuộc chỉ mục.** Chỉ mục cũ hoặc là của checkout chính (worktree liên kết, CR-CV-001 1.5) làm số dòng sai. Phép chống đỡ (độ tin cậy theo tệp) chỉ phát hiện được tệp đổi giữa `indexedCommit` và HEAD, không phát hiện chỉ mục vốn dựng từ working tree khác. Kết quả cho worktree liên kết phải được UI diễn đạt như "ước lượng".
- Không biết `gitnexus analyze` lập chỉ mục từ **working tree hay từ commit** khi có thay đổi chưa commit (tài liệu `--branch` nói "workspace index follows the checked-out working tree"); giả định bảo thủ: tệp bẩn là `approximate`.
- Nhiều symbol vô danh cùng tên (`handler`) bị gộp bởi GitNexus (CR-CV-002 mục 6) → `changedSymbols` có thể gộp/thiếu.
- Hiệu năng trên diff lớn (446 tệp, 1 508 symbol theo CLI) chưa đo end-to-end; ước lượng 10-20 s. Timeout Go 30 s (`client.go:412-417`) có thể không đủ nếu CR-CV-023 không nâng.
- `git diff -M` có thể bỏ qua dò đổi tên khi quá nhiều tệp (cảnh báo `diff.renameLimit` trên stderr); khi đó đổi tên thành `D` + `A`. Chấp nhận, ghi cảnh báo nếu stderr có chữ `rename detection was skipped` (chưa kiểm chứng chuỗi chính xác).
- `git ls-files --others` trên repo rất lớn có thể chậm (chưa đo); đã giới hạn 200 tệp trả về nhưng lệnh vẫn quét hết.
- Hành vi `gitnexus detect-changes -s compare` (hai chấm hay ba chấm, có gồm working tree) chưa rõ; chỉ dùng làm gợi ý (2.6).
- Sửa `branchCompare` để nhận `headRef` (2.2.3) có thể ảnh hưởng người gọi hiện có; cần `gitnexus impact` và test hiện có (`git-handler.test.ts`, `agent-git-handler-extended` ...) trước khi sửa.

## 7. Câu hỏi mở

- **Q1.** `head` có nên hỗ trợ `HEAD` mặc định = "commit" (bỏ thay đổi chưa commit) để so sánh các lượt review theo commit không (CR-CV-060)? Hiện: đặt `head` tường minh.
- **Q2.** Ngưỡng 2 000 symbol có cần phân trang (offset) thay vì cắt? Hiện cắt và báo `truncated`.
- **Q3.** Có nên thêm `codegraph affected` vào chính method này (tiết kiệm một lượt RPC cho CR-CV-036)? Hiện tách để giữ thời gian.
- **Q4.** Chấp nhận quy ước "tệp bẩn luôn `approximate`" hay thử xác định chỉ mục có dựng từ working tree không (cần thử nghiệm `gitnexus analyze` trên repo thử)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 2 O7; mục 3.2, 3.4)
- `/opt/repos/orca/docs/research/view-code/05-graph-schemas.md` §2.7, `08-views-and-review-models.md`
- `/opt/repos/orca/guides/reference/git-compatibility.md`
- `/opt/repos/orca/agent/src/relay/git-handler-ops.ts` (`branchCompare` `:124`), `/opt/repos/orca/agent/src/relay/agent-git-handler-extended.ts` (`git` `:42`, `handleGitBranchCompare` `:91`), `/opt/repos/orca/agent/src/relay/agent-git-base-ref-handler.ts`, `/opt/repos/orca/agent/src/relay/git-exec-validator.ts`, `/opt/repos/orca/agent/src/relay/git-buffer-overflow.ts`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (`execTimeoutForMethod` `:412`)
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md`, `CR-CV-002-gitnexus-extraction.md`, `CR-CV-003-codegraph-extraction.md`
- Đầu ra `gitnexus detect-changes` (chạy ngày 2026-10-05, `-s unstaged`, `-s compare -b HEAD~3`, `-b HEAD~30`, `-l 5`, `-l 2000`)
