# TASK-REQ-027-06: ID hiển thị, `artifact_index`, quan hệ chuẩn, bảng phủ và kiểm tra ngữ nghĩa

**From Solution:** BE-REQ-SOL-027
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/artifact_display_id.go`, `internal/domain/relation_rules.go`, `internal/domain/artifact_semantic_validation.go`, `internal/usecase/mint_artifact_ids.go`, `internal/usecase/replace_request_coverage.go`, `internal/usecase/get_artifact_graph.go`, `internal/usecase/resolve_artifact_ref.go`, `internal/usecase/ports.go` (sửa), `internal/adapter/{postgres,mysql}/{artifact_index,artifact_relation,request_coverage}_repository.go` và test
**Depends on:** TASK-REQ-027-01 (bảng), TASK-REQ-027-03 (`Violation`, schema), TASK-REQ-027-05 (`AcceptanceCriteria`)
**Status:** [x] DONE

---

## Context

CR-REQ-027 mục 2.2, 2.3, 2.5 (bảng phủ), 2.8. Quan hệ: tám loại, trong đó `contains` và `depends_on` **không sao chép** (nguồn là cạnh `parent_child` và `depends_on` của `task-service`, đọc qua `GetSubtree`, RPC có sẵn dòng 61 của `task.proto`); `verifies` lưu ở `request_coverage`; `spawned_by` đọc từ `request_links` (SOL-002). Năm quan hệ còn lại (`derived_from`, `implements`, `supersedes`, `evidenced_by`, và `verifies` dạng Check → AC chỉ ở bảng phủ) nằm ở `artifact_relations`.

ID hiển thị là bí danh bất biến; khoá thật là UUID. `TSK-` **không** thay `TaskNumber` của `task-service` (README O2 giữ nguyên: Plan, Phase không có số). `evidenced_by` trỏ tới Evidence mà CR-REQ-030 (chưa tồn tại) mới định nghĩa: `to_id` là chuỗi tham chiếu, bảng Evidence chưa có; hoãn (Q6 của SOL-027), chỉ giữ hằng và luật để không phá schema.

Kiểm ngữ nghĩa chạy ở server sau kiểm cấu trúc, mỗi lần sinh và mỗi lần commit; `ValidateProposal` (SOL-012) gọi lại hàm này, không nhân đôi. Các mã về số phương án, hồi quy `bug`/`security`, giới hạn Phase vẫn thuộc SOL-007 và SOL-012.

## Việc cần làm

1. `artifact_display_id.go`: `FormatRequestID(n int64)`, `FormatAC(reqNum int64, ac string)`, `FormatSolutionID(reqNum int64, seq int)`, `FormatOptionID(solutionID string, k int)`, `FormatPlanID(reqNum int64, seq int)`, `FormatPhaseID(reqNum int64, planSeq, k int)`, `FormatTaskID(reqNum int64, planSeq, k int)` và `ParseDisplayID(s string) (DisplayID, error)` trả `{Kind, ReqNum, Seq, Index, Option}`;
   - regexp neo `^...$`, từ chối khoảng trắng và chữ thường (`REQUEST_ARTIFACT_ID_INVALID`, `InvalidArgument`).
2. `relation_rules.go`: `type Relation string` tám hằng;
   - `AllowedRelation(rel Relation, fromKind, toKind ArtifactKind) bool` theo bảng CR 2.3
   - `StoredInRelationsTable(rel) bool` (đúng năm quan hệ)
   - `NewArtifactRelation(...) (ArtifactRelation, error)` trả `REQUEST_ARTIFACT_RELATION_NOT_ALLOWED` khi ngoài bảng.
   - Test phủ toàn bộ tích Descartes `rel x fromKind x toKind` (8 x 6 x 6) để khẳng định chỉ các bộ ba trong bảng được chấp nhận.
3. `artifact_semantic_validation.go`: `ArtifactContext{Request RequestContent; Solution *SolutionOptions; ChosenOptionID string; Plan *PlanSpecs}`;
   - `PlanSpecs{Tasks []TaskSpecView; Phases []PhaseSpecView; Edges []DependsEdge}`
   - `ValidateArtifactSemantics(ctx ArtifactContext) []Violation` trả: `REQUEST_ARTIFACT_DEPENDENCY_CYCLE` (Kahn trên task cùng container và Phase; nêu id đầu vòng), `REQUEST_ARTIFACT_UNKNOWN_AC` (`satisfies`/`ac_id` tới AC không có hoặc `retired`), `REQUEST_ARTIFACT_AC_NOT_COVERED` (AC `active` không task nào phủ, trừ task `exempt_from_coverage`), `REQUEST_ARTIFACT_TASK_NO_CHECK`, `REQUEST_ARTIFACT_TASK_NO_AC` (task không miễn trừ), `REQUEST_ARTIFACT_AC_UNCOVERED_BY_OPTION` (phương án được chọn chưa trả lời đủ AC `active`; `out_of_scope` bắt buộc có `note`), `REQUEST_ARTIFACT_LIMIT_EXCEEDED` (số AC, số task, số check).
   - Quy tắc miễn trừ: `exempt_from_coverage=true` chỉ hợp lệ khi task mang nhãn `rollback` hoặc `check:*` (nhãn từ `plan_labels.go` của SOL-012), nếu không thì `REQUEST_ARTIFACT_EXEMPT_NOT_ALLOWED`.
   - Hàm thuần, trả hết lỗi (không dừng ở lỗi đầu), xác định thứ tự.
4. `ports.go`: `ArtifactIndexRepository{Insert(ctx, e IndexEntry) error; Resolve(ctx, displayID string) (IndexEntry, error); NextSolutionSeq(ctx, requestID string) (int, error); NextPlanSeq(ctx, requestID string) (int, error)}`;
   - `ArtifactRelationRepository{Insert(ctx, r ArtifactRelation) error; ListByRequest(ctx, requestID string) ([]ArtifactRelation, error)}`
   - `RequestCoverageRepository{ReplaceForPlan(ctx, requestID, planTaskID string, rows []CoverageRow) error; ListByRequest(ctx, requestID string) ([]CoverageRow, error)}`.
   - Hai thao tác ghi dùng executor trong ctx (tham gia transaction của caller, SOL-001 mục 2.D).
5. Adapter hai dialect: `Insert` index và relation dùng `INSERT ... ON CONFLICT DO NOTHING` (Postgres) / `INSERT IGNORE` (MySQL) để giao lặp an toàn (kiểm `RowsAffected`);
   - `NextSolutionSeq` **không** tự tăng: chỉ đọc `MAX(seq)+1`, va chạm UNIQUE do tầng trên retry tối đa 3 lần
   - `ReplaceForPlan` là `DELETE ... WHERE tenant_id AND request_id AND plan_task_id` rồi `INSERT` nhiều hàng trong transaction của caller
   - `Resolve` trả `REQUEST_ARTIFACT_NOT_FOUND` khi không có hoặc khác tenant.
6. `mint_artifact_ids.go`: `MintSolutionID(ctx, requestNumber int64, sol *Solution) error` (gán `Seq`, ghi `artifact_index` kind `solution` và mỗi `option`);
   - `MintPlanIDs(ctx, requestNumber int64, planSeq int, tree PlanTreeIDs) error` (ghi `plan`, `phase`, `task`; `n` của task là thứ tự trong cây lúc commit).
   - Gọi từ `CommitPlan` và `RunSolutionGeneration` (nối ở 027-07).
7. `replace_request_coverage.go`: `BuildCoverageRows(plan PlanSpecs) []CoverageRow` (một dòng mỗi cặp `(ac_id, task_id, check_id?)`), `ReplaceRequestCoverage.Execute(ctx, tx, requestID, planTaskID, plan)`;
   - **không** tự mở transaction (chạy trong transaction CAS `plan_task_id` của `CommitPlan`).
8. `get_artifact_graph.go`: `GetArtifactGraph.Execute(ctx, requestID)` gộp `artifact_relations`, `request_links` (đổi `reason` thành `spawned_by`) và cạnh `contains`, `depends_on` suy từ `TaskClient.GetSubtree(planTaskID)`;
   - trả `[]Edge{Rel, FromKind, FromID, ToKind, ToID}` sắp theo `(Rel, From, To)`
   - lỗi `task-service` thì trả phần có được kèm cờ `partial=true`, không lỗi cả RPC.
   - `resolve_artifact_ref.go`: bọc `ArtifactIndexRepository.Resolve` kèm kiểm quyền đọc Request.

## Kiểm thử

- `TestDisplayID_FormatParse_RoundTrip` (mọi loại)
- `TestParseDisplayID_RejectsMalformed` (chữ thường, khoảng trắng, `SOL-142`
- `SOL-142.2/opt-`).
- `TestRelationRules_FullCartesian`
- `TestStoredInRelationsTable_FiveOnly`.
- `TestSemantic_UnknownAC`
- `TestSemantic_RetiredAC`
- `TestSemantic_ACNotCovered_ExemptTaskExcluded`
- `TestSemantic_TaskNoCheck`
- `TestSemantic_TaskNoAC`
- `TestSemantic_ExemptWithoutLabel`
- `TestSemantic_DependencyCycle_ReportsStartNode`
- `TestSemantic_OptionLeavesACUncovered`
- `TestSemantic_OutOfScopeNeedsNote`
- `TestSemantic_ReturnsAllViolationsDeterministically`.
- `TestReplaceRequestCoverage_RollsBackWithCaller` (lỗi sau `ReplaceForPlan` làm cả hai biến mất)
- `TestMintSolutionID_ConflictRetries`.
- `TestGetArtifactGraph_MergesThreeSources_PartialOnTaskServiceError`.
- Integration hai dialect: `ArtifactIndexRepository` (`Resolve` chéo tenant trả không có), UNIQUE relation giao lặp, `ReplaceForPlan` thay nguyên khối, 20 `MintSolutionID` đồng thời cho 20 `seq` khác nhau (có retry).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... -run 'DisplayID|RelationRules|Semantic|Coverage|MintArtifact|ArtifactGraph' && go test -tags=integration ./services/request-service/internal/adapter/... -run 'ArtifactIndex|ArtifactRelation|RequestCoverage'`.

## Tiêu chí hoàn thành

- [x] Mười mã ngữ nghĩa mỗi mã có đúng một test; hàm trả đủ mọi lỗi, thứ tự xác định.
- [x] Chỉ các bộ ba quan hệ ở bảng CR 2.3 được chấp nhận (test tích Descartes).
- [x] `request_coverage` thay nguyên khối, rollback cùng caller.
- [x] `GetArtifactGraph` gộp đúng ba nguồn và không thất bại khi `task-service` lỗi.
- [x] Hai Solution đồng thời của một Request nhận `seq` khác nhau.

## Rủi ro và lưu ý

- `MAX(seq)+1` dưới tải cao có thể va chạm liên tục; retry 3 lần đủ cho một Request vì thao tác này hiếm, nhưng chưa đo.
- `GetArtifactGraph` phụ thuộc `GetSubtree` của task-service có giới hạn độ sâu; cây lớn có thể chậm (chưa đo).
- Ngưỡng giới hạn số AC, task, check là số đề xuất.
- Ranh giới giữa `verifies` ở `request_coverage` và Check chạy được (CR-REQ-029, `request_checks` của CR-REQ-014): task này chỉ lưu liên kết, không lưu kết quả chạy.
