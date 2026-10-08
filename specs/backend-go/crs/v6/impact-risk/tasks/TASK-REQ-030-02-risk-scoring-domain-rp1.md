# TASK-REQ-030-02: Domain điểm rủi ro `rp/1`: chín chiều, `Score`, luật cứng, digest, so lệch kế hoạch

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.C, 2.G
**Priority:** P1
**Service/Area:** `request-service` (mới) / domain thuần, không I/O
**File:** `internal/domain/impact_assessment.go` (mới), `internal/domain/risk_dimension.go` (mới), `internal/domain/risk_rules_v1.go` (mới), `internal/domain/risk_scoring.go` (mới), `internal/domain/risk_hard_rules.go` (mới), `internal/domain/impact_drift.go` (mới), `internal/domain/risk_acceptance.go` (mới), `internal/domain/risk_policy.go` (mới), `internal/domain/testdata/risk_rp1_golden.json` (mới), và các `_test.go`
**Depends on:** TASK-REQ-001-01 (module); không phụ thuộc DB, proto hay công cụ
**Status:** [ ] TODO

---

## Context

- CR-REQ-030 mục 2.3: chín chiều (Kiến trúc, Tương thích hợp đồng, Dữ liệu, Phạm vi ảnh hưởng, Bảo mật và quyền, Vận hành, Chất lượng, Bất định, Quy mô), mỗi chiều cho điểm 0 đến 100 theo bảng ngưỡng cố định (cộng dồn, trần 100); chiều không đo được mang điểm trung tính 50 khi gộp.
- Mục 2.4: `base = round(0.6*max(s_d) + 0.4*Σ(w_d*s_d)/Σ(w_d))`; cắt mức Thấp (<25), Trung bình (25 đến 49), Cao (50 đến 74), Nghiêm trọng (≥ 75); `level = max(level, mức tối thiểu của luật cứng)`. Trọng số `rp/1`: bảo mật 1,2; kiến trúc, hợp đồng, dữ liệu, phạm vi ảnh hưởng 1,0; vận hành, chất lượng 0,8; bất định, quy mô 0,6. Mọi hằng nằm trong `RulePolicy`; đổi hằng thì tăng `rules_version` và golden test bắt buộc cập nhật. Cùng `dims` cho cùng `score`, `level`, `digest`.
- Luật cứng (mục 2.4.1) và quy tắc index lỗi thời: xem bảng trong CR. Quy tắc bất định: ≥ 3 chiều không đo được thì Trung bình kèm `confidence=low`.
- **Sai khác so với CR (SOL-030 mục 1, điều 1):** mỗi `DimensionResult` có `Basis` (`tool|path|declared`); chiều có `Basis != tool` giới hạn `confidence` tối đa `medium`. `solution_option` luôn tối đa `medium`.
- Domain của `request-service` thuần Go (stdlib), không import `common/*` hay adapter (kiến trúc `arch/03`). Không `time.Now`, không `os.Getenv`, không `math/rand`.
- Quy ước tên: không `helpers`/`utils`/`common`/`misc`.

## Việc cần làm

1. `risk_dimension.go`: `Dimension` (chín hằng), `Level` (`LevelLow|Medium|High|Critical`, `Rank()`, `FromScore(score int, cuts [3]int)`), `Confidence` (`ConfidenceHigh|Medium|Low`), `Signal{Code string; Value float64; Points int; Measured bool; Note string}`, `DimensionResult{Dimension; Score int; Measured bool; Basis string; Signals []Signal}`; `func SumPoints(signals []Signal, cap int) int` (cộng dồn, trần 100, chỉ cộng tín hiệu `Measured`).
2. `impact_assessment.go`: `SubjectType` (sáu hằng, `IsActual()`), `Mode` (`ModeShadow|ModeEnforce`), `Status` (năm hằng, `Terminal()`), `Finding{FindingID, Dimension, Code, Severity string; Path, Message, EvidenceRunID string}` (không bao giờ chứa giá trị bí mật; `Message` ≤ 500 ký tự), `HardRule{Code string; MinLevel Level; Detail string}`, `ImpactAssessment{...}` mirror bảng (không có hành vi ghi), `ToolRunRef{ID, Tool, Status string}`.
3. `risk_rules_v1.go`: `RulePolicy{RulesVersion string; Weights map[Dimension]float64; MaxWeight float64; LevelCuts [3]int; Thresholds map[Dimension][]ThresholdStep; ServiceCountHardRule int}` và `DefaultRulePolicyV1() RulePolicy` với **đúng** bảng của CR 2.3 (ví dụ Phạm vi ảnh hưởng: gọi trực tiếp <5: 10; 5 đến 20: 35; 21 đến 50: 60; >50: 85; mỗi luồng thực thi bị chạm +5; ≥3 service +20). `ThresholdStep{Code string; Points int; Cap int}`; bảng là dữ liệu, không là `if` rải rác. Hàm `(p RulePolicy) Validate() error` (trọng số > 0, `LevelCuts` tăng dần trong `[1,99]`, `MaxWeight` trong `[0,1]`) dùng cho `RiskPolicy` tenant (`REQUEST_RISK_POLICY_INVALID`).
4. `risk_hard_rules.go`: `EvaluateHardRules(dims [9]DimensionResult, ctx HardRuleContext, p RulePolicy) []HardRule` với `HardRuleContext{BufBreakingCount int; ChannelRemovedOrRenamed bool; MigrationIrreversible bool; TouchesAuthTenantCredOPA bool; NoRollbackPath bool; IrreversibleWithoutFlag bool; ServiceCount int; MaxLinesDisableAdded bool; IndexStale bool; IndexMissing bool}`; mỗi điều kiện của bảng 2.4.1 ra một `HardRule` với `MinLevel` và `Code` ổn định (`HR_BUF_BREAKING`, `HR_CHANNEL_REMOVED`, `HR_MIGRATION_IRREVERSIBLE`, `HR_SENSITIVE_AREA`, `HR_NO_ROLLBACK`, `HR_IRREVERSIBLE_NO_FLAG`, `HR_SERVICE_COUNT`, `HR_MAX_LINES_DISABLE`, `HR_INDEX_STALE` (tăng một bậc), `HR_UNMEASURED_DIMS` (Trung bình, `confidence=low`)).
5. `risk_scoring.go`: `Score(dims [9]DimensionResult, ctx HardRuleContext, p RulePolicy) Scored` (`Scored{Score int; Level Level; Triggers []HardRule; Confidence Confidence; Digest string; UnmeasuredCount int}`) theo công thức:
   - chiều `Measured=false` dùng điểm 50 và tăng `UnmeasuredCount`
   - `Level = max(FromScore(score), max(HardRule.MinLevel))`, rồi `HR_INDEX_STALE` tăng thêm một bậc (tối đa Nghiêm trọng)
   - `Confidence`: bắt đầu `high`
   - `low` nếu `IndexStale|IndexMissing` hoặc `UnmeasuredCount >= 3`
   - `medium` nếu có chiều `Basis != tool` hoặc có công cụ lỗi
   - áp trần theo `SubjectType` ngoài hàm (đối số `subject SubjectType` nếu cần; chọn một nơi và test). `Digest` = `SHA256Hex(CanonicalJSON({dimensions, triggers, findings, level, rules_version}))` — dùng `CanonicalJSON` và `Digest` của TASK-REQ-027-03 (`canonical_json_digest.go`; `Digest` trả `sha256:<hex>`, cột `digest CHAR(64)` lưu hex không tiền tố nên cắt tiền tố một chỗ)
   - task này không tạo hàm canonical mới. **Loại** `narrative`.
6. `impact_drift.go`: `CompareActual(expected, actual AssessmentSummary, delta int) DriftResult{Drift bool; Reasons []string}`:
   - `AssessmentSummary{Level; Score int; Services []string; BufBreaking int; MigrationIrreversible bool}`
   - điều kiện đúng như solution 2.G (service thêm, cao hơn một bậc, chênh điểm > `delta`, tín hiệu tuyệt đối mới).
7. `risk_acceptance.go`: `RiskAcceptance` và `func (a RiskAcceptance) ValidFor(currentDigest string) bool`; `func NewRiskAcceptance(...) (RiskAcceptance, error)` kiểm `rationale` ≥ 10 ký tự (đếm rune, sau `strings.TrimSpace`).
8. `risk_policy.go`: `RiskPolicy{ID, TenantID string; Version int; Status PolicyStatus; Thresholds, Weights, HardRules, GateMapping []byte...}` hoặc kiểu có cấu trúc `GateMapping{RiskApproverTeam, RiskOverrideTeam string; EnforceFromLevel Level}` (chặn từ mức nào ở `enforce`, CR 2.9):
   - `func (p RiskPolicy) ToRulePolicy() (RulePolicy, error)`
   - `func (s PolicyStatus) CanTransitionTo(to PolicyStatus) bool` (`draft->shadow->active->retired`, `shadow->retired`, không ngược).
9. Golden: `testdata/risk_rp1_golden.json` chứa ≥ 6 ca (đầu vào `dims` + `ctx`, đầu ra mong đợi `score`, `level`, `triggers`, `digest`): (a) mọi chiều thấp, (b) một chiều Dữ liệu 100 với migration không đảo ngược (luật cứng nâng Cao dù điểm thấp), (c) `buf breaking` ≥ 1, (d) chiều không đo được (50), (e) index lỗi thời tăng một bậc, (f) ≥ 3 chiều không đo được thì Trung bình và `confidence=low`, (g) trần 100 cho chiều cộng dồn. Cờ `-update` khai báo trong test.

## Kiểm thử

- `TestScore_Golden_RP1` (đọc tệp golden).
- `TestScore_Deterministic1000Runs` (cùng đầu vào cho cùng `Score`, `Level`, `Digest`; kể cả khi thứ tự `Signals` bị xáo trộn trong cùng chiều: `Digest` phải bất biến, nên sắp xếp `Signals` theo `Code` trước khi băm).
- `TestScore_MaxDimensionDominates` (một chiều 100, tám chiều 0: điểm ≥ 60).
- `TestScore_UnmeasuredCountsAs50`, `TestScore_NeverLowWhenUnmeasured` (nếu cả chín chiều không đo được thì không bao giờ `Low`).
- `TestHardRules_Table`: mỗi dòng bảng 2.4.1 nâng đúng mức tối thiểu kể cả khi điểm 0.
- `TestScore_ConfidenceRules` (basis path/declared, công cụ lỗi, index missing, solution_option trần `medium`).
- `TestRulePolicy_Validate` (trọng số âm, cắt sai thứ tự).
- `TestCompareActual_Table` (bốn điều kiện drift và trường hợp không drift).
- `TestNewRiskAcceptance_RationaleLength` (9 ký tự lỗi, 10 ký tự đúng, Unicode tiếng Việt đếm rune).
- `TestRiskPolicyStatus_Transitions`.
- Thuộc tính: `TestScore_ProgressionMonotonic` (tăng điểm một chiều không bao giờ làm giảm `Score`).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run "Score|Hard|Drift|RulePolicy|RiskAcceptance|RiskPolicy"`.

## Tiêu chí hoàn thành

- [ ] `Score` thuần: cùng đầu vào cho cùng `score`, `level`, `digest` qua 1000 lần.
- [ ] Golden `rp/1` có ≥ 6 ca và mỗi luật cứng nâng đúng mức tối thiểu.
- [ ] Đổi một hằng trong `DefaultRulePolicyV1` làm golden đỏ (test guard: băm bảng ngưỡng so với `RulesVersion`).
- [ ] Chiều không đo được không bao giờ dẫn tới `Low`.
- [ ] Domain không import `common`, adapter, `os`, `time.Now`, `math/rand`.
- [ ] Không có `max-lines` disable; tệp bảng ngưỡng tách riêng nếu quá dài.

## Rủi ro và lưu ý

- Bảng ngưỡng, trọng số, `N=3`, độ lệch 15 điểm đều là ước lượng chưa hiệu chỉnh; vì vậy `shadow` mặc định (task 08) và giai đoạn 0 trước khi viết use case.
- "Cộng dồn, trần 100" trong CR chưa nêu thứ tự khi nhiều tín hiệu cùng loại (ví dụ nhiều vi phạm `buf`); cài như CR: `80 + 5/vi phạm thêm`, trần 100; ghi vào test để đổi có chủ ý.
- Điểm tính từ `Signals` do các bộ quét điền (task 04, 05); domain chỉ tổng hợp. Nếu bộ quét sinh tín hiệu khác bảng thì `SumPoints` bỏ qua `Code` lạ và ghi `finding UNKNOWN_SIGNAL` (không làm hỏng điểm).
- Cùng mức `Level` của hai chiều khác nhau: không phân xử; `max` là đủ.
