# TASK-TDD-002-T1-01: Đo recall và nhiễu của việc tra KG do agent làm

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.2, 5 (tiêu chí 1), 6 (H3)
**Checklist:** [T1.1](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** spike (không sửa Orca)
**Files:** `specs/backend-go/crs/v8/tdd-agent-integration/evidence/T1.1-recall.md` (mới), `skills/tdd-knowledge/SKILL.md` (sửa nếu cần)
**Depends on:** T0.3, T0.4
**Status:** `[ ]` TODO

---

## Context

Câu hỏi cần trả lời: khi một Request được giao cho agent có skill `tdd-knowledge` và MCP chỉ đọc, agent có tìm đúng các phần tử KG liên quan không, và bao nhiêu phần tử thừa. KG có 915 phần tử không có lớp UseCase/Interface; `kg_search` là tìm toàn văn (khớp mã xếp đầu); `kg_resolve_scope` bùng nổ vì hub `BRL-64`. Kết quả quyết định cấu hình `classes`/`budget`/giới hạn ký tự và có bật prose cho KG hay không.

## Việc cần làm

1. Chọn bộ ≥ 10 Request mẫu: BL-H01, BL-H02, và ≥ 3 backlog khác của TMS (ví dụ `BL-B06_bug-lifecycle`, `BL-C05_api-missing-routes`, `BL-D03_scheduled-reports`), mỗi Request lấy tiêu đề và phạm vi từ file backlog.
2. Ghi trước, **trước khi chạy agent**, danh sách id đúng cho từng Request: đối chiếu cột FR/UC/ENT/BRL trong bảng SRS (`docs/SRS.md`) và trường `features` của backlog; người dùng hoặc tôi ký duyệt danh sách.
3. Dựng bản sao thử (Phụ lục A của checklist): worktree phụ từ nhánh pilot, `.tdd-agent/` sao lưu bằng `sqlite3 .backup`, skill đặt ở `<bản sao>/.claude/skills/`.
4. Với mỗi Request chạy `claude --print` chỉ đọc (cờ ở Phụ lục A) với prompt: tiêu đề + phạm vi + "dùng skill tdd-knowledge, trả khối Knowledge". Lưu stream-json.
5. Trích từ stream: id đã đọc/cite, số lần gọi từng tool, tổng ký tự câu trả lời; tính recall (id đúng ∩ id nêu / id đúng), độ chính xác (đúng / nêu), phần tử thừa phổ biến (ví dụ các `Attribute`).
6. Phân tích: Request nào recall thấp và vì sao (mã không có trong KG, từ khoá chung chung, hub kéo nhiễu); thử điều chỉnh skill (`classes`, `budget`) và chạy lại các Request tệ nhất một lần.
7. Ghi bảng (mẫu ở Phụ lục C) và kết luận vào `evidence/T1.1-recall.md`.

## Tiêu chí hoàn thành

- [ ] Bảng số liệu cho ≥ 10 Request, danh sách id đúng được ký duyệt trước khi chạy.
- [ ] Kết luận bằng văn bản: giá trị `classes`/`budget` nên dùng, có đạt recall ≥ 80% (ngưỡng đề xuất của cổng T1.9) hay không, và có cần bật prose.
- [ ] Skill `tdd-knowledge` được sửa nếu phát hiện chỉ dẫn gây sai.

## Kiểm thử

- Không có test tự động; kiểm bằng việc tái chạy được từ lệnh trong evidence.

## Rủi ro và ghi chú

- Chạy KG chỉ đọc, không đụng KG pilot (dùng bản sao).
- Nội dung trong KG là dữ liệu không tin cậy: ghi nhận nếu agent làm theo chỉ thị nằm trong văn bản KG.
