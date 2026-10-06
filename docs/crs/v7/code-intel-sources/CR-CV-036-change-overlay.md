# CR-CV-036 — Change overlay: thay đổi của worktree → symbol, luồng, bảng, hợp đồng, thứ tự đọc và điểm rủi ro minh bạch

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-036 |
| **Tên** | Dựng `ChangeOverlay` và `readingOrder` từ diff worktree so với merge-base, `codeintel.detectChanges` và `codeintel.impact`; chấm rủi ro bằng quy tắc cộng điểm có lý do hiển thị được |
| **Loại** | Feature (backend `code-intel-service`) |
| **Priority** | 🔴 P0 (lõi MVP review, đợt 3) |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-005 (`codeintel.detectChanges`), CR-CV-002 (`codeintel.impact`, cần thêm tham số, xem 2.2), CR-CV-020 (`SymbolRef`, kind), CR-CV-021 (collector gọi agent), CR-CV-030 (đọc file/`git.*` an toàn), CR-CV-012 (ánh xạ worktree → dev server → repo, `stale`). Dùng tuỳ chọn, suy giảm nhẹ nếu thiếu: CR-CV-031 (`touchedTables`), CR-CV-032 (`touchedContracts`), CR-CV-033 (nhóm theo component C4), CR-CV-037 (`violations`), CR-CV-038 (phân loại phá vỡ tương thích). Nguồn ngoài (research 09): **E5** lịch sử/diff git, **E11** coverage (chưa xác nhận; mặc định dùng cạnh test→hàm của GitNexus), **E16** ánh xạ project/worktree → repo |
| **Mở khoá** | CR-CV-040 (kênh `codeIntel.changeOverlay`, `codeIntel.readingOrder`), CR-CV-051/052/053 (khung Review, thứ tự đọc, lens Ảnh hưởng), CR-CV-037/038 (bổ sung vào overlay), CR-CV-060 (so sánh với lượt trước) |
| **Tác động** | `backend-go/services/code-intel-service` (mới): package change overlay + RPC `GetChangeOverlay`, `GetReadingOrder`; `backend-go/proto/orca/codeintel/v1` (message `ChangeOverlay`, `ReadingStep`, `RiskAssessment`…); không sửa agent (chỉ dùng RPC `git.*` có sẵn và `codeintel.*` của CR-CV-002/005) |

---

## 1. Bối cảnh và vấn đề

Mục tiêu của series v7 là review nhanh sau khi agent code. Câu hỏi đầu tiên của người review: "đã đổi gì, ảnh hưởng đâu, nên đọc theo thứ tự nào, rủi ro bao nhiêu và vì sao". Hiện chưa có gì trả lời (không có `code-intel-service`, README v7 mục 1).

Đã đọc code thật (2026-10-05), các điểm quyết định thiết kế:

1. **`git.branchCompare` chỉ so commit, không so working tree.** `agent/src/relay/git-handler-ops.ts:124` (`branchCompare`) tính `merge-base baseOid headOid` rồi `diff --name-status -M -C mergeBase headOid` (`agent/src/relay/agent-git-handler-extended.ts:104`–`112`) — `headOid` là `rev-parse HEAD`. Công việc agent chưa commit **không có** trong kết quả. Trả về `{summary{baseRef, baseOid, compareRef, headOid, mergeBase, changedFiles, commitsAhead, status}, entries[{path, oldPath?, status, added?, removed?}]}`; `status` ∈ `loading|ready|unborn-head|invalid-base|no-merge-base|error`. Phần chưa commit lấy từ `git.status` (`agent-rpc-dispatch-git-status.ts:20`, giới hạn `DEFAULT_GIT_STATUS_LIMIT`; cấu trúc kết quả của `getStatusOp` **chưa đọc**, người triển khai phải đọc).
2. Nhánh gốc mặc định: `git.baseRefDefault` (`agent-rpc-dispatch-git.ts:190`, `agent-git-base-ref-handler.ts`): ưu tiên `refs/remotes/origin/HEAD`, rồi `git ls-remote --symref origin HEAD` (timeout 10 s), rồi nhánh hiện tại.
3. **`git.exec` (Part A) có whitelist**: `status diff add restore commit push pull fetch branch checkout merge rebase stash log worktree remote tag show rev-parse config describe shortlog` (`agent/src/relay/agent-git-handler.ts`, `ALLOWED_GIT_SUBCOMMANDS`). **Không có** `merge-base`, `rev-list`, `ls-tree`, `diff-tree`. Mọi đối số bị từ chối nếu chứa `& | ; $ \` < > \ !` (`SHELL_METACHARACTERS`); cờ `-c`, `-o`, `--output`, `--upload-pack` bị cấm (`agent-git-exec-validator.ts`); tối đa 60 s; stdout đọc vào bộ nhớ không giới hạn. Hệ quả: lấy merge-base bằng `git.branchCompare`, không gọi `git merge-base` trực tiếp; đếm commit bằng `git log` có `-n`.
4. **`gitnexus detect-changes` in văn bản, không JSON** (đã chạy: `gitnexus detect-changes -r orca -s compare -b HEAD~3 -l 5` ra `Changes: 36 files, 6 symbols / Affected processes: 0 / Risk level: low` rồi dòng `Symbol <tên> → <file>`). Scope: `unstaged|staged|all|compare` (+ `--base-ref`, `--limit`). Ví dụ cho thấy tiêu đề Markdown cũng thành "symbol" (`Section`: `GitNexus — Code Intelligence → AGENTS.md`) → phải lọc kind tài liệu ra khỏi rủi ro và test. Việc parse thuộc CR-CV-005; CR này chỉ yêu cầu hợp đồng đầu ra ở 2.2.
5. **`gitnexus impact` trả JSON** (đã chạy): `risk` ∈ `LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN`, `summary{direct, processes_affected, modules_affected}`, `byDepthCounts`, `byDepth{<n>:[{id,name,filePath,relationType,confidence,processes}]}`, `affected_processes`, `affected_modules[{name,hits,impact}]`, `pagination{limit,offset,truncated}`, `epistemic:"exact"`. Cờ `--include-tests` đưa hàm test vào kết quả (đã xác nhận: `ParseCodeowners` có `TestParseCodeowners…` ở độ sâu 1 qua `CALLS`). Mặc định không có test. Tên không rõ → `error: "Target … not found"`, `risk:"UNKNOWN"`; cần `-f <file>`/`--uid` để khử mơ hồ. Mỗi lệnh CLI ~1,8 s (README v7 mục 1) → không thể gọi `impact` cho hàng nghìn symbol.
6. **`gitnexus context`** trả `incoming.calls[]` / `outgoing.calls[]` (uid, name, filePath) — nguồn thay thế để tìm test phủ và cạnh `CALLS` giữa symbol đã đổi.
7. Chỉ có index theo commit đã `analyze`; `.gitnexus/meta.json` có `lastCommit`. Index có thể cũ hơn HEAD và không biết thay đổi chưa commit → xem 2.6.

Vấn đề cần giải: (a) hợp nhất ba nguồn rời rạc thành một mô hình, (b) đưa ra thứ tự đọc có thể giải thích, (c) điểm rủi ro mà người dùng có thể kiểm tra từng dòng, (d) trung thực khi dữ liệu thiếu hoặc cũ.

## 2. Giải pháp đề xuất

### 2.1 Phạm vi và đầu vào (O7)

RPC (mới, tên theo README mục 3.6): `GetChangeOverlay(GetChangeOverlayRequest) → GetChangeOverlayResponse`, `GetReadingOrder(GetReadingOrderRequest) → GetReadingOrderResponse`. Tham số: `worktree_id` (hoặc `repo_binding_id`), `base_ref?` (mặc định: `git.baseRefDefault`), `mode` ∈ `worktree` (mặc định) | `committed`, `detail` ∈ `summary` | `full`. Mặc định đúng O7: **thay đổi của worktree so với merge-base với nhánh gốc**, gồm cả chưa commit.

Các bước (collector CR-CV-021, tuần tự, mỗi bước có thể thất bại độc lập):

1. `git.baseRefDefault` (nếu client không gửi `base_ref`).
2. `git.branchCompare {worktreePath, baseRef}` → `mergeBase`, `headOid`, `entries` (đã commit). `status != ready` → xem 2.8.
3. Nếu `mode=worktree`: `git.status` → các tệp staged/unstaged/untracked. Hợp nhất theo `path`: trạng thái cuối = trạng thái trong working tree nếu có, ngược lại trạng thái đã commit; cộng dồn `added/removed` không trùng lặp bằng cách lấy số liệu working tree cho tệp có ở cả hai (số liệu `git diff mergeBase` của tệp đó; gọi `git.exec` `diff --numstat <mergeBase> -- <path>` cho đúng các tệp ở cả hai phía; `diff` có trong whitelist).
4. `codeintel.detectChanges {base: mergeBase, head?}` (CR-CV-005): `head` bỏ trống = working tree. Nếu CR-CV-005 chưa hỗ trợ "mergeBase → working tree" thì dùng `mode=committed` và hiển thị cảnh báo; xem 7.
5. `codeintel.impact` cho tối đa `K=25` symbol ưu tiên (2.4), `direction=upstream`, `depth=2`, **`includeTests=true`**.
6. Tra cứu bảng/hợp đồng (2.5), vi phạm (CR-CV-037), component C4 (CR-CV-033) từ cache/snapshot của các view đó theo `commit` — không tính lại ở đây.
7. Tính `readingOrder` (2.3), rủi ro (2.4), độ cũ (2.6), cắt giới hạn (2.7); lưu snapshot `graph_snapshots(view="changeOverlay")` (CR-CV-022).

Đường dẫn trong dữ liệu tương đối gốc repo (README 3.4). Lệnh git mới thêm: chỉ `git log` có giới hạn (độ cũ) và `diff --numstat`; cả hai là tính năng từ git ≤ 2.25, không cần `GitCapabilityCache`. Không dùng `|`, `<`, `>`, `!`, `\`, `$` trong đối số (xem điểm 3 ở mục 1); không dùng `--format` có các ký tự đó.

### 2.2 Hợp đồng đầu vào từ CR khác (cần chốt khi duyệt)

- `codeintel.detectChanges` phải trả: `changedSymbols[{symbol: SymbolRef, filePath, startLine, endLine, hunks[], changeKind?}]`, `affectedFlows[]` (id `Process`, label, `processType`, `stepCount`), `affectedClusters[]`, và `mappingConfidence` theo tệp (xem 2.6). Lọc kind tài liệu (`Section`) hoặc gắn `kind:"doc"` để CR này loại ra.
- `codeintel.impact` phải có tham số `includeTests` (cờ `--include-tests` của GitNexus). README mục 3.2 hiện liệt kê `target, direction, depth` — thiếu `includeTests` (ghi vào "Điều chỉnh hợp đồng").
- `touchedTables` lấy từ `ErdModel` (CR-CV-031): `Table.accessedBy` (mã đọc/ghi bảng) và tập bảng bị migration thay đổi trong diff.
- `touchedContracts` lấy từ danh mục hợp đồng của CR-CV-032 (proto RPC, kênh `wscompat`); tên kiểu do CR-CV-032 chốt.

### 2.3 `ChangeOverlay` và `readingOrder`

Mở rộng [05 §2.7](../../../research/view-code/05-graph-schemas.md) và [08 §7](../../../research/view-code/08-views-and-review-models.md), giữ đúng tên:

```
ChangeOverlay {
  scope { baseRef, baseOid, mergeBase, headOid, mode, includesUncommitted: bool },
  changedFiles:   ChangedFile[],     // đủ, cắt ở 2 000
  changedSymbols: ChangedSymbol[],   // ≤ 2 000 (khớp detectChanges)
  affectedFlows: FlowSummary[],      // ≤ 100, cắt theo cross_community trước
  affectedClusters: ClusterRef[],
  touchedTables:  TouchedTable[],    // {table, service, via:"migration|code", migrations[], accessors: SymbolRef[]}
  touchedContracts: TouchedContract[], // {kind:"proto-rpc|ws-channel|route", name, change:"added|modified|removed|unknown", files[], breaking?: bool}
  uncoveredSymbols: SymbolRef[],
  violations: ViolationRef[],        // {findingKey, rule, severity, file, status:"touched|introduced"}
  readingOrder: ReadingStep[],       // ≤ 300 bước
  components: ComponentGroup[],
  risk: RiskAssessment,
  indexFreshness: IndexFreshness,
  limits: { truncated: {files, symbols, flows, steps}, totalCounts: {...} }
}
ChangedFile  { path, oldPath?, status:"added|modified|deleted|renamed|copied|untracked", added?, removed?, area, componentId?,
               isTest, isGenerated, isDoc, mappingConfidence:"exact|approx|none" }
ChangedSymbol { symbol: SymbolRef, changeKind:"added|modified|renamed|unknown", linesChanged, directCallers?: int,
                flows: int, tested:"yes|no|unknown" }
ReadingStep  { stepKey, n, file, symbols: SymbolRef[], hunks[{startLine,endLine}], reason: ReasonCode, reasonParams,
               dependsOn: stepKey[], tests: SymbolRef[], cycleGroup?: string, layer }
```

**`stepKey`** = hash ổn định của `(file, sorted(symbol.key))`, không phụ thuộc thứ tự bước; CR-CV-052 lưu tiến độ theo `stepKey` để thứ tự đổi không mất trạng thái "đã xem".

**Thuật toán thứ tự đọc (callee trước, caller sau):**

1. Nút = `changedSymbols` thuộc kind thực thi (function, method, type) không phải test/doc/generated.
2. Cạnh `A → B` nếu A gọi B (`CALLS`) hoặc A truy cập/dùng B (`ACCESSES`), cả hai đều là symbol đã đổi. Lấy bằng một truy vấn Cypher chỉ-đọc có tham số trên GitNexus (chia lô 200 id; cú pháp `IN $ids` và hiệu năng phải chạy thử trên LadybugDB, xem 6) hoặc `context.outgoing.calls` cho từng symbol khi lô quá lớn. Nếu thiếu cạnh (công cụ lỗi): dùng cạnh `IMPORTS` giữa các tệp đã đổi; nếu cũng thiếu: không có cạnh, thứ tự theo lớp.
3. **Xử lý vòng**: chạy Tarjan SCC; mỗi SCC cỡ > 1 thành **một nút nén** `cycleGroup` (`reason: cycle`), các thành viên sắp theo `(path, startLine)` và nằm liền nhau trong cùng bước/nhóm. Đồ thị nén là DAG.
4. **Kahn có hàng đợi ưu tiên** trên DAG theo chiều "phụ thuộc trước": nút được phát khi mọi callee đã phát. Khoá ưu tiên khi nhiều nút sẵn sàng: `(layerRank, path, startLine)`. `layerRank`: 0 hợp đồng (`.proto`, migration), 1 `domain`, 2 `usecase`, 3 `adapter`, 4 gateway/frontend/agent/khác, 5 test, 6 tài liệu/cấu hình. Xác định `layer` theo vị trí đường dẫn (`internal/{domain,usecase,adapter}`) hoặc component C4 nếu có. Kết quả **xác định** (cùng đầu vào, cùng thứ tự).
5. **Gộp bước theo tệp**: các symbol liên tiếp trong thứ tự trên cùng tệp thành một `ReadingStep` (người đọc mở tệp một lần). Tệp không có symbol (tài liệu, cấu hình, migration, `.proto`) thành bước riêng theo `layerRank`.
6. Tệp `isGenerated` (ví dụ `backend-go/proto/gen/**`, `*.pb.go`, `*_grpc.pb.go`; thư mục `backend-go/proto/gen/go` có thật) gộp thành **một** bước cuối `reason: generated`, không tính vào rủi ro dòng thay đổi.
7. `reason` ∈ `contract | dependency-of | leaf | cycle | no-edges | test | doc | generated`, kèm `reasonParams` (ví dụ `dependency-of: <stepKey của caller đầu tiên>`). Không dùng LLM; văn bản hiển thị sinh từ khoá i18n (`reasonKey`) ở frontend.
8. Hạn mức: ≤ 300 bước; vượt thì `limits.truncated.steps=true`, phần còn lại gom vào một bước `overflow` kèm số lượng.
9. Chi phí: O(V+E) với V ≤ 2 000, E ≤ vài nghìn; chạy trong bộ nhớ, không gọi mạng thêm ngoài bước 2 của thuật toán.

### 2.4 Chấm rủi ro minh bạch (`RiskAssessment`)

```
RiskAssessment { level:"LOW|MEDIUM|HIGH|CRITICAL", score:int, incomplete: bool, confidence:"high|medium|low",
                 reasons: RiskReason[], modelVersion:"1", toolRisk?: "low|medium|high|…" }
RiskReason { code, points:int, messageKey, params, evidence: (SymbolRef|path|findingKey)[] }
```

Quy tắc cộng điểm (**tổng điểm = Σ `points` của các `reasons`**; người dùng thấy từng dòng; không có điểm nào ẩn). Hằng số nằm trong mã và gắn `modelVersion`; đổi hằng số = tăng `modelVersion`. Giá trị khởi điểm (cần hiệu chỉnh bằng fixture ở CR-CV-070, xem 6):

| Mã | Điều kiện | Điểm |
|---|---|---|
| `SIZE_MEDIUM` / `SIZE_LARGE` | số dòng thêm+xoá của tệp mã thật (không test, tài liệu, generated) > 200 / > 800 | 1 / 2 (chọn một) |
| `IMPACT_MEDIUM` / `HIGH` / `CRITICAL` | rủi ro `impact` cao nhất trong các symbol đã chấm (GitNexus `risk`) | 1 / 3 / 5 (chọn mức cao nhất) |
| `FLOWS` | số luồng bị ảnh hưởng ≥ 1 / ≥ 5 | 1 / 2 (chọn một); +1 nếu có luồng `cross_community` |
| `CLUSTERS` | số cụm bị ảnh hưởng ≥ 3 / ≥ 6 | 1 / 2 |
| `MIGRATION` | có migration trong diff | 3 |
| `MIGRATION_DESTRUCTIVE` | migration có `DROP`, đổi kiểu cột, thêm `NOT NULL` không default (phân loại do CR-CV-038) | +2 |
| `CONTRACT` | có `.proto`/kênh `wscompat`/route bị chạm | 2 |
| `CONTRACT_BREAKING` | CR-CV-038 phân loại phá vỡ | +4 |
| `UNTESTED` | `uncoveredSymbols` ≥ 3 **và** ≥ 50 % symbol thực thi đã đổi | 2 |
| `UNTESTED_HIGH_IMPACT` | có symbol không test mà `impact` ≥ HIGH | +1 |
| `VIOLATION` | mỗi finding CR-CV-037 `introduced` (tối đa 2 finding tính) | 2 mỗi cái |
| `SENSITIVE_PATH` | chạm `backend-go/services/auth-service`, `credential-broker-service`, `common/jwtauth`, `common/secrets`, `backend-go/policy` | 2 |

Ngưỡng mức: `0–2 LOW`, `3–5 MEDIUM`, `6–9 HIGH`, `≥10 CRITICAL`.

Quy tắc trung thực:
- `incomplete=true` nếu bất kỳ nguồn nào lỗi/thiếu (impact `UNKNOWN`, detectChanges lỗi, index cũ): UI hiển thị "≥ mức này"; **không** hạ mức khi thiếu dữ liệu, chỉ thêm `reason` `DATA_MISSING` với `points=0` và nêu nguồn nào thiếu.
- `confidence` = `high` khi index khớp HEAD và không có tệp `approx|none`, `medium` khi lệch nhẹ, `low` khi index cũ nhiều hoặc detectChanges lỗi.
- `toolRisk` chỉ là thông tin tham chiếu (dòng `Risk level` của `detect-changes`), **không** vào điểm.
- Mỗi mức hiển thị: danh sách `reasons` sắp theo `points` giảm dần, dòng "Tổng X điểm → HIGH (6–9)". Văn bản qua khoá i18n; `evidence` bấm được tới symbol/tệp (CR-CV-053).
- Chọn `K=25` symbol để chạy `impact`: ưu tiên (1) symbol được export, (2) có nhiều `direct` caller (nếu biết từ detectChanges), (3) nằm ở tầng `usecase`/`domain`, (4) thứ tự ổn định theo `key`. Nếu còn symbol chưa chấm: `limits.truncated.impact=true` và `IMPACT_*` có chú thích "dựa trên 25/N symbol".

### 2.5 Bảng và hợp đồng bị chạm

- **`touchedTables`**: (a) tệp đổi nằm trong `backend-go/services/*/migrations/{postgres,mysql}/*.sql` → parse bằng bộ parse của CR-CV-031, lấy bảng bị `CREATE/ALTER/DROP`; `via:"migration"`; (b) symbol đã đổi thuộc `Table.accessedBy` của bảng nào (CR-CV-031) → `via:"code"`, `accessors` = các symbol đó. Hai dialect (`postgres`/`mysql`) của cùng migration gộp thành một bảng logic.
- **`touchedContracts`**: tệp `.proto` đổi → RPC/message bị đổi (chi tiết `breaking` do CR-CV-038); tệp `wscompat/channels_*.go` đổi → kênh liên quan (CR-CV-032); symbol là handler gRPC server (`adapter/grpc`) đã đổi → RPC tương ứng (`change:"unknown"` cho tới khi có diff hợp đồng). Nếu CR-CV-032/038 chưa có: chỉ trả theo cấp tệp (`kind:"file"`) và `incomplete`.

### 2.6 Độ cũ index và độ tin cậy ánh xạ

`IndexFreshness { state:"fresh|behind|dirty|missing", indexedCommit, headOid, commitsBehind?: int, dirtyFiles: int, unindexedFiles: string[], generatedAt }`.

- `indexedCommit` lấy từ `codeintel.status` (CR-CV-001/002). `fresh` ⇔ `indexedCommit == headOid` và không có tệp chưa commit; `dirty` ⇔ có tệp chưa commit; `behind` ⇔ `indexedCommit != headOid`. `commitsBehind` đếm bằng `git log --format=%H -n 200 <indexedCommit>..<headOid>` (không dùng `rev-list`, ngoài whitelist); vượt 200 thì hiển thị "200+".
- **Cơ chế sai lệch cần chặn**: GitNexus ánh xạ hunk → symbol theo phạm vi dòng trong index. Nếu tệp đã đổi sau `indexedCommit` (có trong `git diff --name-only indexedCommit` ∪ tệp chưa commit), phạm vi dòng của index có thể không còn đúng với bản worktree → gắn `mappingConfidence:"approx"` (tệp có trong index) hoặc `"none"` (tệp mới chưa có trong index, đưa vào `unindexedFiles`). Tệp `none` chỉ xuất hiện ở `changedFiles` (không có symbol), `components`, và bước đọc theo tệp.
- Overlay **luôn** trả `indexFreshness` và `confidence`; frontend hiển thị chip và nút "Làm mới index" (O3, `codeintel.reindex`). Không bao giờ ẩn trạng thái cũ.
- Chưa kiểm chứng: `detect-changes` dùng phía nào của diff (số dòng cũ hay mới) để khớp phạm vi; xem 6.

### 2.7 Gom nhóm theo component C4 và giới hạn kích thước

- `ComponentGroup { componentId, containerId, label, files: int, symbols: int, added: int, removed: int, riskPoints: int, stepKeys[] }`. Ánh xạ tệp → component bằng khớp **tiền tố đường dẫn dài nhất** trên `C4ComponentView.components[].path` (CR-CV-033). Khi CR-CV-033 chưa có: nhóm dự phòng theo đường dẫn: `backend-go/services/<svc>/internal/<domain|usecase|adapter/<tên>>`, `agent/src/<relay|main|shared>`, `frontend/src/renderer/src/<thư mục cấp 1>`; `componentId:"path:<prefix>"` và nhãn "suy luận".
- `riskPoints` của nhóm = tổng điểm của các `reasons` có `evidence` nằm trong nhóm (để UI xếp nhóm theo rủi ro); không cộng vào tổng toàn cục lần hai.
- **Giới hạn** (có `truncated` + `totalCount`, theo README mục 6 "Không bao giờ trả cả graph"): `changedFiles ≤ 2 000`, `changedSymbols ≤ 2 000`, `affectedFlows ≤ 100`, `readingOrder ≤ 300 bước`, `impact` chạy cho ≤ 25 symbol, payload JSON ≤ 2 MiB (`graph_snapshots.payload` có giới hạn). Vượt: giữ tệp theo thứ tự `|added|+|removed|` giảm dần và đặt `truncated`; rủi ro tính trên toàn bộ tệp đã biết (đếm trước khi cắt) nên không bị thấp đi do cắt.
- `detail:"summary"` chỉ trả `scope`, đếm, `risk`, `components`, `indexFreshness` (cho thanh tóm tắt, CR-CV-051); `full` trả đủ.
- Cache: khoá `(tenant, repo, mergeBase, headOid, mode, params_hash)`; worktree bẩn **không** cache dài (TTL ngắn, ví dụ 30 s, và huỷ khi nhận `codeintel.indexChanged`/`fs.changed`) vì không có cách rẻ để băm nội dung chưa commit. Singleflight theo cùng khoá (CR-CV-022).

### 2.8 Lỗi và suy giảm

| Tình huống | Kết quả |
|---|---|
| `branchCompare.status = unborn-head` | overlay rỗng, `emptyReason:"unborn-head"`; không phải lỗi |
| `invalid-base`, `no-merge-base` | `CODEINTEL_INVALID_PARAMS` kèm thông điệp từ agent; UI cho chọn lại nhánh gốc |
| Thiếu công cụ/chỉ mục (`CODEINTEL_TOOL_UNAVAILABLE`, `CODEINTEL_INDEX_MISSING`) | vẫn trả `changedFiles` + nhóm theo đường dẫn từ `git.*`; `risk.incomplete=true`, `confidence:"low"`, `sources[]` ghi nguồn lỗi |
| `impact` quá hạn (`CODEINTEL_TIMEOUT`) | các `IMPACT_*` bỏ qua cho symbol đó; `limits.truncated.impact`, `DATA_MISSING` |
| Dev server offline | mã lỗi lớp collector (CR-CV-021); không trả overlay cũ trừ khi có snapshot cùng `headOid` (kèm cờ `stale`) |
| `CODEINTEL_AMBIGUOUS_SYMBOL` | bỏ symbol đó khỏi `impact`, dùng `uid`/`file` (đã có từ `detectChanges`) để tránh mơ hồ |

## 3. Quyết định thiết kế

1. **Hợp nhất ở backend, không ở agent**: agent chỉ chạy lệnh hẹp (D5); topo, rủi ro, nhóm là logic thuần, dễ kiểm thử, nằm trong `code-intel-service` (D3).
2. **`mode=worktree` mặc định**: người review thường mở khi agent chưa commit; chỉ dùng `git.branchCompare` sẽ bỏ sót phần lớn thay đổi (xem mục 1.1).
3. **Callee trước caller** (khớp R6 trong 08): đọc phần nền tảng trước khiến phần dùng nó dễ hiểu; có vòng thì nén thay vì cố sắp.
4. **Quy tắc cộng điểm thay vì một con số do công cụ trả**: GitNexus chỉ có `risk` theo từng symbol; người review cần biết "vì sao HIGH". Bảng cộng điểm cho phép kiểm tra và tranh luận; `modelVersion` cho phép đổi có kiểm soát.
5. **Không hạ rủi ro khi thiếu dữ liệu**: `incomplete` + "≥ mức". Một điểm số đẹp mà sai tệ hơn điểm số ghi rõ là chưa đủ.
6. **Cắt sau khi đếm**: rủi ro dựa trên toàn bộ dữ liệu đã biết; cắt chỉ ảnh hưởng hiển thị.
7. **`stepKey` ổn định** để tiến độ review (O6, lưu ở backend) sống qua lần tính lại và qua lượt agent sửa tiếp (CR-CV-060).
8. **Không tự chạy `reindex`** (O3): chỉ báo cũ và gợi ý.
9. **Provider/SSH**: không đưa tên nhà cung cấp git vào model; mọi dữ liệu đến qua agent nên chạy được trên dev server từ xa (độ trễ 50–200 ms nên thao tác song song có hạn mức, không lặp theo từng tệp).

## 4. Tiêu chí chấp nhận

- [ ] `GetChangeOverlay` trên worktree có (a) 2 commit trên nhánh gốc và (b) 1 tệp sửa chưa commit trả `changedFiles` gồm cả hai nhóm; tệp có ở cả hai nhóm chỉ xuất hiện một lần với số `added/removed` so với `mergeBase`.
- [ ] `mode=committed` cho kết quả đúng bằng `git.branchCompare` (không gồm tệp chưa commit).
- [ ] Không gọi lệnh git ngoài whitelist Part A (`merge-base`, `rev-list`, `ls-tree` không xuất hiện trong lời gọi); không có đối số chứa ký tự bị chặn.
- [ ] `readingOrder`: với đồ thị fixture A→B→C (A gọi B gọi C) thứ tự là C, B, A; với vòng A↔B cộng C gọi A, A và B cùng một `cycleGroup` liền nhau và đứng trước C; thứ tự **không đổi** qua 100 lần chạy; mỗi bước có `reason` hợp lệ.
- [ ] `stepKey` không đổi khi chỉ đổi thứ tự bước (thêm một symbol không liên quan không làm đổi `stepKey` của các bước khác).
- [ ] Mỗi `RiskAssessment` có `score == Σ reasons[].points`; mỗi `reason` có `messageKey`, `evidence` không rỗng (trừ `DATA_MISSING`); mức trùng với ngưỡng ở 2.4. Test bảng quy tắc từng dòng.
- [ ] Khi `impact` lỗi cho mọi symbol: `incomplete=true`, mức không thấp hơn mức tính từ nguồn còn lại, có `DATA_MISSING`.
- [ ] `uncoveredSymbols` = symbol thực thi đã đổi không có `CALLS` từ tệp test trong độ sâu ≤ 2; khi `impact` bị cắt → `tested:"unknown"` (không đếm là chưa test).
- [ ] Tệp doc/test/generated không vào `uncoveredSymbols`, `SIZE_*`; `Section` của Markdown không xuất hiện ở `changedSymbols` thực thi.
- [ ] Index cũ: `indexFreshness.state` đúng; tệp đổi sau `indexedCommit` có `mappingConfidence` `approx|none`; tệp mới chưa index nằm trong `unindexedFiles`.
- [ ] Giới hạn: fixture 3 000 tệp đổi → `truncated.files=true`, `totalCount` đúng, payload ≤ 2 MiB, rủi ro vẫn tính trên 3 000.
- [ ] Nhóm component: khi chưa có CR-CV-033, nhóm dự phòng theo đường dẫn, `label` đánh dấu "suy luận".
- [ ] Cô lập tenant: gọi chéo tenant → từ chối (CR-CV-013); snapshot có `tenant_id`.
- [ ] `buf lint` / `buf breaking` pass; không khai báo RPC chưa có message.

## 5. Kiểm thử

- **Unit thuần** (không I/O): topo + SCC (cây, kim cương, vòng, tự gọi, đồ thị rỗng, 2 000 nút); gom bước theo tệp; hàm chấm điểm (bảng quy tắc từng dòng, biên 2/3, 5/6, 9/10); hợp nhất `branchCompare` + `status`; lọc kind; cắt giới hạn.
- **Fixture git thật**: tạo repo tạm bằng git (chạy với 2.25 và bản mới theo CI của `git-compatibility.md`) có nhánh gốc, 2 commit, sửa chưa commit, đổi tên tệp, tệp mới chưa theo dõi; so kết quả với lệnh git tham chiếu.
- **Hợp đồng công cụ** (CR-CV-070): mẫu thật đầu ra `detect-changes` (văn bản) và `impact` (JSON) ghi theo phiên bản (GitNexus 1.6.9); thêm ca `error: Target … not found`, `status: ambiguous`.
- **Suy giảm**: mô phỏng từng nguồn lỗi (agent trả `CODEINTEL_*`, timeout); xác nhận `incomplete` và nội dung phần còn lại.
- **Hai dialect DB** (O1) cho `graph_snapshots`; kiểm tra khoá cache và TTL cho worktree bẩn.
- **Hiệu năng**: ngân sách tổng cho `summary` (số lệnh CLI ≤ 1 + K, K=25, song song theo hạn mức CR-CV-013); đo trên Orca (247 556 nút) ở CR-CV-071.
- **Ổn định**: chạy lặp 100 lần so sánh byte-by-byte đầu ra (xác định).

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy hệ thống.** Chưa thử `git.branchCompare` + `git.status` trên worktree thật qua WS; `getStatusOp` chưa đọc.
- **`detect-changes` dùng dòng cũ hay mới của diff?** Chưa kiểm chứng. Nếu index ở `indexedCommit` và diff là sau đó, ánh xạ có thể lệch (2.6). Cần thử: sửa một hàm, không `analyze`, chạy `detect-changes -s all`, xem hàm nào được báo. Ngoài ra chưa rõ `compare` là `base..HEAD` hay `base` so với working tree. Nếu CR-CV-005 không hỗ trợ "mergeBase → working tree", `mode=worktree` phải hợp nhất kết quả hai lần gọi (`compare` + `unstaged`/`staged`).
- Symbol **bị xoá** hoàn toàn: index ở HEAD cũ không còn/không có; `detect-changes` có thể không báo. Hiện chỉ đưa ở mức tệp (`status:"deleted"`).
- Chưa kiểm chứng cú pháp Cypher `WHERE a.id IN $ids` (tham số danh sách) và chi phí trên LadybugDB; tham số hoá qua CLI chưa thử (`gitnexus cypher` nhận chuỗi truy vấn; cách truyền tham số **chưa xác minh**, có thể phải nội suy id đã kiểm tra chặt `^[A-Za-z]+:[^\s'"]+$`).
- Ngưỡng và điểm ở 2.4 là **giá trị khởi điểm chưa hiệu chỉnh**; chưa có dữ liệu thật về phân bố. Cần hiệu chỉnh với 10–20 PR lịch sử trong fixture (CR-CV-070) trước khi coi `HIGH`/`CRITICAL` đáng tin.
- Phát hiện "test phủ" dựa trên cạnh `CALLS` từ hàm trong tệp test tới symbol (đã xác nhận ở Go `*_test.go`); chưa kiểm chứng với TypeScript (`*.test.ts`, mock, `vi.mock`) và gọi gián tiếp qua interface. Dự kiến báo thiếu cả những hàm có test gián tiếp (báo nhầm). Không có số coverage thật (E11 chưa xác nhận).
- Giới hạn 60 s và stdout không giới hạn của `git.exec` (Part A) với `diff --numstat` trên diff khổng lồ.
- Hai nguồn (GitNexus/CodeGraph) khác nhau về phạm vi trích xuất; id có thể không khớp (CR-CV-020).
- Phụ thuộc "mềm" (C4, ERD, hợp đồng, finding) chưa tồn tại lúc viết CR này; hợp đồng trường chưa chốt.
- Relay SSH (Part B) không có `git.exec` kiểu này và không có `codeintel.*` ở MVP (CR-CV-006 là P2); overlay chỉ chạy `direct-websocket`.
- Số lượng `K=25` và thời gian ~1,8 s/lệnh là ước tính từ README; chưa đo độ trễ song song.

## 7. Câu hỏi mở

1. Chấp nhận `mode=worktree` làm mặc định (gồm chưa commit)? Hay yêu cầu agent luôn commit trước khi review (đơn giản hơn nhưng đổi quy trình)?
2. Có cần cấu hình ngưỡng/điểm theo tenant, hay giữ cố định theo `modelVersion` (đề xuất giữ cố định ở MVP)?
3. `SENSITIVE_PATH`: danh sách đường dẫn nhạy cảm do ai duyệt; có đưa vào `c4.yaml` (E6) không?
4. Có cần hiển thị điểm rủi ro theo từng tệp/component trong lens, hay chỉ tổng thể và danh sách `reasons`?
5. Chấp nhận xếp thứ tự đọc chỉ dựa trên `CALLS`/`ACCESSES`, hay cần thêm cạnh `IMPLEMENTS`/`METHOD_IMPLEMENTS` (interface trước, cài đặt sau)?
6. Hướng xử lý symbol bị xoá: bổ sung bằng `git log -p`/`git diff` phân tích dòng bị xoá, hay chấp nhận giới hạn ở cấp tệp?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md`; `/opt/repos/orca/docs/research/view-code/05-graph-schemas.md` (§2.5, 2.7), `08-views-and-review-models.md` (§7, R1, R5, R6), `09-external-inputs-required.md` (E5, E11, E16), `04-raw-data-and-pipeline.md`
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch-git.ts` (dòng 25–90, 190), `/opt/repos/orca/agent/src/relay/agent-git-handler-extended.ts` (dòng 90–135), `/opt/repos/orca/agent/src/relay/git-handler-ops.ts` (dòng 124), `/opt/repos/orca/agent/src/relay/git-handler-utils.ts` (`parseBranchDiff`), `/opt/repos/orca/agent/src/relay/agent-git-handler.ts` (`ALLOWED_GIT_SUBCOMMANDS`, `handleGitExec`), `/opt/repos/orca/agent/src/relay/agent-git-exec-validator.ts`, `/opt/repos/orca/agent/src/relay/agent-git-base-ref-handler.ts`, `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch-git-status.ts`, `/opt/repos/orca/agent/src/shared/git-uncommitted-line-stats.ts`
- Chạy CLI chỉ-đọc (GitNexus 1.6.9): `gitnexus detect-changes --help`, `gitnexus detect-changes -r orca -s compare -b HEAD~3 -l 5`, `gitnexus impact -r orca … --include-tests`, `gitnexus context -r orca ParseCodeowners`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`
- CR liên quan: CR-CV-002, 005, 012, 013, 020, 021, 022, 030, 031, 032, 033, 037, 038, 040, 051, 052, 053, 060, 070, 071
