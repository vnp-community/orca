# TASK-TDD-002-T1-02: So Solution có KG (agent) với `ai.complete` hiện tại

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.3, 5 (tiêu chí 1)
**Checklist:** [T1.2](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** spike (không sửa Orca)
**Files:** `evidence/T1.2-solution-ab.md` (mới)
**Depends on:** T0.3, T1.3 (skill chạy được)
**Status:** `[ ]` TODO

---

## Context

Giả thuyết của SOL-002: Solution do agent có KG nêu đúng phần tử bị ảnh hưởng và phát hiện xung đột Decision/Constraint (ví dụ ADR-12 "tính năng hoãn ẩn hoàn toàn, không trả 501" mâu thuẫn FR-SYS-07 cũ) mà `ai.complete` không biết. Cần đo trước khi sửa `modeFor`.

## Việc cần làm

1. Tạo Request BL-H01 bằng CLI `orca` trên project thử có repo (project tạm theo cách đã làm ở phiên CLI: `POST /v1/projects` rồi `POST /v1/projects/{id}/repos`), trả lời clarification, `confirm-type change_request --size L`.
2. Chạy `orca solution generate <R> --wait` (đường `ai.complete` hiện có); lưu Solution JSON (`orca --json solution list`).
3. Chạy một lượt agent chỉ đọc có KG theo Phụ lục A, dùng cùng `BuildSolutionPrompt` (lấy nội dung prompt từ `solution_prompt.go`, thêm chỉ dẫn dùng skill) và cùng schema; lưu JSON.
4. Chấm hai Solution theo bảng: phần tử thiết kế nêu đúng/thừa/thiếu (FR-SYS-07, ENT-17, BRL-64…), xung đột ADR-12 có được nêu không và ở phương án nào, câu hỏi mở có chỉ ra gap thật không, nhiễu.
5. Ghi bảng và 2 đến 3 ví dụ đoạn văn so sánh vào `evidence/T1.2-solution-ab.md`; kết luận ngắn có đáng làm T2.9 không.

## Tiêu chí hoàn thành

- [ ] Có hai Solution cùng Request, bảng chấm và ví dụ minh hoạ.
- [ ] Kết luận: ngữ cảnh KG làm Solution tốt hơn rõ rệt, ngang nhau hay tệ hơn; nêu điều kiện (loại Request, chất lượng KG).

## Kiểm thử

- Kiểm bằng bảng chấm; không có test tự động.

## Rủi ro và ghi chú

- Project thử phải xoá sau khi đo (`DELETE /v1/projects/{id}`), huỷ Request.
- Kết quả của một Request không đủ khái quát; ghi rõ giới hạn.
