# TASK-REQ-030-05: `ImpactCollector` (công cụ qua `AgentRelay`), worker lease, consumer kích hoạt và `AssessActual`

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.D, 2.E, 2.G
**Priority:** P1
**Service/Area:** `request-service` (mới) / usecase, adapter eventbus, adapter grpcclient
**File:** `internal/usecase/request_impact_assessment.go` (mới), `internal/usecase/collect_impact.go` (mới), `internal/usecase/collect_impact_steps.go` (mới), `internal/usecase/assess_actual_impact.go` (mới), `internal/usecase/recover_impact_assessments.go` (mới), `internal/usecase/record_risk_outcome.go` (mới), `internal/adapter/grpcclient/impact_tool_runner.go` (mới), `internal/adapter/eventbus/impact_trigger_consumer.go` (mới), `internal/config/config.go` (sửa), `cmd/server/main.go` (sửa), và các `_test.go`
**Depends on:** TASK-REQ-030-02, 030-03, 030-04; TASK-REQ-029-06 (`AgentRelay`, `WorktreeResolver`), TASK-REQ-029-08 (sự kiện `execution.verified`), TASK-REQ-033-04 (`DevServerCapabilityReader`), SOL-007 (sự kiện `solution.proposed`, mẫu lease), SOL-012 (`plan.generated`), SOL-013 (`phase.completed`, `processed_events` consumer)
**Status:** [ ] TODO

---

## Context

- Mẫu bền: `analysis_runs` ở SOL-007 (lease 90 giây, gia hạn 30 giây/lần, `RecoverInterruptedAnalysisRuns.RunRecoveryLoop(ctx, 30s)`); consumer bền ở SOL-013 mục 2.5 (`Subscribe(ctx, stream, consumerName, subject, fn)` của `common/eventbus/eventbus.go:128`, khử trùng bằng `processed_events(tenant_id, event_id)`, đặt tenant từ `Event.TenantID`).
- Sự kiện nguồn: `orca.request.solution.proposed {tenant_id, request_id, solution_id, kind, option_count, recommended_option_id}` (TASK-REQ-007-05); `orca.request.plan.generated {request_id, plan_task_id, phase_task_ids, task_ids, type, size, task_count}` (SOL-012); `orca.request.execution.verified {request_id, task_id, attempt, status, failure_class, finding_codes[]}` (TASK-REQ-029-08); `orca.request.phase.completed` (SOL-013). Payload không chứa `options`, diff; cần đọc thêm bằng RPC/repository của chính `request-service` (Solution, TaskSpec qua `task-service.GetTaskSpecs`).
- Công cụ chạy qua `AgentRelay.RunCommand{Binary, Args, Cwd, Env, TimeoutMS}` (không shell, tối đa 300 giây). Lệnh: `git rev-list --count <lastCommit>..<base_sha>`, `node .gitnexus/run.cjs impact <symbol> --direction upstream --depth 3 --summary-only`, `node .gitnexus/run.cjs detect-changes --scope compare --base-ref <base_sha>`, `buf breaking --against '<repo>/.git#ref=<base_sha>,subdir=backend-go/proto' --error-format=json` (cwd `backend-go/proto`, **không** `|| true`), `git diff --name-only -z <base_sha>`, `go test -cover ./<pkg>/...` (tối đa 5 package; dòng `coverage: X% of statements`).
- Điểm quan trọng (SOL-030 mục 1, điều 1): `plan|phase|task` chưa có diff, nên chỉ chạy tín hiệu `basis=path` và `go test -cover` trên package hiện có; công cụ thật chạy ở `actual_*`.
- Tuổi index: đọc `.gitnexus/gitnexus.json` bằng `ReadFile` (`lastCommit`, `indexedAt`); không tự `analyze` (cờ `REQUEST_IMPACT_REINDEX` mặc định `false` dành cho sau này).
- Nguồn `base_sha` và `head`: `task_readiness_reports.base_sha` của lần thử (SOL-029), `HEAD` bằng `git rev-parse HEAD` trong worktree. `actual_phase` lấy `base_sha` của task đầu Phase tới `HEAD` cuối Phase.
- `go.work` không liệt kê `services/request-service` ở thời điểm viết; kiểm module trước khi chạy `go test`.

## Việc cần làm

1. Config: `ImpactEnabled bool` (`REQUEST_IMPACT_ENABLED`, false), `ImpactBudget time.Duration` (`REQUEST_IMPACT_BUDGET`, 10m, hợp lệ `[1m,30m]`), `ImpactIndexMaxCommitsBehind int` (`REQUEST_IMPACT_INDEX_MAX_COMMITS_BEHIND`, 30), `ImpactMaxSymbols int` (`REQUEST_IMPACT_MAX_SYMBOLS`, 20, trần 50), `RiskDriftDelta int` (`REQUEST_RISK_DRIFT_DELTA`, 15). Giá trị sai thì khởi động lỗi rõ.
2. `RequestImpactAssessment.Execute(ctx, in RequestImpactInput{RequestID, SubjectType, SubjectID string; Force bool; Actor string}) (ImpactAssessment, error)`: kiểm cờ (tắt thì `REQUEST_IMPACT_DISABLED`, `FailedPrecondition`):
   - tính `subject_digest` bằng `SubjectDigester` theo loại (Option: digest Option của SOL-007; Plan/Phase/Task: digest cây theo SOL-012; actual: `base_sha..head`)
   - có bản `ready|partial` cùng `subject_digest` và không `Force` thì trả
   - ngược lại `InsertCollecting(leaseTTL=90s)`
   - `mode` từ `RiskPolicyRepository` (`active` thì `enforce`, `shadow` hoặc không có thì `shadow`)
   - phát tín hiệu cho worker (kênh nội bộ hoặc đơn giản để vòng quét nhặt, không gọi trực tiếp công cụ trong luồng RPC).
3. `CollectImpact.Run(ctx, assessmentID)`: gia hạn lease 30 giây/lần trong goroutine kèm `defer` dừng:
   - chạy các bước theo bảng solution 2.D bằng hàm trong `collect_impact_steps.go` (mỗi bước nhận `StepInput` và trả `StepOutput{Signals, Findings, ToolRun}`): `stepCapabilities`, `stepIndexAge`, `stepGitNexus`, `stepBufBreaking`, `stepMigrationScan`, `stepContractScan`, `stepCoverage`, `stepBaselineChecks`, `stepMaxLines`, `stepDeclared` (từ `affected_areas` qua `AreaResolver`, `open_questions`)
   - mỗi bước ghi một `impact_tool_runs` (`output` cắt 256 KB, `output_digest`), lỗi/timeout thì `status=error|timeout`, bước đó không làm dừng các bước khác
   - tổng thời gian ≤ `ImpactBudget` (hết thì các bước còn lại `skipped`). Tái dùng kết quả: trước khi chạy một lệnh, `FindRecentToolRun(command_digest, now-30m)` cùng `index_commit`/`base_sha` thì dùng lại và ghi `status=ok` kèm ghi chú `reused`.
4. Dựng `[9]DimensionResult` từ các tín hiệu (bảng ánh xạ `Signal.Code` sang chiều ở một tệp dữ liệu `impact_signal_dimensions.go`), `Basis` mỗi chiều theo nguồn tín hiệu mạnh nhất có mặt, `Measured=false` khi chiều không có tín hiệu `Measured` nào:
   - `HardRuleContext` từ các tín hiệu
   - `domain.Score(...)`
   - `status`: `ready` nếu mọi bước thiết yếu `ok`, `partial` nếu có bước `error|timeout` (không phải `skipped` do thiết kế)
   - ghi `CompleteImpactAssessment`
   - outbox `orca.request.impact.assessed` cùng giao dịch.
5. `RecoverInterruptedImpactAssessments.RunRecoveryLoop(ctx, 30s)`: `ClaimExpiredImpactAssessments(batch)`, đặt `failed` với `finding INTERRUPTED`; không chạy lại tự động (người gọi có thể `Force`).
6. `impact_tool_runner.go`: cài `ImpactToolRunner` bằng `AgentRelay`; kiểm `Binary` thuộc danh sách cho phép (`git`, `node`, `buf`, `go`), `Args` tách, **không** nhận chuỗi shell; `Cwd` phải nằm trong đường dẫn repo/worktree đã giải; giới hạn `TimeoutMS ≤ 300000`; không log `Stdout`.
7. `impact_trigger_consumer.go`: bốn consumer bền (`request-service-impact` trên stream `REQUEST`, bốn subject), mỗi cái khử trùng bằng `processed_events`, bỏ qua khi cờ tắt, gọi `RequestImpactAssessment` với `Actor="system"`; với `plan.generated` tạo bản `plan` rồi các bản `phase`/`task` dẫn xuất (một lần thu thập chung; các bản dẫn xuất dùng cùng `impact_tool_runs` qua `assessment_id` gốc: lưu `parent_assessment_id` trong `triggers`? Không: dùng một bản `plan` thu thập đầy đủ, rồi `DeriveSubset(planAssessment, taskIDs)` thuần tính lại điểm cho từng Phase/Task từ cùng `Signals` đã lọc theo đường dẫn của chúng, ghi thành các bản `ready` riêng không có `impact_tool_runs` riêng).
8. `AssessActual.Execute(ctx, in{RequestID, SubjectType (actual_task|actual_phase), SubjectID, BaseSHA, HeadSHA})`: tạo bản `actual_*`, thu thập bằng công cụ thật:
   - sau `ready` tìm bản dự kiến cùng chủ thể gốc (`task`/`phase`) và gọi `domain.CompareActual`
   - `Drift` thì outbox `orca.request.impact.drift_detected` (hành động chặn do task 06)
   - ghi `actual_assessment_id` vào `task_run_outcomes` (cột `actual_assessment_id` NULL do migration của TASK-REQ-029-04/013; nếu chưa có thì thêm vào migration `impact_risk` ở task 01 hoặc migration riêng; chốt khi làm và ghi PR).
9. `RecordRiskOutcome.Execute(ctx, requestID)`: gọi khi Request `completed` hoặc về backlog (từ consumer `status_changed`): ghi `risk_outcomes.predicted_level` (mức cao nhất của bản `plan`), `actual_level` (cao nhất của các `actual_*`).
10. Wiring `main.go`: dựng use case, worker lease, hai vòng (quét lease, consumer); tắt hết khi cờ tắt (vẫn đăng ký RPC trả `REQUEST_IMPACT_DISABLED`).

## Kiểm thử

- Với fake `AgentRelay`/`ImpactToolRunner`: `TestCollect_AllToolsOk_Ready`, `_BufBreakingTimeout_Partial_DimensionUnmeasured`, `_GitNexusParseError_StatusError`, `_IndexStale_RaisesLevelAndLowConfidence`, `_IndexMissing`, `_BudgetExhausted_RemainingSkipped`, `_ReusesRecentToolRun`, `_NoShellStringEver` (relay giả chỉ nhận danh sách lệnh hợp lệ), `_OutputTruncatedTo256KB`, `_PlanSubject_OnlyPathBasisAndCoverage`, `_SolutionOption_ConfidenceAtMostMedium`, `_AreaUnresolvedIncreasesUncertainty`.
- `TestRequestImpact_ReturnsExistingReady`, `_ReturnsCollecting`, `_ForceCreatesNewRevision`, `_DisabledFlag`, `_ModeFromPolicy` (không có policy thì `shadow`).
- Lease: `TestWorker_RenewsLease`, `TestRecover_ExpiredBecomesFailedInterrupted`, `_SkipLocked`.
- Consumer: `TestTrigger_SolutionProposed_AssessesEachOption`, `_PlanGenerated_PlanThenDerivedPhaseTask`, `_ExecutionVerified_Passed_AssessesActualTask`, `_DuplicateEventNoDuplicateAssessment`, `_FlagOff_Ignored`.
- `TestAssessActual_DriftDetected_EmitsEvent`, `_NoDrift_NoEvent`, `_StoresActualAssessmentID`.
- `TestRecordRiskOutcome_PredictedVsActual`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/eventbus/... ./services/request-service/internal/adapter/grpcclient/... -run "Impact|Collect|Actual|Recover|Trigger"`; tích hợp: `-tags=integration`.

## Tiêu chí hoàn thành

- [ ] Công cụ lỗi hoặc hết thời gian cho `status=partial`, chiều tương ứng `Measured=false`, `confidence` hạ; không bản `ready` nào im lặng bỏ chiều.
- [ ] Index lỗi thời được phát hiện từ `gitnexus.json` và tăng một bậc kèm nhãn "chưa đánh giá được".
- [ ] `solution_option` chạy sau `solution.proposed`; `plan|phase|task` sau `plan.generated`; `actual_*` sau `execution.verified`/`phase.completed`; mỗi bản có `impact_tool_runs` làm bằng chứng (trừ bản dẫn xuất).
- [ ] Sự kiện giao lặp không tạo hai bản đánh giá (`processed_events` + một `collecting` mỗi chủ thể).
- [ ] `buf breaking` chạy trực tiếp, không `|| true`; thử thật với một thay đổi xoá trường proto mẫu phát hiện vi phạm (kiểm thủ công và test fake).
- [ ] Không log `Stdout`/diff; không lệnh nào nối chuỗi shell từ dữ liệu người dùng.
- [ ] `gitnexus_impact` đã chạy cho các hàm của SOL-013/029 mà task này gọi trước khi sửa.

## Rủi ro và lưu ý

- Thời gian: 20 symbol × `impact` trên repo 247 nghìn symbol có thể vượt 10 phút; chưa đo. `--summary-only` và bộ nhớ đệm giảm tải; `REQUEST_IMPACT_MAX_SYMBOLS` điều chỉnh.
- Worktree `task/<id>` có dùng chung index GitNexus của repo gốc hay không chưa kiểm chứng; nếu không, `impact` trong worktree báo "không có index" và chiều Phạm vi ảnh hưởng `Measured=false`.
- `buf breaking --against '<.git>#ref=<sha>'` cần `ref` có trong clone của dev server; clone nông hỏng; chưa thử SSH.
- `DeriveSubset` (bước 7) là chỗ dễ sai: điểm Phase/Task suy từ tín hiệu lọc theo đường dẫn, không phải đo riêng; ghi `basis=path` và không nâng `confidence` quá `medium`.
- `go test -cover` chạy trên checkout nào ở thời điểm Plan (có thay đổi chưa commit)? câu hỏi mở Q6 của solution; tạm dùng `repo_path` và ghi `index_commit`/`head_sha`.
