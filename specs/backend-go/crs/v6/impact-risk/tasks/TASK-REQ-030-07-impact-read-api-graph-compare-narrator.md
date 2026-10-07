# TASK-REQ-030-07: RPC đọc, `GetImpactGraph` (4 lens), `CompareImpact`, `GetPlanRiskHeatmap`, `ImpactNarrator`

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.H, 2.I
**Priority:** P1
**Service/Area:** `request-service` (mới) / domain đồ thị, usecase đọc, proto, grpc handler
**File:** `internal/domain/impact_graph.go` (mới), `internal/domain/impact_graph_lenses.go` (mới), `internal/usecase/get_impact_assessment.go` (mới), `internal/usecase/get_impact_graph.go` (mới), `internal/usecase/compare_impact.go` (mới), `internal/usecase/plan_risk_heatmap.go` (mới), `internal/usecase/list_impact_findings.go` (mới), `internal/usecase/impact_narrator.go` (mới), `internal/adapter/grpc/server_impact.go` (mới), `backend-go/proto/orca/request/v1/request.proto` (sửa), `backend-go/services/request-service/testdata/contract/impact_graph.schema.json` (mới), và các `_test.go`
**Depends on:** TASK-REQ-030-05 (bản đánh giá `ready`), TASK-REQ-030-03 (repository), CR-REQ-032 (`GraphPayload`, `graph-types.ts`), CR-REQ-036 (tên kênh), CR-REQ-034 hoặc SOL-007 (`AICompleter`), TASK-REQ-001-05 (server gRPC)
**Status:** [x] DONE

---

## Context

- Hợp đồng đồ thị là `GraphPayload` của CR-REQ-032 (frontend, `shared/graph-types.ts`): node `{id, kind, label, group, risk, status}`, cạnh `{from, to, kind, change}`, `totalNodes`, `truncated`, `assessedAt`, `tool`, `stale`; JSON của proto, camelCase khi qua gateway. `risk ∈ low|medium|high|critical|unknown` (`unknown` cho chiều không đo được hoặc index lỗi thời, **không bao giờ** `low` thay cho nó); `change ∈ added|removed|unchanged` độc lập với `risk`; `status` là chuỗi theo lens (`modified`, `breaking`, `irreversible`, trạng thái task); `id` ổn định `<kind>:<tên>` (ví dụ `svc:task-service`); server cắt ở `max_nodes` (mặc định 50, `truncated=true`, `totalNodes` là số trước khi cắt). Gom nhóm hiển thị là việc của client (`graph-grouping.ts`).
- Backend chỉ dựng bốn lens: `architecture` (nền: danh mục service CR-REQ-031, lớp phủ: diff), `contract` (`buf breaking`, `ContractScanner`), `data` (`MigrationScanner`: bảng, migration, cờ không đảo ngược, hai dialect), `impact` (GitNexus `impact`: symbol, luồng theo bước nhảy). `plan`, `execution` do client dựng; `flow` hoàn toàn phía client. Danh mục service (CR-REQ-031) có thể chưa có: lens `architecture` khi đó chỉ có phần diff, không có nền (CR mục 6).
- Bảng RPC và kênh WS (CR 2.8): `RequestImpactAssessment` (`impact.request`), `GetImpactAssessment` (`impact.get`), `GetImpactGraph` (`impact.graph`), `CompareImpact{solution_id}` (`impact.compare`: ma trận Option × 9 chiều), `ListImpactFindings`/`GetImpactEvidence` (`impact.findings`, `impact.evidence`), `GetPlanRiskHeatmap{plan_task_id}` (`impact.heatmap`), `AcceptRisk` (`impact.accept`, task 06), `GetImpactDrift{phase_id}` (`impact.drift`), `OverrideRiskGate` (`risk.override`), `GetRiskPolicy`/`SetRiskPolicy` (`risk.policy.get|set`, task 08). **Tên kênh WS do CR-REQ-016/036 chốt**; mỗi kênh cần `ToolSpec` hoặc dòng loại trừ trong `excluded_channels.yaml` (`parity_test.go` đỏ nếu thiếu, README v6 mục 8 điều 13): việc đó thuộc gateway (SOL-016/017), task này chỉ liệt kê danh sách kênh cần thêm trong mô tả PR.
- Quyền: mọi RPC của `request-service` tự kiểm quyền; đọc theo quyền đọc Request (hành động `impact.*` và `risk.*` trong `request.rego`, CR-REQ-035; `risk.policy.set` chỉ admin tenant).
- `ImpactNarrator` (CR 2.4.2): AI chỉ diễn giải; đầu vào **chỉ** JSON của bản đánh giá trong khối rào (`path`, `message` có thể mang nội dung từ repo); đầu ra bắt buộc `{summary, top_reasons[3], mitigations[]}`; bị loại nếu có trường `score`/`level` hoặc lệch tên chiều; ghi vào `narrative` với `generated_by=ai`, ngoài `digest`; lỗi AI không làm bản đánh giá `failed`. Đường AI hiện có: relay `ai.complete` (README v6 mục 8 điều 1, `AICompleter` của TASK-REQ-007-04), cần project có dev server đang kết nối; `PromptRegistry` của CR-REQ-034 nếu đã có.

## Việc cần làm

1. Proto `request.proto`: thêm message và RPC tương ứng bảng trên. Gợi ý trường: `GetImpactAssessmentRequest{subject_type, subject_id}` trả `ImpactAssessmentSummary{id, level, score, confidence, mode, status, rules_version, top_reasons[3], index_age_seconds, narrative_json, digest, assessed_at, dimensions[DimensionSummary{dimension, score, measured, basis}]}` (**không** trả `findings` đầy đủ ở đây):
   - `GetImpactGraphRequest{request_id, subject_type, subject_id, lens, base, max_nodes}` trả `GraphPayload{lens, repeated GraphNode nodes, repeated GraphEdge edges, int32 total_nodes, bool truncated, google.protobuf.Timestamp assessed_at, string tool, bool stale}`
   - `CompareImpactRequest{solution_id}` trả `ImpactMatrix{repeated OptionRow{option_id, level, score, confidence, repeated DimensionCell}}`
   - `ListImpactFindingsRequest{assessment_id, dimension, min_level, page_size, page_token}`
   - `GetImpactEvidenceRequest{finding_id}` trả `ImpactEvidence{tool, command_digest, status, output (đã cắt), index_commit}`
   - `GetPlanRiskHeatmapRequest{plan_task_id}` trả `Heatmap{repeated HeatmapCell{phase_task_id, task_id, level, score, assessment_id}}`
   - `GetImpactDriftRequest{phase_id}` trả `ImpactDrift{expected, actual, drift, reasons[]}`. Chạy `make proto` rồi `buf lint`, `buf breaking` trực tiếp (không `make proto-lint`).
2. `impact_graph.go`: kiểu `GraphPayload`, `GraphNode`, `GraphEdge`, hàm thuần `BuildLens(lens Lens, in LensInput) GraphPayload` với `LensInput{Findings []Finding; Signals []Signal; ChangedFiles []string; Catalog []ServiceEntry (có thể rỗng); ImpactSymbols []ImpactSummary; Stale bool; MaxNodes int}`:
   - id nút `svc:<tên>`, `proto:<tên>`, `table:<tên>`, `migration:<đường dẫn>`, `channel:<tên>`, `symbol:<tên>`
   - sắp xếp xác định (theo `id`) để `id` và thứ tự ổn định giữa hai lần gọi.
3. `impact_graph_lenses.go`: bốn hàm dựng: `architectureLens` (nút service từ `ChangedFiles` và, khi có, `Catalog`; cạnh `calls` giữa service mới thêm: nhận diện từ finding `NEW_GRPC_EDGE` của `contractscan`... nếu bộ quét chưa sinh thì chỉ nút):
   - `contractLens` (nút `proto`, `channel`, `outbox_event`; `status=breaking` cho vi phạm `buf`; `change=added|removed` cho kênh)
   - `dataLens` (nút `table`, `migration`; `status=irreversible` khi thiếu down hoặc phá huỷ; ghi hai dialect)
   - `impactLens` (nút symbol, cạnh người gọi theo bước nhảy từ `ImpactSummary`; chỉ có khi `basis=tool`). `risk` của nút = mức của chiều liên quan hoặc `unknown` khi chiều `Measured=false` hoặc `Stale`. Cắt `max_nodes`: ưu tiên nút `risk` cao rồi `id`
   - đặt `truncated`, `totalNodes` trước cắt.
4. `get_impact_assessment.go`: `Execute(ctx, in) (ImpactAssessmentSummary, error)`: `RequireTenantID`, quyền đọc Request, bản `ready|partial` mới nhất (hoặc `collecting`, kèm trạng thái để UI chờ):
   - `top_reasons` = ba finding/tín hiệu điểm cao nhất (sắp xếp xác định)
   - `index_age_seconds` từ `impact_tool_runs`. Không có bản nào thì `REQUEST_IMPACT_NOT_FOUND` (`NotFound`).
5. `get_impact_graph.go`: gọi `LatestImpactAssessment`, đọc `impact_tool_runs` và `findings`, gọi `BuildLens`; chủ thể không có bản `ready` thì `REQUEST_IMPACT_NOT_FOUND`; `max_nodes` ngoài `[1, 200]` thì kẹp về mặc định 50; `lens` ngoài bốn giá trị thì `InvalidArgument`.
6. `compare_impact.go`: lấy các bản `solution_option` của Option của `solution_id` (một dòng mỗi Option, bản `ready|partial` mới nhất; Option chưa có bản thì dòng `status=collecting|missing`); trả ma trận Option × 9 chiều (`DimensionCell{dimension, score, measured, basis}`) cùng `level`, `score`, `confidence` (nhãn "ước lượng" cho `confidence` tối đa `medium`).
7. `plan_risk_heatmap.go`: lấy bản `plan`, các bản `phase`, `task` của Plan (qua `ListImpactAssessmentsByRequest`), ghép với cây Phase/Task từ `task-service.GetSubtree` hoặc từ `plan.generated` (id Phase/Task); trả ô theo Phase và Task kèm `level`.
8. `list_impact_findings.go`: phân trang (`page_size` ≤ 100, token bất biến theo `(severity, finding_id)`):
   - `min_level` lọc
   - sắp xếp theo phạm vi ảnh hưởng kèm độ phủ (CR 2.8). `GetImpactEvidence`: tra `finding.evidence_run_id` ra `impact_tool_runs.output` đã cắt
   - **quét `secretscan` trước khi trả** (`output` có thể chứa đường dẫn/biến)
   - không trả nếu người gọi thiếu quyền đọc Request.
9. `impact_narrator.go`: `ImpactNarrator.Narrate(ctx, assessmentID)` gọi sau khi `CompleteImpactAssessment` (task 05 phát tín hiệu):
   - dựng prompt từ JSON của bản đánh giá (**chỉ** `dimensions`, `triggers`, `findings` rút gọn: bỏ `output`, cắt `message` 300 ký tự, tối đa 40 finding), bọc trong khối ranh giới suy ra từ nội dung (như SOL-029 mục 2.C)
   - gọi `AICompleter.Complete` (timeout `REQUEST_AI_COMPLETE_TIMEOUT`)
   - `ValidateNarrative(raw) (Narrative, error)`: JSON đối tượng `{summary, top_reasons[3], mitigations[]}`
   - từ chối khi có khoá `score`/`level`/`rank` hoặc khi `top_reasons`/`mitigations` nhắc tên chiều ngoài chín tên
   - `summary` ≤ 1500 ký tự
   - thử lại **một** lần với lỗi cụ thể
   - thất bại thì để `narrative` NULL và log (không đổi `status`). Lưu bằng `SetImpactNarrative` (`generated_by=ai` trong JSON).
10. `server_impact.go`: handler cho từng RPC, `RequireTenantID`, ánh xạ lỗi `apperrors` → gRPC; mã lỗi `REQUEST_IMPACT_NOT_FOUND`, `REQUEST_IMPACT_DISABLED`, `REQUEST_IMPACT_NO_CONNECTION`.
11. `testdata/contract/impact_graph.schema.json`: JSON Schema của `GraphPayload` (cùng mẫu dùng chung với frontend, CR-REQ-018/032); test `TestGraphPayloadMatchesSchema` cho từng lens.

## Kiểm thử

- `TestBuildLens_StableIDsAcrossCalls`, `_TruncatesAtMaxNodesWithFlags`, `_UnknownRiskWhenDimensionUnmeasured`, `_NeverLowForUnmeasured`, `_StaleSetsFlag`, `_ChangeIndependentOfRisk`, `_ArchitectureLensWithoutCatalog_DiffOnly`, `_DataLensMarksIrreversible`, `_ContractLensMarksBreaking`.
- `TestGetImpactGraph_AllLensesMatchSchema`, `_InvalidLens`, `_MaxNodesClamped`, `_NoAssessmentNotFound`.
- `TestGetImpactAssessment_TopReasonsDeterministic`, `_CollectingStatusReturned`, `_TenantIsolation`.
- `TestCompareImpact_MatrixShape`, `_OptionWithoutAssessmentMarked`, `_ConfidenceLabelAtMostMedium`.
- `TestHeatmap_PhaseAndTaskCells`.
- `TestListFindings_PaginationStable`, `_MinLevelFilter`; `TestEvidence_SecretScanned`, `_Unauthorized`.
- `TestNarrative_RejectsScoreOrLevelFields`, `_RejectsUnknownDimensionName`, `_RetriesOnceWithSpecificError`, `_AIErrorLeavesAssessmentReady`, `_NotInDigest` (đổi `narrative` không đổi `digest`), `_PromptHasNoToolOutput`, `_PromptFencedAgainstInjection` (finding có chuỗi "ignore previous instructions" nằm trong khối rào).
- Handler/authz: `TestServer_Impact_RequiresTenantAndReadPermission`, `_PolicySetAdminOnly` (khi có task 08).
- Hợp đồng: `buf lint`, `buf breaking`; schema đồ thị khớp mẫu của frontend.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpc/... -run "Graph|Impact|Compare|Heatmap|Narrat|Findings"`.

## Tiêu chí hoàn thành

- [x] `GetImpactGraph` mọi lens trả đúng hợp đồng node, cạnh; `id` ổn định giữa hai lần gọi; vượt `max_nodes` thì `truncated=true` và `totalNodes` đúng.
- [x] `risk=unknown` cho chiều không đo được hoặc index lỗi thời, không bao giờ `low`.
- [x] Narrative chứa `score` hoặc `level` bị loại; lỗi AI không làm bản đánh giá `failed`; `narrative` không nằm trong `digest`.
- [x] `GetImpactEvidence` không trả bí mật (quét trước khi trả).
- [x] Danh sách kênh WS cần thêm ghi trong PR (`impact.*`, `risk.*`), kèm đề nghị `ToolSpec` hoặc dòng loại trừ cho gateway.
- [x] Không file nào tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable (tách `impact_graph_lenses.go` theo lens nếu dài).

## Rủi ro và lưu ý

- Hợp đồng `GraphPayload` do CR-REQ-032 (frontend) giữ; nếu hai bên đổi tên trường, schema chung bắt: phải đồng bộ trước khi merge.
- Lens `architecture` cần danh mục service (CR-REQ-031); khi chưa có chỉ hiện phần diff.
- Lens `impact` chỉ có ở `actual_*` (xem SOL-030 mục 1 điều 1); đồ thị cho Plan nghèo hơn kỳ vọng của người dùng; cần UI nêu `basis`.
- `top_reasons` do mã chọn (xác định), khác `narrative.top_reasons` do AI diễn giải; UI phải phân biệt hai nguồn.
- Tên kênh WS chưa chốt (CR-REQ-016/036): đừng cứng tên kênh ở backend; chỉ tên RPC.
