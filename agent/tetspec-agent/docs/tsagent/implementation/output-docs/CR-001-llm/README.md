# CR-001 (SCHEDULED-TRANSFER): đầu ra workflow minipay-demo, chế độ Claude, tsagent bật LLM

Chạy ngày 2026-10-09, 10:57 đến 11:19, trên server `for-dev`. Workflow `mode=claude`, testspec `agent`. tsagent chạy `TSAGENT_LLM=claude-cli`, các bước `author` và `infer`. Workflow dừng ở `gate_testspec`, nên test specs vẫn là **v1 Draft, chưa baseline**.

Các cổng `gate_impact`, `gate_kg` và `gate_solution` được **tự duyệt**. Câu hỏi ở `gate_impact` được trả lời bằng giá trị mặc định mà impact-analyst đề xuất (xem `workflow/state.json`).

## Thư mục

| Đường dẫn | Nội dung |
| --- | --- |
| `tdd/06-TDD-MiniPay-before-CR-001.md` | TDD nền, trước CR |
| `tdd/06-TDD-MiniPay-after-CR-001.md` | TDD render từ KG, gồm KG nền và patch CR-001 đã duyệt |
| `tdd/tdd_patch.json`, `tdd/tdd_patch.approved.json` | Patch do kg-updater viết (56 nút, 116 cạnh) và bản đã gộp khi duyệt |
| `tdd/kg-slice.ttl` | Lát cắt KG gửi cho tsagent: 56 nút của CR cần test và 96 nút ngữ cảnh |
| `solution/solution.md` | Change spec do solution-designer viết |
| `solution/solution-sent.md` | Phần solution gửi cho tsagent (phụ lục có cấu trúc: 6 EP, 26 FLD, 34 RSP, 43 AC, 2 SM, 15 TRN) |
| `testspec/01-test-requirements.md`, `02-test-design.md`, `03-test-cases.md` | Test specs do tsagent sinh |
| `testspec/supplement.md`, `testspec/review.json` | Kết quả rà soát của phiên testspec-agent (6 AC bổ sung, 7 phát hiện) |
| `testspec/testspec.base.json`, `testspec/testspec.json` | Tóm tắt lần sinh đầu và lần sinh sau khi có supplement |
| `workflow/` | Đầu vào (`cr.md`, `srs.md`), `impact.json`, trạng thái, sự kiện, nhật ký từng phiên Claude |

## Số liệu

| | Giá trị |
| --- | --- |
| Test Requirement / coverage item / Test Case | 25 / 183 / 183 (trước khi rà soát là 177 TC) |
| TC được LLM viết lại lời văn | 183/183. Assertion và số liệu vẫn do luật sinh |
| AC do LLM suy ra (`inferred`) | 0, vì solution đã có đủ AC cho mọi FR |
| Lỗi Chặn T1–T20 | 0 |
| Nút `pendingConfirmation` | 123, do còn gắn với OQ-010, OQ-011, OQ-012 chưa đóng trong patch TDD |
| Chi phí các phiên Claude của workflow | khoảng $2,78 (impact 0,43; kg_update 0,81; solution 1,06; testspec-agent 0,47), chưa tính các lượt `claude-cli` của tsagent |
