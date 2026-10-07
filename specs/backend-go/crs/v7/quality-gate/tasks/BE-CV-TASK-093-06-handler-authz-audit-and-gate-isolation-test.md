# BE-CV-TASK-093-06: Handler, OPA `quality_read ∧ read_source`, mã lỗi AI và test tách khỏi cổng

**From Solution:** BE-CV-SOL-093-ai-review-summary
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/adapter/grpc/ai_review_server.go`, `internal/domain/quality_gate_isolation_test.go` (mới)
**Depends on:** BE-CV-TASK-093-05, BE-CV-TASK-085-07
**Status:** [x] DONE

## Việc cần làm
1. Handler: kiểm `level` (`metadata|diff`), `locale`, `base_ref`; OPA hai quyền; ánh xạ mã `CODEINTEL_AI_REVIEW_DISABLED`, `AI_NO_RELAY`, `AI_BAD_OUTPUT`, `INVALID_PARAMS`.
2. Test cấu trúc (`go/parser`): `domain/quality_gate_*.go` không import package `ai_review`.
3. Test `verdict` không đổi khi có/không có tóm tắt.

## Kiểm thử / Tiêu chí hoàn thành
- [x] thiếu `read_source` ⇒ `NOT_AUTHORIZED`; [ ] test cấu trúc fail khi cố tình import; [ ] audit không chứa nội dung.

## Rủi ro
- `read_source` phụ thuộc OPA của SOL-013.
