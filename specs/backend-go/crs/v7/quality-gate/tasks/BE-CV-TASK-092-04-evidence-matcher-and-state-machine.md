# BE-CV-TASK-092-04: Domain ghép bằng chứng và máy trạng thái `state`

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/domain/requirement_evidence_matcher.go`, `requirement_trace.go` (mới)
**Depends on:** BE-CV-TASK-092-03
**Status:** [x] DONE

## Việc cần làm
1. Nguồn bằng chứng: `satisfies` (derived), `commit_trailer`, `test_edge`, `check_profile`, `name_overlap` (inferred), `user_confirmed`; `reject` loại.
2. `state`: `has_evidence|partial|no_evidence|manual_pending|unknown` theo bảng CR §2.4; `inferred` không đổi `state` (chỉ gợi ý); `unknown` khi index cũ/thiếu overlay/check bắt buộc chưa chạy.
3. `unlinkedChanges` (≤ 100), `summary`; giới hạn 50/20.
4. Không có chuỗi "đã đáp ứng" ở mô hình (chỉ khoá).

## Kiểm thử
- Bảng đủ tổ hợp (change/test/check × explicit/derived/inferred × verifyHint × stale); `CONFIRM` nâng gợi ý; `REJECT` loại.

## Tiêu chí hoàn thành
- [x] `inferred`-only ⇒ `no_evidence`; [ ] index cũ ⇒ `unknown`; [ ] `manual` ⇒ `manual_pending`.

## Rủi ro
- Phụ thuộc `changedSymbols[].tested`/`readingOrder[].tests` của CR-036 (đề xuất).
