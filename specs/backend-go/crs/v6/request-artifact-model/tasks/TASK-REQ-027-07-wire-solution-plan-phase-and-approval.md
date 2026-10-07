# TASK-REQ-027-07: Nối vào Solution, Plan, Phase và Approval: schema `options`, provenance, bảng phủ, khoá spec

**From Solution:** BE-REQ-SOL-027
**Priority:** P0
**Service:** `request-service` · `task-service`
**File:** `request-service/internal/domain/solution_options.go` (sửa, của SOL-007), `internal/usecase/run_solution_generation.go` (sửa), `internal/usecase/solution_approval_handler.go` (sửa), `internal/domain/plan_proposal.go` (sửa, của SOL-012), `internal/domain/plan_proposal_validation.go` (sửa), `internal/usecase/commit_plan.go` (sửa), `internal/usecase/plan_subject_handler.go` (sửa), `internal/usecase/start_phase.go` (sửa, của SOL-013), `internal/adapter/eventbus/approval_decided_consumer.go` (mới), `task-service/internal/usecase/create_plan_tree.go` (sửa), `task-service/internal/usecase/ports.go` (sửa), `proto/orca/task/v1/task.proto` (sửa) và test
**Depends on:** TASK-REQ-027-02, 027-05, 027-06, TASK-REQ-007-05/-06, TASK-REQ-012-01/-05/-06, TASK-REQ-013-05
**Status:** [x] DONE

---

## Context

Task này **không sửa tài liệu CR hay solution khác**; nó chỉ sửa mã của các task đó **sau khi chúng được làm**, hoặc (nếu chúng chưa làm) là danh sách thay đổi bắt buộc cho task tương ứng. Khuyến nghị mạnh: thực hiện bước 1 và 2 **trước** TASK-REQ-007-02 và TASK-REQ-012-03 để khỏi phải migration dữ liệu (CR mục 6, SOL-027 C4).

Điểm nối thật đã đọc:
- BE-REQ-SOL-007 mục B: `SolutionOptions{SchemaVersion, Options []Option, Recommendation, Assumptions, OpenQuestions []string}`, `Validate(minOptions)`; mục E: `RunSolutionGeneration` gọi `AICompleter.Complete` (chỉ trả `content`), `SolutionApprovalHandler.ValidateForRequest` trả `DigestOptions(options, chosen)`.
- `agent/src/relay/ai-complete-handler.ts` trả `{content, model}`: `model` hiện bị SOL-007 bỏ đi; provenance cần nó.
- BE-REQ-SOL-012 mục 2.3, 2.4, 2.6, 2.7: `PlanProposal`, `ValidateProposal`, `CommitPlan` (ghi `plan_task_id` bằng CAS, outbox, `OpenApproval`, `TransitionRequest`), `CreatePlanTree` (`RunInTx`), `PlanSubjectHandler`.
- BE-REQ-SOL-013: `StartPhase`, `phase_starts`; BE-REQ-SOL-009: `OnApproved` trong transaction quyết định, outbox `orca.request.approval.decided`.
- `task-service` `TxRunner.RunInTx(fn(ctx, tasks TaskRepository, edges EdgeRepository))` (`ports.go` dòng 431): không có chỗ cho `TaskSpecRepository`.

## Việc cần làm

1. **Schema `options` (SOL-007, sửa `solution_options.go`):** thêm `RequirementCoverage []CoverageEntry{ACID string; OptionIDs []string; Status string; Note string}`, `Constraints []string`, `NonFunctional []NonFunctionalReq{Kind, Text string}`, `TestStrategy string`, `EvidenceRefs []string`;
   - đổi `OpenQuestions` thành `[]OpenQuestion{ID, Text string; Blocking bool}` (`Q-<n>`) và `Assumptions` thành `[]Assumption{ID, Text string; NeedsConfirmation bool}` (`A-<n>`)
   - `ParseSolutionOptions` nhận cả dạng cũ `string[]` (đọc vào, gán `ID` tự động) để không vỡ dữ liệu thử nghiệm.
   - `Validate` giữ luật cũ và thêm: `ID` duy nhất
   - `requirement_coverage.status` thuộc `covered|partial|out_of_scope`.
2. **Provenance (SOL-007):** đổi `AICompleter.Complete(ctx, ...) (content string, err error)` thành `(CompleteResult{Content, Model string}, error)`;
   - `RunSolutionGeneration` dựng `Provenance` (generator `native`, tool `ai.complete`, `model` từ phản hồi, `model_source=agent_response` hoặc `unknown` khi rỗng; prompt `solution_prompt`/`v1`/digest; `run_id`; `attempt`; `input_digest` bằng `ComputeInputDigest` từ snapshot Request ở `content_revision`, digest `prior_artifacts`, phiên bản prompt) và ghi cùng `seq` (`MintSolutionID`), `input_request_revision`, `content_digest` (`CanonicalJSON` của `options`) trong transaction lưu kết quả (SOL-007 bước thành công).
3. **Duyệt Solution:** `SolutionApprovalHandler.ValidateForRequest` gọi `ValidateArtifactSemantics` (chỉ luật `AC_UNCOVERED_BY_OPTION` cho Solution `kind=solution` đã `chosen_option`);
   - thiếu thì `REQUEST_ARTIFACT_AC_UNCOVERED_BY_OPTION`.
   - Repository `Solution.Update` chỉ ghi `options` khi `status='draft'` (`UPDATE ... WHERE status='draft'`)
   - sau `approved` chỉ đổi `status` thành `superseded` (CR bất biến 2.9).
   - Sửa nội dung = Solution mới (`seq` mới) + `artifact_relations(supersedes)`.
4. **`PlanProposal` (SOL-012):** thêm `Satisfies []string`, `Acceptance []string`, `Checks []CheckProposal{ID, Kind, Description, Command, Expect string}`, `ExemptFromCoverage bool` vào `TaskProposal`;
   - Plan thêm `Goal, Scope, Rollback, VerificationStrategy string; Risks []string; ImplementsOptionID string; SatisfiesAll bool`
   - Phase thêm `EntryCriteria, ExitCriteria []string`.
   - Thêm `ParseTaskSpec()`/`ToTaskSpecJSON()` sinh `spec_json` (schema `task` v1).
   - Tên trường khớp với `Satisfies` mà TASK-REQ-026-03 cần (thống nhất ở PR đầu tiên).
5. **`ValidateProposal`:** sau các luật của SOL-012 mục 2.5, gọi `ValidateArtifactSemantics` với `PlanSpecs` dựng từ đề xuất;
   - trả hết lỗi trong một `FailedPrecondition` kèm chi tiết (SOL-012 hiện trả mã đầu tiên; thống nhất kiểu `details`).
6. **`CreatePlanTree` (task-service):** `CreatePlanTreeRequest`, `PlanTreePhase`, `PlanTreeTask` thêm `string spec_json` (số trường kế tiếp theo file thật).
   - Vì `TxRunner.RunInTx` không có `TaskSpecRepository`, thêm **phương thức mới** `RunInTxWithSpecs(ctx, fn func(ctx context.Context, tasks TaskRepository, edges EdgeRepository, specs TaskSpecRepository) error) error` (hai adapter, dùng chung `Repository` đã ở trong giao dịch), giữ nguyên `RunInTx` để không đổi nơi gọi `AIApply`
   - `CreatePlanTree` dùng phương thức mới và gọi `specs.Upsert` cho mỗi nút có `spec_json` (kiểm schema bằng `NewTaskSpec`, lỗi thì rollback cả cây).
7. **`CommitPlan` (SOL-012):** trong transaction CAS `plan_task_id`: `MintPlanIDs` (kèm `Plan.seq` từ `NextPlanSeq`), `ReplaceRequestCoverage`, `artifact_relations` `derived_from` (Plan→Solution), `implements` (Plan→Option), `supersedes` (Plan→Plan cũ khi replan);
   - `ValidateProposal` đã chạy trước.
   - Task-service ghi spec trong `CreatePlanTree` (bước 6), sau đó `request-service` ghi bảng phủ: nếu một trong hai lỗi thì thử lại an toàn (`already_exists`, `ReplaceForPlan` thay nguyên khối).
8. **Khoá spec khi duyệt:** `approval_decided_consumer.go` (consumer bền của `orca.request.approval.decided`, khử trùng `processed_events`): khi `subject_type` thuộc `plan|task_list` và `decision=approved` gọi `TaskClient.LockTaskSpecs(plan_task_id)`;
   - khi `subject_type=phase` và `approved` gọi `LockTaskSpecs(phase_task_id)` cho cây Phase.
   - Lỗi gọi thì thử lại (backoff, tối đa 5 lần) và để nguyên (dedup chỉ đánh dấu sau khi thành công)
   - `LockTaskSpecs` idempotent.
   - Không khoá trong transaction của `OnApproved` vì đó là gọi mạng sang service khác.
9. `StartPhase` (SOL-013) **không** khoá thêm (đã khoá ở bước 8); ghi chú để tránh gọi trùng.

## Kiểm thử

- `TestSolutionOptions_ParseLegacyStringArrays`
- `TestSolutionOptions_StructuredQuestionsAndAssumptions_Validate`
- `TestSolutionOptions_DuplicateIDsRejected`.
- `TestRunSolutionGeneration_StoresProvenanceSeqRevisionDigest` (fake `AICompleter` trả `{Content, Model:"x"}`; kiểm `model_source=agent_response`; `Model` rỗng thì `unknown`)
- `TestProvenance_NoSecretFields_InStoredRow`.
- `TestSolutionHandler_Validate_AcUncoveredByOption`
- `TestSolutionRepository_OptionsImmutableAfterApproved`.
- `TestValidateProposal_CallsSemantic_ReturnsAllViolations`
- `TestPlanProposal_ToTaskSpecJSON_ValidatesAgainstSchema`.
- `TestCreatePlanTree_WritesSpecsInSameTx` (spec sai ở nút thứ 5 làm rollback cả cây, 0 task, 0 spec)
- `TestRunInTxWithSpecs_DoesNotAffectRunInTx`.
- `TestCommitPlan_ReplacesCoverageAndWritesRelationsSameTx`
- `TestCommitPlan_ReplanSupersedesOldPlanRelation`
- `TestCommitPlan_RetryAfterPartialFailure_Converges`.
- `TestApprovalDecidedConsumer_LocksOnPlanApproved_Idempotent`
- `TestApprovalDecidedConsumer_PhaseApproved_LocksPhaseSubtree`
- `TestApprovalDecidedConsumer_RejectedDoesNotLock`
- `TestApprovalDecidedConsumer_RetryOnTaskServiceError`.
- Integration hai dialect cho `CommitPlan` + `CreatePlanTree` + bảng phủ với fake `TaskClient` hoặc `task-service` thật trong docker compose (nếu có).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'SolutionOptions|Provenance|SolutionHandler|ValidateProposal|CommitPlan|ApprovalDecidedConsumer' && go test ./services/task-service/... -run 'CreatePlanTree|RunInTxWithSpecs' && go test -tags=integration ./services/request-service/... ./services/task-service/... -run 'CommitPlan|CreatePlanTree'`.

## Tiêu chí hoàn thành

- [x] Solution lưu `provenance`, `seq`, `input_request_revision`, `content_digest`; `provenance` không chứa `credential_ref`, khoá hay `env`.
- [x] `Approve` Solution bị `AC_UNCOVERED_BY_OPTION` khi phương án chọn chưa trả lời mọi AC `active`.
- [x] Solution `approved` không sửa `options` được.
- [x] Commit Plan thiếu `satisfies` hoặc `checks` bị `TASK_NO_AC` hoặc `TASK_NO_CHECK`; `request_coverage` thay nguyên khối cùng transaction.
- [x] Sau `approval.decided` của Plan, `SetTaskSpec` trả `TASK_SPEC_LOCKED` còn `UpdateTask` đổi `status` vẫn thành công.
- [x] `RunInTx` cũ không đổi chữ ký; test `AIApply` không đổi.

## Rủi ro và lưu ý

- Task này chạm năm solution; xung đột merge cao. Làm thành các PR nhỏ theo bước (1 và 2, 3, 4 và 5, 6, 7 và 8) và báo người giữ từng solution.
- Khoá spec bằng consumer có độ trễ: giữa `approved` và khoá có khoảng `SetTaskSpec` vẫn ghi được; chấp nhận (Plan đã duyệt, sửa trong khoảng đó là hiếm) nhưng ghi ở README. Phương án khác (khoá đồng bộ trong `OnApproved`) làm một lỗi mạng chặn cả quyết định duyệt, nên không chọn.
- `ai.complete` có thể không trả `model` (agent cũ); `model_source=unknown` là hợp lệ.
- Thay `Complete` thành trả `CompleteResult` đổi cổng dùng bởi SOL-008 (agent_readonly) và SOL-026 (`nativeEngine`): cập nhật cả hai.
