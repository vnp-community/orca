# TASK-REQ-030-06: `RiskGate`, `AcceptRisk`, `OverrideRiskGate`, drift Approval và `PlanPreconditions` theo mức rủi ro

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.F, 2.G
**Priority:** P1
**Service/Area:** `request-service` (mới) / usecase, tích hợp với Approval (SOL-009) và `AdvanceExecution` (SOL-013)
**File:** `internal/usecase/risk_gate.go` (mới), `internal/usecase/accept_risk.go` (mới), `internal/usecase/override_risk_gate.go` (mới), `internal/usecase/drift_review_handler.go` (mới), `internal/usecase/plan_risk_preconditions.go` (mới), `internal/domain/risk_gate_rules.go` (mới), `internal/usecase/approval_guard_ports.go` (mới), `internal/domain/outbox_subjects.go` (sửa), và các `_test.go`
**Depends on:** TASK-REQ-030-05 (bản đánh giá `ready`), TASK-REQ-009-04 (`DecideApproval`, `SubjectHandler`, `UpdatePendingDigest`), TASK-REQ-010-03 (`ApproverPolicy`, `team:<id>`), TASK-REQ-013-05 (`AdvanceExecution`), TASK-REQ-012-03 (`plan_labels.go`), TASK-REQ-014-04 (`pre_deploy`), CR-REQ-024 (`AppendDetailed`)
**Status:** [ ] TODO

---

## Context

Đã đọc ngày 2026-10-06:
- `BE-REQ-SOL-009` mục D, F: `ApprovalRepository.UpdatePendingDigest(ctx, tx, tenantID, subjectType, subjectID, digest) (bool, error)` (chỉ ghi khi còn `pending`); `SubjectHandler` có `ValidateForRequest`, `OnApproved`, `OnRejected`, `OnClosedWithoutDecision`; `DecideApproval.Approve` làm tuần tự: khoá Request, `GetForUpdate`, kiểm hạn, `authorizer.CanDecide`, `expected_digest == subject_digest` (`REQUEST_APPROVAL_DIGEST_MISMATCH`), kiểm `stage`, `UpdateDecision`, `handler.OnApproved`. **Không có** cổng ghép `ApprovalGuard`, `viewed_impact_digest`, `accepted_finding_ids` (SOL-030 mục 1, điều 3): phải bổ sung bên SOL-009/TASK-REQ-009-04 và 009-05 (proto `DecideApprovalRequest`); task này định nghĩa giao diện mà SOL-009 cần gọi và dùng chúng.
- `SubjectType` của SOL-009 gồm `request_type`, `solution`, `findings`, `answer`, `plan`, `task_list`, `phase`, `pre_deploy` (theo chủ sở hữu handler đã nêu); `stage=drift_review` hợp lệ cho `subject_type=phase` (CR-REQ-030 mục 9): Approval `drift_review` dùng cùng `phase` nên cần `FindPendingBySubject` phân biệt theo `stage` hoặc `subject_id` riêng (`<phase_task_id>:drift`): chốt khi làm với SOL-009 (chỉ mục "một `pending` mỗi chủ thể" có thể va chạm với Approval `phase` đang `pending`).
- Mức rủi ro và hệ quả (CR 2.6): Thấp bình thường; Trung bình cần `viewed_impact_digest == assessment.digest` và không duyệt hàng loạt; Cao: người duyệt thuộc `team:<risk_approver_team>`, mỗi phát hiện từ Cao trở lên có `RiskAcceptance`, Plan có `gate:feature_flag` và task `rollback`, cổng `pre_deploy` cho mọi loại; Nghiêm trọng: mọi điều của Cao, Plan chia Phase để mỗi Phase tối đa Cao (`REQUEST_PLAN_RISK_TOO_HIGH` ở `PlanPreconditions`), mỗi Phase có Approval `phase` riêng, hai người duyệt (v1: giữ chỗ bằng cờ `REQUEST_RISK_REQUIRE_TWO_APPROVERS`), task `check:rollback_rehearsal` trước Phase đầu. Chỉ `mode=enforce` mới chặn (và chỉ từ mức `GateMapping.EnforceFromLevel`).
- Quyền: README v6 mục 8 điều 10, 13: vai trò chỉ có `admin|user` và team; người duyệt đặc biệt dựng bằng `team:<id>`; mọi RPC của `request-service` tự kiểm quyền.
- Audit: `common/auditclient.Append` không có `actor_type`/`target_type`; `AppendDetailed` do CR-REQ-024 (TASK-REQ-024-01). Nếu chưa có, ghi qua `request_audit_outbox`/bản tạm và đánh dấu.
- `risk_outcomes.override_count` do task 03.

## Việc cần làm

1. `approval_guard_ports.go`: `ApprovalGuard` và `GuardInput{Approval domain.Approval; DeciderUserID string; DeciderTeams []string; ViewedImpactDigest string; AcceptedFindingIDs []string; Now time.Time}` đúng như solution 2.F:
   - `ApprovalGuardChain` chạy lần lượt, trả lỗi đầu tiên. **Giao việc cho SOL-009:** `DecideApproval.Approve` gọi chuỗi này sau `CanDecide`, trước `OnApproved`
   - mô tả điểm móc này vào mô tả PR của task 009-04 (không sửa file solution khác).
2. `domain/risk_gate_rules.go`: hàm thuần `EvaluateRiskGate(in RiskGateInput) RiskGateDecision` với `RiskGateInput{Mode; Level; EnforceFromLevel; Status AssessmentStatus; AssessmentDigest, ViewedDigest string; Findings []Finding; ValidAcceptances []RiskAcceptance; DeciderTeams []string; RiskApproverTeam string; PlanHasFeatureFlagLabel, PlanHasRollbackTask bool; RequireTwoApprovers bool; PriorDeciders []string}` và `RiskGateDecision{Allow bool; Code string; MissingFindingIDs []string}`:
   - thứ tự kiểm: `shadow` hoặc dưới `EnforceFromLevel` thì cho qua
   - `collecting` thì `REQUEST_RISK_ASSESSMENT_PENDING`
   - Trung bình: `ViewedDigest` khác `AssessmentDigest` thì chặn (`REQUEST_RISK_ASSESSMENT_STALE` khi digest cũ, `REQUEST_RISK_VIEW_REQUIRED` khi chưa mở)
   - Cao trở lên: người duyệt ngoài `team:<RiskApproverTeam>` thì `REQUEST_RISK_APPROVER_NOT_ALLOWED`, phát hiện từ Cao trở lên thiếu `RiskAcceptance` hợp lệ thì `REQUEST_RISK_ACCEPTANCE_REQUIRED` (kèm `MissingFindingIDs`), thiếu nhãn/task thì `REQUEST_RISK_MITIGATION_MISSING`
   - `RequireTwoApprovers` và Nghiêm trọng thì cần hai người khác nhau (`REQUEST_RISK_SECOND_APPROVER_REQUIRED`), mặc định tắt.
3. `risk_gate.go`: `RiskGate` cài `ApprovalGuard` và `ExecutionGuard`:
   a. `Check(ctx, GuardInput)`: lấy bản đánh giá hiện hành của chủ thể (`solution` thì bản `solution_option` của Option đã chọn; `plan`/`task_list` thì bản `plan`; `phase` thì bản `phase`), `ListValidAcceptances(currentDigest)`, `ApproverPolicy` để biết `team:<id>`, gọi `EvaluateRiskGate`;
   b. `BlockExecution(ctx, requestID, taskID) (blocked bool, reason string, err error)`: `AdvanceExecution` (SOL-013 bước 3) gọi cạnh `PreExecutionGate`: chặn khi `enforce` và (Phase chưa có Approval `phase` đã duyệt ở mức Cao trở lên, hoặc có drift chưa giải quyết, tức Approval `stage=drift_review` còn `pending`).
4. `OpenApproval` và `UpdatePendingDigest`: khi bản đánh giá hoàn tất sau khi Approval đã mở, gọi `UpdatePendingDigest` với digest chủ thể mới (digest chủ thể `solution`/`plan` gồm `assessment.digest`; hàm dựng digest chủ thể do SubjectHandler của SOL-007/012 sở hữu và cần nhận `assessmentDigest` tuỳ chọn: chốt điểm này với hai solution đó). `Approve` với `expected_digest` cũ bị từ chối như thường (`REQUEST_APPROVAL_DIGEST_MISMATCH`).
5. `accept_risk.go`: `AcceptRisk.Execute(ctx, in AcceptRiskInput{AssessmentID, FindingID, Rationale, AssessmentDigest, ActorID string}) (RiskAcceptance, error)`: `RequireTenantID`:
   - quyền (người có quyền duyệt Approval của chủ thể hoặc admin; kiểm bằng `authorizer`)
   - bản đánh giá phải `ready|partial`, `assessment_digest` trùng bản hiện hành (`REQUEST_RISK_ASSESSMENT_STALE`)
   - finding tồn tại và mức ≥ Cao (mức thấp hơn thì `REQUEST_RISK_FINDING_NOT_ACCEPTABLE`)
   - `NewRiskAcceptance` (rationale ≥ 10)
   - ghi + outbox `orca.request.risk.accepted {assessment_id, finding_id, accepted_by}` (không có `rationale`) trong cùng giao dịch
   - trùng UNIQUE thì trả bản cũ (idempotent).
6. `override_risk_gate.go`: `OverrideRiskGate.Execute(ctx, in{RequestID, Gate, Reason, FindingID string})`: người thuộc `team:<RiskPolicy.gate_mapping.risk_override_team>` hoặc admin (`REQUEST_RISK_APPROVER_NOT_ALLOWED` khác):
   - `reason` ≥ 20 ký tự (đếm rune; `REQUEST_RISK_OVERRIDE_REASON_REQUIRED`)
   - `Gate` thuộc `solution|plan|phase|execute`
   - ghi audit (`AppendDetailed`: `actor_type=user`, `target_type=assessment`, `action=risk.override`, chi tiết có `gate`, `finding_id`, **không** có `reason` nguyên văn nếu CR-REQ-035 cấm; theo quy tắc audit đã chốt), `IncrementOverrideCount`
   - ghi một bản ghi override có hiệu lực cho `(request_id, gate)` (bảng nhỏ hoặc cột JSON trên `risk_outcomes`: chốt khi làm; cần thêm vào migration task 01 nếu chọn bảng) để `RiskGate` bỏ qua đúng cổng đó
   - override **không** bỏ yêu cầu "xác nhận từng phát hiện Cao" trừ khi `FindingID` được nêu.
7. `drift_review_handler.go`: `SubjectHandler` cho `phase` với `stage=drift_review`: `ValidateForRequest` kiểm có bản `actual_*` mang `Drift`:
   - `OnApproved` ghi bản đánh giá `actual` mới làm baseline (đánh dấu bản dự kiến `superseded`) và gọi lại `AdvanceExecution`
   - `OnRejected` thì `ReturnToBacklog(stage=phase, category=rejected)`
   - `OnClosedWithoutDecision` (hết hạn) thì như bị từ chối. Khi `drift` được phát hiện (task 05) và `mode=enforce`: `OpenApproval(subject_type=phase, stage=drift_review)`
   - `shadow`: không mở.
8. `plan_risk_preconditions.go`: `PlanRiskPreconditions.Check(ctx, planTaskID) error` cho `PlanPreconditions` của SOL-012/014: Nghiêm trọng ở Phase nào thì `REQUEST_PLAN_RISK_TOO_HIGH` (liệt kê Phase vượt mức); Cao trở lên thì kiểm nhãn `gate:feature_flag` và task `rollback` (CR-REQ-014 mục 2.5), Nghiêm trọng thì thêm `check:rollback_rehearsal` đứng trước Phase đầu. Chỉ `enforce`. Nhãn mới cần có trong `plan_labels.go` (TASK-REQ-012-03): nếu chưa có, **không** tự thêm ở đây, ghi phụ thuộc và để kiểm báo `REQUEST_RISK_MITIGATION_MISSING` kèm hướng dẫn.
9. Cờ cấm Noop: nếu `REQUEST_IMPACT_ENABLED=true` và `ApprovalGuardChain` rỗng thì `main.go` thoát lỗi (tránh bật chặn mà không móc vào `Approve`).

## Kiểm thử

- `TestEvaluateRiskGate_Table`: shadow cho qua mọi mức:
     - dưới `EnforceFromLevel` cho qua
     - `collecting` chặn
     - Trung bình thiếu `ViewedDigest`/digest cũ
     - Cao thiếu người duyệt trong team
     - Cao thiếu `RiskAcceptance` (kèm danh sách `MissingFindingIDs`)
     - acceptance cũ digest không còn hiệu lực
     - Cao thiếu nhãn/task giảm thiểu
     - Nghiêm trọng hai người duyệt (cờ bật/tắt).
- `TestRiskGate_AsApprovalGuard_BlocksApproveWhenAcceptanceMissing` (ghép với `DecideApproval` thật của SOL-009 qua fake repository), `_DigestChangeInvalidatesAcceptance`, `_PendingAssessmentBlocks`, `_UpdatePendingDigestAfterAssessment`.
- `TestAcceptRisk_StaleDigest`, `_RationaleTooShort`, `_FindingBelowHigh`, `_Idempotent`, `_EventHasNoRationale`, `_Unauthorized`.
- `TestOverride_ReasonTooShort`, `_NotInTeam`, `_BypassesOnlyNamedGate`, `_DoesNotWaiveFindingAcceptanceUnlessNamed`, `_IncrementsOverrideCountAndAudits`.
- `TestAdvance_RiskGateBlocksWhenDriftPending`, `_AllowsAfterDriftApproved`, `_ShadowNeverBlocks`.
- `TestDriftReview_OnApproved_SetsBaselineAndAdvances`, `_OnRejected_ReturnsToBacklogPhaseRejected`, `_Expired_AsRejected`.
- `TestPlanRiskPreconditions_CriticalPhaseRejected`, `_HighRequiresFlagAndRollback`, `_ShadowSkips`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... -run "RiskGate|AcceptRisk|Override|Drift|PlanRisk"`; tích hợp `-tags=integration` cho ghép với Approval.

## Tiêu chí hoàn thành

- [ ] `enforce`: `Approve` thiếu `RiskAcceptance` cho phát hiện Cao bị `REQUEST_RISK_ACCEPTANCE_REQUIRED`; đánh giá đổi digest làm chấp nhận cũ mất hiệu lực; `collecting` thì `REQUEST_RISK_ASSESSMENT_PENDING`; người ngoài team thì `REQUEST_RISK_APPROVER_NOT_ALLOWED`.
- [ ] `shadow`: không chặn gì, không đòi chấp nhận; nhãn "tham khảo" qua `mode` trong dữ liệu trả về.
- [ ] Plan có Phase Nghiêm trọng bị `REQUEST_PLAN_RISK_TOO_HIGH`.
- [ ] Lệch vượt `REQUEST_RISK_DRIFT_DELTA` mở Approval `stage=drift_review` và chặn `AdvanceExecution`; `OnRejected` về backlog `phase`.
- [ ] Override ghi audit và `override_count`; không bỏ luật xác nhận từng phát hiện Cao nếu không nêu `finding_id`.
- [ ] Sự kiện `risk.accepted` không chứa `rationale`.
- [ ] `gitnexus_impact` đã chạy cho `DecideApproval`, `AdvanceExecution`, `PlanPreconditions` (thuộc solution khác) và cảnh báo rủi ro nếu HIGH/CRITICAL.

## Rủi ro và lưu ý

- Phụ thuộc cứng vào SOL-009 bổ sung `ApprovalGuard`, `viewed_impact_digest`, `accepted_finding_ids`; nếu không, chặn theo mức rủi ro không thi hành được (chỉ hiển thị). Đây là điểm cần điều phối viên chốt.
- Approval `stage=drift_review` cho `phase` có thể va chạm với chỉ mục "một `pending` mỗi chủ thể": cần quyết định ở SOL-009 (subject_id riêng hoặc chỉ mục có `stage`).
- Chặn `AdvanceExecution` không dừng task đang chạy (không có RPC dừng run, CR-REQ-013 mục 6): drift chỉ chặn task kế.
- `risk_approver_team`/`risk_override_team` lấy từ `RiskPolicy.gate_mapping` (Q2): admin chưa cấu hình thì mọi Approval mức Cao bị chặn; cần thông điệp lỗi chỉ rõ thiếu cấu hình (`REQUEST_RISK_POLICY_INVALID`), không im lặng cho qua.
- Hai người duyệt (Q1) chưa chốt; cờ chỉ giữ chỗ.
