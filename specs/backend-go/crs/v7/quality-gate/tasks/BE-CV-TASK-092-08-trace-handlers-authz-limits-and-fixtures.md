# BE-CV-TASK-092-08: Handler 3 RPC, quyền, cờ, audit, giới hạn, fixture

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/adapter/grpc/requirement_trace_server.go`, `testdata/requirement-trace/*.json` (mới)
**Depends on:** BE-CV-TASK-092-07, BE-CV-TASK-085-07
**Status:** [ ] TODO

## Việc cần làm
1. Handler: `quality_read` (Get), `review_write` (Confirm, Link); cờ chất lượng tắt ⇒ `CODEINTEL_QUALITY_GATE_DISABLED`.
2. Kiểm tham số (`requirement_key ≤ 160`, `evidence_ref ≤ 255`, `link_kind` hợp lệ); id con tenant khác ⇒ `NOT_FOUND`.
3. Golden `RequirementTrace` (ui-api §4.7) cho FE; test cô lập tenant hai dialect.

## Kiểm thử / Tiêu chí hoàn thành
- [ ] khớp ui-api §3.2; [ ] hai người dùng với quyền task khác nhau; [ ] audit `codeintel.trace.confirm` ghi đúng.

## Rủi ro
- Giới hạn gateway 8 KiB cho confirm/link thuộc BE-040; ở đây chỉ kiểm lại phòng thủ.
