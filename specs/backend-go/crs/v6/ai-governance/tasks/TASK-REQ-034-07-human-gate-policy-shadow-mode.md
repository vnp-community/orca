# TASK-REQ-034-07: `HumanGatePolicy` và chế độ chạy bóng (`ai_gate_decisions`)

**From Solution:** BE-REQ-SOL-034 (mục E, CR 2.4)
**Priority:** P2
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/human_gate_policy.go` (mới), `.../internal/usecase/human_gate_decider.go` (mới), `.../internal/adapter/{postgres,mysql}/ai_gate_decision.go` (task 01 đã khai báo cổng; hiện thực ở đây nếu chưa), `.../internal/domain/human_gate_policy_test.go`, `.../internal/usecase/human_gate_decider_test.go` (mới)
**Depends on:** TASK-REQ-034-01 (bảng `ai_gate_decisions`, cột `ai_gate_mode`); BE-REQ-SOL-009 (khi tạo Approval); BE-REQ-SOL-003 (loại Request và `subject_type`)
**Status:** [ ] TODO

---

## Context

- README v6 mục 3.4 đặt cổng duyệt theo loại; **mặc định mọi cổng là `required`** và không đổi hành vi. CR 2.4 chỉ mở khả năng `auto_allowed` khi tenant bật và đồng thời: `subject_type ∈ {request_type, task_list}`, `request_type ∈ {task, docs}`, `size=S`, không nhãn rủi ro cao, `confidence ≥ ngưỡng (0,9)`; không bao giờ cho `hotfix`, `security`, `ops_request`, `pre_deploy`, `size=L`.
- Điểm rủi ro `risk` đến từ mô-đun chấm điểm (CR-REQ-030, chưa có spec backend): khi chưa có, `risk=unknown` ⇒ luôn `required`. Task này **không** chấm điểm; chỉ nhận `risk` làm đầu vào.
- Chế độ `shadow` (mặc định khi bật): chỉ ghi "sẽ tự duyệt" vào `ai_gate_decisions` (solution C2: CR viết "vào ledger" nhưng ledger là sổ mỗi lời gọi AI) và metric; **không** tự duyệt thật. `enforce` bị chặn ở v1 (task 05 từ chối `SetSettings(enforce)`); code vẫn có nhánh `enforce` để test, nhưng cờ cấu hình `REQUEST_AI_GATE_ENFORCE_ALLOWED=false` chặn ở production.
- Tự duyệt thật (khi sau này bật) tạo Approval do hệ thống quyết `decided_by='system:policy'`, `comment` ghi `policy_version`; cần CR-REQ-009 hỗ trợ `decided_by_kind` (`user|system_policy`), việc đó thuộc CR 009 (ghi ở "Tác động" của CR 034). Task này chỉ gọi cổng `ApprovalAutoDecider` nếu nó tồn tại; chưa có thì `enforce` trả lỗi `ErrAutoApprovalUnsupported` và quyết định ghi `required`.
- Agent (`actor_type=agent`) không bao giờ duyệt (CR 035); quyết định hệ thống là một loại tác nhân riêng (`system`), chỉ qua đường nội bộ.

## Việc cần làm

1. `human_gate_policy.go` (hàm thuần, stdlib):

```go
const PolicyVersion = "hg/1"
type GateInput struct {
    SubjectType, RequestType, Size string
    Confidence                     float64
    Risk                           string  // unknown|low|medium|high
    HighRiskLabels                 bool
    ThresholdConfidence            float64 // mặc định 0.9
}
type GateDecision string // required | auto_allowed
func Decide(in GateInput) (GateDecision, string) // trả kèm lý do (mã ngắn) để ghi
```

   Thứ tự: (1) `RequestType ∈ {hotfix, security, ops_request}` hoặc `SubjectType == pre_deploy` hoặc `Size == "L"` ⇒ `required` (`never_auto`); (2) `Risk != "low"` (kể cả `unknown`) ⇒ `required` (`risk_not_low`); (3) `HighRiskLabels` ⇒ `required`; (4) `Confidence < Threshold` ⇒ `required` (`low_confidence`); (5) `SubjectType ∈ {request_type, task_list}` và `RequestType ∈ {task, docs}` và `Size == "S"` ⇒ `auto_allowed` (`eligible`); (6) mọi trường hợp khác ⇒ `required` (`not_eligible`).
2. `human_gate_decider.go`: `HumanGateDecider.Decide(ctx, in GateInput, requestID string) (GateDecision, error)`: đọc `ai_gate_mode` (`off` ⇒ trả `required` và **không** ghi gì); `shadow`: `GateDecisionRepository.Insert(mode=shadow, decision=Decide(...))` rồi **luôn trả `required`**; `enforce`: nếu `ApprovalAutoDecider` có và cờ cho phép thì thực hiện tự duyệt (hợp đồng ở CR 009), ngược lại `required`. Lỗi ghi bảng không làm lỗi luồng chính (log cảnh báo, trả `required`).
3. Điểm gọi: ngay trước khi `RequestApproval` tạo cổng ở các use case của CR 005, 007, 008, 012, 014 (ghi vào "Tác động", không sửa các solution đó ở task này): họ gọi `Decide` với `SubjectType` và thuộc tính Request; kết quả `required` luôn dẫn đến Approval `pending` như cũ.
4. Metric: `request_ai_gate_decisions_total{subject_type,mode,decision}` (không nhãn tenant); log chỉ mã lý do.
5. Bảng báo cáo hiệu chỉnh (đọc, không RPC công khai ở v1): hàm `SummaryByPeriod(ctx, from, to) (GateSummary)` ở repository để chạy bằng truy vấn tay/dashboards: số `would_auto` so với số `required` theo `(request_type, size)`; sau này đối chiếu với kết quả thực thi (CR-REQ-013 `task_run_outcomes`) để hiệu chỉnh ngưỡng. Không có UI.
6. Bất biến: không đường mã nào tạo Approval `approved` mà `Decide` không phải `auto_allowed` **và** `mode=enforce` **và** cờ production cho phép; test bất biến ở dưới.

## Kiểm thử

- `human_gate_policy_test.go`: bảng đủ tổ hợp (11 loại × 3 size × `subject_type` × `risk` × `confidence` quanh 0,9): khẳng định bất biến "`hotfix`, `security`, `ops_request`, `pre_deploy`, `size=L`, `risk=unknown` luôn `required`" bằng vòng lặp; `confidence=0.9` đúng ngưỡng ⇒ `auto_allowed` (khi mọi điều kiện khác thoả); `0.8999` ⇒ `required`.
- `human_gate_decider_test.go`: `off` không gọi `Insert`; `shadow` ghi một dòng `auto_allowed` nhưng **trả `required`** (không tạo Approval); `enforce` khi `ApprovalAutoDecider` không có ⇒ `required` + `ErrAutoApprovalUnsupported` được log; lỗi ghi bảng ⇒ vẫn `required`.
- Integration (hai dialect): `TestGateDecision_InsertAndSummary`; cách ly tenant.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/...`.

## Tiêu chí hoàn thành

- [ ] `HumanGatePolicy` ở chế độ chạy bóng không tạo Approval tự động; ghi đúng "sẽ tự duyệt" vào `ai_gate_decisions`.
- [ ] `hotfix`, `security`, `ops_request`, `pre_deploy`, `size=L`, `risk=unknown` luôn `required` (test bảng).
- [ ] `ai_gate_mode=off` mặc định: không ghi, không đổi hành vi.
- [ ] `enforce` không thể bật ở production v1.
- [ ] Domain chỉ dùng stdlib.

## Ví dụ tham khảo

Bảng quyết định mẫu (đưa vào test; `risk=low`, ngưỡng 0,9):

| `subject_type` | `request_type` | `size` | `confidence` | Kết quả |
|---|---|---|---|---|
| `request_type` | `task` | `S` | 0,95 | `auto_allowed` |
| `task_list` | `docs` | `S` | 0,90 | `auto_allowed` |
| `task_list` | `docs` | `M` | 0,99 | `required` |
| `request_type` | `hotfix` | `S` | 0,99 | `required` |
| `pre_deploy` | `task` | `S` | 0,99 | `required` |

## Rủi ro và lưu ý

- Ngưỡng 0,9 và quy tắc chưa hiệu chỉnh; chế độ chạy bóng có mục đích duy nhất là thu số liệu (enterprise-readiness-checklist mục 5).
- Không có điểm rủi ro thì `auto_allowed` không bao giờ xảy ra: đúng thiết kế; đừng nới `unknown`.
- Câu hỏi mở 6 của CR: `auto_allowed` có nên tồn tại ở v1; task này giữ nhỏ để bỏ được mà không đụng phần khác (xoá file và một lời gọi).
- Cần CR-REQ-009 xác nhận `decided_by_kind` trước khi nghĩ đến `enforce`.
