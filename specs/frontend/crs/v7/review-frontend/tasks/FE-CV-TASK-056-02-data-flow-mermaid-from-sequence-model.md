# FE-CV-TASK-056-02: `buildSequenceDiagram` từ `SequenceModel` và `escapeMermaidLabel`

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 4.3
**Priority:** P1
**Area:** frontend / review-map
**File:** `components/review-map/data-flow-mermaid.ts` (mới), test
**Depends on:** FE-CV-TASK-050-01
**Status:** [x] DONE

## Context

- `SequenceModel` (§4.4); `MermaidBlock` `securityLevel:'strict'` + DOMPurify; `MAX_TEXTLENGTH` 50 000 (theo CR, Mermaid 11.15.0). U9.

## Việc cần làm

1. `buildSequenceDiagram(seq, {changedMessages, maxMessages:60, maxParticipants:14, maxChars:40000})` theo SOL 4.3 (id `p1…pn`, `autonumber`, mũi tên theo `sync`/`dashedReturn`, tiền tố `[đổi] `, `note`).
2. `escapeMermaidLabel` (thực thể cho `; # % < > " \``, bỏ điều khiển, xuống dòng ⇒ dấu cách; cắt 48 ký tự); không sinh `click`/HTML.
3. Kết quả `{ok:false, reason}` khi quá lớn/rỗng.

## Kiểm thử

- Từng tổ hợp `sync`/`dashedReturn`; bơm chỉ thị (`%%{init:…}%%`, `---`, `;`, `participant evil`, xuống dòng) không đổi số dòng cấu trúc; > 14 participant; > 40 000 ký tự; xác định.

## Tiêu chí hoàn thành

- [ ] Test xanh; kiểm tay hiển thị thực thể trên Mermaid 11.15.

## Rủi ro

- Thực thể `#59;` chưa chạy trên bản cài.
