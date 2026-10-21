# BE-OR-SOL-005 — Cơ sở thiết kế đơn điệu, phủ flow và kết quả agent bền hơn

**CR:** CR-OR-040, CR-OR-041, CR-OR-042 (`docs/crs/v9/outcome-review-fidelity/`) · **Service:** `request-service`, `api-gateway` · **Checklist:** `tasks/phase-6-design-basis.md`, `tasks/phase-7-coverage-and-resilience.md` · **Trạng thái:** 📝 Đề xuất (2026-10-21)

## 1. Hiện trạng liên quan (đã đọc code)

| Thành phần | Hiện làm gì | Vị trí |
|---|---|---|
| Khoá thiết kế | `ReviewDesignLock.Save/Load`: một bản thiết kế mới nhất cho mỗi Request, hạn TTL, loại bỏ khi `promptVersion` hoặc `schema` khác | `usecase/outcome_review_design_lock.go` |
| `Force` | "ignores the cache and the locked design: it is what regenerate means"; `keepDesign` lưu bản trích mới | `usecase/outcome_review_context_run.go:63,131,474` |
| Phạm vi sinh lại | `RegenArtifact`, `RegenContext` ("reads the graph and the code again"), `RegenAll` | `domain/outcome_review_regenerate_policy.go` |
| Flow thiết kế | `mergeFlows`: thay flow agent bằng flow parse từ khối `sequenceDiagram` agent chép; tên "Flow <mục>" | `domain/outcome_review_design.go:626` |
| So sánh cạnh | `Design.HasCall(from,to)` quét mọi flow; `DeriveState` ba chiều | `domain/outcome_review_design.go:124`, `outcome_review_edge_state.go` |
| Cắt hình | `capDiagram`: giữ nút "đáng xem xét" rồi theo ID; `MaxDiagramNodes = 12`, `MaxDiagramLanes = 6` | `domain/outcome_review_diagram_model.go:447` |
| Trần số hình | `MaxContextDiagrams = 16` ⇒ vi phạm toàn bộ kết quả; `MaxDrawingsPerRun = 4` | `domain/outcome_review_context.go:37`, `review_artifact.go:14` |
| Kiểm kết quả | `ParseContextResult`, `ValidateContext`; vòng `for attempt < 2` | `domain/outcome_review_context*.go`, `usecase/outcome_review_context_run.go` |
| Cổng file | `ReviewContextRunner.Files` có kiểu `RepoFileStat` (chỉ `Stat`); adapter `grpcclient.NewProposalWorkspace` đã cài `ReadFile` (`adapter/grpcclient/proposal_workspace.go:35`, ≤ 256 KiB) nhưng runner chỉ khai báo `Stat` | `usecase/ports_workspace.go:7,52`, `cmd/server/wire_outcome_review.go` |
| Verdict | `ComputeDesignVerdicts` đếm trên các hình `delta` | `domain/outcome_review_diagram_model.go:643` |

## 2. Cơ sở thiết kế đơn điệu (CR-OR-040 R1–R3, R7)

**Dữ liệu.** Mở rộng bản ghi `outcome_review_contexts` (kind=`design`) hoặc thêm bảng nhỏ `outcome_review_design_versions` (quyết ở T602). Đã kiểm chứng: `Put` dùng `ON CONFLICT (tenant_id, request_id, kind, digest) DO UPDATE` (`adapter/postgres/outcome_review_context_repository.go:43`), nên một lần trích lại có cùng digest sẽ **ghi đè** bản cũ; digest gồm `docSha256` mà agent khai nên hai lần trích của #2318 ra hai digest khác nhau (có thể do agent liệt kê tài liệu khác nhau): mỗi dòng có `request_id`, `version` tăng dần, `digest`, `snapshot`, `extracted_at`, `counts{containers,edges,flows,mermaid,text}`, `supersedes`, `payload`. `Load` trả phiên bản **đang dùng** (con trỏ), không phải "mới nhất theo thời gian".

**Quy tắc không thu hẹp** (`DesignShrank(old, new)`), chạy khi cùng `(snapshot, docSha256)`:
`shrank = len(new.Containers) < len(old.Containers) || len(new.Edges) < len(old.Edges) || len(new.Flows) < len(old.Flows)`
- `shrank` ⇒ **giữ** phiên bản cũ làm con trỏ, lưu bản mới như ứng viên (không dùng), cảnh báo `design_shrank{old,new}` vào `model.warnings` và `designBasis`.
- Snapshot hoặc `docSha256` đổi ⇒ đây là thiết kế mới hợp lệ, thay con trỏ.
- Người dùng chọn "thay bằng bản mới" ⇒ chuyển con trỏ sang ứng viên (RPC `request.review.designBasis.adopt`, ghi audit).

**Tách phạm vi.** `RegenScope` thêm `code` và `design`; `context` giữ làm bí danh `all` một phiên bản rồi gỡ. `StartContextInput.Force` tách thành `IgnoreCache` và `ReextractDesign`:
- `code`: `IgnoreCache=true`, `ReextractDesign=false` ⇒ `plan.locked` được nạp, prompt ở chế độ "đã khoá" (không `kg_*`).
- `design`: `ReextractDesign=true`.
- `all`: cả hai.

**Đọc tài liệu.** Mở rộng cổng `RepoFileStat` thành `RepoFiles{Stat, ReadFile}`; `wire_outcome_review.go` truyền adapter sẵn có. Hàm `DiscoverDesignBlocks(files, docPaths)` đọc từng tài liệu (đường dẫn lấy từ khoá `docSha256` mà agent khai, O5), dùng `ParseMermaid` (đã có) để liệt kê khối `sequenceDiagram`/`flowchart` theo mục. Kết quả:
- Khối agent không chép ⇒ Orca bổ sung vào `sourceBlocks`, ánh xạ participant qua alias container như `readBlocks` hiện làm, cảnh báo `design_blocks_added:<mục>`.
- Tài liệu không đọc được (không có, quá 256 KiB) ⇒ giữ hành vi cũ, cảnh báo `design_docs_unreadable`.

**`flowRef` và nguồn.** `DesignFlow` thêm `FlowRef string` và `Origin` ∈ `mermaid|text`. Flow parse từ mermaid: `FlowRef = <mục SAD>` (thay vì tên "Flow 8.1"); `Name` giữ nhãn đọc được lấy từ tiêu đề mục. Flow `text` (agent trích từ chữ): `FlowRef` là mục SAD hoặc mã yêu cầu, **bắt buộc** `Evidence` loại `source` có `doc`, `section`, `line`; thiếu ⇒ vi phạm (nặng).

**Lát cắt kiến trúc.** `ScopeSlice(design, facts, requirements) []containerID`:
1. `touched` = container có file đổi (khớp tiền tố đường dẫn tới container qua alias/đường dẫn trong thiết kế) ∪ container xuất hiện trong hình delta ∪ container có `componentsChanged>0`.
2. `linked` = container mà FR/AC của Request nối tới (khi KG nối được; chưa kiểm chứng).
3. Điểm = 3 nếu `touched`, 2 nếu `linked`, 1 nếu là hàng xóm một bước của (1)∪(2), 0 còn lại.
4. `BuildDesignDiagrams` và `capDiagram` giữ theo điểm giảm dần rồi theo ID; nút 0 điểm gom vào "+N" có `drilldown` về danh sách container đầy đủ.

## 3. Phủ flow (CR-OR-041)

**Flow ứng viên.** `FlowCandidates(design, touched, requirements)`:
`candidate = {f ∈ design.Flows : f.Messages ∩ touched ≠ ∅}` (lý do `code_touches`) ∪ `{f : f.FlowRef ∈ requirement flows}` (lý do `requirement_link`). Kết quả được tính ở Orca **trước** khi gọi agent và đưa vào prompt trong `<candidate_flows>` để agent trả lời từng cái.

**Hợp đồng agent** (schema `outcome-review-context/2`, đọc được `/1`):
```
"flows": [ { "flowRef": "8.5", "status": "drawn|unchanged|not_applicable|undrawable",
             "reason": "…", "evidence": [ … ] } ]
```
`diagrams[]` thêm `flowRef`. Quy tắc kiểm:
- mỗi flow ứng viên có đúng một phần tử trong `flows[]`; `drawn` ⇒ có ít nhất một hình `flowRef` tương ứng; thiếu ⇒ vi phạm (nặng, vào retry).
- `flowRef` không thuộc ứng viên ⇒ hợp lệ nếu hình có `source=code` (flow mới, R3); gắn `origin=code`, không tạo `deviation`.
- `unchanged`/`not_applicable`/`undrawable` cần `reason` không rỗng và ít nhất một bằng chứng.

**So sánh theo flow.** `DiagramModel.FlowRef`. Trong `buildDiagram`: nếu `isFlow` và có `FlowRef` khớp một `DesignFlow` ⇒ `InDesign = flow.HasCall(from,to)` (hàm mới, chỉ trong flow đó); không khớp ⇒ `InDesign = nil`. `Design.HasCall` toàn cục chỉ còn dùng cho kiến trúc khi không có flow.

**Cơ sở yếu ⇒ `advisory`.** `WeakBasis(design) bool` = có `design_shrank` đang hiệu lực, hoặc `len(Flows)==0`, hoặc flow tương ứng thiếu. Khi `WeakBasis`, `AdvisoryFor` trả `true` cho mọi `deviation`/`missing` và ghi `advisoryReason` (`weak_basis`).

**Mô hình.** `model.flowCoverage{total, drawn, items[{flowRef, name, reason, status, hasDesign, hasDelta}]}` và `model.designBasis{version, digest, snapshot, counts, extractedAt, warnings}`. Verdict Flow tính trên các hình đã ghép; `unplannedCalls` đếm cạnh `deviation` thật (không `advisory` do cơ sở yếu).

**Trần.** `ValidateDiagrams`: thay `len(raw) > MaxContextDiagrams` (từ chối) bằng sắp theo ưu tiên (flow ứng viên → kiến trúc → data → behavior) và cắt, `diagrams_capped:N`; hình bị cắt thành mục `capped` trong `flowCoverage`. Cấu hình `REQUEST_REVIEW_MAX_FLOW_DIAGRAMS` (mặc định 8, giới hạn 1–16).

**Vẽ theo yêu cầu.** RPC sinh lại hình nhận `sections` + tuỳ chọn `flowRef`; `selectModels` lọc theo `flowRef`; `estimate` trả chi phí cho đúng số hình sẽ vẽ.

## 4. Kết quả agent bền hơn (CR-OR-042)

**Phân loại vi phạm.** `ContextViolations` đổi thành danh sách `Violation{Code, Path, Message, Severity}` với `Severity ∈ local|fatal` (vẫn in như cũ cho prompt retry). Bảng:

| Mã | Mức | Xử lý |
|---|---|---|
| `type_mismatch` ép được (`"true"`/`"false"` → bool, số chuỗi → số) | local | ép, cảnh báo `coerced:<path>` |
| `design_container_source_code` | local | bỏ container, bỏ cạnh/tin nhắn nối tới nó khỏi thiết kế, cảnh báo |
| giá trị enum lạ ở `reviewState`/`source` | local | thay bằng `unknown`/bỏ mục, cảnh báo |
| không phải JSON, sai `schema`, vượt 200 KB | fatal | từ chối |
| `kgId` ngoài `elementsRead`, bằng chứng tham chiếu thứ không có | fatal | từ chối (giữ quy tắc an toàn hiện có) |
| thiếu câu trả lời flow ứng viên | fatal | từ chối, vào retry |

**Ngân sách thử lại.** Trong vòng `for attempt < 2`: sau lần một, nếu `stats.Cost + predicted > REQUEST_REVIEW_RETRY_BUDGET_USD` thì không thử tiếp, trả `REQUEST_REVIEW_RESULT_INVALID` kèm `error_detail`.

**Lưu lý do lỗi.** Cột `error_detail TEXT NOT NULL DEFAULT ''` trong `outcome_review_runs` (migration cả hai dialect, có `down`); ghi bằng `RedactRaw`, tối đa 30 dòng × 200 ký tự; `request.review.runs` trả `errorDetail`.

**Suy giảm.** Khi `execute` thất bại: nếu có phiên bản thiết kế đang dùng ⇒ `ApplyLockedDesignOnly` hợp nhất thiết kế vào review (hình `design`, `designChecked=true`, `codeCompared=false`) và đặt `warnings += code_compare_failed`. Approve vẫn bị chặn (`REQUEST_REVIEW_CONTEXT_FAILED`, đã có).

**Chi phí vẽ.** `outcome_review_runs` đã có `cost_actual`; thêm `diagram_count` (cột tuỳ chọn) để tính chi phí mỗi hình; `estimate` dùng trung bình gần nhất theo engine.

## 5. Hợp đồng ngoài (proto, gateway, CLI/MCP)

- `RegenScope` enum: thêm `CODE`, `DESIGN`; giữ `CONTEXT` (bí danh `ALL`) một phiên bản.
- `OutcomeReview.model`: thêm `designBasis`, `flowCoverage`; `DiagramRef`: thêm `flowRef`; `Run`: thêm `errorDetail`, `diagramCount`.
- Kênh mới: `request.review.designBasis.get`, `request.review.designBasis.adopt` (ghi, cần vai trò duyệt); cập nhật `channel_inventory.json`, `TestRequestRPCChannelParity`, danh mục MCP (đọc được, ghi nằm trong `excluded_channels.yaml`), chính sách OPA `agent_rpcs`.
- Mọi trường mới **tùy chọn**; model đã lưu không có chúng vẫn hiển thị.

## 6. Tương thích và triển khai

| Điểm | Cách |
|---|---|
| Lock cũ | `Load` chấp nhận `schema 1` (không có `flowRef`): suy `flowRef` từ `Evidence.Section` của flow; chỉ ghi lock mới ở định dạng mới (O4). |
| `promptVersion` | Nâng khi đổi `_contract.tmpl`; cập nhật golden `testdata/prompt-golden/RenderReviewContextPrompt` bằng `-update`. |
| Cờ | `REQUEST_REVIEW_FIDELITY=on|off` (mặc định `on` ở môi trường dev, `off` nơi khác tới khi qua e2e); `off` giữ hành vi hiện tại. |
| Migration | Một migration thêm `error_detail`, `diagram_count`, và (nếu chọn) bảng phiên bản thiết kế; có `up` và `down`, cả hai dialect. |

## 7. Chiến lược kiểm

Bảng ca trong từng task. Fixture: `SAD.md` thật của `vnp-qa-platform` (3 sequence, 10 mermaid), ba lần trích thiết kế của #2318 (05:41, 06:43, 07:14), và khối kết quả đã làm hỏng #2318 (`expected` chuỗi; 4 container `source=code`). Kiểm hồi quy: toàn bộ `go test ./services/request-service/...` và `./services/api-gateway/...` (parity kênh).

## 8. Spike đầu pha

| Spike | Câu hỏi | Kết quả cần |
|---|---|---|
| BE-OR-T601 | Đọc `docs/SAD.md` qua `ReadFile`; đường dẫn lấy từ đâu; SAD > 256 KiB | quyết cách (O5) |
| BE-OR-T602 | Đã biết `Put` ghi đè cùng `digest`; còn cần: `docSha256` do agent khai có ổn định không, và chọn bảng phiên bản hay cột | quyết cách lưu phiên bản |
| BE-OR-T701a | KG có truy vấn nối FR/AC → flow không (`kg_neighbors`, `kg_resolve_scope`)? | có hay không R1(b) |
| BE-OR-T713 | Engine `agent` so với `builtin` trên mô hình #2318 | quyết mặc định (O3) |

## 9. Rủi ro tổng hợp

| Rủi ro | Giảm thiểu |
|---|---|
| Đổi schema agent làm tăng vi phạm tạm thời | Chấp nhận một phần (§4); retry có ngân sách; theo dõi chỉ số R8. |
| Quy tắc không thu hẹp che một thiết kế đúng nhỏ hơn thật | Chỉ áp cùng `(snapshot, docSha256)`; có nút thay bản và lịch sử. |
| Lát cắt bỏ sót container | `+N` có drilldown; kiểm AC5 với #2318; cho phép "Hiện toàn bộ". |
| Nhiều flow làm run vẽ đắt | Trần 8; vẽ theo yêu cầu; ước tính trước. |
