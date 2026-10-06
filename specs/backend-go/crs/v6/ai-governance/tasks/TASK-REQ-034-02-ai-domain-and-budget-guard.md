# TASK-REQ-034-02: Domain AI (bước, ngân sách, ledger, chính sách bước) và `BudgetGuard`

**From Solution:** BE-REQ-SOL-034 (mục A, C, D)
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/{ai_step.go,ai_budget.go,ai_usage_ledger.go,ai_step_policy.go,provenance.go}` (mới), `.../internal/usecase/{budget_guard.go,ai_pricing.go}` (mới), `.../internal/domain/domain_ai_test.go`, `.../internal/usecase/budget_guard_test.go`, `.../internal/usecase/budget_guard_integration_test.go` (mới)
**Depends on:** TASK-REQ-034-01
**Status:** `[ ] TODO`

---

## Context

- Hàm xác định thời gian cửa sổ phải theo **UTC** (CR 2.1). Đồng hồ lấy từ cổng `Clock` (SOL-001/007), không `time.Now()` trực tiếp, để test ranh giới 23:59:59 và 00:00:00.
- `BudgetGuard` chạy trước mọi lời gọi AI (`AIGateway`, task 04): `Reserve` trong **một** giao dịch (solution mục D); `Settle` sau lời gọi. Không giữ giao dịch qua lời gọi Relay.
- Mặc định không giới hạn: không ngân sách `enabled` khớp thì `Reserve` trả `Reservation{}` rỗng (không lỗi) nhưng ledger vẫn ghi (task 04).
- Ước lượng token dùng **số rune** (`utf8.RuneCountInString`) chia 4 (làm tròn lên) cộng `MaxTokens`; tiếng Việt có dấu nhiều byte hơn ký tự nên `len(s)` sẽ phóng đại. Đây là đề xuất của CR, chưa hiệu chỉnh.
- Phụ thuộc vào `AuditRecorder` (TASK-REQ-024-02) chỉ ở task 05; task này không audit.

## Việc cần làm

1. `ai_step.go`: `type AIStep string` với 8 hằng (`StepClassify = "classify"`, `StepSolution`, `StepDiagnosis`, `StepFindings`, `StepAnswer`, `StepPlan`, `StepTaskSpec = "taskspec"`, `StepExecute`); `ParseAIStep(string) (AIStep, error)`; `type AIMode string` (`complete`, `agent_readonly`, `agent_write`); `func (s AIStep) DefaultMode() AIMode` (`diagnosis|findings|answer` → `agent_readonly`, `execute` → `agent_write`, còn lại `complete`).
2. `ai_budget.go`: `BudgetScopeKind` (`tenant|request_type|step|project`), `BudgetPeriod` (`day|month|request`), `BudgetAction` (`block|warn_only`); `AIBudget{ID, TenantID string; ScopeKind; ScopeValue string; Period; LimitTokens *int64; LimitCalls *int32; LimitAgentSeconds *int32; LimitCostUSDEst *Money; WarnRatio float64; Action; Enabled bool; Version int64; UpdatedBy string; UpdatedAt time.Time}`; `func (b AIBudget) Validate() error` (`WarnRatio` trong `(0,1]`; `ScopeKind=tenant` thì `ScopeValue==""`; `step` phải `ParseAIStep` hợp lệ; `request_type` phải thuộc 11 loại; ít nhất một giới hạn khác nil hoặc `Action=warn_only`; mọi giới hạn > 0). `func (b AIBudget) Matches(f BudgetFilter) bool`. `func WindowKey(p BudgetPeriod, now time.Time, requestID string) (key string, start time.Time)`: `day` → `now.UTC().Format("2006-01-02")`, `month` → `"2006-01"`, `request` → `"request:"+requestID` (`start` = thời điểm Request đầu tiên = `now`; cột chỉ để thông tin).
3. `ai_usage_ledger.go`: `LedgerEntry` (đủ cột solution B), `UsageSource` (`reported|estimated`), `Usage{InputTokens, OutputTokens int64; Source UsageSource}`; `func EstimateTokens(prompt string, maxTokens int) int64`; `func EstimateOutput(chars int) int64` (`(chars+3)/4` theo rune). `Money` kiểu micro-USD `int64` nếu repo không có decimal (xem task 01 bước 8) với `func (m Money) String()`.
4. `ai_pricing.go`: `type PriceTable map[string]ModelPrice`, `ModelPrice{InPerMTok, OutPerMTok Money}`; `ParsePriceTable(json []byte) (PriceTable, error)` đọc `REQUEST_AI_PRICE_TABLE`; `func (p PriceTable) Cost(model string, u Usage) Money` (thiếu model: 0).
5. `ai_step_policy.go`: `ChainItem{Model string; Class string; EgressClass EgressClass; MaxTokens int; TimeoutSeconds int}`, `EgressClass` (`external|internal`), `StepPolicy{ID, TenantID string; Step AIStep; RequestType, Size string; Chain []ChainItem; EscalateOnSchemaFailure bool; Version int64}`; `Validate()` (chain 1 đến 4 phần tử, `Model` hoặc `Class` không rỗng, `TimeoutSeconds` trong `[10, 600]`); `DefaultStepPolicies(classes ModelClasses) []StepPolicy` dựng mặc định đề xuất (`classify`: `[haiku-class, sonnet-class]`; `solution`, `plan`: `[opus-class, sonnet-class]`; còn lại một phần tử) với `ModelClasses` đọc từ cấu hình `REQUEST_AI_MODEL_CLASSES` (JSON `{"haiku":"<id>","sonnet":"<id>","opus":"<id>"}`); **không** cứng id model trong mã (id đổi nhanh, chưa kiểm chứng id hiện hành).
6. `provenance.go`: `Provenance{Tool, Step, PromptID, PromptVersion, Model, RunID, InputDigest string; GeneratedAt time.Time}` với tag JSON đúng CR 2.3 (`tool`, `step`, `prompt_id`, `prompt_version`, `model`, `run_id`, `input_digest`, `generated_at`) và `func (p Provenance) MarshalJSON` ổn định (thứ tự khoá cố định, dùng struct).
7. `budget_guard.go`: 

```go
type Reservation struct{ Items []reservedItem } // budgetID, windowKey, delta
type Estimate struct{ Tokens int64; AgentSeconds int32; CostUSD Money }
func (g *BudgetGuard) Reserve(ctx context.Context, c BudgetContext, est Estimate) (Reservation, error)
func (g *BudgetGuard) Settle(ctx context.Context, r Reservation, actual Estimate) error
```

   `Reserve`: `tx.InTx`: `ListEnabled(filter)` → với mỗi ngân sách `EnsureCounter` rồi `TryAdd` (giới hạn lấy từ `AIBudget`); `!ok && action=block` → trả `ErrAIBudgetExceeded{BudgetID, Scope}` (giao dịch rollback toàn bộ); `!ok && action=warn_only` → bỏ qua `TryAdd`, thêm outbox `orca.request.ai_budget.exceeded`; `ok` → tính `ratio = max(used/limit)` theo từng chiều; `ratio >= WarnRatio` thì `MarkWarned`, nếu `first` thêm outbox `orca.request.ai_budget.warning` (cùng giao dịch). `Settle`: `Adjust` với `actual - est` (có thể âm), không kiểm hạn mức.
8. `domain.ErrAIBudgetExceeded` ánh xạ `REQUEST_AI_BUDGET_EXCEEDED` (ResourceExhausted) ở tầng gRPC (task 05); `Reserve` chỉ trả lỗi miền.
9. Outbox: hằng subject `orca.request.ai_budget.warning` và `.exceeded` trong `domain/outbox_subjects.go` (sửa file của SOL-001), payload `{tenant_id, budget_id, scope, ratio, user_ids}` (gợi `user_ids = [budget.UpdatedBy]`, C6).

## Kiểm thử

- `domain_ai_test.go`: `TestWindowKey_UTCBoundaries` (`2026-10-06T23:59:59Z` và `2026-10-07T00:00:00+07:00` cho ngày UTC khác nhau; `month` qua 31/12); `TestAIBudgetValidate` bảng; `TestEstimateTokens_VietnameseUsesRunes` (chuỗi "Việt Nam" có số rune khác `len`); `TestDefaultStepPolicies_NoHardcodedModelIDs` (id đến từ `ModelClasses`); `TestProvenanceJSONGolden` (`testdata/provenance.golden.json`).
- `budget_guard_test.go` (repo giả trong bộ nhớ): hạn tokens 100, hai lần Reserve 70 → lần hai `ErrAIBudgetExceeded`, **bộ đếm không đổi** (rollback); `warn_only` không chặn nhưng phát `exceeded`; `warn_ratio=0.8` vượt lần đầu phát đúng một `warning`, lần hai không; `Settle` trả phần dư (est 200, thật 120 → `-80`); nhiều ngân sách (tenant day + step month): một chặn thì cả hai không đổi.
- `budget_guard_integration_test.go` (hai dialect): `TestReserve_ConcurrentOnlyOnePasses` (hai goroutine, `limit_tokens` đủ cho một); `TestReserve_RequestPeriodIsolatedPerRequest` (hai Request khác nhau không dùng chung bộ đếm).
- Lệnh: `cd backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... && go test -tags=integration ./services/request-service/internal/usecase/...`.

## Tiêu chí hoàn thành

- [ ] Hai lời gọi đồng thời cùng làm ngân sách vượt: đúng một qua, một `REQUEST_AI_BUDGET_EXCEEDED`.
- [ ] Vượt `warn_ratio` phát đúng một sự kiện mỗi cửa sổ.
- [ ] Ngân sách chặn thì không bộ đếm nào bị cộng dở.
- [ ] Không id model nào cứng trong mã.
- [ ] Domain chỉ dùng stdlib.

## Ví dụ tham khảo

Hai ngân sách cùng áp cho một lời gọi `solution` của tenant (JSON minh hoạ):

```json
[{"scope_kind": "tenant", "scope_value": "", "period": "day", "limit_tokens": 2000000, "warn_ratio": 0.8, "action": "block"},
 {"scope_kind": "step", "scope_value": "solution", "period": "request", "limit_tokens": 1000000, "warn_ratio": 0.8, "action": "block"}]
```

Khoá cửa sổ cho lời gọi lúc `2026-10-06T23:59:59Z` của Request `R1`: `("2026-10-06")` và `("request:R1")`; một giây sau, ngân sách ngày chuyển sang `("2026-10-07")` còn ngân sách Request giữ nguyên.

## Rủi ro và lưu ý

- Ước lượng sai làm chặn sớm hoặc muộn; với `agent_readonly` chưa có token thật, `agent_seconds` đáng tin hơn (ghi vào README của ngân sách).
- Cửa sổ theo UTC; tenant ở múi giờ khác có thể thấy "ngày" lệch: ghi trong tài liệu admin.
- `ratio` dùng chiều cao nhất; hiển thị cho admin chiều nào chạm ngưỡng ở `payload.scope`.
- Một ngân sách `request` mặc định đề xuất 1 000 000 token (chưa đo): chỉ seed khi chủ sản phẩm xác nhận (Q3), không tự bật.
