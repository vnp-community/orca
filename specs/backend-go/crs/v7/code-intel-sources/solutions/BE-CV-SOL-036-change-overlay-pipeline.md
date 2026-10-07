# BE-CV-SOL-036-change-overlay-pipeline: `GetChangeOverlay`: hợp nhất `detectChanges` + `impact` + làm giàu mềm thành `ChangeOverlay`

> **📋 Proposed.** Chưa triển khai, chưa chạy build/test/CLI. P0 (đợt 3, lõi MVP review). Nửa còn lại của CR-CV-036 (thuật toán thứ tự đọc, chấm rủi ro, gom component, cắt giới hạn) nằm ở [`BE-CV-SOL-036-reading-order-and-risk`](./BE-CV-SOL-036-reading-order-and-risk.md); hai solution **dùng chung** một proto và một kết quả snapshot. Khẳng định về code hiện có do người soạn đọc ngày 2026-10-06; chưa kiểm chứng ghi rõ; phần mới ghi "(mới)".

**CR:** [CR-CV-036](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md)
**Service:** `code-intel-service` (mới, `BE-CV-SOL-010`) · `proto/orca/codeintel/v1`
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md): **PQ-04** (`selector`), **PQ-07** (`codeintel_change_overlay.proto`), **PQ-09** (`ChangeOverlay` do CR-036 sở hữu; `RiskAssessment.level` 4 mức + `incomplete`), **PQ-12** (phong bì phẳng, ETag), **PQ-13** (timeout, `CODEINTEL_TIMEOUT` + hoàn tất nền), **PQ-14** (≤ 2 MiB, TTL 7 ngày), **PQ-15** (`graph_snapshots`), **PQ-19** (hình dạng `detectChanges`, `impact`), **PQ-20** (`SymbolRef` đã chuẩn hoá ở agent), **PQ-21** (bộ method agent), **PQ-30** (`TouchedContract.compatibility`), **PQ-31** (`ReadingStep`/`stepKey`), **PQ-32** (`risk.level` chữ HOA); §2.1 dòng 14, §3.1 (`GetChangeOverlay`, `GetReadingOrder`), §4.2 T3. [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md) §2.1–2.5, §4.5 (`impact`), §4.8 (`detectChanges`). [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md) §3.1 (kênh `changeOverlay`, `readingOrder`), §4.3 (kiểu `ChangeOverlay`).
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md) (ranh giới: service chỉ hợp nhất, agent chạy công cụ), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (`domain/` thuần), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) §Multi-tenancy, [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) §Multi-tenancy isolation, [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) §Talking to the Dev Server Agent, [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (span/metric/timeout), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md) §7 (relay dispatch).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `agent/src/relay/git-handler-ops.ts` (`branchCompare`, dòng 124–150: tính `summary{baseRef, baseOid, compareRef, headOid, mergeBase, changedFiles, status}`; `headOid` là `HEAD`, nên **không** gồm thay đổi chưa commit), `agent/src/relay/agent-git-handler.ts` (`ALLOWED_GIT_SUBCOMMANDS` dòng 41–62 gồm `status diff add restore commit push pull fetch branch checkout merge rebase stash log worktree remote tag show rev-parse config describe shortlog`; **không** có `merge-base`, `rev-list`, `ls-tree`, `diff-tree`; `SHELL_METACHARACTERS = /[&|;$\`<>\\!]/` dòng 70), thư mục `agent/src/relay/` (không có tệp `codeintel-*` nào: toàn bộ phía agent là đề xuất của `AG-CV-SOL-005-detect-changes`), `backend-go/proto/buf.yaml` (`lint: STANDARD`, `breaking: FILE`), `backend-go/services/code-intel-service` (**không tồn tại**), `guides/reference/git-compatibility.md` (baseline Git 2.25, `GitCapabilityCache`).

Xác nhận đúng: whitelist git của agent Part A không có `merge-base`/`rev-list`; `branchCompare` chỉ so commit; chưa có service và chưa có method `codeintel.*` ở agent.

### Correction relative to CR-CV-036

| # | CR nói | Hợp đồng / mã thật | Xử lý trong solution này |
|---|--------|--------------------|--------------------------|
| C1 | §2.1 bước 1–3: backend gọi `git.baseRefDefault`, `git.branchCompare`, `git.status` rồi `git.exec diff --numstat` để hợp nhất commit + chưa commit | Hợp đồng agent §4.8: **agent** tự chạy `git diff --raw/--numstat/--unified=0` và `git ls-files --others` rồi trả `changedFiles[{path, oldPath, status raw, additions, deletions, hunks, untracked, binary, indexed, driftedFromIndex}]` đã gồm cây làm việc; `base` mặc định do agent suy ra; `head` vắng = cây làm việc | Backend chỉ gọi **một** `codeintel.detectChanges`; bỏ hẳn bước hợp nhất git ở backend (ít lệnh qua SSH hơn, ít bề mặt hơn) |
| C2 | §2.2: `detectChanges` phải trả `processType` cho `affectedFlows` | Hợp đồng §4.8: `affectedFlows[{flowId, label, stepCount, changedSymbolKeys, earliestChangedStep}]`, **không** có `processType` | Không tính "+1 nếu có luồng `cross_community`" ở `modelVersion "1"` (xem `BE-CV-SOL-036-reading-order-and-risk` mục 4) |
| C3 | §2.3 thuật toán lấy cạnh bằng Cypher `IN $ids` hoặc `context.outgoing.calls` | Agent **không** nhận Cypher (H3, PQ-21); `codeintel.subgraph` chỉ nhận một tâm; không có method lấy cạnh theo lô | Dùng `levels[0]` (độ sâu 1, `direct:true`) của `codeintel.impact` hướng `upstream` đã chạy cho tối đa K symbol: caller `C` của `S` cho cạnh `S → C` (chi tiết ở solution kia mục 2.A) |
| C4 | `ChangedFile.status` gồm `untracked`, `renamed`, `copied` | Agent trả mã raw `M|A|D|R|C|T|U` + cờ `untracked` | Ánh xạ cố định (2.C) |
| C5 | `ChangedSymbol.changeKind: added|modified|renamed|unknown` | Agent `change ∈ modified|added|deleted` | `deleted` ⇒ `unknown` và loại khỏi thứ tự đọc/`tested`; `renamed` không dùng ở v7 (agent không báo) |
| C6 | §2.6 `commitsBehind` bằng `git log --format=%H -n 200 <indexed>..<head>` do backend chạy | Hợp đồng agent không có method; `git.exec` Part A cho phép `log` và `..` không phạm `SHELL_METACHARACTERS` | Qua port `CommitDistanceReader` (mềm; hiện thực trên `RepoSourceReader` của `BE-CV-SOL-030` nếu cổng đó cho `git log`); thiếu ⇒ bỏ trường `commitsBehind` (không phải lỗi) |
| C7 | K = 25 symbol chạy `impact`, mỗi lệnh ≈ 1,8 s | Hợp đồng §2.3: `gitnexus` ≤ 2 tiến trình/dev server; đọc ở gateway 20 s | Chạy `impact` theo **ngân sách thời gian** còn lại (2.D); phần chưa xong ⇒ `limits.truncated.impact=true` + `tested:"unknown"`, hoàn tất nền cho lần gọi sau |
| C8 | Request `worktree_id`/`repo_binding_id` | PQ-04: `selector` | Theo hợp đồng |
| C9 | `ViolationRef.status`, `TouchedContract.breaking` | PQ-06/30: `status ∈ introduced|touched`; `TouchedContract` thêm `compatibility`, giữ `breaking = (compatibility=="breaking")` | Theo hợp đồng |

## 2. Giải pháp

### 2.A Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_change_overlay.proto           # message + GetChangeOverlay*, GetReadingOrder* (dùng chung với solution reading-order)
backend-go/services/code-intel-service/
  internal/domain/changeoverlay/{change_overlay.go, changed_file.go, changed_symbol.go, index_freshness.go, overlay_enrichment.go}
  internal/domain/changeoverlay/{path_classification.go, change_set_mapping.go, impact_selection.go}
  internal/usecase/get_change_overlay.go           # điều phối (2.D); gọi domain của solution reading-order qua port
  internal/usecase/change_overlay_ports.go         # ChangeSetSource, ImpactSource, TouchedTableSource, TouchedContractSource,
                                                   # ViolationSource, ComponentIndex, CommitDistanceReader (mới)
  internal/adapter/grpc/change_overlay_handler.go
  testdata/change-overlay/{detect-changes-*.json, impact-*.json}               # từ tệp vàng agent (G1, CR-CV-070)
```

Domain thuần (stdlib). Ports khai báo ở `usecase`; adapter agent do `BE-CV-SOL-021-agent-collector` hiện thực (ở đây chỉ **dùng**).

### 2.B Proto `codeintel_change_overlay.proto` (mới; PQ-07, PQ-09, PQ-30, PQ-31)

Message theo hợp đồng §2.1 dòng 14: `ChangeOverlay, ChangedFile, ChangedSymbol, ReadingStep, ComponentGroup, RiskAssessment, RiskReason, IndexFreshness, TouchedTable, TouchedContract, ViolationRef, OverlayLimits`; request/response `GetChangeOverlay*` (`selector=1`, `base_ref`, `head_ref`, `mode`, `detail`) và `GetReadingOrder*` (`selector=1`, `base_ref`, `head_ref`). Hình dạng trường theo UI §4.3 (đã chốt tên); số field **do chủ sở hữu CR gán** theo thứ tự khai báo ở CR-036 §2.3/2.4 và `codeintel_common.proto` (`SymbolRef`, `FlowSummary`, `ClusterRef`). `RiskAssessment.level` là chuỗi `LOW|MEDIUM|HIGH|CRITICAL` (PQ-32), `incomplete` là bool, `model_version` chuỗi `"1"`. Enum chuỗi nhỏ (`status`, `changeKind`, `reason`, `state`) là `string` chữ thường (PQ-32), không enum proto, để thêm giá trị không phá `buf breaking`.

### 2.C Ánh xạ kết quả agent → `ChangeOverlay` (domain thuần)

| Nguồn (agent §4.8) | Đích | Quy tắc |
|---|---|---|
| `base{ref,oid,mergeBase}`, `head{oid,includesUncommitted,dirtyFileCount}` | `scope` | `mode = "worktree"` khi `head.includesUncommitted`, ngược lại `"committed"` |
| `changedFiles[].status` | `ChangedFile.status` | `M,T`⇒`modified`; `A`⇒`added` (`untracked=true`⇒`untracked`); `D`⇒`deleted`; `R`⇒`renamed`; `C`⇒`copied`; `U`⇒`modified` |
| `additions/deletions` | `added/removed` | số nguyên ≥ 0; `binary` ⇒ cả hai 0 |
| `indexed`, `driftedFromIndex` | `mappingConfidence` | `indexed && !drifted`⇒`exact`; `indexed && drifted`⇒`approx`; `!indexed`⇒`none` |
| `changedSymbols[]` | `ChangedSymbol` | `symbol` giữ nguyên (đã `SymbolRef` chuẩn, PQ-20); `change`: `added|modified` giữ, `deleted`⇒`unknown` (C5); `linesChanged = linesTouched`; `flows` = số `affectedFlows` có `changedSymbolKeys` chứa `symbol.key`; kind `doc` loại khỏi tập thực thi |
| `affectedFlows[]` | `FlowSummary` | ≤ 100; sắp theo `stepCount` giảm dần rồi `flowId` |
| `affectedClusters[]` | `ClusterRef` | `{id,label}` |
| `index{commit,stale,driftedFileCount,mappingConfidence}`, `unmapped.filesNotIndexed` | `IndexFreshness` | `state`: `missing` nếu `index.commit` rỗng; `behind` nếu `index.commit != head.oid`; `dirty` nếu `head.dirtyFileCount > 0`; còn lại `fresh`; `dirtyFiles = head.dirtyFileCount`; `unindexedFiles = unmapped.filesNotIndexed` (≤ 100); `commitsBehind` chỉ khi `CommitDistanceReader` có |
| `riskHint.level` | `risk.toolRisk` | chỉ tham chiếu, **không** vào điểm |

Phân loại đường dẫn (`path_classification.go`, hàm thuần, bảng cố định, có test): `isGenerated` ⇐ `backend-go/proto/gen/**`, `*.pb.go`, `*_grpc.pb.go`; `isTest` ⇐ `*_test.go`, `*.test.*`, `*.spec.*`, thư mục `__tests__`, `tests/`; `isDoc` ⇐ `*.md`, `docs/**`, `specs/**`, `guides/**`; `area` ⇐ `backend-go/<service>` cho `backend-go/services/<service>/**`, còn lại thư mục cấp 1. Kind `doc` và tệp `isDoc|isTest|isGenerated` **không** vào `SIZE_*`, `uncoveredSymbols` (xem solution reading-order).

### 2.D Điều phối `usecase.GetChangeOverlay`

1. **Cổng**: guard nội bộ, tenant, cờ `code_intel_enabled` (PQ-24), quyền OPA `read`, phân giải `selector` ⇒ `repo_binding` (`BE-CV-SOL-012/013`). Quyền **trước** cache.
2. **Khoá**: `mode = "committed"` khi `head_ref != ""` hoặc `mode=="committed"`; khi đó `head` gửi agent là `head_ref`/`HEAD` (agent: đặt `head` thì **không** tính thay đổi chưa commit). Quyết định: `head_ref` khác rỗng ngầm hiểu `committed` (mục 8 Q3).
3. **Cache**: khoá `(tenant, binding, "changeOverlay", head_oid, params_hash)` (PQ-15), `params_hash = sha256(base_ref|head_ref|mode|detail|modelVersion|schemaVersion)`; snapshot DB chỉ khi `includesUncommitted=false` **hoặc** `dirtyFileCount == 0`; ngược lại bộ nhớ TTL 30 s, huỷ khi nhận `orca.codeintel.index.changed`/`reindex.finished` (hợp đồng §5). Tín hiệu thay đổi tệp (`fs.changed`) **không có** trong hợp đồng ⇒ chỉ TTL (mục 7). Singleflight cùng khoá, nền 100 s (PQ-13).
4. **Gọi agent**: `codeintel.detectChanges {base?, head?, includeUntracked:true, withClusters:true}` (một lần). Lỗi agent: `INDEX_MISSING`/`TOOL_UNAVAILABLE`/`REPO_NOT_REGISTERED` ⇒ suy giảm: **không** có `changedSymbols`; nhưng vẫn cần `changedFiles` ⇒ gọi lại ở chế độ tối thiểu qua `RepoSourceReader` chỉ khi `BE-CV-SOL-030` cung cấp `git diff --name-status`; **mặc định v7**: trả `CODEINTEL_INDEX_MISSING`/`CODEINTEL_TOOL_UNAVAILABLE` của agent (UI hiện "Làm mới index"), vì `detectChanges` của agent cũng cần chỉ mục để ánh xạ. (CR-036 §2.8 muốn "vẫn trả `changedFiles` + nhóm theo đường dẫn": giữ làm câu hỏi mở Q1, vì cần agent trả `changedFiles` khi thiếu chỉ mục; hợp đồng §4.8 chưa nói.)
5. **Chọn K symbol** (`impact_selection.go`): tối đa 25 symbol thực thi (không `doc`/test/generated/`deleted`), ưu tiên (1) `isExported`, (2) tầng `usecase|domain` theo đường dẫn, (3) kích thước thay đổi giảm dần, (4) khoá `symbol.key` tăng dần (ổn định). Còn thừa ⇒ `limits.truncated.impact=true`.
6. **`impact`**: `codeintel.impact {target:{key}, direction:"upstream", depth:2, includeTests:true, limit:300}` song song **tối đa 2** (khớp `gitnexus ≤ 2` mỗi dev server, hợp đồng §2.3); mỗi cuộc gọi dùng phần ngân sách còn lại (deadline toàn bộ 20 s − 2 s); `CODEINTEL_AMBIGUOUS_SYMBOL` ⇒ bỏ symbol (đã có `key` nên hiếm); `CODEINTEL_TIMEOUT`/`TOOL_FAILED` ⇒ symbol đó `tested:"unknown"`, ghi nguồn lỗi để `DATA_MISSING`. Những lời gọi chưa xong khi hết ngân sách **tiếp tục nền** và kết quả ghi vào cùng khoá cache; lần gọi sau trả đủ (C7).
7. **Làm giàu mềm** (ports, mỗi cái có `Unavailable` không lỗi): `TouchedTableSource` (ERD `changes[]` theo migration đã đổi + `Table.accessedBy` giao `changedSymbols`, từ `BE-CV-SOL-031-erd-model-and-access-scan`), `TouchedContractSource` (`GetContractDiff` của `BE-CV-SOL-038-contract-diff`; thiếu ⇒ `kind:"file"` theo đường dẫn `.proto`/`wscompat/channels_*.go`/`migrations/**`), `ViolationSource` (`ListFindings(scope=CHANGED)` của `BE-CV-SOL-037`), `ComponentIndex` (C4 snapshot theo container của `BE-CV-SOL-033`; thiếu ⇒ nhóm theo đường dẫn, nhãn "suy luận"). Nguồn nào lỗi/thiếu ⇒ ghi vào danh sách `missingSources` đưa cho bộ chấm rủi ro (`DATA_MISSING`, `incomplete=true`). Không bao giờ tính lại view đó ở đây (CR §2.1 bước 6).
8. **Dựng kết quả**: gọi domain của solution reading-order: `BuildReadingOrder`, `ScoreRisk`, `GroupComponents`, `ApplyLimits` (đếm **trước** khi cắt); điền `indexFreshness`; `detail=="summary"` ⇒ chỉ `scope`, `limits.totalCounts`, `risk`, `components`, `indexFreshness` (mảng chi tiết rỗng, UI §4.3).
9. **Phản hồi**: `proto.Size ≤ 2 MiB` (PQ-14); vượt ⇒ vòng co theo thứ tự cố định (bỏ `uncoveredSymbols` quá 500 → bỏ `changedSymbols` quá 1 000 → bỏ `changedFiles` quá 1 000), mỗi bước đặt `truncated`; vẫn vượt ⇒ `CODEINTEL_RESPONSE_TOO_LARGE`. `ResultMeta` phẳng: `etag`, `head_commit`, `stale` (= `indexFreshness.state != fresh`), `truncated`, `total_count`, `from_cache`, `not_modified`.

Mã lỗi (PQ-03): `CODEINTEL_DISABLED`, `CODEINTEL_NOT_AUTHORIZED`, `CODEINTEL_INVALID_PARAMS` (ref không phân giải, `base_required`, `no_merge_base`, giữ `reason` ở hậu tố JSON), `CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_TIMEOUT` (`retryAfterMs`, `inProgress`), `CODEINTEL_INDEX_MISSING`, `CODEINTEL_TOOL_UNAVAILABLE`, `CODEINTEL_AGENT_UNSUPPORTED`, `CODEINTEL_RESPONSE_TOO_LARGE`. Trường hợp `warnings:["unborn_head"]` của agent ⇒ overlay rỗng với `emptyReason:"unborn-head"`, không lỗi.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Một `detectChanges` thay chuỗi git ở backend | Agent là nơi chạy lệnh hẹp (D5); giảm 3–4 vòng qua SSH 50–200 ms; tránh hai nơi cùng suy diễn merge-base |
| `impact` theo ngân sách thời gian + hoàn tất nền | Khớp PQ-13 (20 s gateway) mà không hạ mức rủi ro: phần thiếu thành `incomplete` chứ không thành `LOW` |
| Làm giàu qua port có `Unavailable` | Thứ tự hợp đồng §7: 036 chạy trước 037/038/033; không chặn MVP |
| Snapshot DB chỉ khi cây sạch | Không có cách rẻ băm nội dung chưa commit (CR §2.7) |
| `status` agent raw ⇒ giá trị overlay ánh xạ ở domain | Tách hợp đồng dây agent khỏi hợp đồng UI; test bằng tệp vàng |
| Không tự `reindex` | O3: chỉ báo cũ và gợi ý |
| Không tên nhà cung cấp git, mọi dữ liệu qua agent | AGENTS.md: SSH/GitLab; ở đây không có nhánh theo provider; chỉ chạy `direct-websocket` (Part A), `relay-ssh` Part B ngoài MVP (O-5) |
| Git: không lệnh git mới ở backend | Baseline Git 2.25 do agent giữ (`-M`, `--numstat -z`, `ls-files --others` đều ≤ 2.24 theo hợp đồng §4.8); không cần `GitCapabilityCache` ở solution này |

## 4. Lệch giữa CR và hợp đồng

| # | CR-CV-036 | Hợp đồng | Theo |
|---|-----------|----------|------|
| L1 | request `worktree_id`/`repo_binding_id`, `mode`, `detail` | PQ-04: `selector`; thêm `head_ref` (§3.1) | Hợp đồng |
| L2 | `RiskAssessment.level` chữ HOA, `Risk` enum 5 giá trị | PQ-09/32: `RiskAssessment.level` 4 mức + `incomplete`; chữ HOA | Hợp đồng |
| L3 | `TouchedContract{…, breaking?}` | PQ-30: thêm `compatibility` | Hợp đồng |
| L4 | `ViolationRef.status:"touched|introduced"` | PQ-06: giữ đúng hai giá trị này | Giống nhau |
| L5 | Hai RPC trả `{meta, …}` | §2.3/PQ-12: `ResultMeta` phẳng | Hợp đồng |
| L6 | `affectedClusters: ClusterRef[]` | UI §4.3: `{id,label}[]`; agent `[{id,label,changedSymbols}]` | Bỏ `changedSymbols` ở overlay |
| L7 | Thiếu index vẫn trả `changedFiles` | Hợp đồng agent: `detectChanges` cần chỉ mục | Theo hợp đồng cho tới khi Q1 được chốt; báo chủ `AG-CV-SOL-005` |
| L8 | §2.1 bước 1: `git.baseRefDefault` | §4.8: `base` mặc định do agent (không gọi mạng) | Hợp đồng |

## 5. Phụ thuộc chéo khu vực và thứ tự

| Cần | Từ | Dạng |
|-----|----|------|
| Method `codeintel.detectChanges`, `codeintel.impact` (tham số, tệp vàng `testdata/agent-results/*.json`) | `AG-CV-SOL-005-detect-changes`, `AG-CV-SOL-002-gitnexus-extraction`, `AG-CV-SOL-070-golden-fixtures-and-parsers` | **cứng** (agent); ghi nhận: backend phát triển trên tệp vàng G1, không cần agent thật |
| Collector gọi agent, dịch lỗi (`AgentRPCError`), timeout 90 s | `BE-CV-SOL-021-agent-collector`, `BE-CV-SOL-023-infra-fleet-codeintel-transport` | cứng |
| `SymbolRef`, `FlowSummary`, `ClusterRef`, `ResultMeta`, `WorktreeSelector` | `BE-CV-SOL-020-canonical-graph-model` | cứng (G0) |
| Phân giải `selector`, quyền, cờ, hạn mức đồng thời | `BE-CV-SOL-012`, `BE-CV-SOL-013-*` | cứng |
| Snapshot | `BE-CV-SOL-022-snapshot-cache` | mềm |
| `RepoSourceReader` (cho `CommitDistanceReader`) | `BE-CV-SOL-030-repo-file-access-gateway` | mềm |
| Làm giàu | `BE-CV-SOL-031-erd-model-and-access-scan`, `BE-CV-SOL-033-c4-component-view`, `BE-CV-SOL-037-structure-findings-and-dismissals`, `BE-CV-SOL-038-contract-diff` | mềm, **sau** solution này |
| Thuật toán thứ tự đọc, rủi ro, giới hạn | `BE-CV-SOL-036-reading-order-and-risk` | cùng CR; TASK-036-05 phụ thuộc TASK-036-09 |
| Kênh `codeIntel.changeOverlay`, `codeIntel.readingOrder` | `BE-CV-SOL-040-codeintel-view-channels` | sau |
| Phía frontend | `FE-CV-SOL-051-review-workspace-shell`, `052`, `053` tiêu thụ | sau |

Thứ tự làm trong CR-036 (hợp đồng §7.2: cần 005 (AG), 020, 021, 030): `036-01 → 036-02 → 036-03 → 036-04 → [036-06 … 036-09] → 036-05 → 036-10`.

## 6. Kiểm thử

- **Unit thuần**: ánh xạ trạng thái, `mappingConfidence`, `IndexFreshness` (bảng tổ hợp `index.commit`/`head.oid`/`dirtyFileCount`), phân loại đường dẫn, chọn K (ổn định qua hoán vị), co payload; `go test ./services/code-intel-service/internal/domain/changeoverlay/...`.
- **Use case với fake cổng**: tệp vàng `detectChanges`/`impact` từ `testdata/agent-results` (G1); nguồn lỗi từng cái (agent `CODEINTEL_*`, timeout, impact một phần); cờ, quyền trước cache; ngân sách thời gian bằng đồng hồ giả; singleflight (hai lời gọi một khoá ⇒ một lần gọi agent); `go test ./services/code-intel-service/internal/usecase/ -run ChangeOverlay`.
- **Cô lập tenant**: hai tenant cùng `head_oid` không đọc chéo cache; mọi truy vấn snapshot có `tenant_id` (ở repository của 022; test ở đó, ở đây kiểm khoá).
- **Hai dialect**: không thêm truy vấn riêng; ma trận ở `BE-CV-SOL-022`. Nếu chưa có 022, ghi rõ.
- **Hợp đồng**: `buf lint`, `buf breaking`; test phản chiếu: backend **không** gọi git ngoài whitelist (không có lời gọi `git.*` trực tiếp trong solution này).
- **Chưa chạy bất kỳ test nào.**

## 7. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy hệ thống.** Hình dạng `detectChanges`/`impact` lấy từ hợp đồng (chính nó ghi: mẫu Cypher `IN`, `MATCH (n)` **chưa chạy**; `head` vắng = cây làm việc **chưa kiểm chứng** ở `branchCompare`).
- `gitnexus detect-changes` dùng dòng cũ hay mới của diff (CR §6) là việc của agent; backend chỉ tin `mappingConfidence`.
- Thời gian: 25 × ~1,8 s ÷ 2 ≈ 23 s > 20 s; phụ thuộc đo thật (O-15).
- Không có tín hiệu thay đổi tệp cho cây bẩn; TTL 30 s có thể hiện số cũ tới 30 s.
- `ChangedSymbol` bị xoá chỉ ở mức `unknown`; symbol xoá hoàn toàn chưa phản ánh vào rủi ro (Q6 của CR).
- `tested` dựa trên `testsCovering` lọc theo mẫu tên ở agent; với TypeScript (`vi.mock`, gián tiếp) sẽ báo thiếu (CR §6) ⇒ nhãn `tested:"no"` có thể sai; UI nên nói "không thấy test gọi trực tiếp".
- Payload lớn: fixture 3 000 tệp là ca bắt buộc (task 036-05).

## 8. Câu hỏi mở

1. Thiếu chỉ mục: có yêu cầu agent trả `changedFiles` + `unmapped` khi `INDEX_MISSING` (đề xuất cho `AG-CV-SOL-005`) để overlay vẫn nhóm theo đường dẫn như CR-036 §2.8?
2. `K = 25` và `depth = 2`: giữ làm cấu hình `CODEINTEL_OVERLAY_IMPACT_MAX` (đề xuất tên; chưa có trong hợp đồng §6.2).
3. `head_ref` khác rỗng ngầm `committed` (2.D bước 2): chấp nhận?
4. `mode=worktree` mặc định (O7) giữ; yêu cầu agent commit trước khi review không được chấp nhận ở CR.
5. Có phát `ChangedSymbol.changeKind="deleted"` (additive) thay vì `unknown`?

## 9. Tiêu chí chấp nhận

- [x] Với tệp vàng `detectChanges` gồm tệp đã commit + tệp chưa commit + tệp `untracked`, `changedFiles` chứa mỗi tệp **một** lần, trạng thái và `mappingConfidence` đúng bảng 2.C.
- [x] `mode=committed` (hoặc `head_ref`) không gồm tệp chưa commit; `includesUncommitted=false`.
- [x] Backend chỉ gửi **một** `detectChanges` và ≤ 25 `impact`, tối đa 2 đồng thời; không gọi `git.*` trực tiếp.
- [x] Khi `impact` lỗi/quá hạn cho một phần symbol: `limits.truncated.impact=true`, `tested:"unknown"`, `risk.incomplete=true`, có `DATA_MISSING`; lần gọi sau (cùng khoá) trả đủ.
- [x] `IndexFreshness.state` đúng cho từng tổ hợp; tệp mới chưa index ⇒ `unindexedFiles` và `mappingConfidence:"none"`.
- [x] Nguồn mềm vắng (ERD, C4, contract diff, findings) ⇒ overlay vẫn trả, `risk.incomplete=true`, nhóm component theo đường dẫn "suy luận".
- [x] Fixture 3 000 tệp: `truncated.files=true`, `totalCounts` đúng, payload ≤ 2 MiB, rủi ro tính trên đủ 3 000.
- [x] Quyền sai ⇒ `CODEINTEL_NOT_AUTHORIZED` **không** chạm cache/agent; chéo tenant bị từ chối.
- [x] `buf lint`, `buf breaking` pass; không RPC thiếu message.

## 10. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md`
- `/opt/repos/orca/agent/src/relay/git-handler-ops.ts` (dòng 124), `/opt/repos/orca/agent/src/relay/agent-git-handler.ts` (dòng 41–70), `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
- `/opt/repos/orca/backend-go/proto/buf.yaml`
- Series: `AG-CV-SOL-005-detect-changes`, `AG-CV-SOL-070-golden-fixtures-and-parsers`, `BE-CV-SOL-010/012/013/020/021/022/023/030/031/033/037/038/040`, `FE-CV-SOL-051/052/053`
