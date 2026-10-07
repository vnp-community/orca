# BE-CV-TASK-093-03: Domain dựng đầu vào (danh sách cho phép), che entropy, làm sạch văn bản

**From Solution:** BE-CV-SOL-093-ai-review-summary
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/domain/ai_review_input_builder.go`, `ai_review_entropy_redactor.go`, `ai_review_text_sanitizer.go` (mới)
**Depends on:** BE-CV-TASK-093-02; BE-CV-SOL-013 (`TextRedactor`, `PathPolicy`)
**Status:** [x] DONE

## Việc cần làm
1. Builder theo bảng CR §2.3 (giới hạn 60/80/20/20; `diff` ≤ 10 tệp × 60 dòng, tổng 16 KB; tổng ≤ 24 000 ký tự, cắt theo thứ tự); manifest; `estimatedTokens=ceil(bytes/3.5)` (ghi rõ ước lượng thô).
2. Loại: tệp chặn nội dung, sinh/lock/nhị phân, untracked không xác nhận (Q3), đường dẫn tuyệt đối, tên người/email, lý do miễn trừ, `ai_context`.
3. Entropy ≥ 32 ký tự base64/hex (trừ hash commit đã biết) ⇒ `[REDACTED_HIGH_ENTROPY]`; tệp ≥ 3 lần che bị loại `redaction_density`; tổng > 20 ⇒ hạ `metadata` + cảnh báo.
4. Sanitizer: ANSI, zero-width, bidi, dòng ≤ 300, cụm dẫn dắt ⇒ `[LINE REMOVED: suspected instruction]`.

## Kiểm thử
- Repo fixture chứa mọi mục cấm; token giả (`ghp_…`, `AKIA…`, JWT, `PRIVATE KEY`, hex 64); bidi/zero-width; mức `metadata` không chứa diff; fuzz sanitizer.

## Tiêu chí hoàn thành
- [x] manifest khớp byte/tệp thật; [ ] không mục cấm nào lọt; [ ] `dry_run` dùng chung builder.

## Rủi ro
- Ngưỡng (3, 20, 24 000, 16 KB) là giá trị khởi điểm chưa hiệu chỉnh.
