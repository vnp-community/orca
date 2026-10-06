# TASK-REQ-030-08: `RiskPolicy` admin, chế độ `shadow` và `enforce`, công cụ hiệu chỉnh ngoại tuyến, wiring, chỉ số và e2e

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.G, 2.H, 3 (shadow trước enforce)
**Priority:** P1
**Service/Area:** `request-service` (mới) / usecase, cmd công cụ ngoại tuyến, config, composition root, test e2e
**File:** `internal/usecase/risk_policy_admin.go` (mới), `internal/usecase/enforce_readiness.go` (mới), `internal/usecase/risk_calibration_stats.go` (mới), `internal/adapter/grpc/server_risk_policy.go` (mới), `cmd/impact-calibrate/main.go` (mới), `cmd/impact-calibrate/runner.go` (mới), `testdata/calibration/cases.json` (mới), `internal/adapter/metrics/impact_metrics.go` (mới), `internal/config/config.go` (sửa), `cmd/server/main.go` (sửa), `internal/e2e/impact_risk_e2e_test.go` (mới), `backend-go/policy/orca-authz/request.rego` (sửa, thuộc CR-REQ-035: chỉ đề nghị), và các `_test.go`
**Depends on:** TASK-REQ-030-02 đến 07; TASK-REQ-024-01 (`AppendDetailed`), TASK-REQ-024-07 (khung chỉ số của `request-service`), TASK-REQ-025-03 (khung e2e), CR-REQ-035 (`request.rego`)
**Status:** [ ] TODO

---

## Context

- CR-REQ-030 mục 2.9: ba giai đoạn. **0. Thử ngoại tuyến** trước khi viết code sản xuất: chạy bộ thu thập bằng CLI trên máy dev cho 3 đến 5 thay đổi đã có trong lịch sử, so mức tính ra với mức thật; qua khi khớp mức hoặc lệch tối đa một bậc ở ≥ 4/5 trường hợp, nếu không chỉnh bảng 2.3 trước. **1. `shadow`** mặc định mọi tenant: đánh giá hiển thị nhãn "tham khảo", không chặn, không đòi `RiskAcceptance`, `risk_outcomes` được điền. **2. `enforce`**: admin tenant bật (`SetRiskPolicy` đặt `active`) khi đạt: ≥ 30 đánh giá có `risk_outcomes`; tỉ lệ khớp mức dự kiến với mức thực tế ≥ 70%; tỉ lệ `partial` hoặc `confidence=low` ≤ 30%; không có phát hiện sai loại "Nghiêm trọng" cho thay đổi đã hoàn tất không sự cố ở > 10% trường hợp (ngưỡng đề xuất, chưa kiểm chứng). Có thể đặt `enforce` theo mức: chỉ chặn từ Cao.
- Ca lịch sử có sẵn: `docs/crs/v3/project-workspace/IMPACT-ASSESSMENT-2026-09-15-worktree-session-jira.md` (đã đọc): CR-PW-007 BUG-010 "gitnexus impact: 3 impacted" mức LOW; CR-PW-010 `ConnectionResolver`/`dispatchExecutor`: "34 direct caller", **CRITICAL**, 1 chỗ sửa ở `resolver.go`; CR-PW-008(b) BUG-012 một dòng thêm vào `git-gateway-service/main.go` (additive); CR-PW-009 chỉ thêm UI; CR-JIRA-001 toàn bộ adapter Jira. Chú ý: "mức" trong tài liệu đó là rủi ro của `gitnexus impact` (LOW/CRITICAL), không phải mức tổng bốn bậc của `rp/1`; ghi `expected_level` cho từng ca kèm lý do và nguồn (người duyệt chốt, không suy đoán).
- Điều kiện bật `enforce` cần dữ liệu: `risk_outcomes` (task 03/05); "khớp mức" = `predicted_level == actual_level` hoặc chênh một bậc? CR nói "khớp mức dự kiến với mức thực tế ≥ 70%": diễn giải chặt là khớp đúng; ghi lựa chọn vào PR (đề xuất: đúng bậc ≥ 70% **và** chênh ≤ 1 bậc ≥ 95%).
- Quyền: `GetRiskPolicy`, `SetRiskPolicy` chỉ admin tenant (README v6 mục 8: vai trò `admin|user`); mọi RPC tự kiểm quyền; `request.rego` thêm `risk.policy.*` (CR-REQ-035).
- Sự kiện `orca.request.risk_policy.changed` (CR 2.10); audit qua `AppendDetailed` (`target_type=policy`).
- Chỉ số: `issue-status-sync` chưa có `/metrics` (README v6 mục 8 điều 14); khung chỉ số của `request-service` do TASK-REQ-024-07. Nếu chưa có, dùng `log` có cấu trúc và để chỗ gắn.

## Việc cần làm

1. **Giai đoạn 0 (làm trước task 05 nếu có thể):** `cmd/impact-calibrate/main.go` (mới, chạy trên máy dev, **không** là một phần của service): đọc `testdata/calibration/cases.json` (`[{id, description, base_ref, head_ref, expected_level, source, notes}]`), với mỗi ca chạy các bước của collector bằng một `LocalToolRunner` (cùng cổng `ImpactToolRunner` của task 05 nhưng thực thi bằng `os/exec` trên máy dev, **không** qua `AgentRelay`), gọi `domain.Score` với `DefaultRulePolicyV1()`, in bảng `{id, expected, computed, score, diff_bậc, triggers}` và tỉ lệ khớp; thoát mã khác 0 nếu < 4/5 trong lệch ≤ 1 bậc. Chạy `go run ./cmd/impact-calibrate -cases testdata/calibration/cases.json -repo /opt/repos/orca`. Kết quả ghi vào PR (bảng) và dùng để chỉnh bảng ngưỡng của task 02 (đổi `RulesVersion` nếu chỉnh).
2. `testdata/calibration/cases.json`: ≥ 5 ca từ tài liệu trên, mỗi ca có `base_ref`/`head_ref` thật (đã `git log`-kiểm), `expected_level` do người duyệt điền (không để agent đoán); ca thiếu `head_ref` rõ ràng thì ghi `skip` kèm lý do.
3. `risk_policy_admin.go`: `GetRiskPolicy.Execute(ctx)` (trả `active`, `shadow`, hoặc "mặc định `rp/1`" khi không có dòng nào: `source="default"`):
   - `SetRiskPolicy.Execute(ctx, in{Status, Thresholds, Weights, HardRules, GateMapping, ExpectedVersion})`: chỉ admin
   - `RulePolicy.Validate` (`REQUEST_RISK_POLICY_INVALID`)
   - `status` đi `draft -> shadow -> active -> retired` (chỉ chuyển hợp lệ)
   - `InsertRiskPolicy` + `SetRiskPolicyStatus` một giao dịch
   - đặt `active` đi qua `EnforceReadiness` (bước 4) trừ khi `force=true` do admin kèm `reason` ≥ 20 ký tự (ghi audit)
   - outbox `orca.request.risk_policy.changed {tenant_id, policy_id, version, status}`
   - audit `AppendDetailed` (`target_type=policy`).
4. `enforce_readiness.go`: `EnforceReadiness.Check(ctx, tenantID) (ReadinessReport, error)` tính từ `risk_outcomes` và `impact_assessments`: `OutcomesCount ≥ 30`, `LevelMatchRate ≥ 0.70` (đúng bậc), `WithinOneLevelRate ≥ 0.95`, `PartialOrLowConfidenceRate ≤ 0.30`, `FalseCriticalRate ≤ 0.10` (Nghiêm trọng dự kiến cho thay đổi đã hoàn tất không sự cố):
   - ngưỡng cấu hình qua `RiskPolicy.gate_mapping.enforce_criteria` (mặc định đúng các số trên)
   - trả từng tiêu chí đạt/không để UI hiển thị. `risk_calibration_stats.go`: truy vấn gộp (đếm, tỉ lệ) bằng `RiskOutcomeRepository.CountOutcomes` (task 03)
   - hai dialect không dùng `FILTER`/hàm riêng Postgres (dùng `SUM(CASE WHEN ...)`).
5. Chế độ: `shadow` là mặc định: `mode` của bản đánh giá lấy từ policy (`active` thì `enforce`, ngược lại `shadow`):
   - `RiskGate` (task 06) và `PlanRiskPreconditions` chỉ chặn khi `enforce`
   - `GateMapping.EnforceFromLevel` (`high` mặc định) cho phép "chỉ chặn từ Cao". Bản đánh giá mang `mode` để UI dán nhãn "tham khảo" (CR 2.9). Đổi policy từ `shadow` sang `active` **không** sửa các bản đánh giá cũ (append-only)
   - bản mới theo `mode` mới.
6. `server_risk_policy.go`: handler `GetRiskPolicy`, `SetRiskPolicy`, `GetEnforceReadiness` (RPC đọc, admin); proto thêm vào `request.proto` (message `RiskPolicyMessage`, `EnforceReadinessMessage`); `buf lint` + `buf breaking` trực tiếp.
7. Cờ và cấu hình `main.go`: `REQUEST_IMPACT_ENABLED` quyết định dựng toàn bộ nhánh; kiểm: bật mà thiếu `AgentRelay`, `ApprovalGuard` (task 06), hoặc `ImpactToolRunner` thì thoát lỗi khởi động (không chạy nửa vời). In một dòng log rõ ràng về mode mặc định (`shadow`) lúc khởi động.
8. Chỉ số (khi có khung): `request_impact_assessments_total{subject_type,status,mode}`, `request_impact_tool_runs_total{tool,status}`, `request_impact_duration_seconds{subject_type}`, `request_risk_gate_decisions_total{decision,level,mode}`, `request_risk_drift_total`; nhãn **không** chứa `request_id`/`path`. Cảnh báo đề nghị: tỉ lệ `partial` > 30% trong 1 giờ.
9. e2e `impact_risk_e2e_test.go` (khung TASK-REQ-025-03; agent giả trả đầu ra công cụ mẫu từ `testdata`): mỗi mức rủi ro một Request: Thấp, Trung bình, Cao, Nghiêm trọng, kiểm `shadow` không chặn và `enforce` chặn đúng:
   - kịch bản drift (task thực tế thêm service) mở Approval `drift_review`, chặn `AdvanceExecution`, `OnRejected` về backlog `phase`
   - kịch bản công cụ hết giờ (`partial`).
10. Tài liệu vận hành (một đoạn ngắn trong README của service hoặc runbook của TASK-REQ-025-08): cách chạy `impact-calibrate`, cách bật `enforce`, cách rollback (`SetRiskPolicy` đặt `retired` hoặc `REQUEST_IMPACT_ENABLED=false`).

## Kiểm thử

- `TestCalibrateRunner_ComputesMatchRate` (với `LocalToolRunner` giả và 5 ca), `_FailsBelowThreshold`.
- `TestGetRiskPolicy_DefaultWhenNoRows`, `TestSetRiskPolicy_AdminOnly`, `_ValidatesRulePolicy`, `_InvalidTransitionRejected`, `_ActivateRequiresReadinessUnlessForced`, `_ForceNeedsReason20Chars`, `_EmitsChangedEventAndAudit`, `_OneActiveOneShadow`.
- `TestEnforceReadiness_Table`: dưới 30 đánh giá; tỉ lệ khớp 69% và 70%; `partial` 31%; false-critical 11%; tất cả đạt.
- `TestMode_DefaultShadow_NoBlock`, `_ActivePolicyEnforce`, `_EnforceFromLevelHighOnly`, `_OldAssessmentsKeepOriginalMode`.
- `TestStartup_FailsWhenImpactEnabledWithoutGuard`.
- Dialect: `TestCountOutcomes_BothDialects` (cùng số liệu trên Postgres và MySQL).
- e2e: các kịch bản mục 9 (`-tags=e2e` hoặc cờ `-short` bỏ qua nếu chưa có khung 025-03).
- Thủ công (chưa kiểm chứng): chạy `impact-calibrate` thật trên máy dev; chạy collector qua dev server SSH.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run "RiskPolicy|Enforce|Calibrat|Mode|Startup"` và `go test -tags=integration ./services/request-service/...`; công cụ: `go run ./services/request-service/cmd/impact-calibrate -cases services/request-service/testdata/calibration/cases.json -repo /opt/repos/orca`.

## Tiêu chí hoàn thành

- [ ] Kết quả giai đoạn 0 (bảng ≥ 5 ca) ghi vào PR; bảng ngưỡng chỉnh nếu < 4/5 trong lệch ≤ 1 bậc (đổi `RulesVersion`).
- [ ] `shadow`: không chặn gì, không đòi chấp nhận, nhãn "tham khảo" qua `mode` trong dữ liệu trả về; `risk_outcomes` được điền.
- [ ] `enforce` chỉ bật qua `SetRiskPolicy` bởi admin sau khi `EnforceReadiness` đạt (hoặc `force` có lý do và audit).
- [ ] Mỗi mức rủi ro có một e2e; drift mở Approval và chặn `AdvanceExecution`.
- [ ] `REQUEST_IMPACT_ENABLED` bật mà thiếu cổng bắt buộc thì khởi động lỗi; tắt thì RPC trả `REQUEST_IMPACT_DISABLED`.
- [ ] `parity_test.go` (gateway) không bị phá: danh sách kênh `risk.policy.get|set`, `impact.*` đã chuyển cho SOL-016/017.
- [ ] Không file nào tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable.

## Rủi ro và lưu ý

- Ngưỡng điều kiện bật `enforce` (30 mẫu, 70%, 30%, 10%) là đề xuất của CR, chưa kiểm chứng; `risk_outcomes.incident`/`rolled_back` do người khai tay (Q4) nên chất lượng dữ liệu hiệu chỉnh phụ thuộc kỷ luật nhập.
- `impact-calibrate` chạy trên máy dev có GitNexus index khác dev server: kết quả có thể lệch với production; ghi rõ khi báo cáo.
- Ca lịch sử cũ (tháng 9) có thể không còn `base_ref` hợp lệ sau rebase; chọn commit băm cố định, không nhánh.
- `GateMapping.EnforceFromLevel` làm hành vi `enforce` khác nhau giữa tenant; kiểm thử chéo tenant.
- Đặt `active` khi đang có Approval `pending` cho chủ thể đã được đánh giá `shadow`: các Approval đó không đòi `RiskAcceptance` (bản đánh giá ghi `mode=shadow`); chấp nhận, ghi trong runbook.
- Số liệu `EnforceReadiness` đọc toàn bộ `risk_outcomes` của tenant: thêm chỉ mục `(tenant_id, recorded_at)` nếu chậm (chưa đo).
