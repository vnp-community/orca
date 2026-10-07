# BE-CV-TASK-093-07: Bộ fixture injection, golden và quy trình đánh giá thủ công

**From Solution:** BE-CV-SOL-093-ai-review-summary
**Priority:** P2
**Service:** `code-intel-service`
**File:** `testdata/ai-review/{injection,diverse}/*.json`, `docs` vận hành (không thuộc task; ghi quy trình vào README service)
**Depends on:** BE-CV-TASK-093-06
**Status:** [x] DONE

## Việc cần làm
1. 20 diff injection + 20 diff đa dạng (Go, TS, SQL migration, proto, chỉ test) cạnh fixture CR-CV-070; bổ sung ca "tóm tắt AI không rò dữ liệu cấm" vào bộ bảo mật CR-CV-072.
2. Golden `AiReviewSummary`, `manifest` cho FE-CV-SOL-093.
3. Quy trình thủ công (CR §2.9): chấm grounding, độ phủ rủi ro, khẳng định sai, độ trễ/token; ngưỡng đề xuất là **giá trị khởi điểm chưa hiệu chỉnh**; **không** chạy trong CI.

## Kiểm thử / Tiêu chí hoàn thành
- [x] fixture chạy qua pipeline với AI giả; [ ] 100% mẫu injection không đổi `verdict`/refs ngoài tập; [ ] quy trình ghi lại.

## Rủi ro
- Chất lượng đa ngôn ngữ chưa kiểm chứng; chạy mô hình thật tốn tiền và không tất định.
