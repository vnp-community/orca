# BE-CV-TASK-090-03: Che bí mật/đường dẫn và sinh văn bản Mermaid

**From Solution:** BE-CV-SOL-090-review-report-model
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/domain/review_report_redaction.go`, `review_report_diagram.go` (mới)
**Depends on:** BE-CV-TASK-090-02; BE-CV-SOL-013 (`TextRedactor`, `PathPolicy`)
**Status:** [ ] TODO

## Việc cần làm
1. `message` ≤ 160, một dòng, qua `TextRedactor`; tệp khớp `PathPolicy` ⇒ giữ tên, bỏ `message` (+ `contentWithheld` nếu Q6 duyệt).
2. Quét đường dẫn tuyệt đối (`^(/|[A-Za-z]:\\)` ≥ 3 phân đoạn ⇒ `<path>`), không hostname/URL cục bộ.
3. `MermaidLabel`: bọc nháy kép, thoát `"`→`#quot;`, bỏ xuống dòng, `;`, backtick, `%%`; `flowchart LR` ≤ 30 nút, `erDiagram` ≤ 15 bảng, ≤ 6 KiB; vượt ⇒ `truncated=true` và **bỏ** sơ đồ; `alt[]` ≤ 12 dòng.
4. Chỉ sinh `components` và `erd` ở v1 (`flow` chưa có nguồn, Q2).

## Kiểm thử
- Token giả (`ghp_…`, `AKIA…`, JWT) ⇒ `[REDACTED]`; nhãn `"`, `]`, `;`, `\n` không vỡ cú pháp (so khớp golden); tên chứa `<script>`.

## Tiêu chí hoàn thành
- [ ] mô hình không chứa đường dẫn tuyệt đối/secret; [ ] sơ đồ vượt ngưỡng bị bỏ; [ ] `alt` đồng bộ với nút.

## Rủi ro
- Chủ dựng Mermaid cần FE-CV-SOL-090 đồng ý (L4).
